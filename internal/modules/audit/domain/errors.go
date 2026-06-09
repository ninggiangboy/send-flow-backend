package domain

import "errors"

var (
	ErrAuditReadDenied    = errors.New("audit read denied")
	ErrAuditEntryInvalid  = errors.New("audit entry invalid")
	ErrAuditFilterInvalid = errors.New("audit filter invalid")
)
