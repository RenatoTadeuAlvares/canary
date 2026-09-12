package daemon

import (
	"context"
	"slices"
	"time"

	"github.com/osauer/canary/v2/internal/rpc"
)

type marketHistoryInterest struct {
	Params   rpc.MarketHistoryParams
	Until    time.Time
	RetryAt  time.Time
	Failures int
}

func (s *Server) rememberMarketHistory(p rpc.MarketHistoryParams) {
	key, normalized, err := marketHistoryIdentity(p)
	if err != nil {
		return
	}
	s.marketData.mu.Lock()
	defer s.marketData.mu.Unlock()
	if s.marketData.interest == nil {
		s.marketData.interest = make(map[string]marketHistoryInterest)
	}
	now := time.Now()
	for key, item := range s.marketData.interest {
		if now.After(item.Until) {
			delete(s.marketData.interest, key)
		}
	}
	item, exists := s.marketData.interest[key]
	if !exists && len(s.marketData.interest) >= 64 {
		return
	}
	if !exists {
		item.Params = normalized
	}
	// The widest observed daily request covers shorter selections too.
	oldDays, _, _ := marketHistoryWindow(item.Params.Range, now)
	newDays, _, _ := marketHistoryWindow(p.Range, now)
	if newDays > oldDays {
		item.Params = normalized
	}
	item.Until = now.Add(24 * time.Hour)
	s.marketData.interest[key] = item
}

func (s *Server) startMarketHistoryRefresh(ctx context.Context) {
	s.marketData.loopWG.Go(func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			s.marketData.mu.Lock()
			keys := make([]string, 0, len(s.marketData.interest))
			for key := range s.marketData.interest {
				keys = append(keys, key)
			}
			slices.Sort(keys)
			s.marketData.mu.Unlock()
			for _, key := range keys {
				if ctx.Err() != nil {
					return
				}
				s.marketData.mu.Lock()
				item := s.marketData.interest[key]
				s.marketData.mu.Unlock()
				if time.Now().After(item.Until) || time.Now().Before(item.RetryAt) {
					continue
				}
				// Reuse the same bounded request path without extending interest
				// merely because this background reader ran.
				readCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				result, err := s.marketHistoryRequest(readCtx, item.Params)
				cancel()
				s.marketData.mu.Lock()
				current := s.marketData.interest[key]
				if err != nil || result != nil && result.Cache != nil && result.Cache.RefreshFailed {
					current.Failures = min(current.Failures+1, 5)
					current.RetryAt = time.Now().Add(min(15*time.Minute, time.Duration(1<<current.Failures)*30*time.Second))
				} else {
					current.Failures = 0
					current.RetryAt = time.Time{}
				}
				s.marketData.interest[key] = current
				s.marketData.mu.Unlock()
			}
		}
	})
}
