package domain

import (
	"net/mail"
	"strings"
)

type EmailAddress string

func NewEmailAddress(raw string) (EmailAddress, error) {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return "", ErrInvalidEmail
	}
	addr, err := mail.ParseAddress(cleaned)
	if err != nil {
		return "", ErrInvalidEmail
	}
	normalized := strings.ToLower(addr.Address)
	return EmailAddress(normalized), nil
}

func (e EmailAddress) String() string { return string(e) }
