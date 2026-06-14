package rotatesecret

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	webhookscontracts "github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/ports"
)

type Options struct {
	ConfigRead    ports.ConfigReadRepository
	ConfigWrite   ports.ConfigWriteRepository
	TxManager     ports.TransactionManager
	OutboxWriter  ports.OutboxWriter
	AccessChecker ports.WorkspaceAccessChecker
	IDGen         func() (string, error)
	Clock         func() time.Time
	Logger        *slog.Logger
}

type Command struct {
	WorkspaceID string
	UserID      string
	WebhookID   string
}

type Result struct {
	Config    domain.WebhookConfig
	RawSecret string
	Hint      string
}

type Handler struct {
	configRead    ports.ConfigReadRepository
	configWrite   ports.ConfigWriteRepository
	txManager     ports.TransactionManager
	outboxWriter  ports.OutboxWriter
	accessChecker ports.WorkspaceAccessChecker
	idGen         func() (string, error)
	clock         func() time.Time
	log           *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		configRead:    opts.ConfigRead,
		configWrite:   opts.ConfigWrite,
		txManager:     opts.TxManager,
		outboxWriter:  opts.OutboxWriter,
		accessChecker: opts.AccessChecker,
		idGen:         opts.IDGen,
		clock:         opts.Clock,
		log:           opts.Logger.With("usecase", "rotate_webhook_secret"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*Result, error) {
	log := h.log.With("workspace_id", cmd.WorkspaceID, "webhook_id", cmd.WebhookID)

	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "webhook.manage"); err != nil {
		return nil, err
	}

	cfg, err := h.configRead.FindByID(ctx, cmd.WorkspaceID, cmd.WebhookID)
	if err != nil {
		if errors.Is(err, domain.ErrConfigNotFound) {
			return nil, err
		}
		log.Error("failed to find webhook config for rotate", "error", err)
		return nil, err
	}

	rawSecret, hash, hint, err := generateSecret()
	if err != nil {
		log.Error("failed to generate secret", "error", err)
		return nil, err
	}

	cfg.SecretHash = hash
	cfg.SecretHint = hint
	cfg.Version++
	cfg.UpdatedAt = h.clock()

	if err := h.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if err := h.configWrite.Update(txCtx, *cfg); err != nil {
			return err
		}
		if h.outboxWriter != nil {
			eventID, err := h.idGen()
			if err != nil {
				return err
			}
			payload, err := json.Marshal(webhookscontracts.SecretRotatedPayload{
				ConfigID:    cfg.ID,
				WorkspaceID: cfg.WorkspaceID,
				Version:     cfg.Version,
				SecretHint:  hint,
			})
			if err != nil {
				return err
			}
			if err := h.outboxWriter.Save(txCtx, ports.OutboxEvent{
				ID:            eventID,
				AggregateType: "webhook_config",
				AggregateID:   cfg.ID,
				EventType:     webhookscontracts.EventSecretRotatedV1,
				Payload:       payload,
				WorkspaceID:   cfg.WorkspaceID,
				OccurredAt:    cfg.UpdatedAt,
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		if errors.Is(err, domain.ErrRotateConflict) {
			return nil, err
		}
		log.Error("failed to update config with new secret", "error", err)
		return nil, err
	}

	log.Info("webhook secret rotated", "version", cfg.Version)
	return &Result{Config: *cfg, RawSecret: rawSecret, Hint: hint}, nil
}

func generateSecret() (raw, signingKey, hint string, err error) {
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		return "", "", "", err
	}
	raw = hex.EncodeToString(rawBytes)

	hash := sha256.Sum256(rawBytes)
	signingKey = hex.EncodeToString(hash[:])

	hint = raw[:8] + "..."
	return raw, signingKey, hint, nil
}
