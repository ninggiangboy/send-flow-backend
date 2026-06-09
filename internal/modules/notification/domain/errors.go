package domain

import "errors"

var (
	ErrNotificationNotFound      = errors.New("notification message not found")
	ErrNotificationReadDenied    = errors.New("notification read access denied")
	ErrNotificationTypeInvalid   = errors.New("notification type is invalid")
	ErrNotificationStatusInvalid = errors.New("notification status is invalid")
	ErrNotificationAlreadySent   = errors.New("notification message is already in a terminal state")
	ErrRecipientEmailInvalid     = errors.New("recipient email is invalid")
	ErrSubjectInvalid            = errors.New("subject is invalid")
	ErrMaxAttemptsExceeded       = errors.New("max attempts exceeded")
)
