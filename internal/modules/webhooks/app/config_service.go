package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	webhookscontracts "github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/ports"
)

type CreateConfigInput struct {
	WorkspaceID   string
	UserID        string
	Name          string
	TargetURL     string
	Subscriptions []string
	Now           time.Time
}

type ConfigResult struct {
	ID              string     `json:"id"`
	WorkspaceID     string     `json:"workspace_id"`
	Name            string     `json:"name"`
	TargetURL       string     `json:"target_url"`
	Status          string     `json:"status"`
	Subscriptions   []string   `json:"subscriptions"`
	SecretHint      string     `json:"secret_hint"`
	RawSecret       string     `json:"raw_secret,omitempty"`
	Version         int64      `json:"version"`
	CreatedByUserID string     `json:"created_by_user_id"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DisabledAt      *time.Time `json:"disabled_at,omitempty"`
}

func configToResult(cfg *domain.WebhookConfig, rawSecret string) ConfigResult {
	r := ConfigResult{
		ID:              cfg.ID,
		WorkspaceID:     cfg.WorkspaceID,
		Name:            cfg.Name,
		TargetURL:       cfg.TargetURL,
		Status:          string(cfg.Status),
		Subscriptions:   cfg.Subscriptions,
		SecretHint:      cfg.SecretHint,
		RawSecret:       rawSecret,
		Version:         cfg.Version,
		CreatedByUserID: cfg.CreatedByUserID,
		CreatedAt:       cfg.CreatedAt,
		UpdatedAt:       cfg.UpdatedAt,
		DisabledAt:      cfg.DisabledAt,
	}
	return r
}

func (s *Service) CreateWebhookConfig(ctx context.Context, input CreateConfigInput) (*ConfigResult, error) {
	log := s.log.With("usecase", "create_webhook_config", "workspace_id", input.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "webhook.manage"); err != nil {
		return nil, err
	}

	cfg := domain.WebhookConfig{
		Name:          input.Name,
		WorkspaceID:   input.WorkspaceID,
		TargetURL:     input.TargetURL,
		Subscriptions: input.Subscriptions,
		Status:        domain.ConfigStatusActive,
	}
	if err := domain.ValidateWebhookConfig(cfg); err != nil {
		log.Warn("invalid webhook config", "error", err)
		return nil, err
	}

	id, err := s.idGen()
	if err != nil {
		return nil, err
	}

	rawSecret, hash, hint, err := GenerateSecret()
	if err != nil {
		log.Error("failed to generate secret", "error", err)
		return nil, err
	}

	now := s.clock()
	cfg.ID = id
	cfg.SecretHash = hash
	cfg.SecretHint = hint
	cfg.Version = 1
	cfg.CreatedByUserID = input.UserID
	cfg.CreatedAt = now
	cfg.UpdatedAt = now

	if err := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if err := s.configWrite.Create(txCtx, cfg); err != nil {
			return err
		}
		if s.outboxWriter != nil {
			eventID, err := s.idGen()
			if err != nil {
				return err
			}
			payload, err := json.Marshal(webhookscontracts.ConfigCreatedPayload{
				ConfigID:    id,
				WorkspaceID: input.WorkspaceID,
				Name:        input.Name,
				TargetURL:   input.TargetURL,
				Status:      string(cfg.Status),
				Subscribed:  input.Subscriptions,
			})
			if err != nil {
				return err
			}
			if err := s.outboxWriter.Save(txCtx, ports.OutboxEvent{
				ID:            eventID,
				AggregateType: "webhook_config",
				AggregateID:   id,
				EventType:     webhookscontracts.EventConfigCreatedV1,
				Payload:       payload,
				WorkspaceID:   input.WorkspaceID,
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
	r := configToResult(&cfg, rawSecret)
	return &r, nil
}

type ListConfigsInput struct {
	WorkspaceID string
	UserID      string
}

func (s *Service) ListWebhookConfigs(ctx context.Context, input ListConfigsInput) ([]ConfigResult, error) {
	log := s.log.With("usecase", "list_webhook_configs", "workspace_id", input.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "webhook.manage"); err != nil {
		return nil, err
	}

	configs, err := s.configRead.ListByWorkspace(ctx, input.WorkspaceID)
	if err != nil {
		log.Error("failed to list webhook configs", "error", err)
		return nil, err
	}

	results := make([]ConfigResult, len(configs))
	for i, cfg := range configs {
		results[i] = configToResult(&cfg, "")
	}
	return results, nil
}

type UpdateConfigInput struct {
	WorkspaceID   string
	UserID        string
	WebhookID     string
	Name          *string
	TargetURL     *string
	Subscriptions []string
	Status        *string
	Now           time.Time
}

func (s *Service) UpdateWebhookConfig(ctx context.Context, input UpdateConfigInput) (*ConfigResult, error) {
	log := s.log.With("usecase", "update_webhook_config", "workspace_id", input.WorkspaceID, "webhook_id", input.WebhookID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "webhook.manage"); err != nil {
		return nil, err
	}

	cfg, err := s.configRead.FindByID(ctx, input.WorkspaceID, input.WebhookID)
	if err != nil {
		if errors.Is(err, domain.ErrConfigNotFound) {
			return nil, err
		}
		log.Error("failed to find webhook config", "error", err)
		return nil, err
	}

	if input.Name != nil {
		cfg.Name = *input.Name
	}
	if input.TargetURL != nil {
		cfg.TargetURL = *input.TargetURL
	}
	if input.Subscriptions != nil {
		cfg.Subscriptions = input.Subscriptions
	}
	if input.Status != nil {
		cfg.Status = domain.ConfigStatus(*input.Status)
	}

	if err := domain.ValidateWebhookConfig(*cfg); err != nil {
		log.Warn("invalid webhook config update", "error", err)
		return nil, err
	}

	cfg.Version++
	cfg.UpdatedAt = input.Now

	if err := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if err := s.configWrite.Update(txCtx, *cfg); err != nil {
			return err
		}
		if s.outboxWriter != nil {
			eventID, err := s.idGen()
			if err != nil {
				return err
			}
			payload, err := json.Marshal(webhookscontracts.ConfigUpdatedPayload{
				ConfigID:    cfg.ID,
				WorkspaceID: cfg.WorkspaceID,
				Version:     cfg.Version,
				Status:      string(cfg.Status),
				Subscribed:  cfg.Subscriptions,
			})
			if err != nil {
				return err
			}
			if err := s.outboxWriter.Save(txCtx, ports.OutboxEvent{
				ID:            eventID,
				AggregateType: "webhook_config",
				AggregateID:   cfg.ID,
				EventType:     webhookscontracts.EventConfigUpdatedV1,
				Payload:       payload,
				WorkspaceID:   cfg.WorkspaceID,
				OccurredAt:    input.Now,
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		if errors.Is(err, domain.ErrConfigNameConflict) {
			return nil, err
		}
		if errors.Is(err, domain.ErrRotateConflict) {
			return nil, domain.ErrRotateConflict
		}
		log.Error("failed to update webhook config", "error", err)
		return nil, err
	}

	log.Info("webhook config updated", "version", cfg.Version)
	r := configToResult(cfg, "")
	return &r, nil
}

type DisableConfigInput struct {
	WorkspaceID string
	UserID      string
	WebhookID   string
}

func (s *Service) DisableWebhookConfig(ctx context.Context, input DisableConfigInput) error {
	log := s.log.With("usecase", "disable_webhook_config", "workspace_id", input.WorkspaceID, "webhook_id", input.WebhookID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "webhook.manage"); err != nil {
		return err
	}

	now := s.clock()
	if err := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if err := s.configWrite.Disable(txCtx, input.WorkspaceID, input.WebhookID, now); err != nil {
			return err
		}
		if s.outboxWriter != nil {
			eventID, err := s.idGen()
			if err != nil {
				return err
			}
			payload, err := json.Marshal(webhookscontracts.ConfigDisabledPayload{
				ConfigID:    input.WebhookID,
				WorkspaceID: input.WorkspaceID,
				DisabledAt:  now.UTC().Format(time.RFC3339),
			})
			if err != nil {
				return err
			}
			if err := s.outboxWriter.Save(txCtx, ports.OutboxEvent{
				ID:            eventID,
				AggregateType: "webhook_config",
				AggregateID:   input.WebhookID,
				EventType:     webhookscontracts.EventConfigDisabledV1,
				Payload:       payload,
				WorkspaceID:   input.WorkspaceID,
				OccurredAt:    now,
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		if errors.Is(err, domain.ErrConfigNotFound) {
			return err
		}
		log.Error("failed to disable webhook config", "error", err)
		return err
	}

	log.Info("webhook config disabled")
	return nil
}

type RotateSecretInput struct {
	WorkspaceID string
	UserID      string
	WebhookID   string
}

type RotateSecretResult struct {
	RawSecret string
	Hint      string
	Version   int64
}

func (s *Service) RotateWebhookSecret(ctx context.Context, input RotateSecretInput) (*RotateSecretResult, error) {
	log := s.log.With("usecase", "rotate_webhook_secret", "workspace_id", input.WorkspaceID, "webhook_id", input.WebhookID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "webhook.manage"); err != nil {
		return nil, err
	}

	cfg, err := s.configRead.FindByID(ctx, input.WorkspaceID, input.WebhookID)
	if err != nil {
		if errors.Is(err, domain.ErrConfigNotFound) {
			return nil, err
		}
		log.Error("failed to find webhook config for rotate", "error", err)
		return nil, err
	}

	rawSecret, hash, hint, err := GenerateSecret()
	if err != nil {
		log.Error("failed to generate secret", "error", err)
		return nil, err
	}

	cfg.SecretHash = hash
	cfg.SecretHint = hint
	cfg.Version++
	cfg.UpdatedAt = s.clock()

	if err := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if err := s.configWrite.Update(txCtx, *cfg); err != nil {
			return err
		}
		if s.outboxWriter != nil {
			eventID, err := s.idGen()
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
			if err := s.outboxWriter.Save(txCtx, ports.OutboxEvent{
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
	return &RotateSecretResult{RawSecret: rawSecret, Hint: hint, Version: cfg.Version}, nil
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
