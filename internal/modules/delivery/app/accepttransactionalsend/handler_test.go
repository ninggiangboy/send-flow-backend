package accepttransactionalsend

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
)

// Mock implementations

type mockTxRequestReadRepo struct {
	ports.TransactionalRequestReadRepository
	findByIdempotencyKey func(ctx context.Context, workspaceID, idempotencyKey string) (*domain.TransactionalSendRequest, error)
	findByID             func(ctx context.Context, workspaceID, requestID string) (*domain.TransactionalSendRequest, error)
}

func (m *mockTxRequestReadRepo) FindByIdempotencyKey(ctx context.Context, workspaceID, idempotencyKey string) (*domain.TransactionalSendRequest, error) {
	if m.findByIdempotencyKey == nil {
		return nil, domain.ErrTransactionalRequestNotFound
	}
	return m.findByIdempotencyKey(ctx, workspaceID, idempotencyKey)
}

func (m *mockTxRequestReadRepo) FindByID(ctx context.Context, workspaceID, requestID string) (*domain.TransactionalSendRequest, error) {
	if m.findByID == nil {
		return nil, domain.ErrTransactionalRequestNotFound
	}
	return m.findByID(ctx, workspaceID, requestID)
}

type mockTxRequestWriteRepo struct {
	ports.TransactionalRequestWriteRepository
	create func(ctx context.Context, request domain.TransactionalSendRequest) error
	update func(ctx context.Context, request domain.TransactionalSendRequest) error
}

func (m *mockTxRequestWriteRepo) Create(ctx context.Context, request domain.TransactionalSendRequest) error {
	if m.create == nil {
		return nil
	}
	return m.create(ctx, request)
}

func (m *mockTxRequestWriteRepo) Update(ctx context.Context, request domain.TransactionalSendRequest) error {
	if m.update == nil {
		return nil
	}
	return m.update(ctx, request)
}

type mockMessageReadRepo struct {
	ports.MessageReadRepository
	findByTransactionalRequestID func(ctx context.Context, workspaceID, requestID string) (*domain.Message, error)
	list                         func(ctx context.Context, query ports.MessageListQuery) ([]domain.Message, string, error)
}

func (m *mockMessageReadRepo) FindByTransactionalRequestID(ctx context.Context, workspaceID, requestID string) (*domain.Message, error) {
	if m.findByTransactionalRequestID == nil {
		return nil, domain.ErrMessageNotFound
	}
	return m.findByTransactionalRequestID(ctx, workspaceID, requestID)
}

func (m *mockMessageReadRepo) List(ctx context.Context, query ports.MessageListQuery) ([]domain.Message, string, error) {
	if m.list == nil {
		return nil, "", nil
	}
	return m.list(ctx, query)
}

type mockMessageWriteRepo struct {
	ports.MessageWriteRepository
	createMany func(ctx context.Context, messages []domain.Message) ([]string, error)
}

func (m *mockMessageWriteRepo) CreateMany(ctx context.Context, messages []domain.Message) ([]string, error) {
	return m.createMany(ctx, messages)
}

type mockSenderChecker struct {
	ports.SenderReadinessChecker
	getSenderReadiness func(ctx context.Context, workspaceID, senderDomainID string) (*ports.SenderReadiness, error)
}

func (m *mockSenderChecker) GetSenderReadiness(ctx context.Context, workspaceID, senderDomainID string) (*ports.SenderReadiness, error) {
	return m.getSenderReadiness(ctx, workspaceID, senderDomainID)
}

type mockContentRenderer struct {
	ports.ContentRenderer
	renderForMessage func(ctx context.Context, workspaceID, templateID, templateVersionID string, data map[string]any) (*ports.RenderedMessage, error)
}

func (m *mockContentRenderer) RenderForMessage(ctx context.Context, workspaceID, templateID, templateVersionID string, data map[string]any) (*ports.RenderedMessage, error) {
	return m.renderForMessage(ctx, workspaceID, templateID, templateVersionID, data)
}

type mockSuppressionChecker struct {
	ports.SuppressionChecker
	checkSuppression func(ctx context.Context, workspaceID, emailNormalized, scope string) (*ports.SuppressionDecision, error)
}

func (m *mockSuppressionChecker) CheckSuppression(ctx context.Context, workspaceID, emailNormalized, scope string) (*ports.SuppressionDecision, error) {
	return m.checkSuppression(ctx, workspaceID, emailNormalized, scope)
}

