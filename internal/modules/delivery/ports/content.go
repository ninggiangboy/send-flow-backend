package ports

import "context"

type RenderedMessage struct {
	Subject  string
	HTMLBody string
	TextBody string
}

type ContentRenderer interface {
	RenderForMessage(ctx context.Context, workspaceID, templateID, templateVersionID string, data map[string]any) (*RenderedMessage, error)
}
