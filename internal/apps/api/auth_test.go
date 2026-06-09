package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	identityapp "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	identitytoken "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/infrastructure/token"
	platformemail "github.com/ninggiangboy/send-flow/backend/internal/platform/email"
	platformhealth "github.com/ninggiangboy/send-flow/backend/internal/platform/health"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/observability"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/security"
)

type memoryUserRepo struct {
	mu      sync.Mutex
	byID    map[string]domain.User
	byEmail map[string]string
}

func newMemoryUserRepo() *memoryUserRepo {
	return &memoryUserRepo{byID: map[string]domain.User{}, byEmail: map[string]string{}}
}

func (r *memoryUserRepo) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byEmail[email]
	if !ok {
		return nil, domain.ErrNotFound
	}
	user := r.byID[id]
	return &user, nil
}

func (r *memoryUserRepo) FindByID(_ context.Context, userID string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.byID[userID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &user, nil
}

func (r *memoryUserRepo) Create(_ context.Context, user domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[user.ID] = user
	r.byEmail[user.Email] = user.ID
	return nil
}

func (r *memoryUserRepo) UpdatePassword(_ context.Context, userID, hashedPassword string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	user := r.byID[userID]
	user.HashedPassword = hashedPassword
	user.UpdatedAt = at
	r.byID[userID] = user
	return nil
}

func (r *memoryUserRepo) MarkEmailVerified(_ context.Context, userID string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	user := r.byID[userID]
	user.EmailVerifiedAt = &at
	user.UpdatedAt = at
	r.byID[userID] = user
	return nil
}

func (r *memoryUserRepo) SetMFAEnabledAt(_ context.Context, userID string, enabledAt *time.Time, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	user := r.byID[userID]
	user.MFAEnabledAt = enabledAt
	user.UpdatedAt = at
	r.byID[userID] = user
	return nil
}

type memorySessionRepo struct {
	mu       sync.Mutex
	sessions map[string]domain.Session
}

func newMemorySessionRepo() *memorySessionRepo {
	return &memorySessionRepo{sessions: map[string]domain.Session{}}
}

func (r *memorySessionRepo) Create(_ context.Context, session domain.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[session.ID] = session
	return nil
}

func (r *memorySessionRepo) RevokeByID(_ context.Context, sessionID string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.sessions[sessionID]
	s.RevokedAt = &now
	r.sessions[sessionID] = s
	return nil
}

func (r *memorySessionRepo) RevokeByUser(_ context.Context, userID string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, sess := range r.sessions {
		if sess.UserID == userID && sess.RevokedAt == nil {
			sess.RevokedAt = &now
			r.sessions[id] = sess
		}
	}
	return nil
}

func (r *memorySessionRepo) RotateTokens(_ context.Context, sessionID, accessJTI, refreshJTI string, expiresAt, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.sessions[sessionID]
	s.AccessJTI = accessJTI
	s.RefreshJTI = refreshJTI
	s.ExpiresAt = expiresAt
	r.sessions[sessionID] = s
	return nil
}

func (r *memorySessionRepo) FindByID(_ context.Context, sessionID string) (*domain.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[sessionID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &s, nil
}

func (r *memorySessionRepo) FindByAccessJTI(_ context.Context, jti string) (*domain.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.sessions {
		if s.AccessJTI == jti {
			copy := s
			return &copy, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *memorySessionRepo) ListByUser(_ context.Context, userID string, now time.Time) ([]domain.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.Session{}
	for _, s := range r.sessions {
		if s.UserID == userID && s.IsActive(now) {
			out = append(out, s)
		}
	}
	return out, nil
}

type memoryRefreshStore struct {
	mu    sync.Mutex
	items map[string]string
}

func newMemoryRefreshStore() *memoryRefreshStore {
	return &memoryRefreshStore{items: map[string]string{}}
}
func (s *memoryRefreshStore) Save(_ context.Context, refreshJTI, sessionID string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[refreshJTI] = sessionID
	return nil
}
func (s *memoryRefreshStore) Find(_ context.Context, refreshJTI string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.items[refreshJTI]
	if !ok {
		return "", domain.ErrNotFound
	}
	return v, nil
}
func (s *memoryRefreshStore) Delete(_ context.Context, refreshJTI string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, refreshJTI)
	return nil
}
func (s *memoryRefreshStore) Replace(_ context.Context, oldRefreshJTI, newRefreshJTI, sessionID string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, oldRefreshJTI)
	s.items[newRefreshJTI] = sessionID
	return nil
}

type memoryAuthTokenRepo struct {
	mu     sync.Mutex
	tokens map[string]domain.AuthToken
}

func newMemoryAuthTokenRepo() *memoryAuthTokenRepo {
	return &memoryAuthTokenRepo{tokens: map[string]domain.AuthToken{}}
}
func (r *memoryAuthTokenRepo) Create(_ context.Context, token domain.AuthToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens[token.TokenHash] = token
	return nil
}
func (r *memoryAuthTokenRepo) FindByHash(_ context.Context, purpose, tokenHash string) (*domain.AuthToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	token, ok := r.tokens[tokenHash]
	if !ok || token.Purpose != purpose {
		return nil, domain.ErrNotFound
	}
	return &token, nil
}
func (r *memoryAuthTokenRepo) Consume(_ context.Context, tokenID string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for hash, token := range r.tokens {
		if token.ID == tokenID {
			token.ConsumedAt = &at
			r.tokens[hash] = token
			return nil
		}
	}
	return nil
}
func (r *memoryAuthTokenRepo) DeleteByUserAndPurpose(_ context.Context, userID, purpose string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for hash, token := range r.tokens {
		if token.UserID == userID && token.Purpose == purpose {
			delete(r.tokens, hash)
		}
	}
	return nil
}

type memoryTOTPRepo struct {
	mu       sync.Mutex
	secrets  map[string]domain.TOTPSecret
	recovery map[string][]domain.RecoveryCode
}

func newMemoryTOTPRepo() *memoryTOTPRepo {
	return &memoryTOTPRepo{secrets: map[string]domain.TOTPSecret{}, recovery: map[string][]domain.RecoveryCode{}}
}
func (r *memoryTOTPRepo) UpsertSecret(_ context.Context, secret domain.TOTPSecret) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.secrets[secret.UserID] = secret
	return nil
}
func (r *memoryTOTPRepo) FindSecretByUser(_ context.Context, userID string) (*domain.TOTPSecret, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	secret, ok := r.secrets[userID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &secret, nil
}
func (r *memoryTOTPRepo) DeleteSecret(_ context.Context, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.secrets, userID)
	return nil
}
func (r *memoryTOTPRepo) ReplaceRecoveryCodes(_ context.Context, userID string, codes []domain.RecoveryCode) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recovery[userID] = codes
	return nil
}
func (r *memoryTOTPRepo) ListRecoveryCodes(_ context.Context, userID string) ([]domain.RecoveryCode, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]domain.RecoveryCode(nil), r.recovery[userID]...), nil
}
func (r *memoryTOTPRepo) ConsumeRecoveryCode(_ context.Context, codeID string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for userID, codes := range r.recovery {
		for i, code := range codes {
			if code.ID == codeID {
				codes[i].ConsumedAt = &at
				r.recovery[userID] = codes
				return nil
			}
		}
	}
	return nil
}

