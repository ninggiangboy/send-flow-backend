package auth

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/transaction"
)

type SignupOptions struct {
	UsersWrite        ports.UserWriteRepository
	Hasher            domain.PasswordHasher
	PasswordValidator ports.PasswordValidator
	IdGen             ports.IDGenerator
	OutboxWriter      ports.OutboxWriter
	UnitOfWork        ports.UnitOfWork
	AuthTokens        *shared.AuthTokenService
	Notifications     *shared.NotificationService
	SessionFactory    *shared.SessionFactory
	VerificationTTL   time.Duration
	Logger            *slog.Logger
}

type SignupHandler struct {
	usersWrite        ports.UserWriteRepository
	hasher            domain.PasswordHasher
	passwordValidator ports.PasswordValidator
	idGen             ports.IDGenerator
	outboxWriter      ports.OutboxWriter
	unitOfWork        ports.UnitOfWork
	authTokens        *shared.AuthTokenService
	notifications     *shared.NotificationService
	sessionFactory    *shared.SessionFactory
	verificationTTL   time.Duration
	log               *slog.Logger
}

type SignupCommand struct {
	Email    string
	Password string
	IP       string
	UA       string
	Now      time.Time
}

func NewSignupHandler(opts SignupOptions) *SignupHandler {
	return &SignupHandler{
		usersWrite:        opts.UsersWrite,
		hasher:            opts.Hasher,
		passwordValidator: opts.PasswordValidator,
		idGen:             opts.IdGen,
		outboxWriter:      opts.OutboxWriter,
		unitOfWork:        opts.UnitOfWork,
		authTokens:        opts.AuthTokens,
		notifications:     opts.Notifications,
		sessionFactory:    opts.SessionFactory,
		verificationTTL:   opts.VerificationTTL,
		log:               opts.Logger.With("usecase", "signup"),
	}
}

func (h *SignupHandler) Execute(ctx context.Context, cmd SignupCommand) (*shared.SessionContext, error) {
	email, err := domain.NewEmailAddress(cmd.Email)
	if err != nil {
		return nil, domain.ErrInvalidCredentials
	}
	if cmd.Password == "" {
		return nil, domain.ErrInvalidCredentials
	}
	if err := h.passwordValidator.Validate(cmd.Password); err != nil {
		return nil, domain.ErrPasswordPolicy
	}
	existing, err := h.usersWrite.FindByEmail(ctx, email.String())
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		h.log.Error("failed to lookup user by email", "error", err)
		return nil, err
	}
	if existing != nil {
		h.log.Warn("signup attempted with existing email")
		return nil, domain.ErrEmailAlreadyExists
	}
	hash, err := h.hasher.Hash(cmd.Password)
	if err != nil {
		h.log.Error("failed to hash password", "error", err)
		return nil, err
	}
	userID, err := h.idGen.New()
	if err != nil {
		h.log.Error("failed to generate user ID", "error", err)
		return nil, err
	}
	user := domain.NewUser(userID, email, hash, "password", cmd.Now)

	createUserWithOutbox := func(txCtx context.Context) error {
		if err := h.usersWrite.Create(txCtx, user); err != nil {
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
		if err := shared.EmitEvent(txCtx, h.outboxWriter, h.idGen, contracts.EventUserRegisteredV1, "user", user.ID, "", bizPayload, cmd.Now); err != nil {
			h.log.Error("failed to emit user registered event", "user_id", user.ID, "error", err)
			return err
		}
		return nil
	}

	if err := transaction.RunInTx(ctx, h.unitOfWork, createUserWithOutbox); err != nil {
		return nil, err
	}
	h.log.Info("user signed up", "user_id", user.ID)

	if token, err := h.authTokens.CreateToken(ctx, user.ID, domain.AuthTokenPurposeEmailVerification, h.verificationTTL, cmd.Now); err != nil {
		h.log.Error("failed to create email verification token", "user_id", user.ID, "error", err)
		return nil, err
	} else if token != "" {
		link := h.notifications.BuildEmailVerificationLink(token)
		if err := h.notifications.SendVerificationEmail(ctx, user.Email, link); err != nil {
			h.log.Error("failed to send verification email", "user_id", user.ID, "error", err)
			return nil, err
		}
	}
	return h.sessionFactory.NewSession(ctx, shared.NewSessionInput{User: user, Method: "password", IP: cmd.IP, UA: cmd.UA, Now: cmd.Now})
}
