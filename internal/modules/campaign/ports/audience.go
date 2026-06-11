package ports

import "context"

type AudienceSelectionRef struct {
	ListID     string
	SegmentID  string
	ContactIDs []string
}

type Recipient struct {
	ContactID       string
	Email           string
	EmailNormalized string
	FirstName       string
	LastName        string
	Tags            []string
	Attributes      map[string]any
}

type AudienceResolver interface {
	ResolveAudienceRecipients(ctx context.Context, workspaceID, userID string, ref AudienceSelectionRef) ([]Recipient, error)
	EstimateAudienceSize(ctx context.Context, workspaceID, userID string, ref AudienceSelectionRef) (int, error)
}
