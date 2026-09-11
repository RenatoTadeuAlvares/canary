package rpc

import "time"

// Market data methods expose daemon-owned observations without broker writes.
const (
	MethodMarketSnapshot = "market.snapshot"
	MethodMarketHistory  = "market.history"
)

// MarketInstrument is a named exact quote with explicit per-instrument failure.
type MarketInstrument struct {
	Key        string `json:"key"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Underlying string `json:"underlying,omitempty"`
	Quote      *Quote `json:"quote,omitempty"`
	Error      string `json:"error,omitempty"`
}

// MarketSnapshotResult separates public quotes from the scope of held underlyings.
type MarketSnapshotResult struct {
	AsOf           time.Time             `json:"as_of"`
	Authority      *AccountDataAuthority `json:"authority,omitempty"`
	Instruments    []MarketInstrument    `json:"instruments"`
	Underlyings    []MarketInstrument    `json:"underlyings"`
	CoverageStatus string                `json:"coverage_status"`
	Truncated      bool                  `json:"truncated,omitempty"`
}

// MarketHistoryParams selects a bounded observed price series, not an analysis.
type MarketHistoryParams struct {
	Contract ContractParams `json:"contract"`
	Range    string         `json:"range"`
}

// MarketHistoryPoint is an observed bar close; volume is absent for midpoint data.
type MarketHistoryPoint struct {
	At     time.Time `json:"at"`
	Value  float64   `json:"value"`
	Volume *int64    `json:"volume,omitempty"`
}

// MarketSessionRange is one completed US equity session's regular-hours trade
// bar. Contract, acquisition time and source belong to the enclosing history.
type MarketSessionRange struct {
	Date  string  `json:"date"`
	High  float64 `json:"high"`
	Low   float64 `json:"low"`
	Close float64 `json:"close"`
}

// MarketHistoryResult preserves actual acquisition, range and pricing basis.
type MarketHistoryResult struct {
	// LastCompletedSession is absent without a valid daily bar for the exact
	// latest completed session, or when this instrument's calendar is unsupported.
	LastCompletedSession *MarketSessionRange  `json:"last_completed_session,omitempty"`
	TimestampKind        string               `json:"timestamp_kind"`
	PriceBasis           string               `json:"price_basis"`
	RegularHoursOnly     bool                 `json:"regular_hours_only"`
	RequestedStart       time.Time            `json:"requested_start"`
	Contract             ContractParams       `json:"contract"`
	Range                string               `json:"range"`
	Interval             string               `json:"interval"`
	Source               string               `json:"source"`
	AsOf                 time.Time            `json:"as_of"`
	Start                time.Time            `json:"start"`
	End                  time.Time            `json:"end"`
	Points               []MarketHistoryPoint `json:"points"`
	Reference            *float64             `json:"reference,omitempty"`
	ReferenceName        string               `json:"reference_name,omitempty"`
	CoverageStatus       string               `json:"coverage_status"`
}
