package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	identityapp "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/ratelimit"
)

type contextKey string

const (
	ctxUserID         contextKey = "user_id"
	ctxSessionID      contextKey = "session_id"
	ctxRequestContext contextKey = "request_context"
)

type requestLogContext struct {
	RequestID string
	UserID    string
	SessionID string
}

type authHTTP struct {
	svc          *identityapp.Service
	rateLimiter  ratelimit.Service
	secureCookie bool
}

func newAuthHTTP(svc *identityapp.Service, rateLimiter ratelimit.Service, secureCookie bool) *authHTTP {
	return &authHTTP{svc: svc, rateLimiter: rateLimiter, secureCookie: secureCookie}
}

func (a *authHTTP) registerRoutes(r chi.Router) {
	r.Post("/auth/signup", a.signup)
	r.Post("/auth/login", a.login)
	r.Post("/auth/login/mfa", a.loginMFA)
	r.Post("/auth/refresh", a.refresh)
	r.Get("/auth/providers", a.providers)
	r.Post("/auth/oauth/{provider}/start", a.oauthStart)
	r.Post("/auth/oauth/{provider}/exchange", a.oauthExchange)
	r.Post("/auth/email/verify", a.verifyEmail)
	r.Post("/auth/password/forgot", a.forgotPassword)
	r.Post("/auth/password/reset", a.resetPassword)

	r.Group(func(r chi.Router) {
		r.Use(a.authz)
		r.Post("/auth/logout", a.logout)
		r.Get("/auth/me", a.me)
		r.Post("/auth/email/verify/request", a.requestVerifyEmail)
		r.Post("/auth/mfa/totp/setup", a.mfaTOTPSetup)
		r.Post("/auth/mfa/totp/enable", a.mfaTOTPEnable)
		r.Post("/auth/mfa/totp/disable", a.mfaTOTPDisable)
		r.Post("/auth/mfa/recovery/regenerate", a.mfaRecoveryRegenerate)
		r.Get("/sessions", a.sessions)
		r.Delete("/sessions/{session_id}", a.revokeSession)
	})
}

func (a *authHTTP) signup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if !a.allow(w, r, "signup", req.Email, 10, time.Minute) {
		return
	}
	sctx, err := a.svc.Signup(r.Context(), req.Email, req.Password, clientIP(r), r.UserAgent(), time.Now().UTC())
	if err != nil {
		writeAuthErr(w, r, err)
		return
	}
	a.setRefreshCookie(w, sctx.Tokens.RefreshToken, sctx.Tokens.RefreshExpiresAt)
	writeEnvelope(w, r, http.StatusCreated, authSessionData(sctx))
}

func (a *authHTTP) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if !a.allow(w, r, "login", req.Email, 10, time.Minute) {
		return
	}
	result, err := a.svc.Login(r.Context(), req.Email, req.Password, clientIP(r), r.UserAgent(), time.Now().UTC())
	if err != nil {
		writeAuthErr(w, r, err)
		return
	}
	if result.MFARequired {
		writeEnvelope(w, r, http.StatusOK, map[string]any{
			"mfa_required":        true,
			"mfa_challenge_token": result.MFAChallengeToken,
			"user":                map[string]any{"id": result.User.ID, "email": result.User.Email, "email_verified": result.User.EmailVerified()},
		})
		return
	}
	a.setRefreshCookie(w, result.SessionContext.Tokens.RefreshToken, result.SessionContext.Tokens.RefreshExpiresAt)
	writeEnvelope(w, r, http.StatusOK, authSessionData(result.SessionContext))
}

