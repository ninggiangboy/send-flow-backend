package operations

const Name = "operations"

const Purpose = "Own operational visibility and workflows for outbox, DLQ, replay, and runtime health state."

var OwnedData = []string{
	"outbox_records",
	"processed_event_markers",
	"dead_letter_records",
	"replay_jobs",
}

var Commands = []string{
	"CreateReplayJob",
	"MoveDLQRecordForReplay",
	"MarkOutboxRecordStuck",
}

var Queries = []string{
	"GetOutboxLag",
	"ListDLQRecords",
	"GetReplayJobStatus",
	"GetRuntimeHealth",
}

var Events = []string{
	"operations.dlq_record.created.v1",
	"operations.replay_job.created.v1",
	"operations.replay_job.completed.v1",
}
