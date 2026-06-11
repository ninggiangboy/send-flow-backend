package oauthexchange

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/transaction"
)

type Handler struct {
	deps       usecase.Deps
	newSession usecase.NewSession
	log        *slog.Logger
}

type Command struct {
	Provider     string
	Code         string
	State        string
	RedirectURI  string
	CodeVerifier string
	IP           string
	UA           string
	Now          time.Time
}

func New(deps usecase.Deps, newSession usecase.NewSession) *Handler {
	return &Handler{deps: deps, newSession: newSession, log: deps.Logger.With("usecase", "oauth_exchange")}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*usecase.SessionContext, *domain.OAuthIdentity, error) {
	p, ok := h.deps.Providers[cmd.Provider]
	if !ok {
		h.log.Warn("unknown OAuth provider", "provider", cmd.Provider)
		return nil, nil, domain.ErrNotFound
	}
	stored, err := h.deps.OAuthState.GetAndDelete(ctx, cmd.State)
	if err != nil || stored == nil || stored.Provider != cmd.Provider || stored.RedirectURI != cmd.RedirectURI {
		h.log.Warn("invalid OAuth state", "provider", cmd.Provider)
		return nil, nil, domain.ErrInvalidOAuthState
	}
	if stored.CodeVerifier != "" && cmd.CodeVerifier != "" && stored.CodeVerifier != cmd.CodeVerifier {
		h.log.Warn("OAuth PKCE code verifier mismatch", "provider", cmd.Provider)
		return nil, nil, domain.ErrInvalidOAuthState
	}
	identity, err := p.Exchange(ctx, cmd.Code, cmd.RedirectURI, cmd.CodeVerifier)
	if err != nil {
		h.log.Error("OAuth provider exchange failed", "provider", cmd.Provider, "error", err)
		return nil, nil, fmt.Errorf("oauth exchange: %w", err)
	}
	account, err := h.deps.ExternalsRead.FindByProviderIdentity(ctx, cmd.Provider, identity.ProviderUserID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		h.log.Error("failed to find external account", "provider", cmd.Provider, "error", err)
		return nil, nil, err
	}
	var user *domain.User
	if account != nil {
		user, err = h.deps.UsersRead.FindByID(ctx, account.UserID)
		if err != nil {
			h.log.Error("failed to find linked user", "provider", cmd.Provider, "user_id", account.UserID, "error", err)
			return nil, nil, err
		}
		if err := h.deps.ExternalsWrite.TouchLogin(ctx, account.ID, cmd.Now); err != nil {
			h.log.Warn("failed to update last login", "provider", cmd.Provider, "user_id", user.ID, "error", err)
		}
		h.log.Info("OAuth exchange: linked existing account", "provider", cmd.Provider, "user_id", user.ID)
	} else {
		email, emailErr := domain.NewEmailAddress(identity.Email)
		if emailErr != nil {
			h.log.Warn("OAuth identity email invalid", "provider", cmd.Provider)
			return nil, nil, domain.ErrUnauthorized
		}
		linkAccount := func(txCtx context.Context) error {
			fetched, err := h.deps.UsersRead.FindByEmail(txCtx, email.String())
			if err != nil && !errors.Is(err, domain.ErrNotFound) {
				return fmt.Errorf("find user by email: %w", err)
			}
			if fetched == nil {
				uID, err := h.deps.IDGen.New()
				if err != nil {
					return fmt.Errorf("generate user ID: %w", err)
				}
				u := domain.NewOAuthUser(uID, email, "oauth_"+cmd.Provider, cmd.Now)
				if err := h.deps.UsersWrite.Create(txCtx, u); err != nil {
					return fmt.Errorf("create user: %w", err)
				}
				fetched = &u
				h.log.Info("OAuth exchange: new user created", "provider", cmd.Provider, "user_id", fetched.ID)
			}
			accID, err := h.deps.IDGen.New()
			if err != nil {
				return fmt.Errorf("generate external account ID: %w", err)
			}
			acc := domain.NewExternalAuthAccount(accID, fetched.ID, cmd.Provider, identity.ProviderUserID, identity.Email, identity.EmailVerified, cmd.Now)
			if err := h.deps.ExternalsWrite.Create(txCtx, acc); err != nil {
				return fmt.Errorf("create external account: %w", err)
			}
			user = fetched
			return nil
		}
		if err := transaction.RunInTx(ctx, h.deps.UnitOfWork, linkAccount); err != nil {
			h.log.Error("failed to link OAuth account", "provider", cmd.Provider, "error", err)
			return nil, nil, err
		}
	}
	sctx, err := h.newSession(ctx, usecase.NewSessionInput{User: *user, Method: "oauth_" + cmd.Provider, IP: cmd.IP, UA: cmd.UA, Now: cmd.Now})
	if err != nil {
		return nil, nil, err
	}
	return sctx, identity, nil
}
