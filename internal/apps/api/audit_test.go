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

	auditapp "github.com/ninggiangboy/send-flow/backend/internal/modules/audit/app"
	auditdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/audit/domain"
	identityapp "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app"
	identitytoken "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/infrastructure/token"
	platformhealth "github.com/ninggiangboy/send-flow/backend/internal/platform/health"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/observability"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/security"
)

type memoryAuditEntryRepo struct {
	mu      sync.Mutex
	entries []auditdomain.AuditEntry
}

func newMemoryAuditEntryRepo() *memoryAuditEntryRepo {
	return &memoryAuditEntryRepo{}
}

func (r *memoryAuditEntryRepo) List(_ context.Context, filter auditdomain.AuditFilter) ([]auditdomain.AuditEntry, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []auditdomain.AuditEntry
	for _, e := range r.entries {
		if e.WorkspaceID == filter.WorkspaceID {
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		return out, "", nil
	}
	next := ""
	if filter.Cursor == "" && len(out) > 0 {
		next = "cursor-1"
	}
	return out, next, nil
}

func (r *memoryAuditEntryRepo) Append(_ context.Context, entry auditdomain.AuditEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, entry)
	return nil
}

func setupAuditRouter(t *testing.T, auditEntries []auditdomain.AuditEntry) (http.Handler, string) {
	t.Helper()
	auditRepo := newMemoryAuditEntryRepo()
	auditRepo.entries = append(auditRepo.entries, auditEntries...)
	auditSvc := auditapp.NewService(auditapp.Options{
		EntriesRead:  auditRepo,
		EntriesWrite: auditRepo,
		PermChecker:  &permitAllAccess{},
		IDGen:        func() (string, error) { return "audit-id", nil },
		Logger:       slog.Default(),
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
		AuditSvc:        auditSvc,
		SecureCookies:   false,
		FrontendBaseURL: "http://localhost:3000",
		HTTPMetrics:     metrics,
		Log:             slog.Default(),
	})
	signupRec := httptest.NewRecorder()
	router.ServeHTTP(signupRec, jsonRequest(t, http.MethodPost, "/api/v1/auth/signup", map[string]string{
		"email":    "audit@example.com",
		"password": "StrongPassword123!",
	}))
	token := decodeData(t, signupRec)["access_token"].(string)
	return router, token
}

func TestAuditListLogs_Success(t *testing.T) {
	now := time.Now().UTC()
	router, token := setupAuditRouter(t, []auditdomain.AuditEntry{
		{
			ID:          "audit-1",
			WorkspaceID: "ws-1",
			ActorUserID: "user-1",
			ActionType:  "workspace.settings.updated",
			TargetType:  "workspace_settings",
			TargetID:    "ws-1",
			OccurredAt:  now,
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/audit-logs", nil)
	authHeader(req, token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, _ := body["data"].(map[string]any)
	entries, _ := data["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	entry := entries[0].(map[string]any)
	if entry["action_type"] != "workspace.settings.updated" {
		t.Fatalf("expected action_type=workspace.settings.updated, got %v", entry["action_type"])
	}
}

func TestAuditListLogs_Empty(t *testing.T) {
	now := time.Now().UTC()
	router, token := setupAuditRouter(t, []auditdomain.AuditEntry{
		{
			ID: "audit-1", WorkspaceID: "other-ws", ActionType: "workspace.created", OccurredAt: now,
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/audit-logs", nil)
	authHeader(req, token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, _ := body["data"].(map[string]any)
	entries, _ := data["entries"].([]any)
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestAuditListLogs_Unauthenticated(t *testing.T) {
	router, _ := setupAuditRouter(t, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/audit-logs", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}
