package usecase

import (
	"context"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

func BuildNewSession(deps Deps) NewSession {
	return func(ctx context.Context, in NewSessionInput) (*SessionContext, error) {
		sessionID, err := deps.IDGen.New()
		if err != nil {
			deps.Logger.Error("failed to generate session ID", "usecase", "session", "error", err)
			return nil, err
		}
		tokens, accessJTI, refreshJTI, err := deps.Tokens.Issue(in.User.ID, sessionID, in.Now)
		if err != nil {
			deps.Logger.Error("failed to issue tokens", "usecase", "session", "user_id", in.User.ID, "error", err)
			return nil, err
		}
		sess := domain.NewSession(sessionID, in.User.ID, in.Method, accessJTI, refreshJTI, tokens.RefreshExpiresAt, in.IP, in.UA, in.Now)
		if err := deps.SessionsWrite.Create(ctx, sess); err != nil {
			deps.Logger.Error("failed to create session", "usecase", "session", "user_id", in.User.ID, "session_id", sessionID, "error", err)
			return nil, err
		}
		if err := deps.RefreshStore.Save(ctx, refreshJTI, sessionID, time.Until(tokens.RefreshExpiresAt)); err != nil {
			deps.Logger.Error("failed to save refresh token mapping", "usecase", "session", "user_id", in.User.ID, "session_id", sessionID, "error", err)
			return nil, err
		}
		deps.Logger.Info("session created", "usecase", "session", "user_id", in.User.ID, "session_id", sessionID, "method", in.Method)
		return &SessionContext{Session: sess, User: in.User, Tokens: tokens}, nil
	}
}
