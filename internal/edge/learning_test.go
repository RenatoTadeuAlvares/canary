package edge

import (
	"math"
	"testing"
)

func TestDecisionPatternsCompareIdenticalSamplesAndDiscloseConcentration(t *testing.T) {
	rows := []Change{
		{ID: "a", ConID: 1, AssetClass: "STK", Action: ActionAdd, Direction: DirectionLong, ExecutedAt: day("2026-01-05"), ExecutionNotionalBase: new(float64(100)), Scores: []HorizonScore{{Sessions: 1, DecisionImpactBase: new(float64(10)), DecisionImpactPct: new(float64(10))}, {Sessions: 5, DecisionImpactBase: new(float64(20)), DecisionImpactPct: new(float64(20))}, {Sessions: 20, Reason: ReasonInterveningChange}}},
		{ID: "b", ConID: 2, AssetClass: "STK", Action: ActionAdd, Direction: DirectionLong, ExecutedAt: day("2026-01-06"), ExecutionNotionalBase: new(float64(200)), Scores: []HorizonScore{{Sessions: 1, DecisionImpactBase: new(float64(-5)), DecisionImpactPct: new(float64(-2.5))}, {Sessions: 5, DecisionImpactBase: new(float64(-10)), DecisionImpactPct: new(float64(-5))}, {Sessions: 20, DecisionImpactBase: new(float64(-20)), DecisionImpactPct: new(float64(-10))}}},
		{ID: "c", ConID: 1, AssetClass: "STK", Action: ActionAdd, Direction: DirectionLong, ExecutedAt: day("2026-02-05"), Scores: []HorizonScore{{Sessions: 1, DecisionImpactBase: new(float64(100)), DecisionImpactPct: new(float64(100))}, {Sessions: 5, Reason: ReasonInterveningChange}, {Sessions: 20, Reason: ReasonInterveningChange}}},
	}
	p := buildDecisionPatterns(rows)[0]
	h := p.Horizons[0]
	if p.EligibleChanges != 3 || p.NotionalKnownCount != 2 || *p.KnownNotionalBase != 300 || h.NotionalCoveragePct != nil || *h.TotalBase != 105 || *h.WithoutLargestBase != 5 || h.DistinctContracts != 2 || h.DistinctDates != 3 || len(h.Months) != 2 || h.Months[0].TotalBase != 5 {
		t.Fatalf("incorrect sample evidence: %+v / %+v", p, h)
	}
	if math.Abs(*h.LargestContractSharePct-110.0/115*100) > 1e-9 {
		t.Fatalf("concentration=%v", h.LargestContractSharePct)
	}
	comparison := p.Comparisons[0]
	if comparison.SampleCount != 2 || *comparison.EarlierTotalBase != 5 || *comparison.LaterTotalBase != 10 || *comparison.DifferenceBase != 5 || *comparison.MedianDifferenceBase != 2.5 {
		t.Fatalf("compared different decision sets: %+v", comparison)
	}
	rows[2].ExecutionNotionalBase = new(float64(700))
	p = buildDecisionPatterns(rows)[0]
	if got := p.Horizons[1].NotionalCoveragePct; got == nil || *got != 30 {
		t.Fatalf("notional coverage=%v", got)
	}
	rows[2].Direction = DirectionShort
	patterns := buildDecisionPatterns(rows)
	if len(patterns) != 2 || patterns[0].EligibleChanges != 2 || patterns[1].EligibleChanges != 1 {
		t.Fatal("directions blended")
	}
}

func TestProtectionContextRequiresEveryExecutionAndAgreement(t *testing.T) {
	records := map[string]ProtectionRecord{"a": {Bucket: "risk_reduction", At: day("2026-01-01")}}
	if got := changeProtectionContext([]string{"a", "b"}, records); got.Status != "partial" || got.Bucket != "" {
		t.Fatalf("partial became purpose: %+v", got)
	}
	records["b"] = records["a"]
	if got := changeProtectionContext([]string{"a", "b"}, records); got.Status != "linked" || got.MatchedExecutions != 2 {
		t.Fatalf("exact context lost: %+v", got)
	}
	records["b"] = ProtectionRecord{Identity: "different", Bucket: "risk_reduction", At: day("2026-01-01")}
	if got := changeProtectionContext([]string{"a", "b"}, records); got.Status != "partial" || got.Bucket != "" {
		t.Fatal("different policy identities blended")
	}
	records["b"] = ProtectionRecord{Bucket: "trailing_stop", At: day("2026-01-01")}
	if got := changeProtectionContext([]string{"a", "b"}, records); got.Status != "partial" || got.Bucket != "" {
		t.Fatalf("conflicting purpose invented: %+v", got)
	}
	if got := changeProtectionContext([]string{"z"}, records); got.Status != "unavailable" {
		t.Fatal(got)
	}
}
