package domain

import "errors"

var (
	ErrReadDenied         = errors.New("suppression read denied")
	ErrManageDenied       = errors.New("suppression manage denied")
	ErrEntryNotFound      = errors.New("suppression entry not found")
	ErrUnsuppressConflict = errors.New("suppression unsuppress conflict")
	ErrScopeInvalid       = errors.New("suppression scope invalid")
	ErrReasonInvalid      = errors.New("suppression reason invalid")
	ErrEmailInvalid       = errors.New("suppression email invalid")
)
