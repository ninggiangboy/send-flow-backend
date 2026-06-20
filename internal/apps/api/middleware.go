package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	identityapp "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app"
	platformconstants "github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
)

func userIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxUserID).(string)
	return v, ok
}

func authzMiddleware(svc *identityapp.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := strings.TrimSpace(headerAuthorization(r))
			if !strings.HasPrefix(strings.ToLower(h), strings.ToLower(platformconstants.BearerPrefix)) {
				writeError(w, r, http.StatusUnauthorized, errCodeAuthInvalidToken, "missing bearer token", nil)
				return
			}
			token := strings.TrimSpace(h[len(platformconstants.BearerPrefix):])
			sess, user, err := svc.AuthenticateAccessToken(r.Context(), token, time.Now().UTC())
			if err != nil {
				writeError(w, r, http.StatusUnauthorized, errCodeAuthInvalidToken, "invalid token", nil)
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
