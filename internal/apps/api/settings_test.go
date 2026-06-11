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
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	identitytoken "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/infrastructure/token"
	platformhealth "github.com/ninggiangboy/send-flow/backend/internal/platform/health"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/observability"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/security"
)

type noopAuditEntryRepo struct {
	mu      sync.Mutex
	entries []auditdomain.AuditEntry
}

func (r *noopAuditEntryRepo) List(_ context.Context, _ auditdomain.AuditFilter) ([]auditdomain.AuditEntry, string, error) {
	return nil, "", nil
}

func (r *noopAuditEntryRepo) Append(_ context.Context, _ auditdomain.AuditEntry) error { return nil }

func newMinimalAuditService() *auditapp.Service {
	repo := &noopAuditEntryRepo{}
	return auditapp.NewService(auditapp.Options{
		EntriesRead:  repo,
		EntriesWrite: repo,
		PermChecker:  &permitAllAccess{},
		IDGen:        func() (string, error) { return "audit-id", nil },
		Logger:       slog.Default(),
	})
}

type memorySettingsRepo struct {
	mu       sync.Mutex
	settings map[string]domain.WorkspaceSettings
}

func newMemorySettingsRepo() *memorySettingsRepo {
	return &memorySettingsRepo{settings: map[string]domain.WorkspaceSettings{}}
}

func (r *memorySettingsRepo) GetByWorkspace(_ context.Context, workspaceID string) (*domain.WorkspaceSettings, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.settings[workspaceID]
	if !ok {
		return nil, domain.ErrSettingsNotFound
	}
	return &s, nil
}

func (r *memorySettingsRepo) CreateDefault(_ context.Context, settings domain.WorkspaceSettings) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.settings[settings.WorkspaceID] = settings
	return nil
}

func (r *memorySettingsRepo) Upsert(_ context.Context, settings domain.WorkspaceSettings, expectedVersion *int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.settings[settings.WorkspaceID]
	if expectedVersion != nil && ok && existing.Version != *expectedVersion {
		return domain.ErrSettingsVersionConflict
	}
	settings.Version++
	r.settings[settings.WorkspaceID] = settings
	return nil
}

func setupSettingsRouter(t *testing.T) http.Handler {
	t.Helper()
	users := newMemoryUserRepo()
	sessions := newMemorySessionRepo()
	refreshStore := newMemoryRefreshStore()
	authTokens := newMemoryAuthTokenRepo()
	totp := newMemoryTOTPRepo()
	workspaces := newMemoryWorkspaceRepo()
	memberships := newMemoryMembershipRepo()
	roles := newMemoryRoleRepo(memberships)
	invitations := newMemoryInvitationRepo()
	settings := newMemorySettingsRepo()
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
		MailSender:        &mailerAdapter{sender: &captureSender{}},
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
		SettingsRead:      settings,
		SettingsWrite:     settings,
		IDGen:             &uuidIDGeneratorAdapter{gen: id.NewUUIDGenerator()},
		TokenGen:          &tokenGeneratorAdapter{},
		TokenHasher:       &tokenHasherAdapter{},
		PasswordValidator: &passwordValidatorAdapter{},
		TOTPVerifier:      &totpVerifierAdapter{},
		TOTPSecretGen:     &totpSecretGeneratorAdapter{},
		RecoveryCodeGen:   &recoveryCodeGeneratorAdapter{},
		RateLimiter:       &rateLimiterAdapter{svc: &mockRateLimiter{}},
		Logger:            slog.Default(),
		UnitOfWork:        &noopTxManager{},
	})
	healthSvc := platformhealth.NewService(platformhealth.Options{
		AppName:       "sendflow",
		PostgresCheck: func(context.Context) error { return nil },
		RedisCheck:    func(context.Context) error { return nil },
	})
	metrics, _ := observability.NewHTTPMetrics(nil)

	auditSvc := newMinimalAuditService()
	return newRouter(&RouterDeps{
		HealthSvc:       healthSvc,
		AuthSvc:         svc,
		AuditSvc:        auditSvc,
		SettingsSvc:     svc,
		SecureCookies:   false,
		FrontendBaseURL: "http://localhost:3000",
		HTTPMetrics:     metrics,
		Log:             slog.Default(),
	})
}

