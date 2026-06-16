package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
)

// --- Mocks ---

type mockMessageReadRepo struct {
	ports.MessageReadRepository
	findByID                      func(ctx context.Context, workspaceID, messageID string) (*domain.Message, error)
	findByIDForUpdate             func(ctx context.Context, workspaceID, messageID string) (*domain.Message, error)
	findByTransactionalRequestID  func(ctx context.Context, workspaceID, transactionalRequestID string) (*domain.Message, error)
	findByProviderMessageID       func(ctx context.Context, provider, providerMessageID string) (*domain.Message, error)
	list                          func(ctx context.Context, query ports.MessageListQuery) ([]domain.Message, string, error)
	listDueQueued                 func(ctx context.Context, query ports.DueMessageQuery) ([]domain.Message, error)
	listDistinctWorkspacesWithDue func(ctx context.Context, messageType string, now time.Time) ([]string, error)
	countByCampaign               func(ctx context.Context, workspaceID, campaignID string) (int64, error)
}

func (m *mockMessageReadRepo) FindByID(ctx context.Context, workspaceID, messageID string) (*domain.Message, error) {
	return m.findByID(ctx, workspaceID, messageID)
}

func (m *mockMessageReadRepo) FindByIDForUpdate(ctx context.Context, workspaceID, messageID string) (*domain.Message, error) {
	if m.findByIDForUpdate != nil {
		return m.findByIDForUpdate(ctx, workspaceID, messageID)
	}
	return m.findByID(ctx, workspaceID, messageID)
}

func (m *mockMessageReadRepo) FindByTransactionalRequestID(ctx context.Context, workspaceID, transactionalRequestID string) (*domain.Message, error) {
	return m.findByTransactionalRequestID(ctx, workspaceID, transactionalRequestID)
}

func (m *mockMessageReadRepo) FindByProviderMessageID(ctx context.Context, provider, providerMessageID string) (*domain.Message, error) {
	return m.findByProviderMessageID(ctx, provider, providerMessageID)
}

func (m *mockMessageReadRepo) List(ctx context.Context, query ports.MessageListQuery) ([]domain.Message, string, error) {
	return m.list(ctx, query)
}

func (m *mockMessageReadRepo) ListDueQueued(ctx context.Context, query ports.DueMessageQuery) ([]domain.Message, error) {
	return m.listDueQueued(ctx, query)
}

func (m *mockMessageReadRepo) ListDistinctWorkspacesWithDue(ctx context.Context, messageType string, now time.Time) ([]string, error) {
	return m.listDistinctWorkspacesWithDue(ctx, messageType, now)
}

func (m *mockMessageReadRepo) CountByCampaign(ctx context.Context, workspaceID, campaignID string) (int64, error) {
	return m.countByCampaign(ctx, workspaceID, campaignID)
}

type mockMessageWriteRepo struct {
	ports.MessageWriteRepository
	createMany       func(ctx context.Context, messages []domain.Message) ([]string, error)
	update           func(ctx context.Context, message domain.Message) error
	claimDueMessages func(ctx context.Context, query ports.DueMessageQuery, now time.Time) ([]domain.Message, error)
	markProcessing   func(ctx context.Context, workspaceID, messageID string, now time.Time) error
	markAccepted     func(ctx context.Context, message domain.Message) error
	markDelivered    func(ctx context.Context, message domain.Message) error
	markBounced      func(ctx context.Context, message domain.Message) error
	markComplained   func(ctx context.Context, message domain.Message) error
	markDelayed      func(ctx context.Context, message domain.Message) error
	markFailed       func(ctx context.Context, message domain.Message) error
}

func (m *mockMessageWriteRepo) CreateMany(ctx context.Context, messages []domain.Message) ([]string, error) {
	return m.createMany(ctx, messages)
}

func (m *mockMessageWriteRepo) Update(ctx context.Context, message domain.Message) error {
	return m.update(ctx, message)
}

