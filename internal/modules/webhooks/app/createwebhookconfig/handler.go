package createwebhookconfig

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
	ConfigWrite   ports.ConfigWriteRepository
	TxManager     ports.TransactionManager
	OutboxWriter  ports.OutboxWriter
	AccessChecker ports.WorkspaceAccessChecker
	IDGen         func() (string, error)
	Clock         func() time.Time
	Logger        *slog.Logger
}

type Command struct {
	WorkspaceID   string
	UserID        string
	Name          string
	TargetURL     string
	Subscriptions []string
}

type Result struct {
	Config    domain.WebhookConfig
	RawSecret string
}

type Handler struct {
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
		configWrite:   opts.ConfigWrite,
		txManager:     opts.TxManager,
		outboxWriter:  opts.OutboxWriter,
		accessChecker: opts.AccessChecker,
		idGen:         opts.IDGen,
		clock:         opts.Clock,
		log:           opts.Logger.With("usecase", "create_webhook_config"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*Result, error) {
	log := h.log.With("workspace_id", cmd.WorkspaceID)

	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "webhook.manage"); err != nil {
		return nil, err
	}

	cfg := domain.WebhookConfig{
		Name:          cmd.Name,
		WorkspaceID:   cmd.WorkspaceID,
		TargetURL:     cmd.TargetURL,
		Subscriptions: cmd.Subscriptions,
		Status:        domain.ConfigStatusActive,
	}
	if err := domain.ValidateWebhookConfig(cfg); err != nil {
		log.Warn("invalid webhook config", "error", err)
		return nil, err
	}

	id, err := h.idGen()
	if err != nil {
		return nil, err
	}

	rawSecret, hash, hint, err := GenerateSecret()
	if err != nil {
		log.Error("failed to generate secret", "error", err)
		return nil, err
	}

	now := h.clock()
	cfg.ID = id
	cfg.SecretHash = hash
	cfg.SecretHint = hint
	cfg.Version = 1
	cfg.CreatedByUserID = cmd.UserID
	cfg.CreatedAt = now
	cfg.UpdatedAt = now

	if err := h.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if err := h.configWrite.Create(txCtx, cfg); err != nil {
			return err
		}
		if h.outboxWriter != nil {
			eventID, err := h.idGen()
			if err != nil {
				return err
			}
			payload, err := json.Marshal(webhookscontracts.ConfigCreatedPayload{
				ConfigID:    id,
				WorkspaceID: cmd.WorkspaceID,
				Name:        cmd.Name,
				TargetURL:   cmd.TargetURL,
				Status:      string(cfg.Status),
				Subscribed:  cmd.Subscriptions,
			})
			if err != nil {
				return err
			}
			if err := h.outboxWriter.Save(txCtx, ports.OutboxEvent{
				ID:            eventID,
				AggregateType: "webhook_config",
				AggregateID:   id,
				EventType:     webhookscontracts.EventConfigCreatedV1,
				Payload:       payload,
				WorkspaceID:   cmd.WorkspaceID,
				OccurredAt:    now,
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		if errors.Is(err, domain.ErrConfigNameConflict) {
			return nil, err
		}
		log.Error("failed to create webhook config", "error", err)
		return nil, err
	}

	log.Info("webhook config created", "webhook_id", id)
	return &Result{Config: cfg, RawSecret: rawSecret}, nil
}

func GenerateSecret() (raw, signingKey, hint string, err error) {
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