func signupAndCreateWorkspace(t *testing.T, router http.Handler) (string, string) {
	t.Helper()
	signupRec := httptest.NewRecorder()
	router.ServeHTTP(signupRec, jsonRequest(t, http.MethodPost, "/api/v1/auth/signup", map[string]string{
		"email":    "settings@example.com",
		"password": "StrongPassword123!",
	}))
	if signupRec.Code != http.StatusCreated {
		t.Fatalf("signup failed: %d body=%s", signupRec.Code, signupRec.Body.String())
	}
	token := decodeData(t, signupRec)["access_token"].(string)

	wsRec := httptest.NewRecorder()
	req := jsonRequest(t, http.MethodPost, "/api/v1/workspaces", map[string]string{"name": "SettingsTest"})
	authHeader(req, token)
	router.ServeHTTP(wsRec, req)
	if wsRec.Code != http.StatusCreated {
		t.Fatalf("create workspace failed: %d body=%s", wsRec.Code, wsRec.Body.String())
	}
	wsID := decodeData(t, wsRec)["id"].(string)
	return token, wsID
}

func TestSettingsGet_Success(t *testing.T) {
	router := setupSettingsRouter(t)
	token, wsID := signupAndCreateWorkspace(t, router)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsID+"/settings", nil)
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
	if data["version"] == nil {
		t.Fatal("expected version in settings response")
	}
}

func TestSettingsGet_NotFoundForMissingWorkspace(t *testing.T) {
	router := setupSettingsRouter(t)
	token, _ := signupAndCreateWorkspace(t, router)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/nonexistent/settings", nil)
	authHeader(req, token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSettingsUpdate_Success(t *testing.T) {
	router := setupSettingsRouter(t)
	token, wsID := signupAndCreateWorkspace(t, router)

	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsID+"/settings", nil)
	authHeader(getReq, token)
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get settings failed: %d", getRec.Code)
	}
	getData := decodeData(t, getRec)
	version := int64(getData["version"].(float64))

	rec := httptest.NewRecorder()
	req := jsonRequest(t, http.MethodPatch, "/api/v1/workspaces/"+wsID+"/settings", map[string]any{
		"version": version,
		"email_defaults": map[string]string{
			"default_sender_domain_id": "domain-1",
		},
	})
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
	emailDefaults, _ := data["email_defaults"].(map[string]any)
	if emailDefaults["default_sender_domain_id"] != "domain-1" {
		t.Fatalf("expected default_sender_domain_id=domain-1, got %v", emailDefaults["default_sender_domain_id"])
	}
}

func TestSettingsUpdate_VersionConflict(t *testing.T) {
	router := setupSettingsRouter(t)
	token, wsID := signupAndCreateWorkspace(t, router)

	rec := httptest.NewRecorder()
	req := jsonRequest(t, http.MethodPatch, "/api/v1/workspaces/"+wsID+"/settings", map[string]any{
		"version": 999,
		"email_defaults": map[string]string{
			"default_sender_domain_id": "domain-2",
		},
	})
	authHeader(req, token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSettingsUpdate_EmptyBody(t *testing.T) {
	router := setupSettingsRouter(t)
	token, wsID := signupAndCreateWorkspace(t, router)

	rec := httptest.NewRecorder()
	req := jsonRequest(t, http.MethodPatch, "/api/v1/workspaces/"+wsID+"/settings", map[string]any{})
	authHeader(req, token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSettingsUpdate_Unauthenticated(t *testing.T) {
	router := setupSettingsRouter(t)

	rec := httptest.NewRecorder()
	req := jsonRequest(t, http.MethodGet, "/api/v1/workspaces/ws-1/settings", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSettingsUpdate_ManageDenied(t *testing.T) {
	router := setupSettingsRouter(t)
	token, _ := signupAndCreateWorkspace(t, router)

	rec := httptest.NewRecorder()
	req := jsonRequest(t, http.MethodPatch, "/api/v1/workspaces/nonexistent/settings", map[string]any{
		"email_defaults": map[string]string{
			"default_sender_domain_id": "domain-1",
		},
	})
	authHeader(req, token)
	router.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("expected non-200 for nonexistent workspace, got %d", rec.Code)
	}
}

func TestSettingsUpdate_FeatureControlsOnly(t *testing.T) {
	router := setupSettingsRouter(t)
	token, wsID := signupAndCreateWorkspace(t, router)

	rec := httptest.NewRecorder()
	req := jsonRequest(t, http.MethodPatch, "/api/v1/workspaces/"+wsID+"/settings", map[string]any{
		"feature_controls": map[string]any{
			"allow_marketing": true,
		},
	})
	authHeader(req, token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}
