package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/osauer/canary/v2/internal/macrosource"
	"github.com/osauer/canary/v2/internal/rpc"
)

const macroStateKind = "macro_public_sources_v1"
const macroFreshness = 30 * time.Minute

type macroRecord struct {
	Source rpc.MacroSource   `json:"source"`
	Batch  macrosource.Batch `json:"batch"`
}
type macroFetcher interface {
	Fetch(context.Context, macrosource.Spec, time.Time) (macrosource.Batch, error)
}
type macroCache struct {
	mu      sync.RWMutex
	records map[string]macroRecord
	client  macroFetcher
}

func coldMacroRecord(spec macrosource.Spec) macroRecord {
	return macroRecord{Source: rpc.MacroSource{ID: spec.ID, Name: spec.Name, URL: spec.URL, Kind: spec.Kind, Availability: "unavailable", Stale: true, Coverage: spec.Coverage, Detail: "Waiting for first source read"}}
}

func (s *Server) loadMacroSources() *macroCache {
	c := &macroCache{records: map[string]macroRecord{}, client: macrosource.NewClient()}
	for _, spec := range macrosource.Specs() {
		row := coldMacroRecord(spec)
		raw, ok, err := loadMarketState(s.coreStore, "public-macro:"+spec.ID, macroStateKind)
		if err != nil {
			row.Source.Detail = "Public source persistence unavailable"
		} else if ok {
			var saved macroRecord
			if json.Unmarshal(raw, &saved) == nil && validateMacroEnvelope(spec, saved, s.orderNow()) == nil {
				row = saved
			} else {
				row.Source.Detail = "Saved public source record is invalid"
			}
		}
		c.records[spec.ID] = row
	}
	return c
}

func (s *Server) startMacroSources(ctx context.Context) {
	c := s.loadMacroSources()
	// A custom StateDatabasePath is the existing offline/test authority seam.
	// It can serve retained fixtures but must not start external network readers.
	s.mu.Lock()
	s.macro = c
	s.mu.Unlock()
	if !s.productionStateDatabase || s.disableMacroSources {
		return
	}
	s.macroLoopWG.Go(func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			s.refreshMacroSources(ctx, c)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	})
}

func (s *Server) refreshMacroSources(ctx context.Context, c *macroCache) {
	var wg sync.WaitGroup
	for _, spec := range macrosource.Specs() {
		wg.Go(func() { s.refreshMacroSource(ctx, c, spec) })
	}
	wg.Wait()
}

func (s *Server) refreshMacroSource(ctx context.Context, c *macroCache, spec macrosource.Spec) {
	if ctx.Err() != nil {
		return
	}
	at := s.orderNow().UTC()
	batch, err := c.client.Fetch(ctx, spec, at)
	if ctx.Err() != nil {
		return
	}
	if err == nil {
		err = macrosource.ValidateBatch(spec, batch, at)
	}
	c.mu.RLock()
	row := c.records[spec.ID]
	c.mu.RUnlock()
	row.Source.LastAttempt = at
	if err != nil {
		row.Source.Availability = "unavailable"
		row.Source.Detail = err.Error()
	} else {
		row.Batch = batch
		row.Source.Availability = "available"
		row.Source.Detail = ""
		row.Source.LastSuccess = at
		row.Source.ValidUntil = at.Add(macroFreshness)
		row.Source.Stale = false
	}
	raw, encodeErr := json.Marshal(row)
	if encodeErr == nil {
		encodeErr = saveMarketDocument(ctx, s.coreStore, "public-macro:"+spec.ID, macroStateKind, raw)
	}
	if encodeErr != nil {
		c.mu.Lock()
		prior := c.records[spec.ID]
		prior.Source.LastAttempt = at
		prior.Source.Availability = "unavailable"
		prior.Source.Detail = "Public source persistence unavailable"
		c.records[spec.ID] = prior
		c.mu.Unlock()
		return
	}
	c.mu.Lock()
	c.records[spec.ID] = row
	c.mu.Unlock()
}

