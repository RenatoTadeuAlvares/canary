package ibkr

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSnapshotPositionsRequiresMatchingPositionEnd(t *testing.T) {
	c, conn, _ := newHistoricalFeeRateTestConnector(t)
	result := make(chan error, 1)
	go func() {
		_, err := c.SnapshotPositions(context.Background())
		result <- err
	}()
	waitForSnapshotHandler(t, conn, msgPositionEnd)
	epoch := conn.BrokerSessionEpoch()
	conn.processMessageAtEpoch(conn.encodeMsg(msgPosition, "3", "DUT111026", "100", "ABC", "STK", "1", "SMART", "GBP", "ABC", "ABC", "2", "10"), epoch)
	conn.processMessageAtEpoch(conn.encodeMsg(msgPositionEnd, "1"), epoch)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotExecutionsRequiresMatchingExecDetailsEnd(t *testing.T) {
	c, conn, _ := newHistoricalFeeRateTestConnector(t)
	result := make(chan ExecutionSnapshot, 1)
	errs := make(chan error, 1)
	go func() {
		snapshot, err := c.SnapshotExecutions(context.Background(), "DUT111026")
		result <- snapshot
		errs <- err
	}()
	waitForSnapshotHandler(t, conn, msgExecDetailsEnd)
	epoch := conn.BrokerSessionEpoch()
	conn.processMessageAtEpoch(conn.encodeMsg(msgExecDetails, "11", "1", "1001", "265598", "ABC", "STK", "", "0", "", "1", "SMART", "GBP", "ABC", "exec-1", "20260912 10:00:00", "DUT111026", "SMART", "BOT", "2", "100", "987654", "31", "0", "2", "100", "reconciliation-test"), epoch)
	conn.processMessageAtEpoch(conn.encodeMsg(msgExecDetailsEnd, "1", "1"), epoch)
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	if snapshot := <-result; !snapshot.Complete || len(snapshot.Executions) != 1 || snapshot.Executions[0].ExecID != "exec-1" {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}

func TestSnapshotExecutionsTimeoutFailsClosed(t *testing.T) {
	c, _, _ := newHistoricalFeeRateTestConnector(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := c.SnapshotExecutions(ctx, "DUT111026")
	if !errors.Is(err, ErrExecutionSnapshotIncomplete) {
		t.Fatalf("err=%v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v does not retain the caller deadline", err)
	}
}

func TestSnapshotPositionsPreviousEpochCompletionFailsClosed(t *testing.T) {
	c, conn, _ := newHistoricalFeeRateTestConnector(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	oldEpoch := conn.BrokerSessionEpoch()
	go func() { _, err := c.SnapshotPositions(ctx); result <- err }()
	waitForSnapshotHandler(t, conn, msgPositionEnd)
	conn.resetOrderIDReadiness()
	conn.processMessageAtEpoch(conn.encodeMsg(msgPositionEnd, "1"), oldEpoch)
	if err := <-result; !errors.Is(err, ErrPositionSnapshotIncomplete) {
		t.Fatalf("err=%v", err)
	}
}

func TestSnapshotExecutionsMixedGenerationFailsClosed(t *testing.T) {
	c, conn, _ := newHistoricalFeeRateTestConnector(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	oldEpoch := conn.BrokerSessionEpoch()
	go func() { _, err := c.SnapshotExecutions(ctx, "DUT111026"); result <- err }()
	waitForSnapshotHandler(t, conn, msgExecDetailsEnd)
	conn.resetOrderIDReadiness()
	conn.processMessageAtEpoch(conn.encodeMsg(msgExecDetails, "11", "1", "1001", "265598", "ABC", "STK", "", "0", "", "1", "SMART", "GBP", "ABC", "exec-old", "20260912 10:00:00", "DUT111026", "SMART", "BOT", "2", "100", "987654", "31", "0", "2", "100", "reconciliation-test"), oldEpoch)
	if err := <-result; !errors.Is(err, ErrExecutionSnapshotIncomplete) {
		t.Fatalf("err=%v", err)
	}
}

func TestSnapshotExecutionsPreviousEpochCompletionFailsClosed(t *testing.T) {
	c, conn, _ := newHistoricalFeeRateTestConnector(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	oldEpoch := conn.BrokerSessionEpoch()
	go func() { _, err := c.SnapshotExecutions(ctx, "DUT111026"); result <- err }()
	waitForSnapshotHandler(t, conn, msgExecDetailsEnd)
	conn.resetOrderIDReadiness()
	conn.processMessageAtEpoch(conn.encodeMsg(msgExecDetailsEnd, "1", "1"), oldEpoch)
	if err := <-result; !errors.Is(err, ErrExecutionSnapshotIncomplete) {
		t.Fatalf("err=%v", err)
	}
}

func TestSnapshotExecutionsDisconnectBeforeCompletionFailsClosed(t *testing.T) {
	c, conn, _ := newHistoricalFeeRateTestConnector(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := c.SnapshotExecutions(ctx, "DUT111026"); result <- err }()
	waitForSnapshotHandler(t, conn, msgExecDetailsEnd)
	conn.status = StatusDisconnected
	if err := <-result; !errors.Is(err, ErrExecutionSnapshotIncomplete) {
		t.Fatalf("err=%v", err)
	}
}

func TestSnapshotPositionsDisconnectBeforeCompletionFailsClosed(t *testing.T) {
	c, conn, _ := newHistoricalFeeRateTestConnector(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := c.SnapshotPositions(ctx); result <- err }()
	waitForSnapshotHandler(t, conn, msgPositionEnd)
	conn.status = StatusDisconnected
	if err := <-result; !errors.Is(err, ErrPositionSnapshotIncomplete) {
		t.Fatalf("err=%v", err)
	}
}

func waitForSnapshotHandler(t *testing.T, conn *Connection, msgID int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		conn.handlersMu.RLock()
		count := len(conn.msgHandlers[msgID])
		conn.handlersMu.RUnlock()
		if count > 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("snapshot handler was not registered")
}
