//go:build integration

package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func setupDeliveryDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("sendflow_test"),
		tcpostgres.WithUsername("sendflow"),
		tcpostgres.WithPassword("sendflow"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		_ = container.Terminate(ctx)
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("build dsn: %v", err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("new pg pool: %v", err)
	}
	t.Cleanup(pool.Close)

	schema := []string{
		`CREATE TABLE workspaces (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE transactional_send_requests (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			idempotency_key TEXT,
			status TEXT NOT NULL,
			request_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
			mode TEXT NOT NULL DEFAULT 'template',
			subject TEXT,
			sender_name TEXT,
			source_api_key_id TEXT,
			request_hash TEXT,
			total_recipients INT NOT NULL DEFAULT 0,
			recipient_terminal_total INT NOT NULL DEFAULT 0,
			recipient_success_total INT NOT NULL DEFAULT 0,
			recipient_failure_total INT NOT NULL DEFAULT 0,
			completed_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			UNIQUE(workspace_id, idempotency_key)
		)`,
		`CREATE TABLE messages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			campaign_id TEXT,
			campaign_candidate_id TEXT,
			transactional_request_id TEXT,
			contact_id TEXT,
			recipient_email_normalized TEXT NOT NULL,
			recipient_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
			template_id TEXT,
			template_version_id TEXT,
			sender_domain_id TEXT,
			message_type TEXT NOT NULL,
			source_type TEXT NOT NULL,
			status TEXT NOT NULL,
			scheduled_at TIMESTAMPTZ,
			queued_at TIMESTAMPTZ,
			processing_started_at TIMESTAMPTZ,
			accepted_at TIMESTAMPTZ,
			delivered_at TIMESTAMPTZ,
			bounced_at TIMESTAMPTZ,
			complained_at TIMESTAMPTZ,
			failed_at TIMESTAMPTZ,
			last_error_class TEXT,
			last_error_message TEXT,
			provider TEXT,
			provider_message_id TEXT,
			subject TEXT,
			sender_name TEXT,
			recipient_role TEXT NOT NULL DEFAULT 'to',
			source_api_key_id TEXT,
			text_body TEXT,
			html_body TEXT,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_ws_created ON messages(workspace_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_ws_status ON messages(workspace_id, status, created_at DESC)`,
		`CREATE TABLE delivery_attempts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
			attempt_no INT NOT NULL,
			provider TEXT NOT NULL,
			status TEXT NOT NULL,
			error_class TEXT,
			error_message TEXT,
			request_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
			response_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
			started_at TIMESTAMPTZ NOT NULL,
			finished_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL,
			UNIQUE(message_id, attempt_no)
		)`,
		`CREATE TABLE retry_states (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			message_id TEXT UNIQUE NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
			attempt_count INT NOT NULL DEFAULT 0,
			max_attempts INT NOT NULL DEFAULT 3,
			next_retry_at TIMESTAMPTZ,
			last_error_class TEXT,
			last_error_message TEXT,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE transactional_attachments (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			transactional_request_id TEXT NOT NULL REFERENCES transactional_send_requests(id) ON DELETE CASCADE,
			storage_key TEXT NOT NULL,
			original_filename TEXT NOT NULL,
			content_type TEXT NOT NULL,
			byte_size BIGINT NOT NULL,
			sha256_digest TEXT NOT NULL,
			disposition TEXT NOT NULL DEFAULT 'attachment',
			content_id TEXT,
			created_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE message_events (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
			transactional_request_id TEXT,
			event_type TEXT NOT NULL,
			status TEXT NOT NULL,
			reason_code TEXT,
			reason_message TEXT,
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
			occurred_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_msg_events_ws_msg ON message_events(workspace_id, message_id, occurred_at DESC)`,
	}
	for _, stmt := range schema {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("create schema: %v\nSQL: %s", err, stmt)
		}
	}

	// Seed a workspace
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspaces (id, name, created_at, updated_at) VALUES ($1, $2, $3, $4)`,
		"ws_1", "Test Workspace", time.Now().UTC(), time.Now().UTC(),
	); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}

	return pool
}

func now() time.Time {
	return time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
}

// ---- MessageEventRepository Tests ----

func TestMessageEventRepository_CreateAndList(t *testing.T) {
	pool := setupDeliveryDB(t)
	repo := NewMessageEventRepository(pool)

	ctx := context.Background()
	now := now()

	// Seed a message
	pool.Exec(ctx, `INSERT INTO messages (id, workspace_id, recipient_email_normalized, message_type, source_type, status, recipient_role, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		"msg_1", "ws_1", "a@test.com", "transactional", "api", "queued", "to", now, now)

	event := domain.MessageEvent{
		ID:          "evt_1",
		WorkspaceID: "ws_1",
		MessageID:   "msg_1",
		EventType:   domain.MessageEventQueued,
		Status:      domain.MessageStatusQueued,
		OccurredAt:  now,
		CreatedAt:   now,
	}

	err := repo.Create(ctx, event)
	if err != nil {
		t.Fatalf("Create event: %v", err)
	}

	events, cursor, err := repo.ListByMessage(ctx, "ws_1", "msg_1", 10, "")
	if err != nil {
		t.Fatalf("ListByMessage: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].ID != "evt_1" {
		t.Fatalf("expected evt_1, got %s", events[0].ID)
	}
	if cursor != "" {
		t.Fatalf("expected empty cursor on single page, got %s", cursor)
	}
}