type noopExternalRepo struct{}

func (noopExternalRepo) FindByProviderIdentity(context.Context, string, string) (*domain.ExternalAuthAccount, error) {
	return nil, domain.ErrNotFound
}
func (noopExternalRepo) Create(context.Context, domain.ExternalAuthAccount) error { return nil }
func (noopExternalRepo) TouchLogin(context.Context, string, time.Time) error      { return nil }

type captureSender struct {
	mu       sync.Mutex
	messages []platformemail.Message
}

func (s *captureSender) Send(_ context.Context, msg platformemail.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
	return nil
}

func (s *captureSender) last() platformemail.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.messages) == 0 {
		return platformemail.Message{}
	}
	return s.messages[len(s.messages)-1]
}

func setupAuthRouter(t *testing.T) (http.Handler, *captureSender) {
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
	sender := &captureSender{}
	svc := identityapp.NewService(identityapp.Options{
		UsersRead:        users,
		UsersWrite:       users,
		ExternalsRead:    noopExternalRepo{},
		ExternalsWrite:   noopExternalRepo{},
		SessionsRead:     sessions,
		SessionsWrite:    sessions,
		Hasher:           security.NewPasswordHasher(4),
		Tokens:           identitytoken.NewJWTManager("send-flow-test", "access-secret", "refresh-secret", time.Minute, time.Hour),
		OAuthState:       nil,
		RefreshStore:     refreshStore,
		AuthTokens:       authTokens,
		TOTP:             totp,
		Providers:        nil,
		OAuthStateTTL:    time.Minute,
		MailSender:       sender,
		FrontendBaseURL:  "http://localhost:3000",
		VerificationTTL:  24 * time.Hour,
		PasswordResetTTL: time.Hour,
		MFAChallengeTTL:  10 * time.Minute,
		WorkspacesRead:   workspaces,
		WorkspacesWrite:  workspaces,
		RolesRead:        roles,
		RolesWrite:       roles,
		MembershipsRead:  memberships,
		MembershipsWrite: memberships,
		InvitationsRead:  invitations,
		InvitationsWrite: invitations,
	})
	healthSvc := platformhealth.NewService(platformhealth.Options{
		AppName:       "sendflow",
		PostgresCheck: func(context.Context) error { return nil },
		RedisCheck:    func(context.Context) error { return nil },
	})
	return newRouter(healthSvc, svc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, false, "http://localhost:3000", observability.NewHTTPMetrics(nil), nil), sender
}

func jsonRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func extractCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func extractTokenFromMessage(t *testing.T, msg platformemail.Message) string {
	t.Helper()
	idx := strings.Index(msg.Text, "token=")
	if idx < 0 {
		t.Fatalf("missing token in email body: %q", msg.Text)
	}
	raw := msg.Text[idx:]
	parsed, err := url.Parse("http://localhost/?" + raw)
	if err != nil {
		t.Fatalf("parse token from message: %v", err)
	}
	return parsed.Query().Get("token")
}

func authHeader(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
}

func decodeData(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, _ := body["data"].(map[string]any)
	return data
}

func TestSignupSetsRefreshCookieAndNoRefreshTokenLeak(t *testing.T) {
	router, _ := setupAuthRouter(t)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, jsonRequest(t, http.MethodPost, "/api/v1/auth/signup", map[string]string{
		"email":    "owner@example.com",
		"password": "StrongPassword123!",
	}))

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	data := decodeData(t, rec)
	if _, ok := data["refresh_token"]; ok {
		t.Fatal("did not expect refresh_token in response body")
	}
	if data["access_token"] == "" {
		t.Fatal("expected access token in response body")
	}
	cookie := extractCookie(rec, "sf_refresh_token")
	if cookie == nil || !cookie.HttpOnly || cookie.Path != "/api/v1/auth/refresh" {
		t.Fatalf("unexpected refresh cookie: %#v", cookie)
	}
}

func TestRefreshUsesCookieOnly(t *testing.T) {
	router, _ := setupAuthRouter(t)
	signup := httptest.NewRecorder()
	router.ServeHTTP(signup, jsonRequest(t, http.MethodPost, "/api/v1/auth/signup", map[string]string{
		"email":    "refresh@example.com",
		"password": "StrongPassword123!",
	}))
	cookie := extractCookie(signup, "sf_refresh_token")
	if cookie == nil {
		t.Fatal("expected refresh cookie")
	}

	req := jsonRequest(t, http.MethodPost, "/api/v1/auth/refresh", map[string]string{"refresh_token": "fake"})
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with cookie refresh, got %d body=%s", rec.Code, rec.Body.String())
	}

	recNoCookie := httptest.NewRecorder()
	router.ServeHTTP(recNoCookie, jsonRequest(t, http.MethodPost, "/api/v1/auth/refresh", map[string]string{"refresh_token": cookie.Value}))
	if recNoCookie.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without cookie, got %d", recNoCookie.Code)
	}
}