func (m *mockMessageWriteRepo) ClaimDueMessages(ctx context.Context, query ports.DueMessageQuery, now time.Time) ([]domain.Message, error) {
	if m.claimDueMessages == nil {
		return nil, nil
	}
	return m.claimDueMessages(ctx, query, now)
}

func (m *mockMessageWriteRepo) MarkProcessing(ctx context.Context, workspaceID, messageID string, now time.Time) error {
	if m.markProcessing == nil {
		return nil
	}
	return m.markProcessing(ctx, workspaceID, messageID, now)
}

func (m *mockMessageWriteRepo) MarkAccepted(ctx context.Context, message domain.Message) error {
	return m.markAccepted(ctx, message)
}

func (m *mockMessageWriteRepo) MarkDelivered(ctx context.Context, message domain.Message) error {
	return m.markDelivered(ctx, message)
}

func (m *mockMessageWriteRepo) MarkBounced(ctx context.Context, message domain.Message) error {
	return m.markBounced(ctx, message)
}

func (m *mockMessageWriteRepo) MarkComplained(ctx context.Context, message domain.Message) error {
	return m.markComplained(ctx, message)
}

func (m *mockMessageWriteRepo) MarkDelayed(ctx context.Context, message domain.Message) error {
	return m.markDelayed(ctx, message)
}

func (m *mockMessageWriteRepo) MarkFailed(ctx context.Context, message domain.Message) error {
	return m.markFailed(ctx, message)
}

type mockAttemptReadRepo struct {
	ports.AttemptReadRepository
	listByMessage     func(ctx context.Context, workspaceID, messageID string) ([]domain.DeliveryAttempt, error)
	nextAttemptNumber func(ctx context.Context, workspaceID, messageID string) (int, error)
}

func (m *mockAttemptReadRepo) ListByMessage(ctx context.Context, workspaceID, messageID string) ([]domain.DeliveryAttempt, error) {
	return m.listByMessage(ctx, workspaceID, messageID)
}

func (m *mockAttemptReadRepo) NextAttemptNumber(ctx context.Context, workspaceID, messageID string) (int, error) {
	return m.nextAttemptNumber(ctx, workspaceID, messageID)
}

type mockAttemptWriteRepo struct {
	ports.AttemptWriteRepository
	create func(ctx context.Context, attempt domain.DeliveryAttempt) error
	update func(ctx context.Context, attempt domain.DeliveryAttempt) error
}

func (m *mockAttemptWriteRepo) Create(ctx context.Context, attempt domain.DeliveryAttempt) error {
	return m.create(ctx, attempt)
}

func (m *mockAttemptWriteRepo) Update(ctx context.Context, attempt domain.DeliveryAttempt) error {
	return m.update(ctx, attempt)
}

type mockRetryStateReadRepo struct {
	ports.RetryStateReadRepository
	findByMessage func(ctx context.Context, workspaceID, messageID string) (*domain.RetryState, error)
}

func (m *mockRetryStateReadRepo) FindByMessage(ctx context.Context, workspaceID, messageID string) (*domain.RetryState, error) {
	return m.findByMessage(ctx, workspaceID, messageID)
}

type mockRetryStateWriteRepo struct {
	ports.RetryStateWriteRepository
	create func(ctx context.Context, state domain.RetryState) error
	update func(ctx context.Context, state domain.RetryState) error
}

func (m *mockRetryStateWriteRepo) Create(ctx context.Context, state domain.RetryState) error {
	return m.create(ctx, state)
}

func (m *mockRetryStateWriteRepo) Update(ctx context.Context, state domain.RetryState) error {
	return m.update(ctx, state)
}

type mockTxRequestReadRepo struct {
	ports.TransactionalRequestReadRepository
	findByID func(ctx context.Context, workspaceID, requestID string) (*domain.TransactionalSendRequest, error)
}

func (m *mockTxRequestReadRepo) FindByID(ctx context.Context, workspaceID, requestID string) (*domain.TransactionalSendRequest, error) {
	return m.findByID(ctx, workspaceID, requestID)
}

