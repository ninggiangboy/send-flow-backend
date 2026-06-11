package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	suppressionapp "github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/ports"

	identityapp "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app"
	identitytoken "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/infrastructure/token"
	platformhealth "github.com/ninggiangboy/send-flow/backend/internal/platform/health"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/observability"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/security"
)

type permitAllAccess struct{}

func (permitAllAccess) RequirePermission(_ context.Context, _, _, _ string) error { return nil }

type memorySuppressionReadRepo struct {
	mu      sync.Mutex
	entries []domain.SuppressionEntry
	byID    map[string]domain.SuppressionEntry
}

type memorySuppressionWriteRepo struct {
	mu       sync.Mutex
	entries  []domain.SuppressionEntry
	byID     map[string]domain.SuppressionEntry
	onCreate func(entry domain.SuppressionEntry) error
	onRemove func(workspaceID, entryID string, removedAt time.Time) error
}

type memorySuppressionStore struct {
	mu      sync.Mutex
	entries map[string]domain.SuppressionEntry
	list    []domain.SuppressionEntry
}

type sharedSuppressionRepo struct {
	store *memorySuppressionStore
}

func newMemorySuppressionRepos() (*sharedSuppressionRepo, *sharedSuppressionRepo) {
	store := &memorySuppressionStore{entries: map[string]domain.SuppressionEntry{}}
	return &sharedSuppressionRepo{store: store}, &sharedSuppressionRepo{store: store}
}

func (r *sharedSuppressionRepo) FindByID(_ context.Context, workspaceID, entryID string) (*domain.SuppressionEntry, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	e, ok := r.store.entries[entryID]
	if !ok || e.WorkspaceID != workspaceID {
		return nil, domain.ErrEntryNotFound
	}
	return &e, nil
}

func (r *sharedSuppressionRepo) List(_ context.Context, query ports.SuppressionListQuery) ([]domain.SuppressionEntry, string, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	var out []domain.SuppressionEntry
	for _, e := range r.store.list {
		if e.WorkspaceID == query.WorkspaceID {
			out = append(out, e)
		}
	}
	return out, "", nil
}

func (r *sharedSuppressionRepo) FindActiveByEmail(_ context.Context, query ports.SuppressionCheckQuery) (*domain.SuppressionEntry, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	for _, e := range r.store.list {
		if e.WorkspaceID == query.WorkspaceID && e.EmailNormalized == query.EmailNormalized && e.Status == domain.SuppressionStatusActive {
			return &e, nil
		}
	}
	return nil, nil
}

func (w *sharedSuppressionRepo) Create(_ context.Context, entry domain.SuppressionEntry) error {
	w.store.mu.Lock()
	defer w.store.mu.Unlock()
	w.store.entries[entry.ID] = entry
	w.store.list = append(w.store.list, entry)
	return nil
}

func (w *sharedSuppressionRepo) Remove(_ context.Context, workspaceID, entryID string, removedAt time.Time) error {
	w.store.mu.Lock()
	defer w.store.mu.Unlock()
	e, ok := w.store.entries[entryID]
	if !ok || e.WorkspaceID != workspaceID {
		return domain.ErrEntryNotFound
	}
	if e.Status == domain.SuppressionStatusRemoved {
		return domain.ErrUnsuppressConflict
	}
	e.Status = domain.SuppressionStatusRemoved
	e.RemovedAt = &removedAt
	w.store.entries[entryID] = e
	return nil
}

func (r *sharedSuppressionRepo) prepopulate(entries ...domain.SuppressionEntry) {
	for _, e := range entries {
		r.store.entries[e.ID] = e
		r.store.list = append(r.store.list, e)
	}
}

