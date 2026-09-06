package cli

import (
	"context"
	"errors"
	"fmt"
	"github.com/osauer/canary/v2/internal/risk"
	"strings"

	"github.com/osauer/canary/v2/internal/rpc"
	"github.com/osauer/canary/v2/internal/stress"
)

func runRegime(ctx context.Context, env *Env, args []string) int {
	fs := flagSet(env, "regime")
	jsonOut := fs.Bool("json", false, "emit the detailed typed regime snapshot")
	explain := fs.Bool("explain", false, "include served thresholds, confirmation reasons, and source health")
	profiles := fs.Bool("profiles", false, "include large gamma profile arrays in JSON")
	if err := fs.Parse(args); err != nil {
		return parseExit(err)
	}
	if fs.NArg() != 0 {
		return failUnexpectedArgs(env, fs)
	}
	if *profiles && !*jsonOut {
		return fail(env, "regime: --profiles requires --json")
	}
	var res rpc.RegimeSnapshotResult
	if err := env.Conn.Call(ctx, rpc.MethodRegimeSnapshot, rpc.RegimeSnapshotParams{}, &res); err != nil {
		return fail(env, "regime: %v", err)
	}
	if *jsonOut {
		if !*profiles {
			rpc.StripRegimeGammaProfiles(&res)
		}
		return printJSON(env, res)
	}
	renderRegime(env, res, *explain)
	return 0
}

func renderRegime(env *Env, res rpc.RegimeSnapshotResult, explain bool) {
	monitor := rpc.CompactRegimeMonitor(&res)
	current := res.AuthorityHealth != nil && res.AuthorityHealth.Status == rpc.RegimeAuthorityFresh
	verdict := res.Composite.Verdict
	if verdict == "" {
		verdict = "unavailable"
	} else if !current {
		verdict = "recorded verdict: " + verdict
	}
	riskReadLine(env, "Regime", verdict)
	if h := res.AuthorityHealth; h != nil {
		riskReadLine(env, "Evidence", string(h.Status), strings.ReplaceAll(string(h.FailureCode), "_", " "))
		if h.LastSuccessAt != nil {
			riskReadLine(env, "Last successful refresh", h.LastSuccessAt.Local().Format("2 Jan 15:04 MST"))
		}
	}
	riskReadLine(env, "Readiness", res.Lifecycle.Readiness)
	if explain {
		riskReadLine(env, "Snapshot generated", res.AsOf.Local().Format("2 Jan 15:04 MST"), res.Lifecycle.Stage)
		riskReadLine(env, "Clusters", res.Summary.Evidence)
	}
	fmt.Fprintln(env.Stdout, "\nIndicators")
	for _, row := range monitor.Indicators {
		reading := row.Reading
		if reading == "" {
			reading = "unavailable"
		}
		band := row.Band
		if !current || row.Status != rpc.RegimeStatusOK {
			band = ""
			if reading != "unavailable" {
				reading = "recorded: " + reading
			}
		}
		riskReadLine(env, "  "+row.Name, reading, row.Status, strings.ReplaceAll(row.FreshnessClass, "_", " "), band)
		if explain && (!current || row.Status != rpc.RegimeStatusOK) && row.Band != "" {
			riskReadLine(env, "    Recorded band", row.Band, "not a current rating")
		}
		if row.Eligibility != nil && !row.Eligibility.Eligible {
			riskReadLine(env, "    Confirmation", "not eligible", strings.Join(row.Eligibility.Reasons, "; "))
		}
		if explain {
			if row.AsOf != nil {
				observed := row.AsOf.Date
				if observed == "" && !row.AsOf.Time.IsZero() {
					observed = row.AsOf.Time.Local().Format("2 Jan 15:04 MST")
				}
				if observed == "" {
					observed = row.AsOf.Label
				}
				riskReadLine(env, "    Observed", observed, row.AsOf.Source)
			}
			if th := row.Thresholds; th != nil {
				riskReadLine(env, "    Thresholds", "green: "+th.Green, "yellow: "+th.Yellow, "red: "+th.Red)
			}
		}
	}
	if !explain {
		if len(res.WarningDetails) > 0 {
			riskReadLine(env, "\nSource issues", fmt.Sprintf("%d reported; see --explain", len(res.WarningDetails)))
		}
		fmt.Fprintln(env.Stdout, "\nDetails: canary regime --explain")
	}
	if explain {
		fmt.Fprintln(env.Stdout, "\nSource diagnostics")
		for _, warning := range res.WarningDetails {
			riskReadLine(env, "  "+warning.Code, warning.Message)
		}
		for _, insight := range monitor.GammaInsights {
			if insight != nil {
				riskReadLine(env, "  Gamma", insight.Interpretation, insight.HorizonInterpretation, insight.SkewInterpretation, insight.Provenance)
			}
		}
		var pending []string
		for _, row := range monitor.Indicators {
			if row.Thresholds != nil && row.Thresholds.PendingBacktest {
				pending = append(pending, row.Name)
			}
		}
		if len(pending) > 0 {
			riskReadLine(env, "Pending backtest", strings.Join(pending, ", "))
		}
	}
	if explain {
		for _, governor := range res.Lifecycle.Governors {
			riskReadLine(env, "Governor", governor.Action, governor.From, governor.To, governor.Reason)
		}
		for _, source := range res.SourceHealth {
			riskReadLine(env, "Source", source.Source, source.Status, source.AsOf.Format("2006-01-02 15:04 MST"), strings.Join(source.Notes, "; "))
		}
	}
}

