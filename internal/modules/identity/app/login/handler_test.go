package login

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

var testLogger = slog.Default()

type userWriteStub struct {
	findByEmail func(ctx context.Context, email string) (*domain.User, error)
}

func (s *userWriteStub) Create(context.Context, domain.User) error                       { return nil }
func (s *userWriteStub) UpdatePassword(context.Context, string, string, time.Time) error { return nil }
func (s *userWriteStub) MarkEmailVerified(context.Context, string, time.Time) error      { return nil }
func (s *userWriteStub) SetMFAEnabledAt(context.Context, string, *time.Time, time.Time) error {
	return nil
}
func (s *userWriteStub) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	if s.findByEmail != nil {
		return s.findByEmail(ctx, email)
	}
	return nil, nil
}
func (s *userWriteStub) FindByID(context.Context, string) (*domain.User, error) { return nil, nil }

type hasherStub struct{ err error }

func (s *hasherStub) Hash(string) (string, error)  { return "", nil }
func (s *hasherStub) Compare(string, string) error { return s.err }

func TestExecuteReturnsInvalidCredentials(t *testing.T) {
	h := New(Options{
		UsersWrite: &userWriteStub{findByEmail: func(_ context.Context, _ string) (*domain.User, error) { return nil, errors.New("db error") }},
		Hasher:     &hasherStub{},
		Logger:     testLogger,
	})
	_, err := h.Execute(context.Background(), Command{Email: "a@example.com", Password: "pw", Now: time.Now().UTC()})
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

type sessFactStub struct{}

func (s *sessFactStub) New() (string, error) { return "sess-1", nil }

type tokMgrStub struct{}

func (s *tokMgrStub) Issue(_, _ string, now time.Time) (ports.TokenPair, string, string, error) {
	return ports.TokenPair{AccessToken: "at", RefreshToken: "rt", AccessExpiresAt: now.Add(time.Hour), RefreshExpiresAt: now.Add(24 * time.Hour)}, "ajti", "rjti", nil
}
func (s *tokMgrStub) ParseAccess(string, time.Time) (*ports.AccessClaims, error)  { return nil, nil }
func (s *tokMgrStub) ParseRefresh(string, time.Time) (*ports.AccessClaims, error) { return nil, nil }

type sessWrtStub struct{}

func (s *sessWrtStub) Create(context.Context, domain.Session) error          { return nil }
func (s *sessWrtStub) RevokeByID(context.Context, string, time.Time) error   { return nil }
func (s *sessWrtStub) RevokeByUser(context.Context, string, time.Time) error { return nil }
func (s *sessWrtStub) RotateTokens(context.Context, string, string, string, time.Time, time.Time) error {
	return nil
}
func (s *sessWrtStub) FindByID(context.Context, string) (*domain.Session, error) { return nil, nil }
func (s *sessWrtStub) FindByAccessJTI(context.Context, string) (*domain.Session, error) {
	return nil, nil
}
func (s *sessWrtStub) ListByUser(context.Context, string, time.Time) ([]domain.Session, error) {
	return nil, nil
}

type rfrshStub struct{}

func (s *rfrshStub) Save(context.Context, string, string, time.Duration) error { return nil }
func (s *rfrshStub) Find(context.Context, string) (string, error)              { return "", nil }
func (s *rfrshStub) Delete(context.Context, string) error                      { return nil }
func (s *rfrshStub) Replace(context.Context, string, string, string, time.Duration) error {
	return nil
}

func TestExecuteSuccess(t *testing.T) {
	sessionFactory := usecase.NewSessionFactory(&sessFactStub{}, &tokMgrStub{}, &sessWrtStub{}, &rfrshStub{}, testLogger)
	h := New(Options{
		UsersWrite: &userWriteStub{findByEmail: func(_ context.Context, _ string) (*domain.User, error) {
			return &domain.User{ID: "u1", Email: "a@example.com", HashedPassword: "hash"}, nil
		}},
		Hasher:         &hasherStub{},
		Logger:         testLogger,
		SessionFactory: sessionFactory,
	})
	out, err := h.Execute(context.Background(), Command{Email: "A@EXAMPLE.com", Password: "pw", Now: time.Now().UTC()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.User.ID != "u1" {
		t.Fatalf("unexpected user: %+v", out.User)
	}
}
