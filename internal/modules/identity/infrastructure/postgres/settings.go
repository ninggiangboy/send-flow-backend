package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	platformpostgres "github.com/ninggiangboy/send-flow/backend/internal/platform/postgres"
)

type SettingsReadRepository struct {
	db platformpostgres.DBTX
}

func NewSettingsReadRepository(db platformpostgres.DBTX) *SettingsReadRepository {
	return &SettingsReadRepository{db: db}
}

func (r *SettingsReadRepository) GetByWorkspace(ctx context.Context, workspaceID string) (*domain.WorkspaceSettings, error) {
	var settingsJSON []byte
	var version int64
	var updatedByUserID *string
	var createdAt, updatedAt time.Time

	err := r.getDB(ctx).QueryRow(ctx, `
		SELECT settings_json, version, updated_by_user_id, created_at, updated_at
		FROM workspace_settings
		WHERE workspace_id = $1
	`, workspaceID).Scan(&settingsJSON, &version, &updatedByUserID, &createdAt, &updatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrSettingsNotFound
		}
		return nil, fmt.Errorf("get workspace settings: %w", err)
	}

	var data map[string]any
	if len(settingsJSON) > 0 {
		if err := json.Unmarshal(settingsJSON, &data); err != nil {
			return nil, fmt.Errorf("unmarshal settings json: %w", err)
		}
	}

	return mapToSettings(workspaceID, data, version, updatedByUserID, createdAt, updatedAt), nil
}

func (w *SettingsWriteRepository) GetByWorkspace(ctx context.Context, workspaceID string) (*domain.WorkspaceSettings, error) {
	var settingsJSON []byte
	var version int64
	var updatedByUserID *string
	var createdAt, updatedAt time.Time

	err := w.getDB(ctx).QueryRow(ctx, `
		SELECT settings_json, version, updated_by_user_id, created_at, updated_at
		FROM workspace_settings
		WHERE workspace_id = $1
	`, workspaceID).Scan(&settingsJSON, &version, &updatedByUserID, &createdAt, &updatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrSettingsNotFound
		}
		return nil, fmt.Errorf("get workspace settings: %w", err)
	}

	var data map[string]any
	if len(settingsJSON) > 0 {
		if err := json.Unmarshal(settingsJSON, &data); err != nil {
			return nil, fmt.Errorf("unmarshal settings json: %w", err)
		}
	}

	return mapToSettings(workspaceID, data, version, updatedByUserID, createdAt, updatedAt), nil
}

type SettingsWriteRepository struct {
	db platformpostgres.DBTX
}

func NewSettingsWriteRepository(db platformpostgres.DBTX) *SettingsWriteRepository {
	return &SettingsWriteRepository{db: db}
}

func (r *SettingsWriteRepository) CreateDefault(ctx context.Context, settings domain.WorkspaceSettings) error {
	settingsJSON, err := marshalSettingsData(settings)
	if err != nil {
		return err
	}
	_, err = r.getDB(ctx).Exec(ctx, `
		INSERT INTO workspace_settings (workspace_id, settings_json, version, updated_by_user_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (workspace_id) DO NOTHING
	`, settings.WorkspaceID, settingsJSON, settings.Version, platformpostgres.Nullable(settings.UpdatedByUserID), settings.CreatedAt, settings.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create default settings: %w", err)
	}
	return nil
}

func (r *SettingsWriteRepository) Upsert(ctx context.Context, settings domain.WorkspaceSettings, expectedVersion *int64) error {
	settingsJSON, err := marshalSettingsData(settings)
	if err != nil {
		return err
	}

	if expectedVersion != nil {
		result, err := r.getDB(ctx).Exec(ctx, `
			UPDATE workspace_settings
			SET settings_json = $1, version = version + 1, updated_by_user_id = $2, updated_at = $3
			WHERE workspace_id = $4 AND version = $5
		`, settingsJSON, platformpostgres.Nullable(settings.UpdatedByUserID), settings.UpdatedAt, settings.WorkspaceID, *expectedVersion)
		if err != nil {
			return fmt.Errorf("upsert settings with version check: %w", err)
		}
		if result.RowsAffected() == 0 {
			return domain.ErrSettingsVersionConflict
		}
	} else {
		var version int64
		err = r.getDB(ctx).QueryRow(ctx, `
			INSERT INTO workspace_settings (workspace_id, settings_json, version, updated_by_user_id, created_at, updated_at)
			VALUES ($1, $2, 1, $3, $4, $5)
			ON CONFLICT (workspace_id) DO NOTHING
			RETURNING version
		`, settings.WorkspaceID, settingsJSON, platformpostgres.Nullable(settings.UpdatedByUserID), settings.CreatedAt, settings.UpdatedAt).Scan(&version)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrSettingsVersionConflict
			}
			return fmt.Errorf("upsert settings: %w", err)
		}
	}
	return nil
}

func marshalSettingsData(settings domain.WorkspaceSettings) ([]byte, error) {
	data := make(map[string]any)
	if settings.EmailDefaults.DefaultSenderDomainID != "" {
		data["email_defaults"] = map[string]any{
			"default_sender_domain_id": settings.EmailDefaults.DefaultSenderDomainID,
		}
	}
	if len(settings.FeatureControls) > 0 {
		data["feature_controls"] = settings.FeatureControls
	}
	for k, v := range settings.SettingsJSON {
		if k != "email_defaults" && k != "feature_controls" {
			data[k] = v
		}
	}
	return json.Marshal(data)
}

func mapToSettings(workspaceID string, data map[string]any, version int64, updatedByUserID *string, createdAt, updatedAt time.Time) *domain.WorkspaceSettings {
	settings := &domain.WorkspaceSettings{
		WorkspaceID:     workspaceID,
		Version:         version,
		SettingsJSON:    data,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
		FeatureControls: make(domain.FeatureControls),
	}
	if updatedByUserID != nil {
		settings.UpdatedByUserID = *updatedByUserID
	}

	if data != nil {
		if ed, ok := data["email_defaults"]; ok {
			if edMap, ok := ed.(map[string]any); ok {
				if sid, ok := edMap["default_sender_domain_id"].(string); ok {
					settings.EmailDefaults.DefaultSenderDomainID = sid
				}
			}
		}
		if fc, ok := data["feature_controls"]; ok {
			if fcMap, ok := fc.(map[string]any); ok {
				settings.FeatureControls = fcMap
			}
		}
	}

	return settings
}
