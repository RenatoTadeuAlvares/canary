package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/osauer/canary/v2/internal/rpc"
	"testing"
)

type marketReadConn struct{ params rpc.MarketHistoryParams }

func (c *marketReadConn) Call(_ context.Context, _ string, p, out any) error {
	c.params = p.(rpc.MarketHistoryParams)
	return errors.New("synthetic stop")
}
func (*marketReadConn) Stream(context.Context, string, any, func(json.RawMessage) error) error {
	return errors.New("unexpected stream")
}
func TestMarketRangeHoistingCannotReplaceRangeWithJSONFlag(t *testing.T) {
	c := &marketReadConn{}
	var out bytes.Buffer
	Run(t.Context(), &Env{Conn: c, Stdout: &out, Stderr: &out}, "market", []string{"--symbol", "SYNTH", "--range", "1D", "--json"})
	if c.params.Range != "1D" || c.params.Contract.Symbol != "SYNTH" {
		t.Fatal("chart identity/range changed by CLI flag hoisting")
	}
}
