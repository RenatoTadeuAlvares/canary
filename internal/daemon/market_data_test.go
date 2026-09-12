package daemon

import (
	"github.com/osauer/canary/v2/internal/rpc"
	"testing"
	"time"
)

func TestMarketHistoryCannotTurnDailyBarsIntoIntraday(t *testing.T) {
	for _, r := range []string{"", "All", "1S", "unbounded"} {
		if _, _, err := marketHistoryWindow(r, time.Now()); err == nil {
			t.Fatal("unbounded history accepted")
		}
	}
	_, interval, err := marketHistoryWindow("1D", time.Now())
	if err != nil || interval != "5 mins" {
		t.Fatal("intraday request became daily data")
	}
}
func TestUnderlyingQuoteRetainsHeldStockIdentity(t *testing.T) {
	g := rpc.PositionGroup{Underlying: "SYNTH", Stock: &rpc.PositionView{ConID: 91, Symbol: "SYNTH", SecType: "STOCK", Currency: "EUR", Exchange: "IBIS"}}
	c, ok := rpc.UnderlyingMarketContract(g)
	if !ok || c.ConID != 91 || c.Currency != "EUR" || c.Exchange != "IBIS" {
		t.Fatal("held identity lost")
	}
	g.Stock = nil
	g.Options = []rpc.PositionView{{Symbol: "SYNTH", Currency: "USD"}}
	c, ok = rpc.UnderlyingMarketContract(g)
	if !ok || c.SecType != "STK" || c.ConID != 0 {
		t.Fatal("option identity masquerades as underlying")
	}
}

func TestHistoryCalendarRangeDoesNotExposeRoundedBrokerLookback(t *testing.T) {
	now := time.Date(2026, 9, 9, 16, 0, 0, 0, time.UTC)
	for r, want := range map[string]string{"1Y": "2025-09-09", "5Y": "2021-09-09", "YTD": "2026-01-01", "1M": "2026-08-09"} {
		if got := marketHistoryStart(r, now).Format("2006-01-02"); got != want {
			t.Fatalf("%s: %s != %s", r, got, want)
		}
	}
}

func TestMarketHistoryClampsMonthEnds(t *testing.T) {
	for _, tc := range []struct{ now, rangeName, want string }{
		{"2026-03-31", "1M", "2026-02-28"},
		{"2024-03-31", "1M", "2024-02-29"},
		{"2026-10-31", "6M", "2026-04-30"},
		{"2024-02-29", "1Y", "2023-02-28"},
		{"2024-02-29", "5Y", "2019-02-28"},
	} {
		now, _ := time.Parse("2006-01-02", tc.now)
		now = now.Add(16*time.Hour + 17*time.Minute)
		got := marketHistoryStart(tc.rangeName, now)
		if got.Format("2006-01-02") != tc.want || got.Hour() != 16 || got.Minute() != 17 {
			t.Fatalf("%s %s: got %s, want %s at 16:17", tc.now, tc.rangeName, got, tc.want)
		}
	}
}