type mockAttachmentRepo struct {
	ports.AttachmentRepository
	createMany func(ctx context.Context, attachments []domain.AttachmentManifest) error
}

func (m *mockAttachmentRepo) CreateMany(ctx context.Context, attachments []domain.AttachmentManifest) error {
	if m.createMany == nil {
		return nil
	}
	return m.createMany(ctx, attachments)
}

type mockEventRepo struct {
	ports.MessageEventRepository
	create func(ctx context.Context, event domain.MessageEvent) error
}

func (m *mockEventRepo) Create(ctx context.Context, event domain.MessageEvent) error {
	if m.create == nil {
		return nil
	}
	return m.create(ctx, event)
}

type mockObjectStorage struct {
	ports.ObjectStorage
	putObject    func(ctx context.Context, key string, body io.Reader, contentType string) error
	deleteObject func(ctx context.Context, key string) error
}

func (m *mockObjectStorage) PutObject(ctx context.Context, key string, body io.Reader, contentType string) error {
	return m.putObject(ctx, key, body, contentType)
}

func (m *mockObjectStorage) DeleteObject(ctx context.Context, key string) error {
	if m.deleteObject == nil {
		return nil
	}
	return m.deleteObject(ctx, key)
}

type mockOutboxWriter struct {
	ports.OutboxWriter
	saveFunc func(ctx context.Context, event ports.OutboxEvent) error
}

func (m *mockOutboxWriter) Save(ctx context.Context, event ports.OutboxEvent) error {
	if m.saveFunc == nil {
		return nil
	}
	return m.saveFunc(ctx, event)
}

type mockTxManager struct {
	ports.UnitOfWork
	withinTxFunc func(ctx context.Context, fn func(ctx context.Context) error) error
}

func (m *mockTxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return m.withinTxFunc(ctx, fn)
}

// Shared test data

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func now() time.Time {
	return time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
}

func baseTemplateInput() Input {
	return Input{
		WorkspaceID:       "ws_1",
		Mode:              domain.MessageModeTemplate,
		SenderDomainID:    "sd_1",
		TemplateID:        "tmpl_1",
		TemplateVersionID: "tv_1",
		TemplateData:      map[string]any{"name": "Alice"},
		To:                []domain.RecipientTarget{{Email: "alice@example.com", Name: "Alice"}},
		Now:               now(),
	}
}

func baseRawInput() Input {
	return Input{
		WorkspaceID:    "ws_1",
		Mode:           domain.MessageModeRaw,
		SenderDomainID: "sd_1",
		Subject:        "Test Subject",
		TextBody:       "Test body",
		HTMLBody:       "<p>Test body</p>",
		To:             []domain.RecipientTarget{{Email: "bob@example.com", Name: "Bob"}},
		Now:            now(),
	}
}

func defaultMocks() (
	*mockTxRequestReadRepo,
	*mockTxRequestWriteRepo,
	*mockMessageReadRepo,
	*mockMessageWriteRepo,
	*mockSenderChecker,
	*mockContentRenderer,
	*mockSuppressionChecker,
	*mockAttachmentRepo,
	*mockEventRepo,
	*mockObjectStorage,
	*mockOutboxWriter,
	*mockTxManager,
) {
	return &mockTxRequestReadRepo{},
		&mockTxRequestWriteRepo{
			create: func(ctx context.Context, request domain.TransactionalSendRequest) error { return nil },
		},
		&mockMessageReadRepo{},
		&mockMessageWriteRepo{
			createMany: func(ctx context.Context, messages []domain.Message) ([]string, error) {
				ids := make([]string, len(messages))
				for i := range messages {
					ids[i] = messages[i].ID
				}
				return ids, nil
			},
		},
		&mockSenderChecker{
			getSenderReadiness: func(ctx context.Context, workspaceID, senderDomainID string) (*ports.SenderReadiness, error) {
				return &ports.SenderReadiness{Ready: true}, nil
			},
		},
		&mockContentRenderer{
			renderForMessage: func(ctx context.Context, workspaceID, templateID, templateVersionID string, data map[string]any) (*ports.RenderedMessage, error) {
				return &ports.RenderedMessage{Subject: "Rendered Subject", HTMLBody: "<p>Rendered</p>", TextBody: "Rendered"}, nil
			},
		},
		&mockSuppressionChecker{
			checkSuppression: func(ctx context.Context, workspaceID, emailNormalized, scope string) (*ports.SuppressionDecision, error) {
				return &ports.SuppressionDecision{Suppressed: false}, nil
			},
		},
		&mockAttachmentRepo{},
		&mockEventRepo{
			create: func(ctx context.Context, event domain.MessageEvent) error { return nil },
		},
		&mockObjectStorage{
			putObject: func(ctx context.Context, key string, body io.Reader, contentType string) error { return nil },
		},
		&mockOutboxWriter{
			saveFunc: func(ctx context.Context, event ports.OutboxEvent) error { return nil },
		},
		&mockTxManager{
			withinTxFunc: func(ctx context.Context, fn func(ctx context.Context) error) error {
				return fn(ctx)
			},
		}
}

