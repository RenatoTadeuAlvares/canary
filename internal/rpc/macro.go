package rpc

import "time"

// MethodMacroSnapshot reads cached public economic and central-bank sources.
const MethodMacroSnapshot = "macro.snapshot"

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
	Truncated      bool               `json:"truncated"`
}
