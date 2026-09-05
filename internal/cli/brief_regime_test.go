package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/osauer/canary/v2/internal/rpc"
)

func TestBriefGammaPreservesInterpretationAndProvenance(t *testing.T) {
	var out bytes.Buffer
	res := rpc.BriefResult{Ready: rpc.BriefReadySection{Gamma: rpc.BriefGammaRow{
		BriefRowState: rpc.BriefRowState{Status: "degraded", Detail: "context only"},
		Underlying:    "SPX", Regime: "short_gamma", Spot: new(100.0),
		Insight: &rpc.GammaInsight{Interpretation: "Amplification in either direction.", SkewInterpretation: "Richer downside pricing; no forecast.", Provenance: "frozen feed; observed 2026-09-04 20:00 UTC; context_only"},
	}}}
	renderBrief(&Env{Stdout: &out, Stderr: &bytes.Buffer{}}, res)
	for _, want := range []string{"SPX", "short gamma", "Amplification in either direction.", "Richer downside pricing; no forecast.", "frozen feed", "2026-09-04 20:00 UTC", "context_only"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in brief", want)
		}
	}
}
