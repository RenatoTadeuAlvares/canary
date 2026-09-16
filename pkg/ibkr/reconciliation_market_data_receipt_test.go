package ibkr

import "testing"

func TestMarketDataSnapshotRetainsDelayedBidAskLastAndSizeReceipts(t *testing.T) {
	c := NewConnector(&ConnectorConfig{})
	c.subMu.Lock()
	c.reqIDMap[42] = "VUAG"
	c.subscriptions["VUAG"] = &Subscription{Symbol: "VUAG", ReqID: 42}
	c.subMu.Unlock()

	c.handleTickPrice([]string{"1", "2", "42", "66", "120.10"})
	c.handleTickPrice([]string{"1", "2", "42", "67", "120.20"})
	c.handleTickPrice([]string{"1", "2", "42", "68", "120.15"})
	c.handleTickSize([]string{"2", "6", "42", "69", "0"})
	c.handleTickSize([]string{"2", "6", "42", "70", "25"})
	// Delayed tick type 75 is the prior close price, not delayed last size.
	c.handleTickSize([]string{"2", "6", "42", "75", "999"})
	if got := c.MarketDataSnapshot()["VUAG"]; got.LastSizeObserved {
		t.Fatalf("delayed close tick incorrectly marked last size received: %+v", got)
	}
	c.handleTickSize([]string{"2", "6", "42", "71", "10"})

	got := c.MarketDataSnapshot()["VUAG"]
	if got == nil || !got.BidObserved || !got.AskObserved || !got.LastObserved || !got.BidSizeObserved || !got.AskSizeObserved || !got.LastSizeObserved {
		t.Fatalf("receipt flags = %+v, want every delayed required field observed", got)
	}
	if got.BidSize != 0 {
		t.Fatalf("BidSize=%d, want observed zero retained", got.BidSize)
	}
}