func (s *Server) handleMacroSnapshot() rpc.MacroSnapshotResult {
	now := s.orderNow().UTC()
	start := now.Add(-24 * time.Hour).Format(time.DateOnly)
	end := now.AddDate(0, 0, 7).Format(time.DateOnly)
	out := rpc.MacroSnapshotResult{AsOf: now, WindowStart: start, WindowEnd: end, CoverageStatus: "partial", Events: []rpc.MacroEvent{}, Publications: []rpc.MacroPublication{}, Sources: []rpc.MacroSource{}}
	s.mu.Lock()
	cache := s.macro
	s.mu.Unlock()
	if cache == nil {
		for _, spec := range macrosource.Specs() {
			row := coldMacroRecord(spec)
			row.Source.Detail = "Public source collector unavailable"
			out.Sources = append(out.Sources, row.Source)
		}
		return out
	}
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	out.CoverageStatus = "available"
	for _, spec := range macrosource.Specs() {
		record, ok := cache.records[spec.ID]
		if !ok {
			record = coldMacroRecord(spec)
		}
		source := record.Source
		source.Stale = source.LastSuccess.IsZero() || now.Before(source.LastSuccess.Add(-time.Minute)) || !now.Before(source.ValidUntil)
		if source.Availability != "available" || source.Stale {
			out.CoverageStatus = "partial"
		}
		out.Sources = append(out.Sources, source)
		for _, event := range record.Batch.Events {
			if event.Date >= start && event.Date <= end {
				out.Events = append(out.Events, event)
			}
		}
		for _, item := range record.Batch.Publications {
			if item.PublishedAt.IsZero() || !item.PublishedAt.Before(now.AddDate(0, 0, -7)) {
				out.Publications = append(out.Publications, item)
			}
		}
	}
	sort.Slice(out.Events, func(i, j int) bool {
		a, b := out.Events[i], out.Events[j]
		if a.Date != b.Date {
			return a.Date < b.Date
		}
		if !a.ScheduledAt.Equal(b.ScheduledAt) {
			return a.ScheduledAt.Before(b.ScheduledAt)
		}
		return a.ID < b.ID
	})
	sort.Slice(out.Publications, func(i, j int) bool {
		a, b := out.Publications[i], out.Publications[j]
		if a.PublishedAt.Equal(b.PublishedAt) {
			return a.ID < b.ID
		}
		return a.PublishedAt.After(b.PublishedAt)
	})
	if len(out.Events) > 48 {
		out.Events = out.Events[:48]
		out.Truncated = true
	}
	if len(out.Publications) > 12 {
		out.Publications = out.Publications[:12]
		out.Truncated = true
	}
	// This public overview is bounded independently of the complete source cache.
	for {
		raw, _ := json.Marshal(out)
		if len(raw) <= 28<<10 {
			break
		}
		out.Truncated = true
		if len(out.Publications) > 0 {
			out.Publications = out.Publications[:len(out.Publications)-1]
		} else if len(out.Events) > 0 {
			out.Events = out.Events[:len(out.Events)-1]
		} else {
			break
		}
	}
	return out
}

func validateMacroEnvelope(spec macrosource.Spec, record macroRecord, now time.Time) error {
	source := record.Source
	if !utf8.ValidString(source.Detail) || len(source.Detail) > 500 {
		return errors.New("invalid public source detail")
	}
	for _, r := range source.Detail {
		if unicode.IsControl(r) {
			return errors.New("invalid public source detail")
		}
	}
	if source.Availability == "available" && source.Detail != "" {
		return errors.New("successful public source carries a failure")
	}
	if source.ID != spec.ID || source.Name != spec.Name || source.URL != spec.URL || source.Kind != spec.Kind || source.Coverage != spec.Coverage {
		return errors.New("invalid public source identity")
	}
	if source.Availability != "available" && source.Availability != "unavailable" {
		return errors.New("invalid public source availability")
	}
	if source.LastAttempt.After(now.Add(time.Minute)) || source.LastSuccess.After(source.LastAttempt) {
		return errors.New("invalid public source attempt clock")
	}
	if source.LastSuccess.IsZero() {
		if source.Availability != "unavailable" || !source.ValidUntil.IsZero() || len(record.Batch.Events)+len(record.Batch.Publications) != 0 {
			return errors.New("public source evidence missing")
		}
		return nil
	}
	if !source.ValidUntil.Equal(source.LastSuccess.Add(macroFreshness)) || source.Availability == "available" && !source.LastAttempt.Equal(source.LastSuccess) {
		return errors.New("invalid public source freshness interval")
	}
	if err := macrosource.ValidateBatch(spec, record.Batch, now); err != nil {
		return err
	}
	for _, event := range record.Batch.Events {
		if !event.RetrievedAt.Equal(source.LastSuccess) {
			return errors.New("calendar retrieval receipt mismatch")
		}
	}
	for _, item := range record.Batch.Publications {
		if !item.RetrievedAt.Equal(source.LastSuccess) {
			return errors.New("publication retrieval receipt mismatch")
		}
	}
	return nil
}
