package contracts

const (
	EventAPIKeyCreatedV1 = "access.api_key.created.v1"
	EventAPIKeyRevokedV1 = "access.api_key.revoked.v1"
)

type APIKeyCreatedPayload struct {
	APIKeyID    string `json:"api_key_id"`
	WorkspaceID string `json:"workspace_id"`
	UserID      string `json:"user_id"`
	Name        string `json:"name"`
	Scopes      string `json:"scopes"`
	KeyPrefix   string `json:"key_prefix"`
	CreatedAt   string `json:"created_at"`
}

type APIKeyRevokedPayload struct {
	APIKeyID    string `json:"api_key_id"`
	WorkspaceID string `json:"workspace_id"`
	RevokedBy   string `json:"revoked_by"`
	RevokedAt   string `json:"revoked_at"`
}
