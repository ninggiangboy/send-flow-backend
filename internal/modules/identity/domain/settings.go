package domain

import "time"

type EmailDefaults struct {
	DefaultSenderDomainID string
}

type FeatureControls map[string]any

type WorkspaceSettings struct {
	WorkspaceID     string
	SettingsJSON    map[string]any
	EmailDefaults   EmailDefaults
	FeatureControls FeatureControls
	Version         int64
	UpdatedByUserID string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type WorkspaceSettingsPatch struct {
	EmailDefaults   *EmailDefaults
	FeatureControls *FeatureControls
	Version         *int64
}
