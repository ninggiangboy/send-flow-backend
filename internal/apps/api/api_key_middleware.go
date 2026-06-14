package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	accessapp "github.com/ninggiangboy/send-flow/backend/internal/modules/access/app"
	accessdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/access/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/httputil"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/observability"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/ratelimit"
)

const (
	ctxAPIKeyWorkspaceID contextKey = "api_key_workspace_id"
	ctxAPIKeyID          contextKey = "api_key_id"
	ctxAPIKeyScopes      contextKey = "api_key_scopes"
	ctxAPIKeyPrefix      contextKey = "api_key_prefix"
)

type apiKeyAuthMiddleware struct {
	svc     *accessapp.Service
	metrics *observability.APIKeyMetrics
	limiter ratelimit.Service
}

func newAPIKeyAuthMiddleware(svc *accessapp.Service, metrics *observability.APIKeyMetrics, limiter ratelimit.Service) *apiKeyAuthMiddleware {
	return &apiKeyAuthMiddleware{svc: svc, metrics: metrics, limiter: limiter}
}

func (m *apiKeyAuthMiddleware) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		token, ok := httputil.ExtractBearerToken(authHeader)
		if !ok {
			if m.metrics != nil {
				m.metrics.RecordAuthAttempt("missing_token")
			}
			writeError(w, r, http.StatusUnauthorized, "api_key.invalid", "missing or malformed bearer token", nil)
			return
		}

		// Rate limit by client IP for invalid attempts
		if m.limiter != nil {
			prefix := accessdomain.DerivePrefix(token)
			rateKey := "api_key:auth:ip:" + clientIP(r)
			allowed, err := m.limiter.Allow(r.Context(), rateKey, 20, time.Minute)
			if err != nil {
				writeError(w, r, http.StatusInternalServerError, "internal.error", "internal error", nil)
				return
			}
			if !allowed {
				if m.metrics != nil {
					m.metrics.RecordAuthAttempt("rate_limited")
				}
				writeError(w, r, http.StatusTooManyRequests, "api_key.rate_limited", "too many api key auth attempts", nil)
				return
			}
			// Also rate limit per key prefix to prevent brute force
			prefixKey := "api_key:auth:prefix:" + prefix
			allowedPrefix, prefixErr := m.limiter.Allow(r.Context(), prefixKey, 10, time.Minute)
			if prefixErr != nil {
				writeError(w, r, http.StatusInternalServerError, "internal.error", "internal error", nil)
				return
			}
			if !allowedPrefix {
				if m.metrics != nil {
					m.metrics.RecordAuthAttempt("rate_limited_prefix")
				}
				writeError(w, r, http.StatusTooManyRequests, "api_key.rate_limited", "too many api key auth attempts for this key", nil)
				return
			}
		}

		key, err := m.svc.AuthenticateAPIKey(r.Context(), accessapp.AuthenticateAPIKeyInput{
			BearerToken: token,
		})
		if err != nil {
			if m.metrics != nil {
				m.metrics.RecordAuthAttempt("invalid")
			}
			if errors.Is(err, accessdomain.ErrAPIKeyInvalid) {
				writeError(w, r, http.StatusUnauthorized, "api_key.invalid", "invalid, revoked, or expired api key", nil)
				return
			}
			writeError(w, r, http.StatusInternalServerError, "internal.error", "internal error", nil)
			return
		}

		if m.metrics != nil {
			m.metrics.RecordAuthAttempt("success")
		}

		ctx := context.WithValue(r.Context(), ctxAPIKeyWorkspaceID, key.WorkspaceID)
		ctx = context.WithValue(ctx, ctxAPIKeyID, key.APIKeyID)
		ctx = context.WithValue(ctx, ctxAPIKeyScopes, key.Scopes)
		ctx = context.WithValue(ctx, ctxAPIKeyPrefix, key.KeyPrefix)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requireAPIKeyScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scopes, _ := r.Context().Value(ctxAPIKeyScopes).([]string)
			if !accessdomain.HasScope(scopes, scope) {
				writeError(w, r, http.StatusForbidden, "api_key.scope_denied", "api key does not have required scope: "+scope, nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
