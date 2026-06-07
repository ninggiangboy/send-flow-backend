package domain

import (
	"strings"
	"unicode"
)

func NormalizeEmail(email string) string {
	trimmed := strings.TrimSpace(email)
	return strings.ToLower(trimmed)
}

func ValidContactStatus(s string) bool {
	switch ContactStatus(s) {
	case ContactStatusActive, ContactStatusArchived:
		return true
	default:
		return false
	}
}

func ValidSegmentStatus(s string) bool {
	switch SegmentStatus(s) {
	case SegmentStatusReady, SegmentStatusProcessing, SegmentStatusDisabled:
		return true
	default:
		return false
	}
}

func ValidJobStatus(s string) bool {
	switch JobStatus(s) {
	case JobStatusQueued, JobStatusRunning, JobStatusCompleted, JobStatusFailed:
		return true
	default:
		return false
	}
}

func ValidDedupeMode(s string) bool {
	switch DedupeMode(s) {
	case DedupeModeByEmail:
		return true
	default:
		return false
	}
}

func ValidExportFormat(s string) bool {
	switch ExportFormat(s) {
	case ExportFormatCSV:
		return true
	default:
		return false
	}
}

func ValidListMembershipMode(s string) bool {
	switch ListMembershipMode(s) {
	case ListMembershipModeMerge, ListMembershipModeReplace:
		return true
	default:
		return false
	}
}

func IsPrintable(s string) bool {
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}