type mockTxRequestWriteRepo struct {
	ports.TransactionalRequestWriteRepository
	create func(ctx context.Context, request domain.TransactionalSendRequest) error
	update func(ctx context.Context, request domain.TransactionalSendRequest) error
}

func (m *mockTxRequestWriteRepo) Create(ctx context.Context, request domain.TransactionalSendRequest) error {
	return m.create(ctx, request)
}

func (m *mockTxRequestWriteRepo) Update(ctx context.Context, request domain.TransactionalSendRequest) error {
	return m.update(ctx, request)
}

type mockCampaignReader struct {
	ports.CampaignCandidateReader
	listCandidates  func(ctx context.Context, workspaceID, campaignID string, limit int, cursor string) ([]ports.CampaignCandidate, string, error)
	countCandidates func(ctx context.Context, workspaceID, campaignID string) (int64, error)
}

func (m *mockCampaignReader) ListCandidates(ctx context.Context, workspaceID, campaignID string, limit int, cursor string) ([]ports.CampaignCandidate, string, error) {
	return m.listCandidates(ctx, workspaceID, campaignID, limit, cursor)
}

func (m *mockCampaignReader) CountCandidates(ctx context.Context, workspaceID, campaignID string) (int64, error) {
	return m.countCandidates(ctx, workspaceID, campaignID)
}

type mockContentRenderer struct {
	ports.ContentRenderer
	renderForMessage func(ctx context.Context, workspaceID, templateID, templateVersionID string, data map[string]any) (*ports.RenderedMessage, error)
}

func (m *mockContentRenderer) RenderForMessage(ctx context.Context, workspaceID, templateID, templateVersionID string, data map[string]any) (*ports.RenderedMessage, error) {
	return m.renderForMessage(ctx, workspaceID, templateID, templateVersionID, data)
}

type mockSenderChecker struct {
	ports.SenderReadinessChecker
	getSenderReadiness func(ctx context.Context, workspaceID, senderDomainID string) (*ports.SenderReadiness, error)
}

func (m *mockSenderChecker) GetSenderReadiness(ctx context.Context, workspaceID, senderDomainID string) (*ports.SenderReadiness, error) {
	return m.getSenderReadiness(ctx, workspaceID, senderDomainID)
}

type mockSuppressionChecker struct {
	ports.SuppressionChecker
	checkSuppression func(ctx context.Context, workspaceID, emailNormalized, scope string) (*ports.SuppressionDecision, error)
}

func (m *mockSuppressionChecker) CheckSuppression(ctx context.Context, workspaceID, emailNormalized, scope string) (*ports.SuppressionDecision, error) {
	return m.checkSuppression(ctx, workspaceID, emailNormalized, scope)
}

type mockRecipientSuppressor struct {
	RecipientSuppressor
	suppressFromSignal func(ctx context.Context, input SuppressFromSignalInput) (*SuppressFromSignalResult, error)
}

func (m *mockRecipientSuppressor) SuppressFromSignal(ctx context.Context, input SuppressFromSignalInput) (*SuppressFromSignalResult, error) {
	return m.suppressFromSignal(ctx, input)
}

type mockEmailProvider struct {
	ports.EmailProvider
	sendEmail func(ctx context.Context, request ports.ProviderSendRequest) (*ports.ProviderSendResult, error)
}

func (m *mockEmailProvider) SendEmail(ctx context.Context, request ports.ProviderSendRequest) (*ports.ProviderSendResult, error) {
	return m.sendEmail(ctx, request)
}

