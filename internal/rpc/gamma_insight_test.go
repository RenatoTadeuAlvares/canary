package rpc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGammaLocalSignControlsRegimeAndDepth(t *testing.T) {
	for _, tc := range []struct {
		spot, net, depth float64
		want             string
	}{
		{106, -10, 6, "short_gamma"}, {94, 10, -6, "long_gamma"}, {101, -10, 1, "transition_gamma"},
	} {
		gap := tc.spot - 100
		c := &GammaZeroComputed{SpotUnderlying: tc.spot, ZeroGamma: new(100.0), GapPct: &gap, ProfileMetrics: &GammaProfileMetrics{GEXAtSpot: tc.net}}
		if got := GammaComputedRegime(c); got != tc.want {
			t.Fatalf("local sign: got %s want %s", got, tc.want)
		}
		if d := RegimeGammaDepth(c); d == nil || *d != tc.depth {
			t.Fatalf("wrong depth: %v", d)
		}
	}
}

func TestCompactRegimePreservesOptionEvidenceAndMissingBreadth(t *testing.T) {
	i := &GammaInsight{Underlying: "SPX", Regime: "short_gamma", Rankability: "blocked", RankabilityReason: "stale", SelectedSkew: &OptionSkew{SkewVolPoints: new(3.0), PutDeltaBracket: []float64{.2, .3}}}
	r := &RegimeSnapshotResult{GammaZero: RegimeGammaZero{Envelope: GammaZeroSPXResult{Status: GammaZeroStatusReady, Result: &GammaZeroComputed{Scope: GammaZeroScopeSPX, Insight: i}}}}
	m := CompactRegimeMonitor(r)
	if len(m.GammaInsights) != 1 || m.GammaInsights[0].Rankability != "blocked" {
		t.Fatal("lost gamma quality")
	}
	*m.GammaInsights[0].SelectedSkew.SkewVolPoints = 9
	if *i.SelectedSkew.SkewVolPoints != 3 {
		t.Fatal("monitor aliases retained observation")
	}
	encoded, err := json.Marshal(BreadthSPXResult{State: BreadthStateReady, Coverage50: 100})
	if err != nil || !strings.Contains(string(encoded), `"pct_above_200dma":null`) || !strings.Contains(string(encoded), `"net_new_highs_pct":null`) {
		t.Fatalf("unknown secondary coverage became zero: %s %v", encoded, err)
	}
}
