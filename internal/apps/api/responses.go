package api

import (
	"time"

	audienceapp "github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app"
)

type AuthSessionUser struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	MFAEnabled    bool   `json:"mfa_enabled"`
}

type AuthSessionSession struct {
	ID         string    `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	IPAddress  string    `json:"ip_address"`
	AuthMethod string    `json:"auth_method"`
}

type AuthSessionResponse struct {
	User        AuthSessionUser    `json:"user"`
	Session     AuthSessionSession `json:"session"`
	AccessToken string             `json:"access_token"`
	TokenType   string             `json:"token_type"`
	ExpiresAt   time.Time          `json:"expires_at"`
}

type MFARequiredResponse struct {
	MFARequired       bool   `json:"mfa_required"`
	MFAChallengeToken string `json:"mfa_challenge_token"`
	User              struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	} `json:"user"`
}

type UserMeResponse struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	MFAEnabled    bool   `json:"mfa_enabled"`
}

type SessionResponse struct {
	ID         string    `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	IPAddress  string    `json:"ip_address"`
	AuthMethod string    `json:"auth_method"`
}

type WorkspaceAccessResponse struct {
	WorkspaceID          string   `json:"workspace_id"`
	MembershipID         string   `json:"membership_id"`
	Status               string   `json:"status"`
	RoleIDs              []string `json:"role_ids"`
	RoleNames            []string `json:"role_names"`
	EffectivePermissions []string `json:"effective_permissions"`
}

type PermissionResponse struct {
	Bit  int    `json:"bit"`
	Name string `json:"name"`
}

type statusResponseDoc struct {
	Revoked  *bool  `json:"revoked,omitempty"`
	Sent     *bool  `json:"sent,omitempty"`
	Verified *bool  `json:"verified,omitempty"`
	Reset    *bool  `json:"reset,omitempty"`
	Disabled *bool  `json:"disabled,omitempty"`
	Updated  *bool  `json:"updated,omitempty"`
	Status   string `json:"status,omitempty"`
}

func ptrBool(v bool) *bool { return &v }

type audienceContactDoc struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspace_id"`
	Email       string         `json:"email"`
	Status      string         `json:"status"`
	FirstName   string         `json:"first_name"`
	LastName    string         `json:"last_name"`
	Tags        []string       `json:"tags"`
	Attributes  map[string]any `json:"attributes"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type audienceListDoc struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspace_id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	ContactCount int64     `json:"contact_count,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type audienceSegmentDoc struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspace_id"`
	Name        string         `json:"name"`
	Definition  map[string]any `json:"definition"`
	Status      string         `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type audienceImportJobDoc struct {
	ID             string     `json:"id"`
	WorkspaceID    string     `json:"workspace_id"`
	SourceURI      string     `json:"source_uri"`
	DedupeMode     string     `json:"dedupe_mode"`
	Status         string     `json:"status"`
	ProcessedCount int64      `json:"processed_count"`
	CreatedCount   int64      `json:"created_count"`
	UpdatedCount   int64      `json:"updated_count"`
	FailedCount    int64      `json:"failed_count"`
	ErrorSummary   string     `json:"error_summary"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	CompletedAt    *time.Time `json:"completed_at"`
}

type audienceExportJobDoc struct {
	ID                   string     `json:"id"`
	WorkspaceID          string     `json:"workspace_id"`
	Format               string     `json:"format"`
	ZipOutput            bool       `json:"zip_output"`
	Status               string     `json:"status"`
	ProcessedCount       int64      `json:"processed_count"`
	EstimatedTotalCount  int64      `json:"estimated_total_count"`
	ArtifactURI          string     `json:"artifact_uri"`
	DownloadURL          string     `json:"download_url,omitempty"`
	DownloadURLExpiresAt *time.Time `json:"download_url_expires_at,omitempty"`
	ErrorSummary         string     `json:"error_summary"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	CompletedAt          *time.Time `json:"completed_at"`
}

type listMembershipUpdateDoc struct {
	AddedCount   int `json:"added_count"`
	RemovedCount int `json:"removed_count"`
	SkippedCount int `json:"skipped_count"`
}

func newContactResponse(c audienceapp.ContactDTO) audienceContactDoc {
	return audienceContactDoc{
		ID:          c.ID,
		WorkspaceID: c.WorkspaceID,
		Email:       c.Email,
		Status:      c.Status,
		FirstName:   c.FirstName,
		LastName:    c.LastName,
		Tags:        c.Tags,
		Attributes:  c.Attributes,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

func newListResponse(l audienceapp.AudienceListDTO) audienceListDoc {
	return audienceListDoc{
		ID:          l.ID,
		WorkspaceID: l.WorkspaceID,
		Name:        l.Name,
		Description: l.Description,
		CreatedAt:   l.CreatedAt,
		UpdatedAt:   l.UpdatedAt,
	}
}

func newSegmentResponse(s audienceapp.SegmentDTO) audienceSegmentDoc {
	return audienceSegmentDoc{
		ID:          s.ID,
		WorkspaceID: s.WorkspaceID,
		Name:        s.Name,
		Definition:  s.DefinitionJSON,
		Status:      s.Status,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

func newImportJobResponse(j audienceapp.ImportJobDTO) audienceImportJobDoc {
	return audienceImportJobDoc{
		ID:             j.ID,
		WorkspaceID:    j.WorkspaceID,
		SourceURI:      j.SourceURI,
		DedupeMode:     j.DedupeMode,
		Status:         j.Status,
		ProcessedCount: j.ProcessedCount,
		CreatedCount:   j.CreatedCount,
		UpdatedCount:   j.UpdatedCount,
		FailedCount:    j.FailedCount,
		ErrorSummary:   j.ErrorSummary,
		CreatedAt:      j.CreatedAt,
		UpdatedAt:      j.UpdatedAt,
		CompletedAt:    j.CompletedAt,
	}
}

func newExportJobResponse(j audienceapp.ExportJobDTO) audienceExportJobDoc {
	return audienceExportJobDoc{
		ID:                   j.ID,
		WorkspaceID:          j.WorkspaceID,
		Format:               j.Format,
		ZipOutput:            j.ZipOutput,
		Status:               j.Status,
		ProcessedCount:       j.ProcessedCount,
		EstimatedTotalCount:  j.EstimatedTotalCount,
		ArtifactURI:          j.ArtifactURI,
		DownloadURL:          j.DownloadURL,
		DownloadURLExpiresAt: j.DownloadURLTTL,
		ErrorSummary:         j.ErrorSummary,
		CreatedAt:            j.CreatedAt,
		UpdatedAt:            j.UpdatedAt,
		CompletedAt:          j.CompletedAt,
	}
}
