package spx

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func TestBreadthRetainsPreceding252SessionExtremes(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	w := ConstituentWindow{Symbol: "SYN"}
	for i := range 253 {
		close := 100.0
		if i == 27 {
			close = 200
		}
		w = SlideWindow(w, close, start.AddDate(0, 0, i).Format("2006-01-02"))
	}
	day := start.AddDate(0, 0, 253).Format("2006-01-02")
	w = SlideWindow(w, 150, day)
	snap := Compute([]string{"SYN"}, map[string]ConstituentWindow{"SYN": w}, day, start)
	if snap.CoverageHighsLows != 1 || snap.NewHighsToday != 0 {
		t.Fatalf("forgot an extreme 226 sessions ago: %+v", snap)
	}
	w = SlideWindow(w, 201, day)
	snap = Compute([]string{"SYN"}, map[string]ConstituentWindow{"SYN": w}, day, start)
	if snap.NewHighsToday != 1 {
		t.Fatal("same-day corrected close must be compared with the same preceding window")
	}
}

func TestBreadthWarmFetchCoversMissedCalendarDays(t *testing.T) {
	now := time.Date(2026, 9, 4, 23, 0, 0, 0, time.UTC)
	e := &Engine{clock: func() time.Time { return now }, coldLookback: 400, warmLookback: 2}
	plan := e.planFetches([]string{"SYN"}, map[string]ConstituentWindow{"SYN": {Closes: []float64{100}, LastBarAt: "2026-08-20"}})
	if len(plan) != 1 || plan[0].LookbackDays < 16 {
		t.Fatalf("missed sessions omitted: %+v", plan)
	}
}

func TestBreadthInvalidBatchPreservesCachedWindow(t *testing.T) {
	for _, bad := range []Bar{{Date: "2026-09-04", Close: 0}, {Date: "2026-09-04", Close: -1}, {Date: "2026-09-04", Close: math.NaN()}, {Date: "2026-09-04", Close: math.Inf(1)}, {Date: "invalid", Close: 100}} {
		t.Run(bad.Date, func(t *testing.T) {
			now := time.Date(2026, 9, 4, 23, 0, 0, 0, time.UTC)
			f := &FakeBarFetcher{Bars: map[string][]Bar{"SYN": {{Date: "2026-09-03", Close: 102}, bad}}}
			e := newTestEngine(t, f, frozenClock(now), []string{"SYN"})
			prior := ConstituentWindow{Symbol: "SYN", Closes: []float64{100}, LastBarAt: "2026-09-02"}
			e.windows = map[string]ConstituentWindow{"SYN": prior}
			_ = e.Refresh(context.Background())
			if !constituentWindowsEqual(e.windows["SYN"], prior) {
				t.Fatal("invalid response partially changed retained history")
			}
			if e.progress.Failed != 1 {
				t.Fatalf("failed responses = %d", e.progress.Failed)
			}
		})
	}
}

func TestBreadthHistoryDistinguishesMissingAndMeasuredZero(t *testing.T) {
	for _, bars := range []int{50, 253} {
		now := time.Date(2026, 9, 4, 23, 0, 0, 0, time.UTC)
		f := &FakeBarFetcher{Bars: map[string][]Bar{"SYN": makeSeries(100, 0, bars, now)}}
		e := newTestEngine(t, f, frozenClock(now), []string{"SYN"})
		if err := e.Refresh(context.Background()); err != nil {
			t.Fatal(err)
		}
		h := e.History(1)[0]
		encoded, err := json.Marshal(h)
		if err != nil {
			t.Fatal(err)
		}
		if h.MemberCount != 1 || h.Coverage50 != 1 {
			t.Fatal("missing coverage denominators")
		}
		if bars == 50 && (h.NewHighs != nil || h.PctAbove200DMA != nil || !strings.Contains(string(encoded), `"new_highs":null`)) {
			t.Fatal("missing history became zero")
		}
		if bars == 253 && (h.NewHighs == nil || *h.NewHighs != 0 || h.CoverageHighsLows != 1 || !strings.Contains(string(encoded), `"new_highs":0`)) {
			t.Fatal("measured zero lost")
		}
	}
}

func TestBreadthColdCalendarRequestCoversTradingYear(t *testing.T) {
	now := time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)
	var bars []Bar
	for i := 450; i >= 0; i-- {
		day := now.AddDate(0, 0, -i)
		if day.Weekday() != time.Saturday && day.Weekday() != time.Sunday {
			bars = append(bars, Bar{Date: day.Format("2006-01-02"), Close: 100})
		}
	}
	f := &FakeBarFetcher{Bars: map[string][]Bar{"SYN": bars}}
	e := newTestEngine(t, f, frozenClock(now.Add(23*time.Hour)), []string{"SYN"})
	if err := e.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	snap, ok := e.Get()
	if !ok || snap.CoverageHighsLows != 1 {
		t.Fatal("cold calendar request did not cover annual history")
	}
}
