package rpc

import (
	"testing"
	"time"
)

func TestVVIXScheduleSourceGradeKeepsRealAge(t *testing.T) {
	now := time.Date(2026, 9, 8, 7, 0, 0, 0, time.UTC)
	deadline := time.Date(2026, 9, 8, 20, 15, 0, 0, time.UTC)
	value := 84.0
	r := RegimeSnapshotResult{
		AsOf: now,
		VIXTermStructure: RegimeVIXTerm{Status: RegimeStatusStale, RegimeIndicatorMeta: RegimeIndicatorMeta{
			Band: "green", AsOf: &RegimeAsOfSummary{Time: now}, Freshness: &RegimeFreshness{Class: RegimeFreshnessNotDue},
		}},
		VolOfVol: RegimeVolOfVol{Status: RegimeStatusOK, Last: &value, RegimeIndicatorMeta: RegimeIndicatorMeta{
			Band: "green", AsOf: &RegimeAsOfSummary{Time: now.Add(-103 * time.Hour)},
			Freshness: &RegimeFreshness{Class: RegimeFreshnessNotDue, NextDueAt: &deadline},
		}},
	}
	r.SourceHealth = BuildRegimeSourceHealth(&r, now)
	if grade := regimeSourceHealthGrade(r, "vol"); grade != RegimeCurrencyGradeNone {
		t.Fatalf("calendar-vouched source grade=%s", grade)
	}
	if r.SourceHealth[0].AgeSeconds <= r.SourceHealth[0].MaxAgeSeconds {
		t.Fatal("calendar context must retain the real observation age")
	}
	r.AsOf = deadline
	if grade := regimeSourceHealthGrade(r, "vol"); grade != RegimeCurrencyGradeFatal {
		t.Fatalf("expired schedule grade=%s", grade)
	}
}