func runStress(ctx context.Context, env *Env, args []string) int {
	fs := flagSet(env, "stress")
	jsonOut := fs.Bool("json", false, "emit the full typed portfolio-stress assessment")
	details := fs.Bool("details", false, "include market indicator and source-health details")
	if err := fs.Parse(args); err != nil {
		return parseExit(err)
	}
	if fs.NArg() != 0 {
		return failUnexpectedArgs(env, fs)
	}
	res, err := stress.FetchStress(ctx, env.Conn)
	if err != nil {
		if *jsonOut {
			return fail(env, "stress: %v", err)
		}
		diagnostic := &Env{Stdout: env.Stderr}
		riskReadLine(diagnostic, "Portfolio stress", "unavailable")
		var rpcErr *rpc.Error
		if errors.As(err, &rpcErr) && rpcErr.Code == rpc.CodeGatewayUnavailable {
			riskReadLine(diagnostic, "", "Gateway unavailable; required inputs could not be read.")
		} else {
			riskReadLine(diagnostic, "", "A required input could not be read.")
		}
		riskReadLine(diagnostic, "Next", "canary status · canary stress --details")
		if *details {
			riskReadLine(diagnostic, "Diagnostic", err.Error())
		}
		return 1
	}
	if *jsonOut {
		return printJSON(env, res)
	}
	renderStress(env, res, *details)
	return 0
}

func renderStress(env *Env, res rpc.StressResult, details bool) {
	riskReadLine(env, "Portfolio stress", strings.ReplaceAll(res.Action, "_", " "), string(res.Severity))
	riskReadLine(env, "Assessment", res.Summary)
	riskReadLine(env, "Inputs", res.InputHealth, "market: "+res.MarketConfirmation, "portfolio fit: "+res.PortfolioFit)
	for _, group := range []struct {
		title   string
		quality bool
	}{{"Findings", false}, {"Coverage gaps", true}} {
		printed := false
		for i, row := range res.Rows {
			if i == 0 && row.Title == "Portfolio stress" {
				continue
			}
			if (row.Direction == risk.DirectionDataQuality) != group.quality {
				continue
			}
			if !details && row.Severity == risk.SeverityObserve {
				continue
			}
			if !printed {
				fmt.Fprintln(env.Stdout, "\n"+group.title)
				printed = true
			}
			riskReadLine(env, "  "+row.Title, string(row.Severity), row.Evidence)
			if details || !group.quality {
				riskReadLine(env, "    ", row.Guidance)
			}
		}
	}
	if details {
		riskReadLine(env, "\nSnapshot generated", res.AsOf.Local().Format("2 Jan 15:04 MST"))
		if len(res.Rows) > 0 && res.Rows[0].Title == "Portfolio stress" {
			riskReadLine(env, "Overall evidence", res.Rows[0].Evidence)
		}
		for _, warning := range res.Warnings {
			riskReadLine(env, "Warning", warning)
		}
		for _, row := range res.MarketIndicators {
			riskReadLine(env, row.Name, row.Status, row.Reading, row.AsOf, row.Comment)
		}
		for _, source := range res.SourceHealth {
			riskReadLine(env, "Source", source.Source, source.Status, source.AsOf.Format("2006-01-02 15:04 MST"), strings.Join(source.Notes, "; "))
		}
	} else {
		fmt.Fprintln(env.Stdout)
		riskReadLine(env, "Details", fmt.Sprintf("canary stress --details · %d source notes", len(res.Warnings)))
	}
	riskReadLine(env, "", res.NotExecution)
}

func riskReadLine(env *Env, label string, values ...string) {
	indent := label[:len(label)-len(strings.TrimLeft(label, " "))]
	text := sanitizeRunText(briefJoin(append([]string{label}, values...)...))
	width := briefProseWidth(env.Stdout)
	for i, line := range wrapVisibleText(text, width-len(indent)-2) {
		if i == 0 {
			fmt.Fprintln(env.Stdout, indent+line)
		} else {
			fmt.Fprintln(env.Stdout, indent+"  "+line)
		}
	}
}
