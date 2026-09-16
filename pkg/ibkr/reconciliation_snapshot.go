package ibkr

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PositionSnapshot is an epoch-bound, read-only reqPositions receipt. Complete
// is true only when positionEnd was received on the request's exact session.
type PositionSnapshot struct {
	Complete   bool
	Positions  []*RawPosition
	AsOf       time.Time
	Session    ConnectorSessionBinding
	Generation uint64
}

// ExecutionSnapshot is an epoch-bound, read-only reqExecutions receipt.
// Complete is true only when execDetailsEnd matched the request ID and session.
type ExecutionSnapshot struct {
	Complete   bool
	Executions []OrderLifecycleEvent
	AsOf       time.Time
	Session    ConnectorSessionBinding
	Generation uint64
}

var (
	ErrPositionSnapshotIncomplete  = errors.New("position snapshot incomplete")
	ErrExecutionSnapshotIncomplete = errors.New("execution snapshot incomplete")
	ErrSnapshotContradiction       = errors.New("reconciliation snapshot contradictory")
)

// SnapshotPositions issues the read-only reqPositions request and returns only
// after positionEnd is positively received on the exact captured session.
func (c *Connector) SnapshotPositions(ctx context.Context) (PositionSnapshot, error) {
	if ctx == nil {
		return PositionSnapshot{}, errors.New("context is nil")
	}
	binding, ok := c.CaptureSession()
	if !ok {
		return PositionSnapshot{}, ErrPositionSnapshotIncomplete
	}
	c.positionSnapshotMu.Lock()
	defer c.positionSnapshotMu.Unlock()
	var mu sync.Mutex
	completed, mixed := false, false
	done := make(chan struct{})
	rowHandler := binding.connection.RegisterHandlerAtEpoch(msgPosition, func(_ []string, epoch uint64) {
		if epoch != binding.epoch {
			mu.Lock()
			mixed = true
			mu.Unlock()
		}
	})
	endHandler := binding.connection.RegisterHandlerAtEpoch(msgPositionEnd, func(_ []string, epoch uint64) {
		mu.Lock()
		defer mu.Unlock()
		if epoch != binding.epoch {
			mixed = true
			return
		}
		if !completed {
			completed = true
			close(done)
		}
	})
	defer binding.connection.UnregisterHandler(msgPosition, rowHandler)
	defer binding.connection.UnregisterHandler(msgPositionEnd, endHandler)
	if err := binding.connection.RequestPositions(); err != nil {
		return PositionSnapshot{AsOf: time.Now().UTC(), Session: binding}, err
	}
	select {
	case <-done:
	case <-ctx.Done():
		return PositionSnapshot{AsOf: time.Now().UTC(), Session: binding}, fmt.Errorf("%w: %v", ErrPositionSnapshotIncomplete, ctx.Err())
	}
	mu.Lock()
	bad := mixed
	mu.Unlock()
	if bad || !c.SessionCurrent(binding) {
		return PositionSnapshot{AsOf: time.Now().UTC(), Session: binding}, ErrSnapshotContradiction
	}
	rows := binding.connection.GetPositionsSnapshot()
	positions := make([]*RawPosition, 0, len(rows))
	for _, row := range rows {
		positions = append(positions, row)
	}
	return PositionSnapshot{Complete: true, Positions: positions, AsOf: time.Now().UTC(), Session: binding, Generation: binding.epoch}, nil
}

// SnapshotExecutions issues an account-bound, read-only reqExecutions request.
// An empty callback set is authoritative only after matching execDetailsEnd.
func (c *Connector) SnapshotExecutions(ctx context.Context, account string) (ExecutionSnapshot, error) {
	if ctx == nil || !accountCodeConcrete(account) {
		return ExecutionSnapshot{}, ErrExecutionSnapshotIncomplete
	}
	binding, ok := c.CaptureSession()
	if !ok {
		return ExecutionSnapshot{}, ErrExecutionSnapshotIncomplete
	}
	reqID, _, err := binding.connection.reserveNextRequestIDForEpoch(binding.epoch)
	if err != nil {
		return ExecutionSnapshot{Session: binding}, err
	}
	var mu sync.Mutex
	completed, mixed, contradictory := false, false, false
	seen := map[string]OrderLifecycleEvent{}
	done := make(chan struct{})
	rowHandler := binding.connection.RegisterHandlerAtEpoch(msgExecDetails, func(fields []string, epoch uint64) {
		mu.Lock()
		defer mu.Unlock()
		if epoch != binding.epoch {
			mixed = true
			return
		}
		ev, ok := ParseOrderLifecycleEvent(fields)
		if !ok || ev.RequestID != reqID || completed {
			return
		}
		if !strings.EqualFold(strings.TrimSpace(ev.Account), strings.TrimSpace(account)) || strings.TrimSpace(ev.ExecID) == "" {
			contradictory = true
			return
		}
		if previous, exists := seen[ev.ExecID]; exists {
			if !reflect.DeepEqual(previous, ev) {
				contradictory = true
			}
			return
		}
		seen[ev.ExecID] = ev
	})
	endHandler := binding.connection.RegisterHandlerAtEpoch(msgExecDetailsEnd, func(fields []string, epoch uint64) {
		mu.Lock()
		defer mu.Unlock()
		if epoch != binding.epoch || len(fields) < 3 {
			mixed = true
			return
		}
		endID, parseErr := strconv.Atoi(strings.TrimSpace(fields[2]))
		if parseErr != nil || endID != reqID {
			contradictory = true
			return
		}
		if !completed {
			completed = true
			close(done)
		}
	})
	defer binding.connection.UnregisterHandler(msgExecDetails, rowHandler)
	defer binding.connection.UnregisterHandler(msgExecDetailsEnd, endHandler)
	// reqExecutions version 3: reqID followed by a filter scoped to account.
	msg := binding.connection.encodeMsg(reqExecutions, "3", reqID, "", account, "", "", "", "", "")
	if err := binding.connection.sendMessageWithTypeContextForEpoch(ctx, msg, RequestTypeGeneral, binding.epoch, true); err != nil {
		return ExecutionSnapshot{AsOf: time.Now().UTC(), Session: binding}, err
	}
	select {
	case <-done:
	case <-ctx.Done():
		return ExecutionSnapshot{AsOf: time.Now().UTC(), Session: binding}, fmt.Errorf("%w: %w", ErrExecutionSnapshotIncomplete, ctx.Err())
	}
	mu.Lock()
	bad := mixed || contradictory
	executions := make([]OrderLifecycleEvent, 0, len(seen))
	for _, ev := range seen {
		executions = append(executions, ev)
	}
	mu.Unlock()
	if bad || !c.SessionCurrent(binding) {
		return ExecutionSnapshot{AsOf: time.Now().UTC(), Session: binding}, ErrSnapshotContradiction
	}
	return ExecutionSnapshot{Complete: true, Executions: executions, AsOf: time.Now().UTC(), Session: binding, Generation: binding.epoch}, nil
}