func testHandler(
	txReqR ports.TransactionalRequestReadRepository,
	txReqW ports.TransactionalRequestWriteRepository,
	msgR ports.MessageReadRepository,
	msgW ports.MessageWriteRepository,
	sc ports.SenderReadinessChecker,
	cr ports.ContentRenderer,
	sup ports.SuppressionChecker,
	attRepo ports.AttachmentRepository,
	evtRepo ports.MessageEventRepository,
	objStor ports.ObjectStorage,
	outbox ports.OutboxWriter,
	txMgr ports.UnitOfWork,
) *Handler {
	return New(txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStor, outbox, txMgr,
		func() (string, error) { return "id_gen_1", nil },
		testLogger(), nil, nil)
}

// Tests

func TestTemplateMode_ValidInput_Success(t *testing.T) {
	txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr := defaultMocks()

	var requestCreated bool
	txReqW.create = func(ctx context.Context, req domain.TransactionalSendRequest) error {
		requestCreated = true
		if req.Mode != domain.MessageModeTemplate {
			t.Errorf("expected mode template, got %s", req.Mode)
		}
		if req.TotalRecipients != 1 {
			t.Errorf("expected 1 recipient, got %d", req.TotalRecipients)
		}
		return nil
	}

	var messagesCreated int
	msgW.createMany = func(ctx context.Context, messages []domain.Message) ([]string, error) {
		messagesCreated += len(messages)
		ids := make([]string, len(messages))
		for i := range messages {
			ids[i] = "msg_" + string(rune('a'+i))
		}
		return ids, nil
	}

	var eventCreated bool
	evtRepo.create = func(ctx context.Context, event domain.MessageEvent) error {
		eventCreated = true
		if event.EventType != domain.MessageEventQueued {
			t.Errorf("expected queued event, got %s", event.EventType)
		}
		return nil
	}

	var outboxEvents int
	outbox.saveFunc = func(ctx context.Context, event ports.OutboxEvent) error {
		outboxEvents++
		return nil
	}

	h := testHandler(txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr)
	result, err := h.Execute(context.Background(), baseTemplateInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.TxRequestStatusAccepted {
		t.Errorf("expected status accepted, got %s", result.Status)
	}
	if !requestCreated {
		t.Error("expected request to be created")
	}
	if messagesCreated != 1 {
		t.Errorf("expected 1 message created, got %d", messagesCreated)
	}
	if !eventCreated {
		t.Error("expected event to be created")
	}
	// 1 queued outbox event per message + 1 accepted outbox event
	if outboxEvents != 2 {
		t.Errorf("expected 2 outbox events, got %d", outboxEvents)
	}
}

func TestRawMode_ValidInput_Success(t *testing.T) {
	txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr := defaultMocks()
	h := testHandler(txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr)

	result, err := h.Execute(context.Background(), baseRawInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.TxRequestStatusAccepted {
		t.Errorf("expected status accepted, got %s", result.Status)
	}
}

func TestDuplicateRecipients_ReturnsError(t *testing.T) {
	txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr := defaultMocks()
	h := testHandler(txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr)

	input := baseTemplateInput()
	input.CC = []domain.RecipientTarget{{Email: "alice@example.com"}}

	_, err := h.Execute(context.Background(), input)
	if !errors.Is(err, domain.ErrDuplicateRecipient) {
		t.Errorf("expected ErrDuplicateRecipient, got %v", err)
	}
}

func TestSuppressedRecipient_ReturnsError(t *testing.T) {
	txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr := defaultMocks()
	sup.checkSuppression = func(ctx context.Context, workspaceID, emailNormalized, scope string) (*ports.SuppressionDecision, error) {
		return &ports.SuppressionDecision{Suppressed: true, Reason: "manual_block"}, nil
	}
	h := testHandler(txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr)

	_, err := h.Execute(context.Background(), baseTemplateInput())
	if !errors.Is(err, domain.ErrSuppressedRecipient) {
		t.Errorf("expected ErrSuppressedRecipient, got %v", err)
	}
}

func TestSenderDomainNotReady_ReturnsError(t *testing.T) {
	txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr := defaultMocks()
	sc.getSenderReadiness = func(ctx context.Context, workspaceID, senderDomainID string) (*ports.SenderReadiness, error) {
		return &ports.SenderReadiness{Ready: false}, nil
	}
	h := testHandler(txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr)

	_, err := h.Execute(context.Background(), baseTemplateInput())
	if !errors.Is(err, domain.ErrSenderDomainNotVerified) {
		t.Errorf("expected ErrSenderDomainNotVerified, got %v", err)
	}
}

func TestInvalidMode_ReturnsError(t *testing.T) {
	txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr := defaultMocks()
	h := testHandler(txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr)

	input := baseTemplateInput()
	input.Mode = "invalid"

	_, err := h.Execute(context.Background(), input)
	if !errors.Is(err, domain.ErrModeInvalid) {
		t.Errorf("expected ErrModeInvalid, got %v", err)
	}
}

func TestTemplateModeWithAttachments_ReturnsError(t *testing.T) {
	txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr := defaultMocks()
	h := testHandler(txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr)

	input := baseTemplateInput()
	input.Attachments = []AttachmentStream{
		{Filename: "test.pdf", ContentType: "application/pdf", Size: 100, SHA256: "abc123"},
	}

	_, err := h.Execute(context.Background(), input)
	if !errors.Is(err, domain.ErrAttachmentNotSupported) {
		t.Errorf("expected ErrAttachmentNotSupported, got %v", err)
	}
}

func TestRawModeWithoutObjectStorage_ReturnsError(t *testing.T) {
	txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, _, outbox, txMgr := defaultMocks()
	h := New(txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo,
		nil, // objectStorage = nil
		outbox, txMgr,
		func() (string, error) { return "id_gen_1", nil },
		testLogger(), nil,
		nil,
	)

	input := baseRawInput()
	input.Attachments = []AttachmentStream{
		{Filename: "test.pdf", ContentType: "application/pdf", Size: 100, SHA256: "abc123", Data: strings.NewReader("test data")},
	}

	_, err := h.Execute(context.Background(), input)
	if !errors.Is(err, domain.ErrObjectStorageDisabled) {
		t.Errorf("expected ErrObjectStorageDisabled, got %v", err)
	}
}

func TestHandleIdempotencyReturnsAllMessageIDs(t *testing.T) {
	txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr := defaultMocks()
	txReqR.findByIdempotencyKey = func(ctx context.Context, workspaceID, idempotencyKey string) (*domain.TransactionalSendRequest, error) {
		return &domain.TransactionalSendRequest{
			ID:          "txreq_1",
			WorkspaceID: workspaceID,
			Status:      domain.TxRequestStatusAccepted,
			RequestHash: "hash_123",
			CreatedAt:   now(),
		}, nil
	}
	msgR.list = func(ctx context.Context, query ports.MessageListQuery) ([]domain.Message, string, error) {
		if query.TransactionalRequestID != "txreq_1" {
			t.Fatalf("expected request txreq_1, got %s", query.TransactionalRequestID)
		}
		return []domain.Message{
			{ID: "msg_1"},
			{ID: "msg_2"},
		}, "", nil
	}

	h := testHandler(txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr)
	result, err := h.handleIdempotency(context.Background(), "ws_1", "idem_1", "hash_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.MessageIDs) != 2 {
		t.Fatalf("expected 2 message ids, got %d", len(result.MessageIDs))
	}
	if result.MessageIDs[0] != "msg_1" || result.MessageIDs[1] != "msg_2" {
		t.Fatalf("unexpected message ids: %#v", result.MessageIDs)
	}
}

func TestStoreAttachmentsUploadsReadableBody(t *testing.T) {
	txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, _, outbox, txMgr := defaultMocks()

	var uploaded []byte
	objStore := &mockObjectStorage{
		putObject: func(ctx context.Context, key string, body io.Reader, contentType string) error {
			data, err := io.ReadAll(body)
			if err != nil {
				return err
			}
			uploaded = data
			return nil
		},
	}

	h := testHandler(txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr)
	_, err := h.storeAttachments(context.Background(), "ws_1", "req_1", []AttachmentStream{{
		Filename:    "hello.txt",
		ContentType: "text/plain",
		Data:        bytes.NewReader([]byte("hello world")),
		Size:        int64(len("hello world")),
		SHA256:      strings.Repeat("a", 64),
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(uploaded) != "hello world" {
		t.Fatalf("expected uploaded body to match, got %q", string(uploaded))
	}
}

func TestExecuteCleansUpUploadedAttachmentsOnTxFailure(t *testing.T) {
	txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, _, outbox, txMgr := defaultMocks()
	txReqW.create = func(ctx context.Context, request domain.TransactionalSendRequest) error {
		return errors.New("db failed")
	}

	var uploadedKey string
	var deletedKey string
	objStore := &mockObjectStorage{
		putObject: func(ctx context.Context, key string, body io.Reader, contentType string) error {
			uploadedKey = key
			return nil
		},
		deleteObject: func(ctx context.Context, key string) error {
			deletedKey = key
			return nil
		},
	}

	idCalls := 0
	h := New(txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr,
		func() (string, error) {
			idCalls++
			switch idCalls {
			case 1:
				return "txreq_1", nil
			case 2:
				return "att_1", nil
			default:
				return "extra_id", nil
			}
		},
		testLogger(), nil, nil)

	input := baseRawInput()
	input.Attachments = []AttachmentStream{{
		Filename:    "hello.txt",
		ContentType: "text/plain",
		Data:        bytes.NewReader([]byte("hello world")),
		Size:        int64(len("hello world")),
		SHA256:      strings.Repeat("a", 64),
	}}

	_, err := h.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected transaction failure")
	}
	if uploadedKey == "" {
		t.Fatal("expected attachment upload before tx failure")
	}
	if deletedKey != uploadedKey {
		t.Fatalf("expected cleanup of %q, got %q", uploadedKey, deletedKey)
	}
}

func TestEmptyWorkspace_ReturnsValidationError(t *testing.T) {
	txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr := defaultMocks()
	h := testHandler(txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr)

	input := baseTemplateInput()
	input.WorkspaceID = ""

	_, err := h.Execute(context.Background(), input)
	if !errors.Is(err, domain.ErrRequestBodyInvalid) {
		t.Errorf("expected ErrRequestBodyInvalid, got %v", err)
	}
}

func TestNoRecipients_ReturnsError(t *testing.T) {
	txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr := defaultMocks()
	h := testHandler(txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr)

	input := baseTemplateInput()
	input.To = nil

	_, err := h.Execute(context.Background(), input)
	if !errors.Is(err, domain.ErrRecipientInvalid) {
		t.Errorf("expected ErrRecipientInvalid, got %v", err)
	}
}

func TestIdempotencyKeyReuse_SameHash_ReturnsExisting(t *testing.T) {
	input := baseTemplateInput()
	input.IdempotencyKey = "idemp_key_1"
	targets, _ := (&Handler{}).explodeRecipients(input.To, input.CC, input.BCC)
	canonical := (&Handler{}).buildCanonical(input, targets)
	expectedHash := computeRequestHash(canonical, nil)

	txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr := defaultMocks()

	txReqR.findByIdempotencyKey = func(ctx context.Context, workspaceID, idempotencyKey string) (*domain.TransactionalSendRequest, error) {
		return &domain.TransactionalSendRequest{
			ID:          "existing_req",
			Status:      domain.TxRequestStatusAccepted,
			RequestHash: expectedHash,
		}, nil
	}

	msgR.list = func(ctx context.Context, query ports.MessageListQuery) ([]domain.Message, string, error) {
		return []domain.Message{{ID: "existing_msg"}}, "", nil
	}

	h := testHandler(txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr)

	result, err := h.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequestID != "existing_req" {
		t.Errorf("expected existing request ID, got %s", result.RequestID)
	}
}

func TestIdempotencyKeyChanged_ReturnsConflict(t *testing.T) {
	txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr := defaultMocks()

	txReqR.findByIdempotencyKey = func(ctx context.Context, workspaceID, idempotencyKey string) (*domain.TransactionalSendRequest, error) {
		return &domain.TransactionalSendRequest{
			ID:          "existing_req",
			Status:      domain.TxRequestStatusAccepted,
			RequestHash: "different_hash",
		}, nil
	}

	h := testHandler(txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr)
	input := baseTemplateInput()
	input.IdempotencyKey = "idemp_key_1"

	_, err := h.Execute(context.Background(), input)
	if !errors.Is(err, domain.ErrIdempotencyKeyConflict) {
		t.Errorf("expected ErrIdempotencyKeyConflict, got %v", err)
	}
}

func TestMultiRecipient_ToAndCC(t *testing.T) {
	txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr := defaultMocks()

	var messageCount int
	msgW.createMany = func(ctx context.Context, messages []domain.Message) ([]string, error) {
		messageCount += len(messages)
		ids := make([]string, len(messages))
		for i := range messages {
			ids[i] = "msg_" + string(rune('a'+i))
		}
		return ids, nil
	}

	h := testHandler(txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr)

	input := baseTemplateInput()
	input.To = []domain.RecipientTarget{
		{Email: "alice@example.com"},
		{Email: "bob@example.com"},
	}
	input.CC = []domain.RecipientTarget{
		{Email: "carol@example.com"},
	}

	result, err := h.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.MessageIDs) != 3 {
		t.Errorf("expected 3 message IDs, got %d", len(result.MessageIDs))
	}
	if messageCount != 3 {
		t.Errorf("expected 3 messages created, got %d", messageCount)
	}
}

func TestRawMode_NoSubject_ReturnsError(t *testing.T) {
	txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr := defaultMocks()
	h := testHandler(txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr)

	input := baseRawInput()
	input.Subject = ""

	_, err := h.Execute(context.Background(), input)
	if !errors.Is(err, domain.ErrSubjectRequired) {
		t.Errorf("expected ErrSubjectRequired, got %v", err)
	}
}

func TestRawMode_NoBody_ReturnsError(t *testing.T) {
	txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr := defaultMocks()
	h := testHandler(txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr)

	input := baseRawInput()
	input.TextBody = ""
	input.HTMLBody = ""

	_, err := h.Execute(context.Background(), input)
	if !errors.Is(err, domain.ErrRawBodyRequired) {
		t.Errorf("expected ErrRawBodyRequired, got %v", err)
	}
}

func TestExceedsMaxRecipients_ReturnsError(t *testing.T) {
	txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr := defaultMocks()
	h := testHandler(txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr)

	input := baseTemplateInput()
	var recipients []domain.RecipientTarget
	for i := 0; i < 55; i++ {
		recipients = append(recipients, domain.RecipientTarget{Email: "user%d@example.com"})
	}
	input.To = recipients

	_, err := h.Execute(context.Background(), input)
	if !errors.Is(err, domain.ErrRecipientInvalid) {
		t.Errorf("expected ErrRecipientInvalid, got %v", err)
	}
}

func TestMessageIDs_GeneratedPerRecipient(t *testing.T) {
	txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr := defaultMocks()

	var callCount int
	msgW.createMany = func(ctx context.Context, messages []domain.Message) ([]string, error) {
		callCount++
		ids := make([]string, len(messages))
		for i := range messages {
			ids[i] = "msg_" + messages[i].RecipientEmailNormalized
		}
		return ids, nil
	}

	h := testHandler(txReqR, txReqW, msgR, msgW, sc, cr, sup, attRepo, evtRepo, objStore, outbox, txMgr)

	input := baseTemplateInput()
	input.To = []domain.RecipientTarget{
		{Email: "a@example.com"},
		{Email: "b@example.com"},
		{Email: "c@example.com"},
	}
	input.BCC = []domain.RecipientTarget{
		{Email: "d@example.com"},
	}

	result, err := h.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.MessageIDs) != 4 {
		t.Errorf("expected 4 message IDs, got %d", len(result.MessageIDs))
	}
	if callCount != 4 {
		t.Errorf("expected 4 CreateMany calls, got %d", callCount)
	}
}
