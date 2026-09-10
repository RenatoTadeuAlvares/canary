package stress

import (
	"testing"
	"time"

	"github.com/osauer/canary/v2/internal/rpc"
)

func TestStressScopeRequiresMatchingCurrentConstituentAuthority(t *testing.T) {
	for _, change := range []string{"current", "account absent", "positions stale", "different account", "different mode", "aggregate", "unknown mode"} {
		t.Run(change, func(t *testing.T) {
			authority := rpc.AccountDataAuthority{Scope: rpc.AccountDataScope{AccountID: "SYNTHETIC", AccountMode: "paper"}, Availability: rpc.AccountDataAvailable, Freshness: rpc.AccountDataFreshnessCurrent}
			account, positions := authority, authority
			input := StressInput{Now: time.Now(), Account: rpc.AccountResult{Authority: &account}, Positions: rpc.PositionsResult{Authority: &positions}}
			switch change {
			case "account absent":
				input.Account.Authority = nil
			case "positions stale":
				input.Positions.Authority.Freshness = rpc.AccountDataFreshnessStale
			case "different account":
				input.Positions.Authority.Scope.AccountID = "OTHER-SYNTHETIC"
			case "different mode":
				input.Positions.Authority.Scope.AccountMode = "live"
			case "aggregate":
				input.Account.Authority.Scope.AccountID = "All"
				input.Positions.Authority = input.Account.Authority
			case "unknown mode":
				input.Account.Authority.Scope.AccountMode = ""
				input.Positions.Authority = input.Account.Authority
			}
			result := ComputeStress(input)
			if change == "current" {
				if result.AccountScope == nil || *result.AccountScope != authority.Scope || result.AccountScopeIssue != "" {
					t.Fatal("usable source lost scope", result.AccountScopeIssue)
				}
			} else if result.AccountScope != nil || result.AccountScopeIssue == "" {
				t.Fatal("unbound stress claimed account identity", result.AccountScopeIssue)
			}
		})
	}
}
