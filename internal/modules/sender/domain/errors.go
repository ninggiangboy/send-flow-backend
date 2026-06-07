package domain

import "errors"

var (
	ErrDomainInvalid          = errors.New("domain is invalid")
	ErrProviderConfigInvalid  = errors.New("provider configuration is invalid")
	ErrDomainNotFound         = errors.New("sender domain not found")
	ErrDomainConflict         = errors.New("sender domain already exists")
	ErrManageDenied           = errors.New("sender manage denied")
	ErrInvalidStateTransition = errors.New("invalid state transition")
)
