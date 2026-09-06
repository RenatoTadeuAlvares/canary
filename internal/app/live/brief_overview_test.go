package live

import (
	"github.com/osauer/canary/v2/internal/rpc"
	"testing"
)

func TestBriefOverviewSnapshotCloneDoesNotAliasServedRuns(t *testing.T) {
	paragraph := []rpc.BriefParagraph{{Runs: []rpc.BriefRun{{Text: "original", AccountSensitive: true}}}}
	in := &rpc.BriefResult{Narrative: &rpc.BriefNarrative{Lead: []rpc.BriefRun{{Text: "lead"}}, Overview: &rpc.BriefOverview{Assessment: []rpc.BriefRun{{Text: "partial"}}, Attention: paragraph, Context: paragraph, Coverage: paragraph}}}
	out := cloneBriefResult(in)
	out.Narrative.Lead[0].Text = "changed"
	out.Narrative.Overview.Assessment[0].Text = "changed"
	for _, rows := range [][]rpc.BriefParagraph{out.Narrative.Overview.Attention, out.Narrative.Overview.Context, out.Narrative.Overview.Coverage} {
		rows[0].Runs[0].Text = "changed"
	}
	if in.Narrative.Lead[0].Text != "lead" || in.Narrative.Overview.Assessment[0].Text != "partial" || paragraph[0].Runs[0].Text != "original" {
		t.Fatal("client mutation changed shared evidence")
	}
}
