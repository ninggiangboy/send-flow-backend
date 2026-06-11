package contracts

const (
	EventDomainCreatedV1  = "sender.domain.created.v1"
	EventDomainVerifiedV1 = "sender.domain.verified.v1"
	EventDomainDisabledV1 = "sender.domain.disabled.v1"
)

type DomainCreatedPayload struct {
	DomainID    string `json:"domain_id"`
	WorkspaceID string `json:"workspace_id"`
	Domain      string `json:"domain"`
	CreatedBy   string `json:"created_by"`
	CreatedAt   string `json:"created_at"`
}

type DomainVerifiedPayload struct {
	DomainID    string `json:"domain_id"`
	WorkspaceID string `json:"workspace_id"`
	Domain      string `json:"domain"`
	VerifiedAt  string `json:"verified_at"`
}

type DomainDisabledPayload struct {
	DomainID    string `json:"domain_id"`
	WorkspaceID string `json:"workspace_id"`
	Domain      string `json:"domain"`
	DisabledAt  string `json:"disabled_at"`
}
