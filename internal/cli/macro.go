package cli

import (
	"context"
	"fmt"
	"github.com/osauer/canary/v2/internal/rpc"
)

func runMacro(ctx context.Context, env *Env, args []string) int {
	fs := flagSet(env, "macro")
	jsonOut := fs.Bool("json", false, "emit retained public calendar, publications and source coverage")
	start := fs.String("window-start", "", "inclusive source-local YYYY-MM-DD; supply with --window-end, at most 31 days")
	end := fs.String("window-end", "", "inclusive source-local YYYY-MM-DD; supply with --window-start")
	if err := fs.Parse(args); err != nil {
		return parseExit(err)
	}
	if fs.NArg() != 0 {
		return failUnexpectedArgs(env, fs)
	}
	var out rpc.MacroSnapshotResult
	if err := env.Conn.Call(ctx, rpc.MethodMacroSnapshot, rpc.MacroSnapshotParams{WindowStart: *start, WindowEnd: *end}, &out); err != nil {
		return fail(env, "macro: %v", err)
	}
	if *jsonOut {
		return printJSON(env, out)
	}
	riskReadLine(env, "Economic calendar", out.WindowStart+" to "+out.WindowEnd, out.CoverageStatus)
	for _, e := range out.Events {
		when := e.Date
		if !e.ScheduledAt.IsZero() {
			when = e.ScheduledAt.Format("2006-01-02 15:04 MST")
		} else if e.TimeLabel != "" {
			when += " · " + e.TimeLabel + " (source label)"
		}
		riskReadLine(env, when, e.Title)
	}
	for _, p := range out.Publications {
		riskReadLine(env, "Publication", p.Title, p.SourceURL)
	}
	for _, s := range out.Sources {
		state := s.Availability
		if s.Stale {
			state += " · stale"
		}
		riskReadLine(env, s.Name, state, s.Detail)
	}
	fmt.Fprintln(env.Stdout, "Official source coverage only; missing feeds do not establish an event-free day.")
	return 0
}
