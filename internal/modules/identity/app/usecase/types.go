package usecase

import (
	"context"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type SessionContext struct {
	Session domain.Session
	User    domain.User
	Tokens  ports.TokenPair
}

type LoginResult struct {
	SessionContext    *SessionContext
	User              *domain.User
	MFARequired       bool
	MFAChallengeToken string
}

type Provider struct {
	Provider    string `json:"provider"`
	Type        string `json:"type"`
	DisplayName string `json:"display_name"`
	Enabled     bool   `json:"enabled"`
}

type OAuthStartResult struct {
	Provider            string `json:"provider"`
	AuthorizationURL    string `json:"authorization_url"`
	State               string `json:"state"`
	CodeChallengeMethod string `json:"code_challenge_method,omitempty"`
}

type NewSessionInput struct {
	User   domain.User
	Method string
	IP     string
	UA     string
	Now    time.Time
}

type NewSession func(ctx context.Context, in NewSessionInput) (*SessionContext, error)
