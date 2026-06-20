package registration

// MetricsRecorder is used by sender domain registration handlers to record
// verification attempts and track pending domain counts.
type MetricsRecorder interface {
	RecordVerificationAttempt(result string)
	SetPendingDomains(count int64)
}