func newTestOpts() Options {
	return Options{
		MessagesRead:  &mockMessageReadRepo{},
		MessagesWrite: &mockMessageWriteRepo{},
		AttemptsRead:  &mockAttemptReadRepo{},
		AttemptsWrite: &mockAttemptWriteRepo{},
		RetryStatesRead: &mockRetryStateReadRepo{
			findByMessage: func(ctx context.Context, workspaceID, messageID string) (*domain.RetryState, error) {
				return nil, nil
			},
		},
		RetryStatesWrite: &mockRetryStateWriteRepo{},
		TxRequestsRead:   &mockTxRequestReadRepo{},
		TxRequestsWrite:  &mockTxRequestWriteRepo{},
		CampaignReader:   &mockCampaignReader{},
		OutboxWriter: &mockOutboxWriter{
			saveFunc: func(_ context.Context, _ ports.OutboxEvent) error { return nil },
		},
		TxManager: &mockTxManager{
			withinTxFunc: func(ctx context.Context, fn func(ctx context.Context) error) error {
				return fn(ctx)
			},
		},
		ContentRenderer: &mockContentRenderer{
			renderForMessage: func(ctx context.Context, workspaceID, templateID, templateVersionID string, data map[string]any) (*ports.RenderedMessage, error) {
				return &ports.RenderedMessage{Subject: "Test Subject", HTMLBody: "<p>Test</p>", TextBody: "Test"}, nil
			},
		},
		SenderChecker: &mockSenderChecker{
			getSenderReadiness: func(ctx context.Context, workspaceID, senderDomainID string) (*ports.SenderReadiness, error) {
				return &ports.SenderReadiness{Ready: true, DomainID: senderDomainID}, nil
			},
		},
		SuppressionChecker: &mockSuppressionChecker{
			checkSuppression: func(ctx context.Context, workspaceID, emailNormalized, scope string) (*ports.SuppressionDecision, error) {
				return &ports.SuppressionDecision{Suppressed: false}, nil
			},
		},
		EmailProvider: &mockEmailProvider{
			sendEmail: func(ctx context.Context, request ports.ProviderSendRequest) (*ports.ProviderSendResult, error) {
				return &ports.ProviderSendResult{Provider: "test", ProviderMessageID: "prov_msg_1", AcceptedAt: time.Now()}, nil
			},
		},
		IDGen:  func() (string, error) { return "test_id_1", nil },
		Logger: slog.Default(),
	}
}

