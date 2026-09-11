package daemon

import (
	"math"
	"testing"
	"time"

	ibkrlib "github.com/osauer/canary/v2/pkg/ibkr"
)

func TestCompletedEquityRangeUsesTheCompletedSession(t *testing.T) {
	for _, test := range []struct{ now, date string }{
		{"2026-09-11T06:00:00Z", "2026-09-10"},
		{"2026-09-12T12:00:00Z", "2026-09-11"},
		{"2026-09-07T12:00:00Z", "2026-09-04"},
		{"2026-11-27T17:00:00Z", "2026-11-25"},
		{"2026-11-27T19:00:00Z", "2026-11-27"},
	} {
		t.Run(test.now, func(t *testing.T) {
			now, _ := time.Parse(time.RFC3339, test.now)
			day, _ := time.Parse(time.DateOnly, test.date)
			series := ibkrlib.ChartSeries{Contract: ibkrlib.Contract{ConID: 91, SecType: "STK", Currency: "USD", PrimaryExch: "NYSE"}, WhatToShow: "TRADES", Bars: []ibkrlib.HistoricalBar{{Time: day, High: 110, Low: 90, Close: 100}, {Time: day.AddDate(0, 0, 1), High: 999, Low: 1, Close: 500}}}
			got := completedEquityRange(series, "1 day", now)
			if got == nil || got.Date != test.date || got.High != 110 || got.Low != 90 || got.Close != 100 {
				t.Fatal("wrong session range", got)
			}
			series.Bars = series.Bars[1:]
			if completedEquityRange(series, "1 day", now) != nil {
				t.Fatal("missing completed session was replaced by a different bar")
			}
		})
	}
}

func TestCompletedEquityRangeRejectsUnsupportedAndInvalidData(t *testing.T) {
	now := time.Date(2026, 9, 11, 6, 0, 0, 0, time.UTC)
	for _, change := range []string{"midpoint", "intraday", "unresolved", "foreign", "unknown exchange", "reversed", "nan", "infinite", "outside close", "calendar unknown"} {
		t.Run(change, func(t *testing.T) {
			series := ibkrlib.ChartSeries{Contract: ibkrlib.Contract{ConID: 91, SecType: "STK", Currency: "USD", PrimaryExch: "NYSE"}, WhatToShow: "TRADES", Bars: []ibkrlib.HistoricalBar{{Time: now.AddDate(0, 0, -1), High: 110, Low: 90, Close: 100}}}
			interval, at := "1 day", now
			switch change {
			case "midpoint":
				series.WhatToShow = "MIDPOINT"
			case "intraday":
				interval = "5 mins"
			case "unresolved":
				series.Contract.ConID = 0
			case "foreign":
				series.Contract.Currency = "EUR"
			case "unknown exchange":
				series.Contract.PrimaryExch = ""
			case "reversed":
				series.Bars[0].High = 80
			case "nan":
				series.Bars[0].Low = math.NaN()
			case "infinite":
				series.Bars[0].High = math.Inf(1)
			case "outside close":
				series.Bars[0].Close = 120
			case "calendar unknown":
				at = time.Date(2040, 9, 11, 6, 0, 0, 0, time.UTC)
			}
			if completedEquityRange(series, interval, at) != nil {
				t.Fatal("unusable range became a completed-session observation")
			}
		})
	}
}
