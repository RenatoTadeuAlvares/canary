package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/osauer/canary/v2/internal/rpc"
)

type riskReadConn struct {
	calls  []string
	result any
	err    error
}

func (c *riskReadConn) Call(_ context.Context, method string, _ any, out any) error {
	c.calls = append(c.calls, method)
	if c.err != nil {
		return c.err
	}
	raw, err := json.Marshal(c.result)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}
func (*riskReadConn) Stream(context.Context, string, any, func(json.RawMessage) error) error {
	return errors.New("unexpected stream")
}

func TestRegimeReadPreservesDetailedAuthority(t *testing.T) {
	want := rpc.RegimeSnapshotResult{AuthorityHealth: &rpc.RegimeAuthorityHealth{Status: rpc.RegimeAuthorityStale, LastSuccessAt: new(time.Date(2026, 9, 4, 20, 0, 0, 0, time.UTC)), LastSuccessAgeSeconds: new(int64(60))}, VIXTermStructure: rpc.RegimeVIXTerm{Status: "stale", Ratio: new(1.1), RegimeIndicatorMeta: rpc.RegimeIndicatorMeta{Band: "red", Eligibility: &rpc.RegimeEligibility{Eligible: false, Reasons: []string{"not current"}}}}, FundingStress: rpc.RegimeFundingStress{Status: "unavailable"}}
	conn := &riskReadConn{result: want}
	var out, stderr bytes.Buffer
	env := &Env{Conn: conn, Stdout: &out, Stderr: &stderr}
	if code := Run(t.Context(), env, "regime", []string{"--json"}); code != 0 {
		t.Fatal(code, stderr.String())
	}
	var got rpc.RegimeSnapshotResult
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(conn.calls, []string{rpc.MethodRegimeSnapshot}) {
		t.Fatal("regime evidence changed or wrong RPC")
	}
	out.Reset()
	renderRegime(env, want, true)
	for _, needle := range []string{"stale", "not eligible", "not current", "Funding", "unavailable", "VVIX", "Breadth", "USD/JPY", "Gamma", "HYG/SPY", "Credit spreads"} {
		if !strings.Contains(out.String(), needle) {
			t.Fatalf("missing %q: %s", needle, out.String())
		}
	}
}

func TestRiskReadsRejectUnsupportedInputsBeforeRPC(t *testing.T) {
	for _, tc := range []struct {
		cmd  string
		args []string
	}{{"regime", []string{"history"}}, {"stress", []string{"history"}}, {"regime", []string{"--force"}}, {"regime", []string{"--profiles"}}, {"stress", []string{"--submit"}}} {
		conn := &riskReadConn{}
		var out bytes.Buffer
		if Run(t.Context(), &Env{Conn: conn, Stdout: &out, Stderr: &out}, tc.cmd, tc.args) == 0 || len(conn.calls) != 0 {
			t.Fatalf("unsupported input reached RPC: %+v", tc)
		}
	}
}

func TestStressReadFailsOnMissingAccountAndSanitizesHumanEvidence(t *testing.T) {
	conn := &riskReadConn{err: errors.New("account unavailable")}
	var out bytes.Buffer
	env := &Env{Conn: conn, Stdout: &out, Stderr: &out}
	if Run(t.Context(), env, "stress", []string{"--json"}) == 0 || !reflect.DeepEqual(conn.calls, []string{rpc.MethodAccountSummary}) {
		t.Fatal("missing account was accepted")
	}
	out.Reset()
	renderStress(env, rpc.StressResult{Summary: "unavailable\x1b[2J\nforged", Rows: []rpc.StressRow{{Title: "Margin", Evidence: "unknown"}}, MarketIndicators: []rpc.StressMarketIndicator{{Name: "Funding", Status: "n/a"}}, NotExecution: "Read-only"}, true)
	if strings.Contains(out.String(), "\x1b") || strings.Contains(out.String(), "\nforged") || !strings.Contains(out.String(), "unknown") || !strings.Contains(out.String(), "Funding") {
		t.Fatal("unsafe or lost evidence", out.String())
	}
}

func TestRegimeBreadthMissingIsNotMeasuredZero(t *testing.T) {
	for _, tc := range []struct {
		status   string
		coverage int
		want     string
	}{{rpc.RegimeStatusUnavailable, 0, "50dma% unavailable"}, {rpc.RegimeStatusComputing, 0, "50dma% unavailable"}, {rpc.RegimeStatusOK, 500, "50dma% 0.00"}, {rpc.RegimeStatusStale, 500, "50dma% 0.00"}} {
		var out bytes.Buffer
		renderRegime(&Env{Stdout: &out}, rpc.RegimeSnapshotResult{Breadth: rpc.RegimeBreadth{Status: tc.status, Envelope: rpc.BreadthSPXResult{Coverage50: tc.coverage}}}, false)
		if !strings.Contains(out.String(), tc.want) {
			t.Fatalf("%+v: %s", tc, out.String())
		}
	}
}

func TestRegimeProfilesAreExplicitAndScalarsSurvive(t *testing.T) {
	for _, profiles := range []bool{false, true} {
		want := rpc.RegimeSnapshotResult{GammaZero: rpc.RegimeGammaZero{Envelope: rpc.GammaZeroSPXResult{Result: &rpc.GammaZeroComputed{SpotUnderlying: 100, Profile: []rpc.GammaProfilePoint{{Spot: 100, GEX: 2}}}}}}
		conn := &riskReadConn{result: want}
		var out bytes.Buffer
		args := []string{"--json"}
		if profiles {
			args = append(args, "--profiles")
		}
		if Run(t.Context(), &Env{Conn: conn, Stdout: &out, Stderr: &out}, "regime", args) != 0 {
			t.Fatal(out.String())
		}
		var got rpc.RegimeSnapshotResult
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.GammaZero.Envelope.Result.SpotUnderlying != 100 || (len(got.GammaZero.Envelope.Result.Profile) > 0) != profiles {
			t.Fatal("profile selection lost scalar or failed opt-in")
		}
	}
}
