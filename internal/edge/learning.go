package edge

import (
	"math"
	"sort"
)

// DecisionPattern describes one action and direction without grading skill.
type DecisionPattern struct {
	Action             string              `json:"action"`
	Direction          string              `json:"direction"`
	EligibleChanges    int                 `json:"eligible_changes"`
	NotionalKnownCount int                 `json:"notional_known_count"`
	KnownNotionalBase  *float64            `json:"known_notional_base,omitempty"`
	Horizons           []PatternHorizon    `json:"horizons"`
	Comparisons        []HorizonComparison `json:"comparisons"`
}

// PatternHorizon discloses the sample and concentration behind a price outcome.
type PatternHorizon struct {
	MarketContext           []MarketContextRollup `json:"market_context"`
	LinkedProtectionCount   int                   `json:"linked_protection_count"`
	PartialProtectionCount  int                   `json:"partial_protection_count"`
	Sessions                int                   `json:"sessions"`
	SampleCount             int                   `json:"sample_count"`
	ScoredNotionalBase      *float64              `json:"scored_notional_base,omitempty"`
	NotionalCoveragePct     *float64              `json:"notional_coverage_pct,omitempty"`
	TotalBase               *float64              `json:"total_base,omitempty"`
	MedianBase              *float64              `json:"median_base,omitempty"`
	MedianImpactPct         *float64              `json:"median_impact_pct,omitempty"`
	PositiveCount           int                   `json:"positive_count"`
	NegativeCount           int                   `json:"negative_count"`
	FlatCount               int                   `json:"flat_count"`
	DistinctDates           int                   `json:"distinct_dates"`
	DistinctContracts       int                   `json:"distinct_contracts"`
	LargestDateSharePct     *float64              `json:"largest_date_share_pct,omitempty"`
	LargestContractSharePct *float64              `json:"largest_contract_share_pct,omitempty"`
	WithoutLargestBase      *float64              `json:"without_largest_base,omitempty"`
	Months                  []PatternMonth        `json:"months"`
	Exclusions              map[string]int        `json:"exclusions"`
}

// PatternMonth is one observed calendar-month sample, not a significance test.
type PatternMonth struct {
	Month       string  `json:"month"`
	SampleCount int     `json:"sample_count"`
	TotalBase   float64 `json:"total_base"`
	MedianBase  float64 `json:"median_base"`
}

// HorizonComparison compares the identical decisions at two horizons.
type HorizonComparison struct {
	EarlierSessions      int      `json:"earlier_sessions"`
	LaterSessions        int      `json:"later_sessions"`
	SampleCount          int      `json:"sample_count"`
	EarlierTotalBase     *float64 `json:"earlier_total_base,omitempty"`
	LaterTotalBase       *float64 `json:"later_total_base,omitempty"`
	DifferenceBase       *float64 `json:"difference_base,omitempty"`
	MedianDifferenceBase *float64 `json:"median_difference_base,omitempty"`
}

func buildDecisionPatterns(changes []Change) []DecisionPattern {
	out := make([]DecisionPattern, 0, 8)
	for _, action := range []string{ActionOpen, ActionAdd, ActionTrim, ActionExit} {
		for _, direction := range []string{DirectionLong, DirectionShort} {
			var rows []Change
			pattern := DecisionPattern{Action: action, Direction: direction}
			notional := 0.0
			for _, c := range changes {
				if !eligibleAsset(c.AssetClass) || c.Action != action || c.Direction != direction {
					continue
				}
				rows = append(rows, c)
				if c.ExecutionNotionalBase != nil {
					pattern.NotionalKnownCount++
					notional += *c.ExecutionNotionalBase
				}
			}
			if len(rows) == 0 {
				continue
			}
			pattern.EligibleChanges = len(rows)
			if pattern.NotionalKnownCount > 0 {
				pattern.KnownNotionalBase = &notional
			}
			for _, h := range Horizons {
				pattern.Horizons = append(pattern.Horizons, buildPatternHorizon(rows, h, pattern))
			}
			for _, later := range []int{5, 20} {
				pattern.Comparisons = append(pattern.Comparisons, compareHorizons(rows, 1, later))
			}
			out = append(out, pattern)
		}
	}
	return out
}

