package rpc

import "time"

// MethodMacroSnapshot reads cached public economic and central-bank sources.
const MethodMacroSnapshot = "macro.snapshot"

// MacroSnapshotParams selects inclusive source-local calendar dates before
// response limits. Both dates must be supplied together, spanning at most 31 days.
// An omitted window retains the default yesterday-through-next-week overview.
type MacroSnapshotParams struct {
	WindowStart string `json:"window_start,omitempty"`
	WindowEnd   string `json:"window_end,omitempty"`
}

// MacroEvent preserves a scheduled source date without inventing unknown times.
type MacroEvent struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Category      string    `json:"category"`
	SourceID      string    `json:"source_id"`
	SourceURL     string    `json:"source_url"`
	Date          string    `json:"date"`
	ScheduledAt   time.Time `json:"scheduled_at,omitzero"`
	TimePrecision string    `json:"time_precision"`
	TimeLabel     string    `json:"time_label,omitempty"`
	Timezone      string    `json:"timezone"`
	RetrievedAt   time.Time `json:"retrieved_at"`
}

// MacroPublication is an official headline, not a generated market judgment.
type MacroPublication struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	SourceID    string    `json:"source_id"`
	SourceURL   string    `json:"source_url"`
	PublishedAt time.Time `json:"published_at,omitzero"`
	RetrievedAt time.Time `json:"retrieved_at"`
}

// MacroSource discloses each feed's last attempt and last successful evidence.
type MacroSource struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	URL          string    `json:"url"`
	Kind         string    `json:"kind"`
	Availability string    `json:"availability"`
	Detail       string    `json:"detail,omitempty"`
	Stale        bool      `json:"stale"`
	LastAttempt  time.Time `json:"last_attempt,omitzero"`
	LastSuccess  time.Time `json:"last_success,omitzero"`
	ValidUntil   time.Time `json:"valid_until,omitzero"`
	Coverage     string    `json:"coverage"`
	// WindowStart and WindowEnd bound a calendar whose publisher supplies one month.
	// Empty bounds mean the feed does not establish an explicit covered interval.
	WindowStart         string    `json:"window_start,omitempty"`
	WindowEnd           string    `json:"window_end,omitempty"`
	ConsecutiveFailures int       `json:"consecutive_failures,omitempty"`
	FirstFailure        time.Time `json:"first_failure,omitzero"`
	NextAttempt         time.Time `json:"next_attempt,omitzero"`
}

// MacroSnapshotResult contains a bounded read of retained public-source evidence.
// Partial/failed feeds never establish an event-free day. No account data is sent.
type MacroSnapshotResult struct {
	AsOf           time.Time          `json:"as_of"`
	WindowStart    string             `json:"window_start"`
	WindowEnd      string             `json:"window_end"`
	CoverageStatus string             `json:"coverage_status"`
	Events         []MacroEvent       `json:"events"`
	Publications   []MacroPublication `json:"publications"`
	Sources        []MacroSource      `json:"sources"`
	// EventsTruncated and PublicationsTruncated identify which returned list lost
	// rows to response limits. They are always present, including when false.
	EventsTruncated       bool `json:"events_truncated"`
	PublicationsTruncated bool `json:"publications_truncated"`
	// Truncated preserves the legacy union of both list-truncation flags.
	Truncated bool `json:"truncated"`
}