func TestQueueCampaignMessages_Success(t *testing.T) {
	opts := newTestOpts()

	campaign := opts.CampaignReader.(*mockCampaignReader)
	campaign.countCandidates = func(ctx context.Context, workspaceID, campaignID string) (int64, error) {
		return 2, nil
	}
	campaign.listCandidates = func(ctx context.Context, workspaceID, campaignID string, limit int, cursor string) ([]ports.CampaignCandidate, string, error) {
		return []ports.CampaignCandidate{
			{ID: "cand_1", WorkspaceID: "ws_1", CampaignID: "camp_1", ContactID: "contact_1", EmailNormalized: "test1@example.com", RecipientSnapshot: json.RawMessage(`{"contact_id":"contact_1","email":"test1@example.com","email_normalized":"test1@example.com"}`)},
			{ID: "cand_2", WorkspaceID: "ws_1", CampaignID: "camp_1", ContactID: "contact_2", EmailNormalized: "test2@example.com", RecipientSnapshot: json.RawMessage(`{"contact_id":"contact_2","email":"test2@example.com","email_normalized":"test2@example.com"}`)},
		}, "", nil
	}

	write := opts.MessagesWrite.(*mockMessageWriteRepo)
	write.createMany = func(ctx context.Context, messages []domain.Message) ([]string, error) {
		ids := make([]string, len(messages))
		for i, m := range messages {
			ids[i] = m.ID
		}
		return ids, nil
	}

	var saveCount int
	outbox := opts.OutboxWriter.(*mockOutboxWriter)
	outbox.saveFunc = func(ctx context.Context, event ports.OutboxEvent) error {
		saveCount++
		return nil
	}

	svc := NewService(opts)
	result, err := svc.QueueCampaignMessages(context.Background(), QueueCampaignMessagesInput{
		WorkspaceID:       "ws_1",
		CampaignID:        "camp_1",
		TemplateID:        "tmpl_1",
		TemplateVersionID: "tv_1",
		SenderDomainID:    "sd_1",
		MessageType:       "marketing",
		ScheduledAt:       time.Now().Add(time.Hour),
		Now:               time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.QueuedCount != 2 {
		t.Errorf("expected QueuedCount 2, got %d", result.QueuedCount)
	}
	if saveCount != 2 {
		t.Errorf("expected outbox save called 2 times, got %d", saveCount)
	}
}

func TestQueueCampaignMessages_PagesAndWritesBatches(t *testing.T) {
	opts := newTestOpts()

	idCounter := 0
	opts.IDGen = func() (string, error) {
		idCounter++
		return fmt.Sprintf("id_%d", idCounter), nil
	}

	firstPage := make([]ports.CampaignCandidate, 500)
	for i := range firstPage {
		firstPage[i] = ports.CampaignCandidate{
			ID:              "cand_first",
			WorkspaceID:     "ws_1",
			CampaignID:      "camp_1",
			ContactID:       "contact_first",
			EmailNormalized: "first@example.com",
			RecipientSnapshot: json.RawMessage(
				`{"contact_id":"contact_first","email":"first@example.com","email_normalized":"first@example.com"}`,
			),
		}
	}
	secondPage := []ports.CampaignCandidate{{
		ID:              "cand_last",
		WorkspaceID:     "ws_1",
		CampaignID:      "camp_1",
		ContactID:       "contact_last",
		EmailNormalized: "last@example.com",
		RecipientSnapshot: json.RawMessage(
			`{"contact_id":"contact_last","email":"last@example.com","email_normalized":"last@example.com"}`,
		),
	}}

	var cursors []string
	campaign := opts.CampaignReader.(*mockCampaignReader)
	campaign.countCandidates = func(ctx context.Context, workspaceID, campaignID string) (int64, error) {
		return 501, nil
	}
	campaign.listCandidates = func(ctx context.Context, workspaceID, campaignID string, limit int, cursor string) ([]ports.CampaignCandidate, string, error) {
		if limit != 500 {
			t.Fatalf("expected page size 500, got %d", limit)
		}
		cursors = append(cursors, cursor)
		switch cursor {
		case "":
			return firstPage, "next", nil
		case "next":
			return secondPage, "", nil
		default:
			t.Fatalf("unexpected cursor %q", cursor)
			return nil, "", nil
		}
	}

	var batchSizes []int
	write := opts.MessagesWrite.(*mockMessageWriteRepo)
	write.createMany = func(ctx context.Context, messages []domain.Message) ([]string, error) {
		batchSizes = append(batchSizes, len(messages))
		ids := make([]string, len(messages))
		for i, m := range messages {
			ids[i] = m.ID
		}
		return ids, nil
	}

	svc := NewService(opts)
	result, err := svc.QueueCampaignMessages(context.Background(), QueueCampaignMessagesInput{
		WorkspaceID:       "ws_1",
		CampaignID:        "camp_1",
		TemplateID:        "tmpl_1",
		TemplateVersionID: "tv_1",
		SenderDomainID:    "sd_1",
		MessageType:       "marketing",
		ScheduledAt:       time.Now().Add(time.Hour),
		Now:               time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CandidateCount != 501 || result.QueuedCount != 501 {
		t.Fatalf("expected 501 candidates and queued messages, got candidates=%d queued=%d", result.CandidateCount, result.QueuedCount)
	}
	if len(cursors) != 2 || cursors[0] != "" || cursors[1] != "next" {
		t.Fatalf("unexpected cursors: %#v", cursors)
	}
	if len(batchSizes) != 2 || batchSizes[0] != 500 || batchSizes[1] != 1 {
		t.Fatalf("unexpected batch sizes: %#v", batchSizes)
	}
}

func TestQueueCampaignMessages_EmitsOutboxEvents(t *testing.T) {
	opts := newTestOpts()

	campaign := opts.CampaignReader.(*mockCampaignReader)
	campaign.countCandidates = func(ctx context.Context, workspaceID, campaignID string) (int64, error) {
		return 1, nil
	}
	campaign.listCandidates = func(ctx context.Context, workspaceID, campaignID string, limit int, cursor string) ([]ports.CampaignCandidate, string, error) {
		return []ports.CampaignCandidate{
			{ID: "cand_1", WorkspaceID: "ws_1", CampaignID: "camp_1", ContactID: "contact_1", EmailNormalized: "test1@example.com", RecipientSnapshot: json.RawMessage(`{"contact_id":"contact_1","email":"test1@example.com","email_normalized":"test1@example.com"}`)},
		}, "", nil
	}

	write := opts.MessagesWrite.(*mockMessageWriteRepo)
	write.createMany = func(ctx context.Context, messages []domain.Message) ([]string, error) {
		ids := make([]string, len(messages))
		for i, m := range messages {
			ids[i] = m.ID
		}
		return ids, nil
	}

	var capturedEvent ports.OutboxEvent
	outbox := opts.OutboxWriter.(*mockOutboxWriter)
	outbox.saveFunc = func(ctx context.Context, event ports.OutboxEvent) error {
		capturedEvent = event
		return nil
	}

	svc := NewService(opts)
	_, err := svc.QueueCampaignMessages(context.Background(), QueueCampaignMessagesInput{
		WorkspaceID:       "ws_1",
		CampaignID:        "camp_1",
		TemplateID:        "tmpl_1",
		TemplateVersionID: "tv_1",
		SenderDomainID:    "sd_1",
		MessageType:       "marketing",
		ScheduledAt:       time.Now().Add(time.Hour),
		Now:               time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedEvent.EventType != "delivery.message.queued.v1" {
		t.Errorf("expected EventType delivery.message.queued.v1, got %s", capturedEvent.EventType)
	}
	if !json.Valid(capturedEvent.Payload) {
		t.Error("expected Payload to be valid JSON")
	}
}

func TestQueueCampaignMessages_EmptyCandidates(t *testing.T) {
	opts := newTestOpts()

	campaign := opts.CampaignReader.(*mockCampaignReader)
	campaign.countCandidates = func(ctx context.Context, workspaceID, campaignID string) (int64, error) {
		return 0, nil
	}

	svc := NewService(opts)
	_, err := svc.QueueCampaignMessages(context.Background(), QueueCampaignMessagesInput{
		WorkspaceID:       "ws_1",
		CampaignID:        "camp_1",
		TemplateID:        "tmpl_1",
		TemplateVersionID: "tv_1",
		SenderDomainID:    "sd_1",
		MessageType:       "marketing",
		ScheduledAt:       time.Now().Add(time.Hour),
		Now:               time.Now(),
	})
	if !errors.Is(err, domain.ErrCampaignCandidatesNotFound) {
		t.Errorf("expected ErrCampaignCandidatesNotFound, got %v", err)
	}
}

func TestQueueCampaignMessages_DuplicateHandling(t *testing.T) {
	opts := newTestOpts()

	campaign := opts.CampaignReader.(*mockCampaignReader)
	campaign.countCandidates = func(ctx context.Context, workspaceID, campaignID string) (int64, error) {
		return 2, nil
	}
	campaign.listCandidates = func(ctx context.Context, workspaceID, campaignID string, limit int, cursor string) ([]ports.CampaignCandidate, string, error) {
		return []ports.CampaignCandidate{
			{ID: "cand_1", WorkspaceID: "ws_1", CampaignID: "camp_1", ContactID: "contact_1", EmailNormalized: "test1@example.com", RecipientSnapshot: json.RawMessage(`{"contact_id":"contact_1","email":"test1@example.com","email_normalized":"test1@example.com"}`)},
			{ID: "cand_2", WorkspaceID: "ws_1", CampaignID: "camp_1", ContactID: "contact_2", EmailNormalized: "test2@example.com", RecipientSnapshot: json.RawMessage(`{"contact_id":"contact_2","email":"test2@example.com","email_normalized":"test2@example.com"}`)},
		}, "", nil
	}

	write := opts.MessagesWrite.(*mockMessageWriteRepo)
	write.createMany = func(ctx context.Context, messages []domain.Message) ([]string, error) {
		return nil, nil
	}

	svc := NewService(opts)
	result, err := svc.QueueCampaignMessages(context.Background(), QueueCampaignMessagesInput{
		WorkspaceID:       "ws_1",
		CampaignID:        "camp_1",
		TemplateID:        "tmpl_1",
		TemplateVersionID: "tv_1",
		SenderDomainID:    "sd_1",
		MessageType:       "marketing",
		ScheduledAt:       time.Now().Add(time.Hour),
		Now:               time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.QueuedCount != 0 {
		t.Errorf("expected QueuedCount 0, got %d", result.QueuedCount)
	}
}

func TestQueueCampaignMessages_InvalidInput(t *testing.T) {
	opts := newTestOpts()
	svc := NewService(opts)
	_, err := svc.QueueCampaignMessages(context.Background(), QueueCampaignMessagesInput{
		WorkspaceID:       "",
		CampaignID:        "camp_1",
		TemplateID:        "tmpl_1",
		TemplateVersionID: "tv_1",
		SenderDomainID:    "sd_1",
		MessageType:       "marketing",
		ScheduledAt:       time.Now().Add(time.Hour),
		Now:               time.Now(),
	})
	if !errors.Is(err, domain.ErrPayloadInvalid) {
		t.Errorf("expected ErrPayloadInvalid, got %v", err)
	}
}

func TestListMessages_Success(t *testing.T) {
	opts := newTestOpts()

	read := opts.MessagesRead.(*mockMessageReadRepo)
	read.list = func(ctx context.Context, query ports.MessageListQuery) ([]domain.Message, string, error) {
		return []domain.Message{
			{ID: "msg_1", WorkspaceID: "ws_1", Status: domain.MessageStatusQueued},
			{ID: "msg_2", WorkspaceID: "ws_1", Status: domain.MessageStatusQueued},
		}, "", nil
	}

	svc := NewService(opts)
	result, err := svc.ListMessages(context.Background(), ListMessagesInput{WorkspaceID: "ws_1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(result.Messages))
	}
}

func TestListMessages_EmptyWorkspaceID(t *testing.T) {
	opts := newTestOpts()
	svc := NewService(opts)
	_, err := svc.ListMessages(context.Background(), ListMessagesInput{WorkspaceID: ""})
	if !errors.Is(err, domain.ErrPayloadInvalid) {
		t.Errorf("expected ErrPayloadInvalid, got %v", err)
	}
}

func TestGetMessage_Success(t *testing.T) {
	opts := newTestOpts()

	read := opts.MessagesRead.(*mockMessageReadRepo)
	read.findByID = func(ctx context.Context, workspaceID, messageID string) (*domain.Message, error) {
		return &domain.Message{ID: "msg_1", WorkspaceID: "ws_1", Status: domain.MessageStatusQueued}, nil
	}

	svc := NewService(opts)
	msg, err := svc.GetMessage(context.Background(), GetMessageInput{WorkspaceID: "ws_1", MessageID: "msg_1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.ID != "msg_1" {
		t.Errorf("expected message ID msg_1, got %s", msg.ID)
	}
}

func TestGetMessage_NotFound(t *testing.T) {
	opts := newTestOpts()

	read := opts.MessagesRead.(*mockMessageReadRepo)
	read.findByID = func(ctx context.Context, workspaceID, messageID string) (*domain.Message, error) {
		return nil, domain.ErrMessageNotFound
	}

	svc := NewService(opts)
	_, err := svc.GetMessage(context.Background(), GetMessageInput{WorkspaceID: "ws_1", MessageID: "msg_1"})
	if !errors.Is(err, domain.ErrMessageNotFound) {
		t.Errorf("expected ErrMessageNotFound, got %v", err)
	}
}