func setupSuppressionRouter(t *testing.T, setup func(repo *sharedSuppressionRepo)) (http.Handler, string) {
	t.Helper()
	read, write := newMemorySuppressionRepos()
	if setup != nil {
		setup(read)
	}
	suppressionSvc := suppressionapp.NewService(suppressionapp.Options{
		EntriesRead:   read,
		EntriesWrite:  write,
		AccessChecker: &permitAllAccess{},
		IDGen:         func() (string, error) { return "stub-id", nil },
		Logger:        slog.Default(),
	})

	users := newMemoryUserRepo()
	sessions := newMemorySessionRepo()
	refreshStore := newMemoryRefreshStore()
	authTokens := newMemoryAuthTokenRepo()
	totp := newMemoryTOTPRepo()
	workspaces := newMemoryWorkspaceRepo()
	memberships := newMemoryMembershipRepo()
	roles := newMemoryRoleRepo(memberships)
	invitations := newMemoryInvitationRepo()
	sender := &captureSender{}
	svc := identityapp.NewService(identityapp.Options{
		UsersRead:         users,
		UsersWrite:        users,
		ExternalsRead:     noopExternalRepo{},
		ExternalsWrite:    noopExternalRepo{},
		SessionsRead:      sessions,
		SessionsWrite:     sessions,
		Hasher:            security.NewPasswordHasher(4),
		Tokens:            identitytoken.NewJWTManager("send-flow-test", "access-secret", "refresh-secret", time.Minute, time.Hour),
		OAuthState:        nil,
		RefreshStore:      refreshStore,
		AuthTokens:        authTokens,
		TOTP:              totp,
		Providers:         nil,
		OAuthStateTTL:     time.Minute,
		MailSender:        &mailerAdapter{sender: sender},
		FrontendBaseURL:   "http://localhost:3000",
		VerificationTTL:   24 * time.Hour,
		PasswordResetTTL:  time.Hour,
		MFAChallengeTTL:   10 * time.Minute,
		WorkspacesRead:    workspaces,
		WorkspacesWrite:   workspaces,
		RolesRead:         roles,
		RolesWrite:        roles,
		MembershipsRead:   memberships,
		MembershipsWrite:  memberships,
		InvitationsRead:   invitations,
		InvitationsWrite:  invitations,
		IDGen:             &uuidIDGeneratorAdapter{gen: id.NewUUIDGenerator()},
		TokenGen:          &tokenGeneratorAdapter{},
		TokenHasher:       &tokenHasherAdapter{},
		PasswordValidator: &passwordValidatorAdapter{},
		TOTPVerifier:      &totpVerifierAdapter{},
		TOTPSecretGen:     &totpSecretGeneratorAdapter{},
		RecoveryCodeGen:   &recoveryCodeGeneratorAdapter{},
		RateLimiter:       &rateLimiterAdapter{svc: &mockRateLimiter{}},
		SettingsWrite:     &noopSettingsWrite{},
		Logger:            slog.Default(),
		UnitOfWork:        &noopTxManager{},
	})
	healthSvc := platformhealth.NewService(platformhealth.Options{
		AppName:       "sendflow",
		PostgresCheck: func(context.Context) error { return nil },
		RedisCheck:    func(context.Context) error { return nil },
	})
	metrics, _ := observability.NewHTTPMetrics(nil)
	router := newRouter(&RouterDeps{
		HealthSvc:       healthSvc,
		AuthSvc:         svc,
		SuppressionSvc:  suppressionSvc,
		SecureCookies:   false,
		FrontendBaseURL: "http://localhost:3000",
		HTTPMetrics:     metrics,
		Log:             slog.Default(),
	})
	signupRec := httptest.NewRecorder()
	router.ServeHTTP(signupRec, jsonRequest(t, http.MethodPost, "/api/v1/auth/signup", map[string]string{
		"email":    "owner@example.com",
		"password": "StrongPassword123!",
	}))
	token := decodeData(t, signupRec)["access_token"].(string)
	return router, token
}

func TestSuppressionList_Success(t *testing.T) {
	router, token := setupSuppressionRouter(t, func(repo *sharedSuppressionRepo) {
		repo.prepopulate(domain.SuppressionEntry{
			ID:          "entry-1",
			WorkspaceID: "ws-1",
			Email:       "bounce@example.com",
			Scope:       domain.SuppressionScopeWorkspace,
			Reason:      domain.SuppressionReasonBounce,
			Status:      domain.SuppressionStatusActive,
			CreatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		})
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/suppression", nil)
	authHeader(req, token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, _ := body["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(data))
	}
	entry := data[0].(map[string]any)
	if entry["email"] != "bounce@example.com" || entry["scope"] != "workspace" {
		t.Fatalf("unexpected entry: %#v", entry)
	}
}

func TestSuppressionList_Empty(t *testing.T) {
	router, token := setupSuppressionRouter(t, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/suppression", nil)
	authHeader(req, token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, _ := body["data"].([]any)
	if len(data) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(data))
	}
}

func TestSuppressionCreate_Success(t *testing.T) {
	router, token := setupSuppressionRouter(t, nil)

	rec := httptest.NewRecorder()
	req := jsonRequest(t, http.MethodPost, "/api/v1/workspaces/ws-1/suppression", map[string]string{
		"email":  "spam@example.com",
		"scope":  "workspace",
		"reason": "complaint",
		"note":   "User reported spam",
	})
	authHeader(req, token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, _ := body["data"].(map[string]any)
	if data["email"] != "spam@example.com" {
		t.Fatalf("expected email=spam@example.com, got %v", data["email"])
	}
	if data["scope"] != "workspace" {
		t.Fatalf("expected scope=workspace, got %v", data["scope"])
	}
	if data["reason"] != "complaint" {
		t.Fatalf("expected reason=complaint, got %v", data["reason"])
	}
}

func TestSuppressionCreate_InvalidBody(t *testing.T) {
	router, token := setupSuppressionRouter(t, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws-1/suppression", nil)
	authHeader(req, token)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSuppressionDelete_Success(t *testing.T) {
	router, token := setupSuppressionRouter(t, func(repo *sharedSuppressionRepo) {
		repo.prepopulate(domain.SuppressionEntry{
			ID:          "entry-1",
			WorkspaceID: "ws-1",
			Email:       "remove@example.com",
			Scope:       domain.SuppressionScopeWorkspace,
			Reason:      domain.SuppressionReasonBounce,
			Status:      domain.SuppressionStatusActive,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		})
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/ws-1/suppression/entry-1", nil)
	authHeader(req, token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSuppressionDelete_NotFound(t *testing.T) {
	router, token := setupSuppressionRouter(t, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/ws-1/suppression/nonexistent", nil)
	authHeader(req, token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSuppressionDelete_AlreadyRemoved(t *testing.T) {
	removedAt := time.Now()
	router, token := setupSuppressionRouter(t, func(repo *sharedSuppressionRepo) {
		repo.prepopulate(domain.SuppressionEntry{
			ID:          "entry-1",
			WorkspaceID: "ws-1",
			Email:       "already@example.com",
			Scope:       domain.SuppressionScopeWorkspace,
			Reason:      domain.SuppressionReasonBounce,
			Status:      domain.SuppressionStatusRemoved,
			RemovedAt:   &removedAt,
		})
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/ws-1/suppression/entry-1", nil)
	authHeader(req, token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSuppression_Unauthenticated(t *testing.T) {
	router, _ := setupSuppressionRouter(t, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/suppression", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}
