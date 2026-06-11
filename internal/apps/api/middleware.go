package api

import (
	"context"
	"net/http"
	"strings"

	identityapp "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app"
)

func userIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxUserID).(string)
	return v, ok
}

func authzMiddleware(svc *identityapp.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := strings.TrimSpace(r.Header.Get("Authorization"))
			if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
				writeError(w, r, http.StatusUnauthorized, "auth.invalid_token", "missing bearer token", nil)
				return
			}
			token := strings.TrimSpace(h[len("Bearer "):])
			sess, user, err := svc.AuthenticateAccessToken(r.Context(), token)
			if err != nil {
				writeError(w, r, http.StatusUnauthorized, "auth.invalid_token", "invalid token", nil)
				return
			}
			if reqCtx, ok := r.Context().Value(ctxRequestContext).(*requestLogContext); ok {
				reqCtx.UserID = user.ID
				reqCtx.SessionID = sess.ID
			}
			ctx := context.WithValue(r.Context(), ctxUserID, user.ID)
			ctx = context.WithValue(ctx, ctxSessionID, sess.ID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
