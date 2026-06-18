package domain

// EmailQuotaLimits defines optional per-window email sending limits for an API key.
// A nil pointer means the window is unlimited. All values must be positive integers.
type EmailQuotaLimits struct {
	PerMinute *int `json:"per_minute,omitempty"`
	PerHour   *int `json:"per_hour,omitempty"`
	PerDay    *int `json:"per_day,omitempty"`
	PerMonth  *int `json:"per_month,omitempty"`
}

// IsEmpty returns true when all windows are nil.
func (l *EmailQuotaLimits) IsEmpty() bool {
	return l == nil || (l.PerMinute == nil && l.PerHour == nil && l.PerDay == nil && l.PerMonth == nil)
}

// Validate checks that all non-nil values are positive integers.
func (l *EmailQuotaLimits) Validate() error {
	if l == nil {
		return nil
	}
	if l.PerMinute != nil && *l.PerMinute <= 0 {
		return ErrEmailQuotaInvalid
	}
	if l.PerHour != nil && *l.PerHour <= 0 {
		return ErrEmailQuotaInvalid
	}
	if l.PerDay != nil && *l.PerDay <= 0 {
		return ErrEmailQuotaInvalid
	}
	if l.PerMonth != nil && *l.PerMonth <= 0 {
		return ErrEmailQuotaInvalid
	}
	return nil
}
