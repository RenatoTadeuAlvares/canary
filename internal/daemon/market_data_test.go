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
