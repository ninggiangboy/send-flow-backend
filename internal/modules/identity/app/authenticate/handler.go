package authenticate

import (
	"context"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

// Handler validates an access token (JWT) and extracts session/user identity claims.
// It is used by the API middleware for request-level authentication.
//
// Design note: this handler performs no database lookup. It relies entirely on JWT claims
// for speed, since it runs on every authenticated request. Token revocation is handled
// at the refresh-token level, not the access-token level.
type Handler struct{ deps usecase.Deps }

func New(deps usecase.Deps) *Handler { return &Handler{deps: deps} }

// Execute parses the JWT access token and returns lightweight Session and User value objects
// populated from claims. Returns ErrUnauthorized for invalid, expired, or incomplete tokens.
func (h *Handler) Execute(ctx context.Context, token string) (*domain.Session, *domain.User, error) {
	claims, err := h.deps.Tokens.ParseAccess(token)
	if err != nil {
		return nil, nil, domain.ErrUnauthorized
	}
	if claims.Subject == "" || claims.SessionID == "" || claims.JWTID == "" {
		return nil, nil, domain.ErrUnauthorized
	}
	sess := &domain.Session{
		ID:        claims.SessionID,
		UserID:    claims.Subject,
		AccessJTI: claims.JWTID,
		ExpiresAt: time.Unix(claims.ExpiresAt, 0).UTC(),
	}
	user := &domain.User{ID: claims.Subject}
	return sess, user, nil
}
