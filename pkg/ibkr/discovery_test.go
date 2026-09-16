package ibkr

import (
	"context"
	"encoding/binary"
	"strconv"
	"strings"
	"testing"
	"time"
)

func discoveryFrames(t *testing.T, b *safeBuffer) [][]string {
	t.Helper()
	raw := b.Bytes()
	var frames [][]string
	for len(raw) >= 4 {
		n := int(binary.BigEndian.Uint32(raw[:4]))
		if len(raw) < n+4 {
			break
		}
		if n < 4 {
			t.Fatal("short message")
		}
		fields := []string{strconv.Itoa(int(binary.BigEndian.Uint32(raw[4:8])))}
		fields = append(fields, strings.Split(strings.TrimSuffix(string(raw[8:4+n]), "\x00"), "\x00")...)
		frames = append(frames, fields)
		raw = raw[4+n:]
	}
	return frames
}

func TestDiscoveryPositiveCompletionAndReadOnlyWire(t *testing.T) {
	c, conn, wire := newBackendConnectivityConnector(t, nil)
	setServerVersionReady(conn, maxClientVersion)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan DiscoveryReceipt, 1)
	errs := make(chan error, 1)
	go func() { r, e := c.DiscoverContracts(ctx, "VUAG"); done <- r; errs <- e }()
	waitForBackendReplay(t, time.Second, func() bool { return len(discoveryFrames(t, wire)) >= 1 })
	frames := discoveryFrames(t, wire)
	if frames[0][0] != "81" || frames[0][2] != "VUAG" {
		t.Fatalf("unexpected request %v", frames)
	}
	epoch := conn.BrokerSessionEpoch()
	conn.processMessageAtEpoch(conn.encodeMsg(79, frames[0][1], 1, 424242, "VUAG", "STK", "LSEETF", "GBP", 0, "Vanguard", ""), epoch)
	waitForBackendReplay(t, time.Second, func() bool { return len(discoveryFrames(t, wire)) >= 2 })
	frames = discoveryFrames(t, wire)
	requestID := frames[1][2]
	fields := syntheticStockContractDetailsFields(maxClientVersion, "ETF")
	fields[1] = requestID
	for i, v := range fields {
		switch v {
		case "SYNTH1":
			fields[i] = "VUAG"
		case "SMART", "PRIMARY_CODE":
			fields[i] = "LSEETF"
		case "USD":
			fields[i] = "GBP"
		case "ID_TAG":
			fields[i] = "ISIN"
		case "ID_VALUE":
			fields[i] = "IE00BFMXXD54"
		}
	}
	args := make([]any, len(fields))
	for i := range fields {
		args[i] = fields[i]
	}
	args[0] = msgContractData
	conn.processMessageAtEpoch(conn.encodeMsg(args...), epoch)
	select {
	case <-done:
		t.Fatal("completed without contractDetailsEnd")
	default:
	}
	conn.processMessageAtEpoch(conn.encodeMsg(msgContractDataEnd, 1, requestID), epoch)
	r := <-done
	if e := <-errs; e != nil || !r.Complete || len(r.Details) != 1 || r.Details[0].ISIN != "IE00BFMXXD54" {
		t.Fatalf("receipt=%+v err=%v", r, e)
	}
	for _, f := range discoveryFrames(t, wire) {
		if f[0] != "81" && f[0] != "9" {
			t.Fatalf("non-discovery request %v", f)
		}
		for _, v := range f {
			if v == "SMART" || v == "USD" {
				t.Fatalf("substitute route/currency %v", f)
			}
		}
	}
}

func TestDiscoveryFailuresNeverComplete(t *testing.T) {
	for _, mode := range []string{"timeout", "stale", "disconnect", "mixed", "details_timeout", "details_stale", "details_malformed", "details_wrong_request"} {
		t.Run(mode, func(t *testing.T) {
			c, conn, wire := newBackendConnectivityConnector(t, nil)
			setServerVersionReady(conn, maxClientVersion)
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			done := make(chan bool, 1)
			go func() { r, e := c.DiscoverContracts(ctx, "VUAG"); done <- e != nil && !r.Complete }()
			waitForBackendReplay(t, time.Second, func() bool { return len(discoveryFrames(t, wire)) > 0 })
			epoch := conn.BrokerSessionEpoch()
			id := discoveryFrames(t, wire)[0][1]
			switch mode {
			case "stale":
				conn.processMessageAtEpoch(conn.encodeMsg(79, id, 0), epoch+1)
			case "disconnect":
				conn.statusMu.Lock()
				conn.status = StatusDisconnected
				conn.statusMu.Unlock()
			case "mixed":
				conn.resetOrderIDReadiness()
				conn.processMessageAtEpoch(conn.encodeMsg(79, id, 0), epoch)
			case "details_timeout", "details_stale", "details_malformed", "details_wrong_request":
				conn.processMessageAtEpoch(conn.encodeMsg(79, id, 1, 424242, "VUAG", "STK", "LSEETF", "GBP", 0, "", ""), epoch)
				waitForBackendReplay(t, time.Second, func() bool { return len(discoveryFrames(t, wire)) >= 2 })
				requestID := discoveryFrames(t, wire)[1][2]
				if mode == "details_stale" {
					conn.resetOrderIDReadiness()
					conn.processMessageAtEpoch(conn.encodeMsg(msgContractDataEnd, 1, requestID), epoch)
				}
				if mode == "details_malformed" {
					conn.processMessageAtEpoch(conn.encodeMsg(msgContractData, requestID, "truncated"), epoch)
					conn.processMessageAtEpoch(conn.encodeMsg(msgContractDataEnd, 1, requestID), epoch)
				}
				if mode == "details_wrong_request" {
					conn.processMessageAtEpoch(conn.encodeMsg(msgContractDataEnd, 1, "99999"), epoch)
				}
			}
			if !<-done {
				t.Fatal("unsafe discovery completed")
			}
		})
	}
}

func TestDiscoveryCandidatesStrictParsing(t *testing.T) {
	for _, version := range []int{175, 176, maxClientVersion} {
		fields := []string{"79", "1", "2", "10", "VUAG", "STK", "LSEETF", "GBP", "0"}
		if version >= 176 {
			fields = append(fields, "description", "")
		}
		fields = append(fields, "11", "VUSA", "STK", "LSE", "GBP", "0")
		if version >= 176 {
			fields = append(fields, "", "")
		}
		rows, e := parseDiscoveryCandidates(fields, version)
		if e != nil || len(rows) != 2 {
			t.Fatalf("version %d rows=%v err=%v", version, rows, e)
		}
		for n := 0; n < len(fields); n++ {
			if _, e := parseDiscoveryCandidates(fields[:n], version); e == nil {
				t.Fatalf("accepted truncated frame at %s", strconv.Itoa(n))
			}
		}
	}
	rows, e := parseDiscoveryCandidates([]string{"79", "1", "0"}, maxClientVersion)
	if e != nil || len(rows) != 0 {
		t.Fatal("positive empty response rejected")
	}
}
