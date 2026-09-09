package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/osauer/canary/v2/internal/rpc"
	ibkrlib "github.com/osauer/canary/v2/pkg/ibkr"
	"sync"
	"time"
)

type marketDataCache struct {
	mu      sync.Mutex
	history map[string]*marketHistoryEntry
	slots   chan struct{}
}
type marketHistoryEntry struct {
	done    chan struct{}
	expires time.Time
	binding ibkrlib.HistoricalSessionBinding
	result  *rpc.MarketHistoryResult
	err     error
}

func (s *Server) handleMarketHistory(ctx context.Context, req *rpc.Request) (*rpc.MarketHistoryResult, error) {
	var p rpc.MarketHistoryParams
	if err := decodeParams(req.Params, &p); err != nil {
		return nil, err
	}
	if _, _, err := marketHistoryWindow(p.Range, time.Now()); err != nil {
		return nil, err
	}
	c := s.gatewayConnector()
	if c == nil {
		return nil, s.gatewayUnavailableError()
	}
	binding, ok := c.CaptureHistoricalSession()
	if !ok {
		return nil, s.gatewayUnavailableError()
	}
	keyBytes, _ := json.Marshal(p)
	key := string(keyBytes)
	cache := &s.marketData
	cache.mu.Lock()
	if cache.history == nil {
		cache.history = map[string]*marketHistoryEntry{}
		cache.slots = make(chan struct{}, 2)
	}
	for k, e := range cache.history {
		select {
		case <-e.done:
			if time.Now().After(e.expires) || !c.HistoricalSessionCurrent(e.binding) {
				delete(cache.history, k)
			}
		default:
		}
	}
	if e := cache.history[key]; e != nil {
		cache.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-e.done:
			if !c.HistoricalSessionCurrent(e.binding) {
				return nil, errors.New("broker session changed")
			}
			return e.result, e.err
		}
	}
	if len(cache.history) >= 64 {
		cache.mu.Unlock()
		return nil, errors.New("history capacity reached; retry later")
	}
	e := &marketHistoryEntry{done: make(chan struct{}), binding: binding}
	cache.history[key] = e
	slots := cache.slots
	cache.mu.Unlock()
	select {
	case slots <- struct{}{}:
		e.result, e.err = s.fetchMarketHistory(ctx, req)
		<-slots
	case <-ctx.Done():
		e.err = ctx.Err()
	}
	cache.mu.Lock()
	e.expires = time.Now().Add(time.Minute)
	if e.err != nil {
		e.expires = time.Now().Add(15 * time.Second)
	}
	close(e.done)
	cache.mu.Unlock()
	return e.result, e.err
}
