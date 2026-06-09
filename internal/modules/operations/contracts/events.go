package contracts

const (
	EventReplayJobCreatedV1   = "operations.replay_job.created.v1"
	EventReplayJobCompletedV1 = "operations.replay_job.completed.v1"
	EventReplayJobFailedV1    = "operations.replay_job.failed.v1"
)

type ReplayJobCreatedPayload struct {
	JobID       string `json:"job_id"`
	WorkspaceID string `json:"workspace_id"`
	TargetType  string `json:"target_type"`
	TargetID    string `json:"target_id"`
	Reason      string `json:"reason,omitempty"`
	RequestedBy string `json:"requested_by,omitempty"`
}

type ReplayJobCompletedPayload struct {
	JobID       string `json:"job_id"`
	WorkspaceID string `json:"workspace_id"`
	TargetType  string `json:"target_type"`
	TargetID    string `json:"target_id"`
	ResultCount int    `json:"result_count"`
}

type ReplayJobFailedPayload struct {
	JobID       string `json:"job_id"`
	WorkspaceID string `json:"workspace_id"`
	TargetType  string `json:"target_type"`
	TargetID    string `json:"target_id"`
	Error       string `json:"error"`
}