func (a *authHTTP) loginMFA(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChallengeToken string `json:"mfa_challenge_token"`
		Code           string `json:"code"`
		RecoveryCode   string `json:"recovery_code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if !a.allow(w, r, "login_mfa", "", 10, time.Minute) {
		return
	}
	sctx, err := a.svc.MFALogin(r.Context(), req.ChallengeToken, req.Code, req.RecoveryCode, clientIP(r), r.UserAgent(), time.Now().UTC())
	if err != nil {
		writeAuthErr(w, r, err)
		return
	}
	a.setRefreshCookie(w, sctx.Tokens.RefreshToken, sctx.Tokens.RefreshExpiresAt)
	writeEnvelope(w, r, http.StatusOK, authSessionData(sctx))
}

func (a *authHTTP) refresh(w http.ResponseWriter, r *http.Request) {
	if !a.allow(w, r, "refresh", "", 20, time.Minute) {
		return
	}
	cookie, err := r.Cookie("sf_refresh_token")
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		writeError(w, r, http.StatusUnauthorized, "auth.invalid_token", "missing refresh token", nil)
		return
	}
	sctx, err := a.svc.Refresh(r.Context(), cookie.Value, time.Now().UTC())
	if err != nil {
		writeAuthErr(w, r, err)
		return
	}
	a.setRefreshCookie(w, sctx.Tokens.RefreshToken, sctx.Tokens.RefreshExpiresAt)
	writeEnvelope(w, r, http.StatusOK, authSessionData(sctx))
}

func (a *authHTTP) providers(w http.ResponseWriter, r *http.Request) {
	providers := a.svc.ListProviders()
	sort.Slice(providers, func(i, j int) bool { return providers[i].Provider < providers[j].Provider })
	writeEnvelope(w, r, http.StatusOK, providers)
}

func (a *authHTTP) oauthStart(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	var req struct {
		RedirectURI   string `json:"redirect_uri"`
		Intent        string `json:"intent"`
		CodeChallenge string `json:"code_challenge"`
		CodeVerifier  string `json:"code_verifier"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := a.svc.OAuthStart(r.Context(), provider, req.RedirectURI, req.Intent, req.CodeChallenge, req.CodeVerifier, time.Now().UTC())
	if err != nil {
		writeAuthErr(w, r, err)
		return
	}
	writeEnvelope(w, r, http.StatusOK, res)
}

