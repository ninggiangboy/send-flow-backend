package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

var DefaultSettingsJSON = domain.DefaultSettingsJSON

func (s *Service) GetWorkspaceSettings(ctx context.Context, workspaceID, userID string) (*domain.WorkspaceSettings, error) {
	if _, err := s.requireWorkspacePermission(ctx, workspaceID, userID, domain.PermissionWorkspaceRead); err != nil {
		if errors.Is(err, domain.ErrWorkspaceAccessDenied) || errors.Is(err, domain.ErrMembershipNotFound) {
			return nil, domain.ErrSettingsManageDenied
		}
		return nil, err
	}

	settings, err := s.settingsRead.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, domain.ErrSettingsNotFound) {
			defaultSettings := domain.WorkspaceSettings{
				WorkspaceID:     workspaceID,
				SettingsJSON:    DefaultSettingsJSON,
				Version:         1,
				CreatedAt:       time.Now().UTC(),
				UpdatedAt:       time.Now().UTC(),
				FeatureControls: make(domain.FeatureControls),
			}
			if em, ok := DefaultSettingsJSON["email_defaults"].(map[string]any); ok {
				if sid, ok := em["default_sender_domain_id"].(string); ok {
					defaultSettings.EmailDefaults.DefaultSenderDomainID = sid
				}
			}
			return &defaultSettings, nil
		}
		return nil, err
	}
	return settings, nil
}

func (s *Service) UpdateWorkspaceSettings(ctx context.Context, workspaceID, userID, requestID string, patch domain.WorkspaceSettingsPatch, now time.Time) (*domain.WorkspaceSettings, error) {
	if _, err := s.requireWorkspacePermission(ctx, workspaceID, userID, domain.PermissionWorkspaceManageSetting); err != nil {
		if errors.Is(err, domain.ErrWorkspaceAccessDenied) || errors.Is(err, domain.ErrMembershipNotFound) {
			return nil, domain.ErrSettingsManageDenied
		}
		if errors.Is(err, domain.ErrRoleManageDenied) || errors.Is(err, domain.ErrMembershipManageDenied) {
			return nil, domain.ErrSettingsManageDenied
		}
		return nil, err
	}

	if patch.EmailDefaults == nil && patch.FeatureControls == nil {
		return nil, fmt.Errorf("%w: patch payload cannot be empty", domain.ErrSettingsPayloadInvalid)
	}
	if patch.Version != nil && *patch.Version < 1 {
		return nil, fmt.Errorf("%w: version must be positive", domain.ErrSettingsPayloadInvalid)
	}

	current, err := s.settingsRead.GetByWorkspace(ctx, workspaceID)
	isNew := false
	if err != nil {
		if errors.Is(err, domain.ErrSettingsNotFound) {
			current = &domain.WorkspaceSettings{
				WorkspaceID:     workspaceID,
				SettingsJSON:    DefaultSettingsJSON,
				Version:         1,
				CreatedAt:       now,
				UpdatedAt:       now,
				FeatureControls: make(domain.FeatureControls),
			}
			isNew = true
		} else {
			return nil, err
		}
	}

	if patch.EmailDefaults != nil {
		current.EmailDefaults.DefaultSenderDomainID = patch.EmailDefaults.DefaultSenderDomainID
	}
	if patch.FeatureControls != nil {
		current.FeatureControls = *patch.FeatureControls
	}

	current.UpdatedByUserID = userID
	current.UpdatedAt = now

	var expectedVersion *int64
	if !isNew {
		expectedVersion = patch.Version
		if expectedVersion == nil {
			expectedVersion = &current.Version
		}
	}

	if err := s.settingsWrite.Upsert(ctx, *current, expectedVersion); err != nil {
		if errors.Is(err, domain.ErrSettingsVersionConflict) {
			return nil, err
		}
		s.logger.Error("failed to upsert settings", "error", err, "workspace_id", workspaceID)
		return nil, err
	}

	if s.auditRecorder != nil {
		auditErr := s.auditRecorder.Record(ctx, RecordAuditInput{
			WorkspaceID: workspaceID,
			ActorUserID: userID,
			ActionType:  "workspace.settings.updated",
			TargetType:  "workspace_settings",
			TargetID:    workspaceID,
			PayloadSummary: map[string]any{
				"updated_fields": patchFields(patch),
			},
			RequestID:  requestID,
			OccurredAt: now,
		})
		if auditErr != nil {
			s.logger.Error("failed to record audit entry for settings update", "error", auditErr, "workspace_id", workspaceID)
			return nil, auditErr
		}
	}

	if !isNew {
		current.Version++
	}
	return current, nil
}

func (s *Service) SetAuditRecorder(recorder AuditRecorder) {
	s.auditRecorder = recorder
}

func (s *Service) RequireWorkspacePermission(ctx context.Context, workspaceID, userID, permission string) error {
	_, err := s.requireWorkspacePermission(ctx, workspaceID, userID, permission)
	return err
}

func patchFields(patch domain.WorkspaceSettingsPatch) []string {
	var fields []string
	if patch.EmailDefaults != nil {
		fields = append(fields, "email_defaults")
	}
	if patch.FeatureControls != nil {
		fields = append(fields, "feature_controls")
	}
	return fields
}
