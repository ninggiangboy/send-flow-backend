package worker

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	audienceports "github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
)

type importContactReadWriteRepo struct {
	mu       sync.Mutex
	contacts map[string]domain.Contact
}

func newImportContactRepo() *importContactReadWriteRepo {
	return &importContactReadWriteRepo{contacts: map[string]domain.Contact{}}
}

func (r *importContactReadWriteRepo) FindContactByID(ctx context.Context, workspaceID, contactID string) (*domain.Contact, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, contact := range r.contacts {
		if contact.WorkspaceID == workspaceID && contact.ID == contactID {
			c := contact
			return &c, nil
		}
	}
	return nil, domain.ErrContactNotFound
}

func (r *importContactReadWriteRepo) FindContactByEmail(ctx context.Context, workspaceID, emailNormalized string) (*domain.Contact, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	contact, ok := r.contacts[workspaceID+"|"+emailNormalized]
	if !ok {
		return nil, domain.ErrContactNotFound
	}
	c := contact
	return &c, nil
}

func (r *importContactReadWriteRepo) ListContacts(ctx context.Context, query audienceports.ContactListQuery) ([]domain.Contact, string, error) {
	return nil, "", nil
}

func (r *importContactReadWriteRepo) CreateContact(ctx context.Context, contact domain.Contact) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.contacts[contact.WorkspaceID+"|"+contact.EmailNormalized] = contact
	return nil
}

func (r *importContactReadWriteRepo) UpdateContact(ctx context.Context, contact domain.Contact) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.contacts[contact.WorkspaceID+"|"+contact.EmailNormalized] = contact
	return nil
}

func (r *importContactReadWriteRepo) ArchiveContact(ctx context.Context, workspaceID, contactID string, archivedAt time.Time) error {
	return nil
}

func (r *importContactReadWriteRepo) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.contacts)
}

type countingTxManager struct {
	count int
}

func (m *countingTxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	m.count++
	return fn(ctx)
}

func TestAudienceImportProcessor_ProcessCSVUsesBatching(t *testing.T) {
	repo := newImportContactRepo()
	tx := &countingTxManager{}
	p := &AudienceImportProcessor{
		contactsRead:  repo,
		contactsWrite: repo,
		txManager:     tx,
		idGen: func() (string, error) {
			return fmt.Sprintf("contact-%d", repo.count()+1), nil
		},
	}

	var b strings.Builder
	b.WriteString("email,first_name,last_name,tags\n")
	for i := 0; i < importChunkSize+5; i++ {
		b.WriteString(fmt.Sprintf("user%d@example.com,First%d,Last%d,\"tag-a,tag-b\"\n", i, i, i))
	}

	counts, err := p.processCSV(context.Background(), importJobForTest(), io.NopCloser(strings.NewReader(b.String())), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	if counts.ProcessedCount != int64(importChunkSize+5) || counts.CreatedCount != int64(importChunkSize+5) || counts.FailedCount != 0 {
		t.Fatalf("unexpected counts: %+v", counts)
	}
	if repo.count() != importChunkSize+5 {
		t.Fatalf("expected %d contacts, got %d", importChunkSize+5, repo.count())
	}
	if tx.count != 2 {
		t.Fatalf("expected 2 transaction batches, got %d", tx.count)
	}
}

func TestAudienceImportProcessor_ProcessJSONStreamsBatches(t *testing.T) {
	repo := newImportContactRepo()
	tx := &countingTxManager{}
	p := &AudienceImportProcessor{
		contactsRead:  repo,
		contactsWrite: repo,
		txManager:     tx,
		idGen: func() (string, error) {
			return fmt.Sprintf("contact-%d", repo.count()+1), nil
		},
	}

	var b strings.Builder
	b.WriteString("[")
	for i := 0; i < importChunkSize*2+5; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(fmt.Sprintf(`{"email":"json%d@example.com","first_name":"First%d","last_name":"Last%d","tags":"tag-a,tag-b"}`, i, i, i))
	}
	b.WriteString("]")

	counts, err := p.processJSON(context.Background(), importJobForTest(), io.NopCloser(strings.NewReader(b.String())), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	want := int64(importChunkSize*2 + 5)
	if counts.ProcessedCount != want || counts.CreatedCount != want || counts.FailedCount != 0 {
		t.Fatalf("unexpected counts: %+v", counts)
	}
	if repo.count() != int(want) {
		t.Fatalf("expected %d contacts, got %d", want, repo.count())
	}
	if tx.count != 3 {
		t.Fatalf("expected 3 transaction batches, got %d", tx.count)
	}
}

func TestAudienceImportProcessor_ProcessJSONRejectsTrailingData(t *testing.T) {
	p := &AudienceImportProcessor{
		contactsRead:  newImportContactRepo(),
		contactsWrite: newImportContactRepo(),
		txManager:     &countingTxManager{},
		idGen: func() (string, error) {
			return "test-contact", nil
		},
	}

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "non-JSON trailing data",
			input:   `[{"email":"a@b.com"}]extra`,
			wantErr: "unexpected content after JSON array",
		},
		{
			name:    "another JSON array after first",
			input:   `[{"email":"a@b.com"}][{"email":"c@d.com"}]`,
			wantErr: "unexpected content after JSON array",
		},
		{
			name:    "JSON object after array",
			input:   `[{"email":"a@b.com"}]{"email":"c@d.com"}`,
			wantErr: "unexpected content after JSON array",
		},
		{
			name:    "trailing scalar value",
			input:   `[{"email":"a@b.com"}]123`,
			wantErr: "unexpected content after JSON array",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.processJSON(context.Background(), importJobForTest(), io.NopCloser(strings.NewReader(tt.input)), time.Now().UTC())
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestAudienceImportProcessor_ProcessJSONAcceptsTrailingWhitespace(t *testing.T) {
	p := &AudienceImportProcessor{
		contactsRead:  newImportContactRepo(),
		contactsWrite: newImportContactRepo(),
		txManager:     &countingTxManager{},
		idGen: func() (string, error) {
			return "test-contact", nil
		},
	}

	input := `[{"email":"a@b.com"},{"email":"c@d.com"}]   `
	_, err := p.processJSON(context.Background(), importJobForTest(), io.NopCloser(strings.NewReader(input)), time.Now().UTC())
	if err != nil {
		t.Fatalf("expected no error for whitespace after array, got: %v", err)
	}
}

func importJobForTest() domain.AudienceImportJob {
	return domain.AudienceImportJob{
		ID:          "import-1",
		WorkspaceID: "workspace-1",
		DedupeMode:  domain.DedupeModeByEmail,
	}
}
