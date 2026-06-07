package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	accessapp "github.com/ninggiangboy/send-flow/backend/internal/modules/access/app"
	accessdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/access/domain"
)

const (
	ctxAPIKeyWorkspaceID contextKey = "api_key_workspace_id"
	ctxAPIKeyID          contextKey = "api_key_id"
	ctxAPIKeyScopes      contextKey = "api_key_scopes"
	ctxAPIKeyPrefix      contextKey = "api_key_prefix"
)

type apiKeyAuthMiddleware struct {
	svc *accessapp.Service
}

func newAPIKeyAuthMiddleware(svc *accessapp.Service) *apiKeyAuthMiddleware {
	return &apiKeyAuthMiddleware{svc: svc}
}

func (m *apiKeyAuthMiddleware) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		token, ok := accessdomain.ExtractBearerToken(authHeader)
		if !ok {
			writeError(w, r, http.StatusUnauthorized, "api_key.invalid", "missing or malformed bearer token", nil)
			return
		}

		key, err := m.svc.AuthenticateAPIKey(r.Context(), accessapp.AuthenticateAPIKeyInput{
			BearerToken: token,
		})
		if err != nil {
			if errors.Is(err, accessdomain.ErrAPIKeyInvalid) {
				writeError(w, r, http.StatusUnauthorized, "api_key.invalid", "invalid, revoked, or expired api key", nil)
				return
			}
			writeError(w, r, http.StatusInternalServerError, "health.runtime_not_ready", "internal error", nil)
			return
		}

		// Store authenticated API key context for downstream handlers.
		// Future transactional handlers will rely on these context values
		// instead of session-based auth.
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
