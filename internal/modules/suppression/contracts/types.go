package contracts

import "time"

type SuppressFromSignalInput struct {
	WorkspaceID     string
	Email           string
	EmailNormalized string
	Scope           string
	Reason          string
	Source          string
	SourceEventID   string
	Note            string
	Now             time.Time
}

type SuppressFromSignalResult struct {
	EntryID string
	Created bool
}