func (a *authHTTP) oauthExchange(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	var req struct {
		Code         string `json:"code"`
		State        string `json:"state"`
		RedirectURI  string `json:"redirect_uri"`
		CodeVerifier string `json:"code_verifier"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	sctx, _, err := a.svc.OAuthExchange(r.Context(), provider, req.Code, req.State, req.RedirectURI, req.CodeVerifier, clientIP(r), r.UserAgent(), time.Now().UTC())
	if err != nil {
		writeAuthErr(w, r, err)
		return
	}
	a.setRefreshCookie(w, sctx.Tokens.RefreshToken, sctx.Tokens.RefreshExpiresAt)
	writeEnvelope(w, r, http.StatusOK, authSessionData(sctx))
}

func (a *authHTTP) logout(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(ctxUserID).(string)
	sessionID, _ := r.Context().Value(ctxSessionID).(string)
	if sessionID == "" {
		writeError(w, r, http.StatusUnauthorized, "auth.invalid_token", "invalid token", nil)
		return
	}
	if err := a.svc.RevokeSession(r.Context(), sessionID, userID, time.Now().UTC()); err != nil {
		writeAuthErr(w, r, err)
		return
	}
	a.clearRefreshCookie(w)
	writeEnvelope(w, r, http.StatusOK, map[string]bool{"revoked": true})
}

func (a *authHTTP) me(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(ctxUserID).(string)
	user, err := a.svc.GetMe(r.Context(), userID)
	if err != nil {
		writeAuthErr(w, r, err)
		return
	}
	writeEnvelope(w, r, http.StatusOK, map[string]any{"id": user.ID, "email": user.Email, "email_verified": user.EmailVerified(), "mfa_enabled": user.MFAEnabled()})
}

func (a *authHTTP) sessions(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(ctxUserID).(string)
	sessions, err := a.svc.ListSessions(r.Context(), userID, time.Now().UTC())
	if err != nil {
		writeAuthErr(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, map[string]any{"id": s.ID, "created_at": s.CreatedAt, "expires_at": s.ExpiresAt, "ip_address": s.IPAddress, "auth_method": s.AuthMethod})
	}
	writeEnvelope(w, r, http.StatusOK, out)
}

func (a *authHTTP) revokeSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "session_id")
	userID, _ := r.Context().Value(ctxUserID).(string)
	if err := a.svc.RevokeSession(r.Context(), sessionID, userID, time.Now().UTC()); err != nil {
		writeAuthErr(w, r, err)
		return
	}
	writeEnvelope(w, r, http.StatusOK, map[string]bool{"revoked": true})
}

func (a *authHTTP) requestVerifyEmail(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(ctxUserID).(string)
	if err := a.svc.RequestEmailVerification(r.Context(), userID, time.Now().UTC()); err != nil {
		writeAuthErr(w, r, err)
		return
	}
	writeEnvelope(w, r, http.StatusOK, map[string]bool{"sent": true})
}

func (a *authHTTP) verifyEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if !a.allow(w, r, "verify_email", "", 20, time.Minute) {
		return
	}
	if err := a.svc.VerifyEmail(r.Context(), req.Token, time.Now().UTC()); err != nil {
		writeAuthErr(w, r, err)
		return
	}
	writeEnvelope(w, r, http.StatusOK, map[string]bool{"verified": true})
}

func (a *authHTTP) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if !a.allow(w, r, "forgot_password", req.Email, 5, time.Minute) {
		return
	}
	if err := a.svc.ForgotPassword(r.Context(), req.Email, time.Now().UTC()); err != nil {
		writeAuthErr(w, r, err)
		return
	}
	writeEnvelope(w, r, http.StatusOK, map[string]bool{"sent": true})
}

func (a *authHTTP) resetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if !a.allow(w, r, "reset_password", "", 10, time.Minute) {
		return
	}
	if err := a.svc.ResetPassword(r.Context(), req.Token, req.NewPassword, time.Now().UTC()); err != nil {
		writeAuthErr(w, r, err)
		return
	}
	writeEnvelope(w, r, http.StatusOK, map[string]bool{"reset": true})
}

func (a *authHTTP) mfaTOTPSetup(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(ctxUserID).(string)
	res, err := a.svc.MFATOTPSetup(r.Context(), userID, time.Now().UTC())
	if err != nil {
		writeAuthErr(w, r, err)
		return
	}
	writeEnvelope(w, r, http.StatusOK, res)
}

func (a *authHTTP) mfaTOTPEnable(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(ctxUserID).(string)
	var req struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := a.svc.MFATOTPEnable(r.Context(), userID, req.Code, time.Now().UTC())
	if err != nil {
		writeAuthErr(w, r, err)
		return
	}
	writeEnvelope(w, r, http.StatusOK, res)
}

func (a *authHTTP) mfaTOTPDisable(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(ctxUserID).(string)
	var req struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := a.svc.MFATOTPDisable(r.Context(), userID, req.Password, req.Code, time.Now().UTC()); err != nil {
		writeAuthErr(w, r, err)
		return
	}
	writeEnvelope(w, r, http.StatusOK, map[string]bool{"disabled": true})
}

func (a *authHTTP) mfaRecoveryRegenerate(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(ctxUserID).(string)
	var req struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := a.svc.MFARegenerate(r.Context(), userID, req.Code, time.Now().UTC())
	if err != nil {
		writeAuthErr(w, r, err)
		return
	}
	writeEnvelope(w, r, http.StatusOK, res)
}

func (a *authHTTP) authz(next http.Handler) http.Handler {
	return authzMiddleware(a.svc)(next)
}

func authSessionData(sctx *identityapp.SessionContext) map[string]any {
	return map[string]any{
		"user": map[string]any{"id": sctx.User.ID, "email": sctx.User.Email, "email_verified": sctx.User.EmailVerified(), "mfa_enabled": sctx.User.MFAEnabled()},
		"session": map[string]any{
			"id": sctx.Session.ID, "created_at": sctx.Session.CreatedAt, "expires_at": sctx.Session.ExpiresAt, "ip_address": sctx.Session.IPAddress, "auth_method": sctx.Session.AuthMethod,
		},
		"access_token": sctx.Tokens.AccessToken,
		"token_type":   "Bearer",
		"expires_at":   sctx.Tokens.AccessExpiresAt,
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, r, http.StatusBadRequest, "auth.invalid_request_body", "invalid request body", nil)
		return false
	}
	return true
}

func writeAuthErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrEmailAlreadyExists):
		writeError(w, r, http.StatusConflict, "identity.email_already_registered", err.Error(), map[string]any{"field": "email"})
	case errors.Is(err, domain.ErrInvalidCredentials):
		writeError(w, r, http.StatusUnauthorized, "auth.invalid_credentials", err.Error(), nil)
	case errors.Is(err, domain.ErrUnauthorized):
		writeError(w, r, http.StatusUnauthorized, "auth.invalid_token", err.Error(), nil)
	case errors.Is(err, domain.ErrPasswordPolicy):
		writeError(w, r, http.StatusUnprocessableEntity, "auth.password_policy_violation", err.Error(), nil)
	case errors.Is(err, domain.ErrRateLimited):
		writeError(w, r, http.StatusTooManyRequests, "auth.rate_limited", err.Error(), nil)
	case errors.Is(err, domain.ErrInvalidOAuthState):
		writeError(w, r, http.StatusUnauthorized, "auth.oauth_state_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrVerificationToken):
		writeError(w, r, http.StatusUnauthorized, "auth.verification_token_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrResetToken):
		writeError(w, r, http.StatusUnauthorized, "auth.reset_token_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrMFARequired):
		writeError(w, r, http.StatusUnauthorized, "auth.mfa_required", err.Error(), nil)
	case errors.Is(err, domain.ErrMFAInvalidCode):
		writeError(w, r, http.StatusUnauthorized, "auth.mfa_invalid_code", err.Error(), nil)
	case errors.Is(err, domain.ErrProviderDisabled), errors.Is(err, domain.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "auth.provider_not_supported", err.Error(), nil)
	case errors.Is(err, domain.ErrSessionNotOwned):
		writeError(w, r, http.StatusForbidden, "auth.session_not_owned", err.Error(), nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "health.runtime_not_ready", "internal error", nil)
	}
}

func clientIP(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	return r.RemoteAddr
}

func (a *authHTTP) setRefreshCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     "sf_refresh_token",
		Value:    token,
		Path:     "/api/v1/auth/refresh",
		HttpOnly: true,
		Secure:   a.secureCookie,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
	})
}

func (a *authHTTP) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "sf_refresh_token",
		Value:    "",
		Path:     "/api/v1/auth/refresh",
		HttpOnly: true,
		Secure:   a.secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func (a *authHTTP) allow(w http.ResponseWriter, r *http.Request, scope, email string, limit int64, window time.Duration) bool {
	if a.rateLimiter == nil {
		return true
	}
	keys := []string{"auth:" + scope + ":ip:" + clientIP(r)}
	if normalized, err := domain.NewEmailAddress(email); err == nil {
		keys = append(keys, "auth:"+scope+":email:"+normalized.String())
	}
	for _, key := range keys {
		ok, err := a.rateLimiter.Allow(r.Context(), key, limit, window)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "health.runtime_not_ready", "internal error", nil)
			return false
		}
		if !ok {
			writeError(w, r, http.StatusTooManyRequests, "auth.rate_limited", "rate limited", nil)
			return false
		}
	}
	return true
}
