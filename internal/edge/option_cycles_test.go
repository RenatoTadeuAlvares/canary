package edge

import (
	"fmt"
	"testing"

	"github.com/osauer/canary/v2/internal/flexstmt"
)

func TestOptionCyclesRequireProvenFlatBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name    string
		deltas  []float64
		markers []string
		final   float64
		want    int
		reason  string
	}{
		{"long adds and partial closes", []float64{2, 1, -1, -2}, []string{"O", "O", "C", "C"}, 0, 1, ""},
		{"short closes", []float64{-2, 1, 1}, []string{"O", "C", "C"}, 0, 1, ""},
		{"reopen", []float64{1, -1, 1, -1}, []string{"O", "C", "O", "C"}, 0, 2, ""},
		{"still open", []float64{2, -1}, []string{"O", "C"}, 1, 0, ""},
		{"missing opening", []float64{-2}, []string{"C"}, 0, 0, "opening_unproved"},
		{"add to missing opening", []float64{1, -2}, []string{"O", "C"}, 0, 0, "opening_unproved"},
		{"cross zero", []float64{1, -2}, []string{"O", "C"}, -1, 0, "cross_zero_execution"},
		{"unknown intent marker", []float64{1, -1}, []string{"", "C"}, 0, 0, "open_close_conflict"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := cycleTestStatement(tc.deltas, tc.markers, tc.final)
			result, err := Analyze(Input{AsOf: day("2026-02-10"), WindowDays: 90, BaseCurrency: "EUR", Statements: []flexstmt.Statement{st}})
			if err != nil {
				t.Fatal(err)
			}
			c := result.Options.Cycles
			if c.CompletedCount != tc.want || tc.reason != "" && c.Reasons[tc.reason] == 0 {
				t.Fatalf("cycles=%+v", c)
			}
			for _, row := range c.Cycles {
				if row.PNLStatus != OptionPNLComplete || row.ExecutionCount < 2 {
					t.Fatalf("cycle evidence=%+v", row)
				}
			}
			if tc.final != 0 && tc.reason == "" && c.OpenContractCount != 1 {
				t.Fatal("open inventory lost")
			}
		})
	}
}

func TestOptionCycleAmbiguityAndLifecycleRemainUnproved(t *testing.T) {
	for _, kind := range []string{"same_time", "missing_quantity", "lifecycle", "anchor_mismatch", "missing_anchor_quantity"} {
		st := cycleTestStatement([]float64{1, -1}, []string{"O", "C"}, 0)
		switch kind {
		case "same_time":
			st.Trades[1].ExecutedAt = st.Trades[0].ExecutedAt
		case "missing_quantity":
			st.Trades[0].Quantity = nil
		case "lifecycle":
			st.OptionEvents = []flexstmt.OptionEvent{{ConID: 456, Date: day("2026-01-15"), Quantity: new(float64(-1))}}
		case "missing_anchor_quantity":
			st.Positions[0].Quantity = nil
		case "anchor_mismatch":
			st.Positions = append(st.Positions, flexstmt.OpenPosition{RecordID: "early", AccountID: "U", ConID: 456, ReportDate: day("2026-01-01"), Quantity: new(float64(5))})
		}
		result, err := Analyze(Input{AsOf: day("2026-02-10"), WindowDays: 90, BaseCurrency: "EUR", Statements: []flexstmt.Statement{st}})
		if err != nil {
			t.Fatal(err)
		}
		if result.Options.Cycles.CompletedCount != 0 || result.Options.Cycles.ExcludedContracts != 1 {
			t.Fatalf("%s became complete: %+v", kind, result.Options.Cycles)
		}
	}
}

func cycleTestStatement(deltas []float64, markers []string, final float64) flexstmt.Statement {
	st := edgeStatement()
	st.Trades = nil
	st.Positions = []flexstmt.OpenPosition{{RecordID: "end", AccountID: "U", ConID: 456, AssetClass: "OPT", ReportDate: day("2026-02-01"), Quantity: &final}}
	for i, delta := range deltas {
		side := "BUY"
		if delta < 0 {
			side = "SELL"
			delta = -delta
		}
		pnl := 0.0
		if markers[i] == "C" {
			pnl = 10
		}
		id := fmt.Sprint(i)
		st.Trades = append(st.Trades, flexstmt.Trade{RecordID: id, AccountID: "U", ConID: 456, Symbol: "SYN CALL", AssetClass: "OPT", Currency: "EUR", OrderID: id, ExecutionID: id, ExecutedAt: day("2026-01-05").AddDate(0, 0, i), Side: side, OpenClose: markers[i], Quantity: &delta, Price: new(float64(2)), Multiplier: new(float64(100)), Commission: new(float64(-1)), Taxes: new(float64(0)), RealizedPNL: &pnl, LevelOfDetail: "EXECUTION"})
	}
	return st
}
