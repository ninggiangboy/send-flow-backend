package domain

import (
	"errors"
	"strings"
	"time"
)

type Workspace struct {
	ID           string
	Name         string
	Plan         *string
	LogoIcon     *string
	MembershipID string
	RoleNames    []string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func NewWorkspace(id, name string, now time.Time) (Workspace, error) {
	if id == "" {
		return Workspace{}, errors.New("workspace id is required")
	}
	if strings.TrimSpace(name) == "" {
		return Workspace{}, errors.New("workspace name is required")
	}
	return Workspace{
		ID:        id,
		Name:      strings.TrimSpace(name),
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}
