package operations

const Name = "operations"

const Purpose = "Own operational visibility and workflows for outbox, DLQ, replay, rate limit, circuit breaker, and runtime health state."

var OwnedData = []string{
	"outbox_records",
	"processed_event_markers",
	"dead_letter_records",
	"replay_jobs",
	"rate_limit_states",
	"circuit_breaker_states",
}

var Commands = []string{
	"CreateReplayJob",
	"MoveDLQRecordForReplay",
	"MarkOutboxRecordStuck",
	"OpenCircuitBreaker",
	"CloseCircuitBreaker",
}

var Queries = []string{
	"GetOutboxLag",
	"ListDLQRecords",
	"GetReplayJobStatus",
	"GetRuntimeHealth",
	"GetQueueLag",
}

var Events = []string{
	"operations.dlq_record.created.v1",
	"operations.replay_job.created.v1",
	"operations.replay_job.completed.v1",
	"operations.circuit_breaker.opened.v1",
}