func buildPatternHorizon(rows []Change, h int, pattern DecisionPattern) PatternHorizon {
	out := PatternHorizon{Sessions: h, Months: []PatternMonth{}, Exclusions: map[string]int{}}
	dates := map[string]float64{}
	contracts := map[int64]float64{}
	months := map[string][]float64{}
	contextValues := map[string][]MarketContext{}
	var amounts, pcts []float64
	total, notional, absolute, largest := 0.0, 0.0, 0.0, 0.0
	known := 0
	for _, c := range rows {
		score := scoreFor(c, h)
		if score == nil || score.DecisionImpactBase == nil {
			reason := ReasonMissingHorizon
			if score != nil && score.Reason != "" {
				reason = score.Reason
			}
			out.Exclusions[reason]++
			continue
		}
		for _, context := range score.MarketContext {
			contextValues[context.Key] = append(contextValues[context.Key], context)
		}
		value := *score.DecisionImpactBase
		if c.ProtectionContext.Status == "linked" {
			out.LinkedProtectionCount++
		} else if c.ProtectionContext.Status == "partial" {
			out.PartialProtectionCount++
		}
		out.SampleCount++
		amounts = append(amounts, value)
		total += value
		absolute += math.Abs(value)
		if math.Abs(value) > math.Abs(largest) {
			largest = value
		}
		if score.DecisionImpactPct != nil {
			pcts = append(pcts, *score.DecisionImpactPct)
		}
		if c.ExecutionNotionalBase != nil {
			known++
			notional += *c.ExecutionNotionalBase
		}
		switch {
		case value > 0:
			out.PositiveCount++
		case value < 0:
			out.NegativeCount++
		default:
			out.FlatCount++
		}
		dates[c.ExecutedAt.Format("2006-01-02")] += math.Abs(value)
		contracts[c.ConID] += math.Abs(value)
		month := c.ExecutedAt.Format("2006-01")
		months[month] = append(months[month], value)
	}
	if out.SampleCount == 0 {
		return out
	}
	out.MarketContext = buildMarketContextRollups(contextValues)
	out.TotalBase = &total
	mid := median(amounts)
	out.MedianBase = &mid
	if len(pcts) == out.SampleCount {
		v := median(pcts)
		out.MedianImpactPct = &v
	}
	if known > 0 {
		out.ScoredNotionalBase = &notional
	}
	if pattern.NotionalKnownCount == pattern.EligibleChanges && known == out.SampleCount && pattern.KnownNotionalBase != nil && *pattern.KnownNotionalBase > 0 {
		v := 100 * notional / *pattern.KnownNotionalBase
		out.NotionalCoveragePct = &v
	}
	out.DistinctDates = len(dates)
	out.DistinctContracts = len(contracts)
	if absolute > 0 {
		maxDate, maxContract := 0.0, 0.0
		for _, v := range dates {
			maxDate = math.Max(maxDate, v)
		}
		for _, v := range contracts {
			maxContract = math.Max(maxContract, v)
		}
		d, c := 100*maxDate/absolute, 100*maxContract/absolute
		out.LargestDateSharePct = &d
		out.LargestContractSharePct = &c
	}
	remainder := total - largest
	out.WithoutLargestBase = &remainder
	keys := make([]string, 0, len(months))
	for key := range months {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		values := months[key]
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		out.Months = append(out.Months, PatternMonth{Month: key, SampleCount: len(values), TotalBase: sum, MedianBase: median(values)})
	}
	return out
}

func compareHorizons(rows []Change, earlier, later int) HorizonComparison {
	out := HorizonComparison{EarlierSessions: earlier, LaterSessions: later}
	first, last := 0.0, 0.0
	var differences []float64
	for _, c := range rows {
		a, b := scoreFor(c, earlier), scoreFor(c, later)
		if a == nil || b == nil || a.DecisionImpactBase == nil || b.DecisionImpactBase == nil {
			continue
		}
		first += *a.DecisionImpactBase
		last += *b.DecisionImpactBase
		differences = append(differences, *b.DecisionImpactBase-*a.DecisionImpactBase)
	}
	out.SampleCount = len(differences)
	if len(differences) > 0 {
		delta, mid := last-first, median(differences)
		out.EarlierTotalBase = &first
		out.LaterTotalBase = &last
		out.DifferenceBase = &delta
		out.MedianDifferenceBase = &mid
	}
	return out
}
