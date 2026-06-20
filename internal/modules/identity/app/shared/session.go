package shared

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type SessionFactory struct {
	idGen         ports.IDGenerator
	tokens        ports.TokenManager
	sessionsWrite ports.SessionWriteRepository
	refreshStore  ports.RefreshStore
	logger        *slog.Logger
}

func NewSessionFactory(idGen ports.IDGenerator, tokens ports.TokenManager, sessionsWrite ports.SessionWriteRepository, refreshStore ports.RefreshStore, logger *slog.Logger) *SessionFactory {
	return &SessionFactory{
		idGen:         idGen,
		tokens:        tokens,
		sessionsWrite: sessionsWrite,
		refreshStore:  refreshStore,
		logger:        logger,
	}
}

func (sf *SessionFactory) NewSession(ctx context.Context, in NewSessionInput) (*SessionContext, error) {
	sessionID, err := sf.idGen.New()
	if err != nil {
		sf.logger.Error("failed to generate session ID", "usecase", "session", "error", err)
		return nil, err
	}
	tokens, accessJTI, refreshJTI, err := sf.tokens.Issue(in.User.ID, sessionID, in.Now)
	if err != nil {
		sf.logger.Error("failed to issue tokens", "usecase", "session", "user_id", in.User.ID, "error", err)
		return nil, err
	}
	sess := domain.NewSession(sessionID, in.User.ID, in.Method, accessJTI, refreshJTI, tokens.RefreshExpiresAt, in.IP, in.UA, in.Now)
	if err := sf.sessionsWrite.Create(ctx, sess); err != nil {
		sf.logger.Error("failed to create session", "usecase", "session", "user_id", in.User.ID, "session_id", sessionID, "error", err)
		return nil, err
	}
	if err := sf.refreshStore.Save(ctx, refreshJTI, sessionID, time.Until(tokens.RefreshExpiresAt)); err != nil {
		sf.logger.Error("failed to save refresh token mapping", "usecase", "session", "user_id", in.User.ID, "session_id", sessionID, "error", err)
		return nil, err
	}
	sf.logger.Info("session created", "usecase", "session", "user_id", in.User.ID, "session_id", sessionID, "method", in.Method)
	return &SessionContext{Session: sess, User: in.User, Tokens: tokens}, nil
}
