package daemon

import (
	"testing"
	"time"

	"github.com/osauer/canary/v2/internal/marketcal"
	"github.com/osauer/canary/v2/internal/rpc"
	ibkrlib "github.com/osauer/canary/v2/pkg/ibkr"
)

func TestQuoteTradeClockCannotBecomeAReceiptClock(t *testing.T) {
	received := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	traded := received.Add(-16 * time.Hour)
	q := &rpc.Quote{AsOf: received, Contract: rpc.ContractParams{Symbol: "SYNTH", SecType: "STK", Currency: "USD"}}
	fillQuoteMarketData(q, &ibkrlib.MarketData{Last: 123, LastTradeTime: traded})
	new(Server).decorateQuote(q, marketcal.MarketUSEquity)
	if !q.TradeAt.Equal(traded) {
		t.Fatal("quote decoration replaced the broker trade clock with local time")
	}
	q.AsOf = received.Add(time.Hour)
	fillQuoteMarketData(q, &ibkrlib.MarketData{Last: 123})
	new(Server).decorateQuote(q, marketcal.MarketUSEquity)
	if !q.TradeAt.IsZero() {
		t.Fatal("missing broker trade time inherited an earlier trade or receipt clock")
	}
}
