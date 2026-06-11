package signup

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/transaction"
)

type Handler struct {
	deps       usecase.Deps
	newSession usecase.NewSession
	log        *slog.Logger
}

type Command struct {
	Email    string
	Password string
	IP       string
	UA       string
	Now      time.Time
}

func New(deps usecase.Deps, newSession usecase.NewSession) *Handler {
	return &Handler{deps: deps, newSession: newSession, log: deps.Logger.With("usecase", "signup")}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*usecase.SessionContext, error) {
	email, err := domain.NewEmailAddress(cmd.Email)
	if err != nil {
		return nil, domain.ErrInvalidCredentials
	}
	if cmd.Password == "" {
		return nil, domain.ErrInvalidCredentials
	}
	if err := h.deps.PasswordValidator.Validate(cmd.Password); err != nil {
		return nil, domain.ErrPasswordPolicy
	}
	existing, err := h.deps.UsersRead.FindByEmail(ctx, email.String())
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		h.log.Error("failed to lookup user by email", "error", err)
		return nil, err
	}
	if existing != nil {
		h.log.Warn("signup attempted with existing email")
		return nil, domain.ErrEmailAlreadyExists
	}
	hash, err := h.deps.Hasher.Hash(cmd.Password)
	if err != nil {
		h.log.Error("failed to hash password", "error", err)
		return nil, err
	}
	userID, err := h.deps.IDGen.New()
	if err != nil {
		h.log.Error("failed to generate user ID", "error", err)
		return nil, err
	}
	user := domain.NewUser(userID, email, hash, "password", cmd.Now)

	createUserWithOutbox := func(txCtx context.Context) error {
		if err := h.deps.UsersWrite.Create(txCtx, user); err != nil {
			if errors.Is(err, domain.ErrEmailAlreadyExists) {
				h.log.Warn("signup failed: duplicate email on write", "error", err)
				return domain.ErrEmailAlreadyExists
			}
			h.log.Error("failed to create user", "error", err)
			return err
		}
		bizPayload := contracts.UserRegisteredPayload{
			UserID:     user.ID,
			Email:      user.Email,
			AuthMethod: user.PrimaryAuthMethod,
			At:         cmd.Now.Format(time.RFC3339),
		}
		if err := usecase.EmitEvent(txCtx, h.deps.OutboxWriter, h.deps.IDGen, contracts.EventUserRegisteredV1, "user", user.ID, "", bizPayload, cmd.Now); err != nil {
			h.log.Error("failed to emit user registered event", "user_id", user.ID, "error", err)
			return err
		}
		return nil
	}

	if err := transaction.RunInTx(ctx, h.deps.UnitOfWork, createUserWithOutbox); err != nil {
		return nil, err
	}
	h.log.Info("user signed up", "user_id", user.ID)

	if token, err := usecase.CreateEmailVerificationToken(ctx, h.deps, user, cmd.Now); err != nil {
		h.log.Error("failed to create email verification token", "user_id", user.ID, "error", err)
		return nil, err
	} else if token != "" {
		link := usecase.BuildEmailVerificationLink(h.deps, token)
		if err := usecase.SendVerificationEmail(ctx, h.deps, user.Email, link); err != nil {
			h.log.Error("failed to send verification email", "user_id", user.ID, "error", err)
			return nil, err
		}
	}
	return h.newSession(ctx, usecase.NewSessionInput{User: user, Method: "password", IP: cmd.IP, UA: cmd.UA, Now: cmd.Now})
}