func TestForgotResetPasswordFlowDoesNotEnumerate(t *testing.T) {
	router, sender := setupAuthRouter(t)
	router.ServeHTTP(httptest.NewRecorder(), jsonRequest(t, http.MethodPost, "/api/v1/auth/signup", map[string]string{
		"email":    "reset@example.com",
		"password": "StrongPassword123!",
	}))

	existing := httptest.NewRecorder()
	router.ServeHTTP(existing, jsonRequest(t, http.MethodPost, "/api/v1/auth/password/forgot", map[string]string{"email": "reset@example.com"}))
	missing := httptest.NewRecorder()
	router.ServeHTTP(missing, jsonRequest(t, http.MethodPost, "/api/v1/auth/password/forgot", map[string]string{"email": "missing@example.com"}))
	if existing.Code != http.StatusOK || missing.Code != http.StatusOK {
		t.Fatalf("expected forgot password to be opaque, got %d and %d", existing.Code, missing.Code)
	}

	token := extractTokenFromMessage(t, sender.last())
	reset := httptest.NewRecorder()
	router.ServeHTTP(reset, jsonRequest(t, http.MethodPost, "/api/v1/auth/password/reset", map[string]string{
		"token":        token,
		"new_password": "NewStrongPassword123!",
	}))
	if reset.Code != http.StatusOK {
		t.Fatalf("expected reset password success, got %d body=%s", reset.Code, reset.Body.String())
	}

	oldLogin := httptest.NewRecorder()
	router.ServeHTTP(oldLogin, jsonRequest(t, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    "reset@example.com",
		"password": "StrongPassword123!",
	}))
	if oldLogin.Code != http.StatusUnauthorized {
		t.Fatalf("expected old password to fail, got %d", oldLogin.Code)
	}
	newLogin := httptest.NewRecorder()
	router.ServeHTTP(newLogin, jsonRequest(t, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    "reset@example.com",
		"password": "NewStrongPassword123!",
	}))
	if newLogin.Code != http.StatusOK {
		t.Fatalf("expected new password to work, got %d body=%s", newLogin.Code, newLogin.Body.String())
	}
}

func TestMFALoginTwoStep(t *testing.T) {
	router, _ := setupAuthRouter(t)
	signup := httptest.NewRecorder()
	router.ServeHTTP(signup, jsonRequest(t, http.MethodPost, "/api/v1/auth/signup", map[string]string{
		"email":    "mfa@example.com",
		"password": "StrongPassword123!",
	}))
	signupData := decodeData(t, signup)
	accessToken, _ := signupData["access_token"].(string)

	setupReq := jsonRequest(t, http.MethodPost, "/api/v1/auth/mfa/totp/setup", nil)
	authHeader(setupReq, accessToken)
	setupRec := httptest.NewRecorder()
	router.ServeHTTP(setupRec, setupReq)
	if setupRec.Code != http.StatusOK {
		t.Fatalf("expected mfa setup success, got %d body=%s", setupRec.Code, setupRec.Body.String())
	}
	setupData := decodeData(t, setupRec)
	secret, _ := setupData["secret"].(string)
	code, err := security.TOTPCode(secret, time.Now().UTC(), 30)
	if err != nil {
		t.Fatalf("generate totp: %v", err)
	}

	enableReq := jsonRequest(t, http.MethodPost, "/api/v1/auth/mfa/totp/enable", map[string]string{"code": code})
	authHeader(enableReq, accessToken)
	enableRec := httptest.NewRecorder()
	router.ServeHTTP(enableRec, enableReq)
	if enableRec.Code != http.StatusOK {
		t.Fatalf("expected mfa enable success, got %d body=%s", enableRec.Code, enableRec.Body.String())
	}

	login := httptest.NewRecorder()
	router.ServeHTTP(login, jsonRequest(t, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    "mfa@example.com",
		"password": "StrongPassword123!",
	}))
	if login.Code != http.StatusOK {
		t.Fatalf("expected login challenge, got %d body=%s", login.Code, login.Body.String())
	}
	loginData := decodeData(t, login)
	if loginData["mfa_required"] != true {
		t.Fatalf("expected mfa_required=true, got %#v", loginData)
	}
	challenge, _ := loginData["mfa_challenge_token"].(string)

	mfaCode, err := security.TOTPCode(secret, time.Now().UTC(), 30)
	if err != nil {
		t.Fatalf("generate totp challenge: %v", err)
	}
	challengeRec := httptest.NewRecorder()
	router.ServeHTTP(challengeRec, jsonRequest(t, http.MethodPost, "/api/v1/auth/login/mfa", map[string]string{
		"mfa_challenge_token": challenge,
		"code":                mfaCode,
	}))
	if challengeRec.Code != http.StatusOK {
		t.Fatalf("expected mfa login success, got %d body=%s", challengeRec.Code, challengeRec.Body.String())
	}
	if extractCookie(challengeRec, "sf_refresh_token") == nil {
		t.Fatal("expected refresh cookie after mfa login")
	}
}
