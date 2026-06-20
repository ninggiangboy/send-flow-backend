package shared

import (
	"context"
	"strings"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type AuthTokenService struct {
	authTokens  ports.AuthTokenRepository
	tokenGen    ports.TokenGenerator
	idGen       ports.IDGenerator
	tokenHasher ports.TokenHasher
}

func NewAuthTokenService(authTokens ports.AuthTokenRepository, tokenGen ports.TokenGenerator, idGen ports.IDGenerator, tokenHasher ports.TokenHasher) *AuthTokenService {
	return &AuthTokenService{
		authTokens:  authTokens,
		tokenGen:    tokenGen,
		idGen:       idGen,
		tokenHasher: tokenHasher,
	}
}

func (s *AuthTokenService) CreateToken(ctx context.Context, userID, purpose string, ttl time.Duration, now time.Time) (string, error) {
	if s.authTokens == nil {
		return "", nil
	}
	if err := s.authTokens.DeleteByUserAndPurpose(ctx, userID, purpose); err != nil {
		return "", err
	}
	raw, err := s.tokenGen.RandomToken(32)
	if err != nil {
		return "", err
	}
	id, err := s.idGen.New()
	if err != nil {
		return "", err
	}
	token := domain.AuthToken{
		ID:        id,
		UserID:    userID,
		Purpose:   purpose,
		TokenHash: s.tokenHasher.HashToken(raw),
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
	}
	if err := s.authTokens.Create(ctx, token); err != nil {
		return "", err
	}
	return raw, nil
}

func (s *AuthTokenService) ConsumeToken(ctx context.Context, raw, purpose string, now time.Time) (*domain.AuthToken, error) {
	if s.authTokens == nil || strings.TrimSpace(raw) == "" {
		return nil, domain.ErrUnauthorized
	}
	token, err := s.authTokens.FindByHash(ctx, purpose, s.tokenHasher.HashToken(raw))
	if err != nil {
		return nil, err
	}
	if !token.IsUsable(now) {
		return nil, domain.ErrUnauthorized
	}
	if err := s.authTokens.Consume(ctx, token.ID, now); err != nil {
		return nil, err
	}
	return token, nil
}
