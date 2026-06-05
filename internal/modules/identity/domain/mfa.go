package domain

import "time"

type TOTPSecret struct {
	UserID    string
	Secret    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type RecoveryCode struct {
	ID         string
	UserID     string
	CodeHash   string
	ConsumedAt *time.Time
	CreatedAt  time.Time
}
