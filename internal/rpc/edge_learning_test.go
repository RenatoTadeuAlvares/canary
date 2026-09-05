package rpc

import (
	"testing"
	"time"
)

func TestEdgeProtectionContractRejectsUnsupportedPurpose(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	good := EdgeProtectionContext{Status: "linked", ExecutionCount: 2, MatchedExecutions: 2, Bucket: TradeProposalBucketRiskReduction, EvidenceAt: at}
	if err := validateEdgeProtectionContext(good); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"unknown bucket", "missing match", "missing date", "unavailable purpose", "partial purpose", "negative count"} {
		t.Run(kind, func(t *testing.T) {
			c := good
			switch kind {
			case "unknown bucket":
				c.Bucket = "profitable means safe"
			case "missing match":
				c.MatchedExecutions = 1
			case "missing date":
				c.EvidenceAt = time.Time{}
			case "unavailable purpose":
				c.Status = "unavailable"
			case "partial purpose":
				c.Status = "partial"
			case "negative count":
				c.ExecutionCount = -1
			}
			if validateEdgeProtectionContext(c) == nil {
				t.Fatal("invalid provenance accepted")
			}
		})
	}
	good.Status = "partial"
	good.Bucket = ""
	if err := validateEdgeProtectionContext(good); err != nil {
		t.Fatal(err)
	}
}