func TestMessageEventRepository_ListByMessage_Empty(t *testing.T) {
	pool := setupDeliveryDB(t)
	repo := NewMessageEventRepository(pool)

	events, _, err := repo.ListByMessage(context.Background(), "ws_1", "nonexistent", 10, "")
	if err != nil {
		t.Fatalf("ListByMessage: %v", err)
	}
	if events == nil || len(events) != 0 {
		t.Fatalf("expected empty slice, got %v", events)
	}
}

// ---- AttachmentRepository Tests ----

func TestAttachmentRepository_CreateAndList(t *testing.T) {
	pool := setupDeliveryDB(t)
	repo := NewAttachmentRepository(pool)

	ctx := context.Background()
	now := now()

	// Seed a tx request
	pool.Exec(ctx, `INSERT INTO transactional_send_requests (id, workspace_id, status, mode, total_recipients, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		"txreq_1", "ws_1", "accepted", "raw", 1, now, now)

	manifest := domain.AttachmentManifest{
		ID:                     "att_1",
		WorkspaceID:            "ws_1",
		TransactionalRequestID: "txreq_1",
		StorageKey:             "attachments/ws_1/txreq_1/file.txt",
		OriginalFilename:       "file.txt",
		ContentType:            "text/plain",
		ByteSize:               100,
		SHA256Digest:           "abc123",
		Disposition:            "attachment",
		CreatedAt:              now,
	}

	err := repo.CreateMany(ctx, []domain.AttachmentManifest{manifest})
	if err != nil {
		t.Fatalf("CreateMany: %v", err)
	}

	manifests, err := repo.ListByRequest(ctx, "ws_1", "txreq_1")
	if err != nil {
		t.Fatalf("ListByRequest: %v", err)
	}
	if len(manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(manifests))
	}
	if manifests[0].ID != "att_1" {
		t.Fatalf("expected att_1, got %s", manifests[0].ID)
	}
}

func TestAttachmentRepository_CreateMany(t *testing.T) {
	pool := setupDeliveryDB(t)
	repo := NewAttachmentRepository(pool)

	ctx := context.Background()
	now := now()

	pool.Exec(ctx, `INSERT INTO transactional_send_requests (id, workspace_id, status, mode, total_recipients, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		"txreq_2", "ws_1", "accepted", "raw", 1, now, now)

	manifests := []domain.AttachmentManifest{
		{
			ID:                     "att_2",
			WorkspaceID:            "ws_1",
			TransactionalRequestID: "txreq_2",
			StorageKey:             "attachments/ws_1/txreq_2/a.txt",
			OriginalFilename:       "a.txt",
			ContentType:            "text/plain",
			ByteSize:               50,
			SHA256Digest:           "def456",
			Disposition:            "attachment",
			CreatedAt:              now,
		},
		{
			ID:                     "att_3",
			WorkspaceID:            "ws_1",
			TransactionalRequestID: "txreq_2",
			StorageKey:             "attachments/ws_1/txreq_2/b.txt",
			OriginalFilename:       "b.txt",
			ContentType:            "text/plain",
			ByteSize:               75,
			SHA256Digest:           "ghi789",
			Disposition:            "inline",
			ContentID:              "cid:img1",
			CreatedAt:              now,
		},
	}

	err := repo.CreateMany(ctx, manifests)
	if err != nil {
		t.Fatalf("CreateMany: %v", err)
	}

	result, err := repo.ListByRequest(ctx, "ws_1", "txreq_2")
	if err != nil {
		t.Fatalf("ListByRequest: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 manifests, got %d", len(result))
	}

	// Verify inline disposition and content_id preserved
	for _, m := range result {
		if m.ID == "att_3" {
			if m.Disposition != "inline" {
				t.Fatalf("expected inline disposition, got %s", m.Disposition)
			}
			if m.ContentID != "cid:img1" {
				t.Fatalf("expected content_id cid:img1, got %s", m.ContentID)
			}
		}
	}
}

// ---- TransactionalRequestRepository Tests ----

func TestTransactionalRequestRepository_UpdateAggregates(t *testing.T) {
	pool := setupDeliveryDB(t)
	readRepo := NewTransactionalRequestReadRepository(pool)
	writeRepo := NewTransactionalRequestWriteRepository(pool)

	ctx := context.Background()
	now := now()

	pool.Exec(ctx, `INSERT INTO transactional_send_requests (id, workspace_id, status, mode, total_recipients, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		"txreq_agg", "ws_1", "accepted", "template", 2, now, now)

	// Update first recipient as success
	err := writeRepo.UpdateAggregates(ctx, "ws_1", "txreq_agg", &ports.AggregateUpdate{IsTerminal: true, IsSuccess: true})
	if err != nil {
		t.Fatalf("UpdateAggregates (success): %v", err)
	}

	req, err := readRepo.FindByID(ctx, "ws_1", "txreq_agg")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if req.RecipientTerminalTotal != 1 {
		t.Fatalf("expected terminal_total 1, got %d", req.RecipientTerminalTotal)
	}
	if req.RecipientSuccessTotal != 1 {
		t.Fatalf("expected success_total 1, got %d", req.RecipientSuccessTotal)
	}
	if req.RecipientFailureTotal != 0 {
		t.Fatalf("expected failure_total 0, got %d", req.RecipientFailureTotal)
	}
	if req.Status == "completed" {
		t.Fatalf("expected request to still be in progress, got completed")
	}

	// Update second recipient as failure
	err = writeRepo.UpdateAggregates(ctx, "ws_1", "txreq_agg", &ports.AggregateUpdate{IsTerminal: true, IsSuccess: false})
	if err != nil {
		t.Fatalf("UpdateAggregates (failure): %v", err)
	}

	req, err = readRepo.FindByID(ctx, "ws_1", "txreq_agg")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if req.RecipientTerminalTotal != 2 {
		t.Fatalf("expected terminal_total 2, got %d", req.RecipientTerminalTotal)
	}
	if req.RecipientSuccessTotal != 1 {
		t.Fatalf("expected success_total 1, got %d", req.RecipientSuccessTotal)
	}
	if req.RecipientFailureTotal != 1 {
		t.Fatalf("expected failure_total 1, got %d", req.RecipientFailureTotal)
	}
}

func TestTransactionalRequestRepository_FindByIdempotencyKey(t *testing.T) {
	pool := setupDeliveryDB(t)
	repo := NewTransactionalRequestReadRepository(pool)

	ctx := context.Background()
	now := now()

	pool.Exec(ctx, `INSERT INTO transactional_send_requests (id, workspace_id, idempotency_key, status, mode, total_recipients, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		"txreq_idem", "ws_1", "key-123", "accepted", "template", 1, now, now)

	req, err := repo.FindByIdempotencyKey(ctx, "ws_1", "key-123")
	if err != nil {
		t.Fatalf("FindByIdempotencyKey: %v", err)
	}
	if req.ID != "txreq_idem" {
		t.Fatalf("expected txreq_idem, got %s", req.ID)
	}
}

// ---- MessageRepository Tests ----

func TestMessageRepository_List_Filters(t *testing.T) {
	pool := setupDeliveryDB(t)
	repo := NewMessageReadRepository(pool)

	ctx := context.Background()
	now := now()

	// Seed messages with various combinations
	testMsgs := []struct {
		id     string
		status string
		mtype  string
		email  string
	}{
		{"m_filter_1", "delivered", "transactional", "a@test.com"},
		{"m_filter_2", "failed", "transactional", "b@test.com"},
		{"m_filter_3", "delivered", "campaign", "c@test.com"},
		{"m_filter_4", "queued", "transactional", "d@test.com"},
	}

	for _, m := range testMsgs {
		_, err := pool.Exec(ctx,
			`INSERT INTO messages (id, workspace_id, recipient_email_normalized, message_type, source_type, status, recipient_role, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			m.id, "ws_1", m.email, m.mtype, "api", m.status, "to", now, now)
		if err != nil {
			t.Fatalf("seed message %s: %v", m.id, err)
		}
	}

	tests := []struct {
		name  string
		query ports.MessageListQuery
		want  int
	}{
		{"filter by status", ports.MessageListQuery{WorkspaceID: "ws_1", Status: "delivered"}, 2},
		{"filter by message_type", ports.MessageListQuery{WorkspaceID: "ws_1", MessageType: "campaign"}, 1},
		{"filter by recipient email", ports.MessageListQuery{WorkspaceID: "ws_1", RecipientEmailNormalized: "a@test.com"}, 1},
		{"no filter", ports.MessageListQuery{WorkspaceID: "ws_1"}, 4},
		{"no matches", ports.MessageListQuery{WorkspaceID: "ws_1", Status: "bounced"}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := repo.List(ctx, tt.query)
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if len(result) != tt.want {
				t.Fatalf("expected %d results, got %d", tt.want, len(result))
			}
		})
	}
}

func TestMessageRepository_List_CursorPagination(t *testing.T) {
	pool := setupDeliveryDB(t)
	repo := NewMessageReadRepository(pool)

	ctx := context.Background()
	now := now()

	// Insert 25 messages
	for i := 0; i < 25; i++ {
		id := fmt.Sprintf("m_cursor_%02d", i)
		ts := now.Add(time.Duration(i) * time.Second)
		_, err := pool.Exec(ctx,
			`INSERT INTO messages (id, workspace_id, recipient_email_normalized, message_type, source_type, status, recipient_role, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			id, "ws_1", fmt.Sprintf("u%d@test.com", i), "transactional", "api", "queued", "to", ts, ts)
		if err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}

	// Fetch with limit 10, verify cursor works
	page1, cursor, err := repo.List(ctx, ports.MessageListQuery{WorkspaceID: "ws_1", Limit: 10})
	if err != nil {
		t.Fatalf("List page 1: %v", err)
	}
	if len(page1) != 10 {
		t.Fatalf("expected 10 results on page 1, got %d", len(page1))
	}
	if cursor == "" {
		t.Fatalf("expected non-empty cursor for page 1")
	}

	page2, cursor, err := repo.List(ctx, ports.MessageListQuery{WorkspaceID: "ws_1", Limit: 10, Cursor: cursor})
	if err != nil {
		t.Fatalf("List page 2: %v", err)
	}
	if len(page2) != 10 {
		t.Fatalf("expected 10 results on page 2, got %d", len(page2))
	}
	if cursor == "" {
		t.Fatalf("expected non-empty cursor for page 2")
	}

	page3, cursor, err := repo.List(ctx, ports.MessageListQuery{WorkspaceID: "ws_1", Limit: 10, Cursor: cursor})
	if err != nil {
		t.Fatalf("List page 3: %v", err)
	}
	if len(page3) != 5 {
		t.Fatalf("expected 5 results on page 3, got %d", len(page3))
	}
	if cursor != "" {
		t.Fatalf("expected empty cursor on final page, got %s", cursor)
	}
}

func TestMessageRepository_List_LegacyRows(t *testing.T) {
	pool := setupDeliveryDB(t)
	repo := NewMessageReadRepository(pool)

	ctx := context.Background()
	now := now()

	// Insert a message with null new columns (legacy row)
	_, err := pool.Exec(ctx,
		`INSERT INTO messages (id, workspace_id, recipient_email_normalized, message_type, source_type, status, recipient_role, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		"m_legacy_1", "ws_1", "legacy@test.com", "transactional", "api", "delivered", "to", now, now)
	if err != nil {
		t.Fatalf("seed legacy message: %v", err)
	}

	result, _, err := repo.List(ctx, ports.MessageListQuery{WorkspaceID: "ws_1"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	msg := result[0]
	// Legacy rows should have default zero values for new columns
	if msg.Subject != "" {
		t.Fatalf("expected empty subject for legacy row, got %s", msg.Subject)
	}
	if msg.RecipientRole != "to" {
		t.Fatalf("expected 'to' role for legacy row, got %s", msg.RecipientRole)
	}
}

// ---- DeliveryAttemptsRepository Tests ----

func TestDeliveryAttemptsRepository_ListByMessage(t *testing.T) {
	pool := setupDeliveryDB(t)
	readRepo := NewAttemptReadRepository(pool)
	writeRepo := NewAttemptWriteRepository(pool)

	ctx := context.Background()
	now := now()

	pool.Exec(ctx, `INSERT INTO messages (id, workspace_id, recipient_email_normalized, message_type, source_type, status, recipient_role, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		"m_att_1", "ws_1", "att@test.com", "transactional", "api", "failed", "to", now, now)

	// Create 3 attempts
	for i := 1; i <= 3; i++ {
		attempt := domain.DeliveryAttempt{
			ID:           fmt.Sprintf("att_%d", i),
			WorkspaceID:  "ws_1",
			MessageID:    "m_att_1",
			AttemptNo:    i,
			Provider:     "ses",
			Status:       "failed",
			ErrorClass:   "temporary",
			ErrorMessage: "connection timeout",
			StartedAt:    now.Add(-time.Duration(4-i) * time.Minute),
			FinishedAt:   &[]time.Time{now.Add(-time.Duration(3-i) * time.Minute)}[0],
			CreatedAt:    now,
		}
		err := writeRepo.Create(ctx, attempt)
		if err != nil {
			t.Fatalf("Create attempt %d: %v", i, err)
		}
	}

	attempts, err := readRepo.ListByMessage(ctx, "ws_1", "m_att_1")
	if err != nil {
		t.Fatalf("ListByMessage: %v", err)
	}
	if len(attempts) != 3 {
		t.Fatalf("expected 3 attempts, got %d", len(attempts))
	}
	// Should be ordered by attempt_no DESC
	if attempts[0].AttemptNo != 3 {
		t.Fatalf("expected first attempt to be #3, got %d", attempts[0].AttemptNo)
	}
}

func TestDeliveryAttemptsRepository_NextAttemptNumber(t *testing.T) {
	pool := setupDeliveryDB(t)
	repo := NewAttemptReadRepository(pool)

	ctx := context.Background()
	now := now()

	pool.Exec(ctx, `INSERT INTO messages (id, workspace_id, recipient_email_normalized, message_type, source_type, status, recipient_role, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		"m_next_1", "ws_1", "next@test.com", "transactional", "api", "failed", "to", now, now)

	// No attempts yet - should return 1
	next, err := repo.NextAttemptNumber(ctx, "ws_1", "m_next_1")
	if err != nil {
		t.Fatalf("NextAttemptNumber: %v", err)
	}
	if next != 1 {
		t.Fatalf("expected 1 for first attempt, got %d", next)
	}

	// Insert 3 attempts
	for i := 1; i <= 3; i++ {
		pool.Exec(ctx,
			`INSERT INTO delivery_attempts (id, workspace_id, message_id, attempt_no, provider, status, started_at, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			fmt.Sprintf("m_na_%d", i), "ws_1", "m_next_1", i, "ses", "failed", now, now)
	}

	next, err = repo.NextAttemptNumber(ctx, "ws_1", "m_next_1")
	if err != nil {
		t.Fatalf("NextAttemptNumber: %v", err)
	}
	if next != 4 {
		t.Fatalf("expected 4 after 3 attempts, got %d", next)
	}
}

// Test that the format package is imported correctly for cursor tests
