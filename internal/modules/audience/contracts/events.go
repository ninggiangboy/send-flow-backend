package contracts

const (
	EventImportCompletedV1 = "audience.import.completed.v1"
	EventExportCompletedV1 = "audience.export.completed.v1"
	EventImportFailedV1    = "audience.import.failed.v1"
	EventExportFailedV1    = "audience.export.failed.v1"
)

type ImportCompletedPayload struct {
	JobID        string `json:"job_id"`
	WorkspaceID  string `json:"workspace_id"`
	SourceURI    string `json:"source_uri"`
	TotalRows    int64  `json:"total_rows"`
	CreatedCount int64  `json:"created_count"`
	UpdatedCount int64  `json:"updated_count"`
	FailedCount  int64  `json:"failed_count"`
}

type ExportCompletedPayload struct {
	JobID       string `json:"job_id"`
	WorkspaceID string `json:"workspace_id"`
	Format      string `json:"format"`
	ArtifactURI string `json:"artifact_uri"`
	TotalRows   int64  `json:"total_rows"`
}

type ImportFailedPayload struct {
	JobID        string `json:"job_id"`
	WorkspaceID  string `json:"workspace_id"`
	ErrorMessage string `json:"error_message"`
}

type ExportFailedPayload struct {
	JobID        string `json:"job_id"`
	WorkspaceID  string `json:"workspace_id"`
	ErrorMessage string `json:"error_message"`
}
