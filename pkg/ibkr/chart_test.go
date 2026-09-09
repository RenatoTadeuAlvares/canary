package ibkr

import (
	"strconv"
	"testing"
	"time"
)

func TestChartChunksKeepExactTimesUntilEnd(t *testing.T) {
	c := NewConnector(&ConnectorConfig{})
	conn := NewConnection(nil)
	t.Cleanup(func() { conn.rateLimiter.Stop() })
	conn.status = StatusConnected
	setServerVersionReady(conn, maxClientVersion)
	c.conn, c.running, c.ready = conn, true, true
	req := c.createHistoricalRequestWithOptions(7109, "SYNTH", historicalRequestOptions{strictDaily: true, waitForEnd: true, formatDate: 2, chartBarSize: "5 mins"})
	for _, at := range []string{"1788958800", "1788959100"} {
		c.handleHistoricalData([]string{strconv.Itoa(msgHistoricalData), "7109", "1", at, "10", "12", "9", "11", "100", "10.5", "4", ""})
		select {
		case <-req.result:
			t.Fatal("chart completed before its end receipt")
		default:
		}
	}
	c.handleHistoricalDataEnd([]string{strconv.Itoa(msgHistoricalDataEnd), "7109", "1788958800", "1788959100", ""})
	got := <-req.result
	if got.err != nil || len(got.bars) != 2 || got.bars[1].Time.Unix() != 1788959100 {
		t.Fatalf("incomplete or misdated chart: %+v", got)
	}
}
func TestChartResolutionCannotAdoptAnotherSameSymbolContract(t *testing.T) {
	requested := Contract{ConID: 91, Symbol: "SYNTH", SecType: "STK", Exchange: "SMART", Currency: "USD"}
	for _, d := range []ContractDetailsLite{{ConID: 92, Symbol: "SYNTH", SecType: "STK", Exchange: "SMART", Currency: "USD"}, {ConID: 91, Symbol: "SYNTH", SecType: "STK", Exchange: "SMART", Currency: "EUR"}} {
		if _, err := exactOrderContract(requested, []ContractDetailsLite{d}); err == nil {
			t.Fatal("different identity accepted")
		}
	}
}

func TestHistoricalPayloadRejectsExtraFields(t *testing.T) {
	for _, trailing := range [][]string{{"extra"}, {"", ""}} {
		if historicalPayloadConsumed(trailing, 0) {
			t.Fatal("corrupt trailing fields accepted")
		}
	}
}

func TestFrontFutureUsesBrokerExpiryAndRejectsAmbiguity(t *testing.T) {
	now := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	want := Contract{Symbol: "SYNTH", SecType: "FUT", Currency: "USD", Exchange: "CME"}
	d := ContractDetailsLite{ConID: 1, Symbol: "SYNTH", TradingClass: "SYNTH", SecType: "FUT", Currency: "USD", Exchange: "CME", Expiry: "20260918"}
	later := d
	later.ConID = 2
	later.Expiry = "20261218"
	got, err := selectFrontFuture(want, []ContractDetailsLite{later, d}, now)
	if err != nil || got.ConID != 1 || got.Expiry != "20260918" {
		t.Fatal("wrong front contract", err)
	}
	later.Expiry = d.Expiry
	if _, err = selectFrontFuture(want, []ContractDetailsLite{later, d}, now); err == nil {
		t.Fatal("ambiguous future accepted")
	}
}
