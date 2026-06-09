package domain

import "time"

type EmailDefaults struct {
	DefaultSenderDomainID string `json:"default_sender_domain_id,omitempty"`
}

type FeatureControls map[string]any

type WorkspaceSettings struct {
	WorkspaceID     string          `json:"workspace_id"`
	SettingsJSON    map[string]any  `json:"settings_json"`
	EmailDefaults   EmailDefaults   `json:"email_defaults"`
	FeatureControls FeatureControls `json:"feature_controls"`
	Version         int64           `json:"version"`
	UpdatedByUserID string          `json:"updated_by_user_id,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type WorkspaceSettingsPatch struct {
	EmailDefaults   *EmailDefaults   `json:"email_defaults,omitempty"`
	FeatureControls *FeatureControls `json:"feature_controls,omitempty"`
	Version         *int64           `json:"version,omitempty"`
}
