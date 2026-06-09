package domain

import "errors"

var (
	ErrSettingsNotFound        = errors.New("settings not found")
	ErrSettingsPayloadInvalid  = errors.New("settings payload invalid")
	ErrSettingsVersionConflict = errors.New("settings version conflict")
	ErrSettingsManageDenied    = errors.New("settings manage denied")
)
