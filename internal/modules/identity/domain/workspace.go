package domain

import "time"

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

func NewWorkspace(id, name string, now time.Time) Workspace {
	return Workspace{
		ID:        id,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
