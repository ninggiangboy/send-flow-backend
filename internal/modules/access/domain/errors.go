package domain

import "errors"

var (
	ErrAPIKeyNotFound       = errors.New("api key not found")
	ErrAPIKeyInvalid        = errors.New("api key is invalid")
	ErrAPIKeyManageDenied   = errors.New("api key management permission denied")
	ErrAPIKeyScopeDenied    = errors.New("api key does not have required scope")
	ErrAPIKeyScopeInvalid   = errors.New("api key scope is invalid")
	ErrAPIKeyConfigInvalid  = errors.New("api key configuration is invalid")
	ErrAPIKeyRotateConflict = errors.New("api key cannot be rotated")
	ErrEmailQuotaInvalid    = errors.New("api key email quota limits are invalid")
	ErrPayloadInvalid       = errors.New("request payload is invalid")
)
