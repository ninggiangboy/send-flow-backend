package contracts

import "errors"

var (
	ErrAPIKeyManageDenied = errors.New("access api key manage denied")
	ErrAPIKeyInvalid      = errors.New("access api key invalid")
	ErrAPIKeyScopeDenied  = errors.New("access api key scope denied")
	ErrAPIKeyNotFound     = errors.New("access api key not found")
)
