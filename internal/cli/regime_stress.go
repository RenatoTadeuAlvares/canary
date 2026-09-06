package cli

import (
	"context"
	"fmt"
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
	riskReadLine(env, "Regime", res.AsOf.Format("2006-01-02 15:04 MST"), res.Composite.Verdict, res.Lifecycle.Stage)
	if h := res.AuthorityHealth; h != nil {
		riskReadLine(env, "Authority", string(h.Status), string(h.FailureCode))
	}
	riskReadLine(env, "Evidence", res.Summary.Evidence)
	riskReadLine(env, "Readiness", res.Lifecycle.Readiness)
	for _, row := range monitor.Indicators {
		reading := row.Reading
		if reading == "" {
			reading = "unavailable"
		}
		riskReadLine(env, row.Name, row.Status, row.Band, reading, row.FreshnessClass)
		if row.Eligibility != nil && !row.Eligibility.Eligible {
			riskReadLine(env, "  Confirmation", "not eligible", strings.Join(row.Eligibility.Reasons, "; "))
		}
		if explain {
			if row.AsOf != nil {
				riskReadLine(env, "  Observed", row.AsOf.Label, row.AsOf.Date, row.AsOf.Freshness, row.AsOf.Source)
			}
			if th := row.Thresholds; th != nil {
				riskReadLine(env, "  Thresholds", "green: "+th.Green, "yellow: "+th.Yellow, "red: "+th.Red)
				if th.PendingBacktest {
					riskReadLine(env, "  Calibration", "pending backtest")
				}
			}
		}
	}
	for _, warning := range res.WarningDetails {
		riskReadLine(env, "Warning", warning.Code, warning.Message)
	}
	for _, insight := range monitor.GammaInsights {
		if insight != nil {
			riskReadLine(env, "Gamma", insight.Interpretation, insight.HorizonInterpretation, insight.SkewInterpretation, insight.Provenance)
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
		return fail(env, "stress: %v", err)
	}
	if *jsonOut {
		return printJSON(env, res)
	}
	renderStress(env, res, *details)
	return 0
}

func renderStress(env *Env, res rpc.StressResult, details bool) {
	riskReadLine(env, "Portfolio stress", res.AsOf.Format("2006-01-02 15:04 MST"), res.Action, string(res.Severity))
	riskReadLine(env, "Assessment", res.Summary)
	riskReadLine(env, "Inputs", res.InputHealth, "market: "+res.MarketConfirmation, "portfolio fit: "+res.PortfolioFit)
	for _, row := range res.Rows {
		riskReadLine(env, row.Title, string(row.Severity), row.Evidence, row.Guidance)
	}
	for _, warning := range res.Warnings {
		riskReadLine(env, "Warning", warning)
	}
	if details {
		for _, row := range res.MarketIndicators {
			riskReadLine(env, row.Name, row.Status, row.Reading, row.AsOf, row.Comment)
		}
		for _, source := range res.SourceHealth {
			riskReadLine(env, "Source", source.Source, source.Status, source.AsOf.Format("2006-01-02 15:04 MST"), strings.Join(source.Notes, "; "))
		}
	}
	riskReadLine(env, "", res.NotExecution)
}

func riskReadLine(env *Env, label string, values ...string) {
	fmt.Fprintln(env.Stdout, sanitizeRunText(briefJoin(append([]string{label}, values...)...)))
}
