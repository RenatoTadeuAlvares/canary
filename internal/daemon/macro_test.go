package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/osauer/canary/v2/internal/macrosource"
)

type macroFixtureFetcher struct {
	fail  bool
	calls int
}

func (f *macroFixtureFetcher) Fetch(_ context.Context, s macrosource.Spec, at time.Time) (macrosource.Batch, error) {
	f.calls++
	if f.fail {
		return macrosource.Batch{}, errors.New("source returned HTTP 403")
	}
	return macrosource.Parse(s, []byte(`{"Synthetic release":{"release_dates":["2026-09-10T12:30:00Z"]}}`), at)
}
func beaMacroSpec(t *testing.T) macrosource.Spec {
	t.Helper()
	for _, s := range macrosource.Specs() {
		if s.ID == "bea-calendar" {
			return s
		}
	}
	t.Fatal("fixture source absent")
	return macrosource.Spec{}
}

func TestMacroFailureRetainsEvidenceAcrossRestartAndNeverExtendsFreshness(t *testing.T) {
	store := openMarketTestCoreStore(t)
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	s := &Server{coreStore: store, now: func() time.Time { return now }}
	c := s.loadMacroSources()
	fixture := &macroFixtureFetcher{}
	c.client = fixture
	s.macro = c
	spec := beaMacroSpec(t)
	s.refreshMacroSource(t.Context(), c, spec)
	first := c.records[spec.ID]
	if first.Source.Availability != "available" || len(first.Batch.Events) != 1 {
		t.Fatal("valid source was not retained")
	}
	now = now.Add(5 * time.Minute)
	fixture.fail = true
	s.refreshMacroSource(t.Context(), c, spec)
	failed := c.records[spec.ID]
	if failed.Source.Availability != "unavailable" || !strings.Contains(failed.Source.Detail, "403") || !failed.Source.LastSuccess.Equal(first.Source.LastSuccess) || !failed.Source.ValidUntil.Equal(first.Source.ValidUntil) || failed.Batch.Events[0].ID != first.Batch.Events[0].ID {
		t.Fatal("failed read replaced or refreshed last-good evidence")
	}
	restored := &Server{coreStore: store, now: func() time.Time { return now }}
	restored.macro = restored.loadMacroSources()
	row := restored.macro.records[spec.ID]
	if row.Source.Availability != "unavailable" || row.Batch.Events[0].ID != first.Batch.Events[0].ID {
		t.Fatal("restart erased source failure or retained event")
	}
	now = now.Add(time.Hour)
	snapshot := restored.handleMacroSnapshot()
	for _, source := range snapshot.Sources {
		if source.ID == spec.ID && !source.Stale {
			t.Fatal("retained calendar did not expire")
		}
	}
	if snapshot.CoverageStatus != "partial" || fixture.calls != 2 {
		t.Fatal("snapshot fabricated complete coverage or dispatched collection")
	}
	fixture.fail = false
	s.refreshMacroSource(t.Context(), c, spec)
	if c.records[spec.ID].Source.Availability != "available" || !c.records[spec.ID].Source.LastSuccess.Equal(now) {
		t.Fatal("successful source recovery did not replace the failure")
	}
}

func TestMacroRestoreRejectsCorruptProvenanceAndIsolatedStartsStayOffline(t *testing.T) {
	store := openMarketTestCoreStore(t)
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	s := &Server{coreStore: store, now: func() time.Time { return now }}
	c := s.loadMacroSources()
	c.client = &macroFixtureFetcher{}
	spec := beaMacroSpec(t)
	s.refreshMacroSource(t.Context(), c, spec)
	valid := c.records[spec.ID]
	for _, mutate := range []func(*macroRecord){
		func(r *macroRecord) { r.Source.Kind = "rss" },
		func(r *macroRecord) { r.Source.ValidUntil = now.AddDate(1, 0, 0) },
		func(r *macroRecord) { r.Batch.Events[0].SourceURL = "https://attacker.test/" },
		func(r *macroRecord) { r.Batch.Events[0].RetrievedAt = now.Add(-time.Hour) },
		func(r *macroRecord) { r.Source.LastSuccess = time.Time{} },
	} {
		raw, _ := json.Marshal(valid)
		var corrupt macroRecord
		_ = json.Unmarshal(raw, &corrupt)
		mutate(&corrupt)
		raw, _ = json.Marshal(corrupt)
		if err := saveMarketDocument(t.Context(), store, "public-macro:"+spec.ID, macroStateKind, raw); err != nil {
			t.Fatal(err)
		}
		restored := s.loadMacroSources().records[spec.ID]
		if restored.Source.Availability != "unavailable" || len(restored.Batch.Events) != 0 || !strings.Contains(restored.Source.Detail, "invalid") {
			t.Fatal("corrupt cached provenance became source authority")
		}
	}
	for _, productionPath := range []bool{false, true} {
		// Both isolated stores and the linker-configured hermetic CLI must load
		// retained records without starting a network worker.
		s.productionStateDatabase = productionPath
		s.disableMacroSources = productionPath
		ctx, cancel := context.WithCancel(context.Background())
		s.startMacroSources(ctx)
		done := make(chan struct{})
		go func() { s.macroLoopWG.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(time.Second):
			cancel()
			<-done
			t.Fatal("offline daemon started a public source worker")
		}
		cancel()
		if !s.macro.records[spec.ID].Source.LastAttempt.IsZero() {
			t.Fatal("offline daemon contacted an external source")
		}
	}
}
