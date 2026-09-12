package macrosource

import (
	"strings"
	"testing"
	"time"
)

const syntheticBLSTimezone = `BEGIN:VTIMEZONE
TZID:US-Eastern
BEGIN:DAYLIGHT
TZOFFSETFROM:-0500
TZOFFSETTO:-0400
DTSTART:20070311T020000
RRULE:FREQ=YEARLY;BYMONTH=3;BYDAY=2SU
TZNAME:EDT
END:DAYLIGHT
BEGIN:STANDARD
TZOFFSETFROM:-0400
TZOFFSETTO:-0500
DTSTART:20071104T020000
RRULE:FREQ=YEARLY;BYMONTH=11;BYDAY=1SU
TZNAME:EST
END:STANDARD
END:VTIMEZONE`

func syntheticBLSCalendar(definition, zone, start string) []byte {
	return []byte("BEGIN:VCALENDAR\n" + definition + "\nBEGIN:VEVENT\nSUMMARY:Synthetic release\nDTSTART;TZID=" + zone + ":" + start + "\nEND:VEVENT\nEND:VCALENDAR\n")
}

// TestBLSDeclaredTimezoneKeepsReleaseInstants witnesses the supported publisher
// profile across seasonal offsets and the two DST transition boundaries.
func TestBLSDeclaredTimezoneKeepsReleaseInstants(t *testing.T) {
	spec := sourceSpec(t, "bls-calendar")
	for _, tc := range []struct{ local, utc string }{
		{"20260115T083000", "2026-01-15T13:30:00Z"},
		{"20260715T083000", "2026-07-15T12:30:00Z"},
		{"20260308T015959", "2026-03-08T06:59:59Z"},
		{"20260308T030000", "2026-03-08T07:00:00Z"},
		{"20261101T005959", "2026-11-01T04:59:59Z"},
		// iCalendar selects the first occurrence of the repeated local hour.
		{"20261101T013000", "2026-11-01T05:30:00Z"},
		{"20261101T020000", "2026-11-01T07:00:00Z"},
	} {
		t.Run(tc.local, func(t *testing.T) {
			now := time.Date(2026, 9, 12, 5, 0, 0, 0, time.UTC)
			batch, err := Parse(spec, syntheticBLSCalendar(syntheticBLSTimezone, "US-Eastern", tc.local), now)
			if err != nil || len(batch.Events) != 1 {
				t.Fatalf("publisher timezone rejected: %v", err)
			}
			event := batch.Events[0]
			if event.ScheduledAt.UTC().Format(time.RFC3339) != tc.utc || event.TimePrecision != "instant" || event.Timezone != "America/New_York" || event.Date != tc.utc[:10] || !event.RetrievedAt.Equal(now) {
				t.Fatalf("release clock or provenance shifted: %+v", event)
			}
		})
	}
}

// TestBLSUndeclaredTimezoneCannotInventReleaseTime rejects similarly named or
// contradictory declarations instead of assigning a convenient local offset.
func TestBLSUndeclaredTimezoneCannotInventReleaseTime(t *testing.T) {
	for _, tc := range []struct{ name, definition, zone, start string }{
		{"missing declaration", "", "US-Eastern", "20260912T083000"},
		{"unknown identifier", syntheticBLSTimezone, "Unknown-Eastern", "20260912T083000"},
		{"different offset", strings.Replace(syntheticBLSTimezone, "TZOFFSETTO:-0400", "TZOFFSETTO:-0300", 1), "US-Eastern", "20260912T083000"},
		{"different DST rule", strings.Replace(syntheticBLSTimezone, "BYMONTH=3", "BYMONTH=4", 1), "US-Eastern", "20260912T083000"},
		{"duplicate declaration", syntheticBLSTimezone + "\n" + syntheticBLSTimezone, "US-Eastern", "20260912T083000"},
		{"incomplete declaration", strings.TrimSuffix(syntheticBLSTimezone, "END:VTIMEZONE"), "US-Eastern", "20260912T083000"},
		{"outside supported profile", syntheticBLSTimezone, "US-Eastern", "20060101T083000"},
		{"nonexistent spring-forward instant", syntheticBLSTimezone, "US-Eastern", "20260308T023000"},
		{"duplicate timezone parameter", syntheticBLSTimezone, "US-Eastern;TZID=America/New_York", "20260912T083000"},
		{"repeated timezone parameter", syntheticBLSTimezone, "US-Eastern;TZID=US-Eastern", "20260912T083000"},
		{"timezone on UTC value", syntheticBLSTimezone, "US-Eastern", "20260912T083000Z"},
		{"timezone on date value", syntheticBLSTimezone, "US-Eastern;VALUE=DATE", "20260912"},
		{"empty quoted timezone", syntheticBLSTimezone, `""`, "20260912T083000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse(sourceSpec(t, "bls-calendar"), syntheticBLSCalendar(tc.definition, tc.zone, tc.start), time.Now()); err == nil {
				t.Fatal("unsupported timezone became a precise release instant")
			}
		})
	}
	spec := sourceSpec(t, "bls-calendar")
	spec.ID = "unrelated-calendar"
	if _, err := Parse(spec, syntheticBLSCalendar(syntheticBLSTimezone, "US-Eastern", "20260912T083000"), time.Now()); err == nil {
		t.Fatal("publisher-specific timezone alias escaped its source")
	}
}
