package ibkr

import (
	"context"
	"testing"
	"time"
)

func TestDisplayClocksScopeAndReset(t *testing.T) {
	c := NewConnection(&ConnectionConfig{})
	c.account = "U_SYNTHETIC"
	c.handleAccountValue([]string{"6", "2", "NetLiquidation", "100", "USD", "U_SYNTHETIC"})
	if c.displayAccount != "U_SYNTHETIC" || c.accountValueTimes["NetLiquidation_USD"].IsZero() {
		t.Fatal("attributed receipt not recorded")
	}
	before := c.accountValueTimes["NetLiquidation_USD"]
	c.handleAccountValue([]string{"6", "2", "NetLiquidation", "999", "USD", "U_FOREIGN"})
	if c.displayAccountValues["NetLiquidation_USD"] != "100" || !before.Equal(c.accountValueTimes["NetLiquidation_USD"]) {
		t.Fatal("foreign account corrupted display")
	}
	c.invalidateUnstampedObservationAuthority()
	if c.displayAccount != "" || len(c.displayAccountValues) != 0 || len(c.accountValueTimes) != 0 {
		t.Fatal("prior session account survived reset")
	}
	sub := &Subscription{}
	at := time.Now()
	stampDisplayPrice(sub, 4, at)
	stampDisplayPrice(sub, 1, at.Add(time.Second))
	stampDisplayPrice(sub, 9, at.Add(time.Minute))
	if !sub.LastAt.Equal(at) || !sub.BidAt.Equal(at.Add(time.Second)) {
		t.Fatal("unrelated tick refreshed last price")
	}
}
func TestDisplayNotificationBroadcastAndStringDispatch(t *testing.T) {
	c := NewConnection(&ConnectionConfig{})
	var d displayChanges
	c.displayChanged = d.notify
	a, b := d.watch(), d.watch()
	called := false
	c.RegisterHandler(msgTickString, func(_ []string) { called = true })
	c.processMessageAtEpoch([]byte("46\x001\x001\x0045\x001700000000\x00"), c.BrokerSessionEpoch())
	if !called {
		t.Fatal("broker timestamp was discarded")
	}
	for _, ch := range []<-chan struct{}{a, b} {
		select {
		case <-ch:
		default:
			t.Fatal("lost cache notification")
		}
	}
	select {
	case <-d.watch():
		t.Fatal("new watch already closed")
	default:
	}
}

func TestDisplayRetiredSessionCannotRemoveSuccessorQuote(t *testing.T) {
	c := NewConnector(&ConnectorConfig{})
	c.ready = true
	c.conn.status = StatusConnected
	binding := ConnectorSessionBinding{connector: c, connection: c.conn, epoch: c.conn.BrokerSessionEpoch()}
	c.conn.brokerSessionEpoch.Add(1)
	c.subscriptions["SYNTH"] = &Subscription{ReqID: 42}
	c.reqIDMap[42] = "SYNTH"
	if err := c.UnsubscribeSharedMarketDataForSession(context.Background(), binding, "SYNTH"); err != nil {
		t.Fatal(err)
	}
	if c.subscriptions["SYNTH"] == nil || c.reqIDMap[42] != "SYNTH" {
		t.Fatal("retired session removed successor quote")
	}
}
