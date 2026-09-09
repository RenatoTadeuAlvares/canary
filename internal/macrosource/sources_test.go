package macrosource

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func sourceSpec(t *testing.T, id string) Spec {
	t.Helper()
	for _, s := range Specs() {
		if s.ID == id {
			return s
		}
	}
	t.Fatal("unknown synthetic fixture source")
	return Spec{}
}

func TestRSSPublicationTimeAndSourceProvenance(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct{ id, date, link, want string }{
		{"bea-news", "Thu, 03 Sep 2026 08:30:00 EDT", "https://www.bea.gov/news/synthetic", "2026-09-03T12:30:00Z"},
		{"bea-news", "Thu, 03 Sep 2026 08:30:00 EDT", "www.bea.gov/news/synthetic", "2026-09-03T12:30:00Z"},
		{"fed-policy", "Thu, 3 Sep 2026 08:30:00 GMT", "https://www.federalreserve.gov/synthetic", "2026-09-03T08:30:00Z"},
		{"ecb-news", "Thu, 03 Sep 2026 08:30:00 +0200", "https://www.ecb.europa.eu/synthetic", "2026-09-03T06:30:00Z"},
	} {
		payload := `<?xml version="1.0" encoding="us-ascii"?><rss><channel><item><title>Synthetic &amp; official</title><link>` + tc.link + `</link><pubDate>` + tc.date + `</pubDate></item></channel></rss>`
		batch, err := Parse(sourceSpec(t, tc.id), []byte(payload), now)
		if err != nil || len(batch.Publications) != 1 {
			t.Fatalf("%s parse: %v", tc.id, err)
		}
		item := batch.Publications[0]
		if item.PublishedAt.UTC().Format(time.RFC3339) != tc.want || item.SourceID != tc.id || !item.RetrievedAt.Equal(now) || item.Title != "Synthetic & official" {
			t.Fatal("publication clock or provenance changed")
		}
	}
	for _, payload := range []string{
		`<rss><channel><item><title>Synthetic</title><link>https://attacker.test/news</link></item></channel></rss>`,
		`<rss><channel><item><title>Synthetic</title><link>https://www.bea.gov/news/synthetic</link><pubDate>Thu, 03 Sep 2026 08:30:00 BAD</pubDate></item></channel></rss>`,
		`<rss><channel><item><title>Synthetic</title><link>https://www.bea.gov/news/synthetic</link><pubDate>Thu, 10 Sep 2026 08:30:00 EDT</pubDate></item></channel></rss>`,
	} {
		if _, err := Parse(sourceSpec(t, "bea-news"), []byte(payload), now); err == nil {
			t.Fatal("unsafe or unparseable publication replaced source evidence")
		}
	}
}

func TestCalendarTruncationAndDatePrecisionCannotEraseLastGood(t *testing.T) {
	spec := sourceSpec(t, "bls-calendar")
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	valid := "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nSUMMARY:Synthetic release\r\nDTSTART:20260101T010000Z\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	batch, err := Parse(spec, []byte(valid), now)
	if err != nil || len(batch.Events) != 1 {
		t.Fatal(err)
	}
	event := batch.Events[0]
	if event.Date != "2025-12-31" || event.Timezone != "America/New_York" || event.TimePrecision != "instant" {
		t.Fatal("UTC instant was assigned to the wrong source-local day")
	}
	for _, payload := range []string{
		strings.Replace(valid, "END:VCALENDAR\r\n", "", 1),
		strings.Replace(valid, "END:VEVENT\r\n", "", 1),
		strings.Replace(valid, "SUMMARY:", "BEGIN:VEVENT\r\nSUMMARY:", 1),
		strings.Replace(valid, "DTSTART:20260101T010000Z", "DTSTART:20260101T010000Z\r\nRRULE:FREQ=DAILY", 1),
		"BEGIN:VCALENDAR\nEND:VCALENDAR",
	} {
		if _, err := Parse(spec, []byte(payload), now); err == nil {
			t.Fatal("incomplete or unsupported calendar accepted as a replacement")
		}
	}
	onlyDate := strings.Replace(valid, "DTSTART:20260101T010000Z", "DTSTART;VALUE=DATE:20260101", 1)
	batch, err = Parse(spec, []byte(onlyDate), now)
	if err != nil || !batch.Events[0].ScheduledAt.IsZero() || batch.Events[0].TimePrecision != "date" {
		t.Fatal("date-only event gained an invented midnight")
	}
}

func TestParsedFeedCannotRestoreTamperedChildEvidence(t *testing.T) {
	spec := sourceSpec(t, "bea-calendar")
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	batch, err := Parse(spec, []byte(`{"file_last_updated":"2026-09-01T08:00:00", "Synthetic release":{"release_dates":["2026-09-10T12:30:00Z"]}}`), now)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Batch){
		func(b *Batch) { b.Events[0].SourceURL = "https://attacker.test/calendar" },
		func(b *Batch) { b.Events[0].SourceID = "fed-calendar" },
		func(b *Batch) { b.Events[0].Date = "2026-09-11" },
		func(b *Batch) { b.Events[0].RetrievedAt = now.Add(time.Hour) },
		func(b *Batch) { b.Events[0].TimePrecision = "date" },
		func(b *Batch) { b.Events[0].ID = "forged" },
	} {
		raw, _ := json.Marshal(batch)
		var changed Batch
		_ = json.Unmarshal(raw, &changed)
		mutate(&changed)
		if ValidateBatch(spec, changed, now) == nil {
			t.Fatal("tampered source evidence restored as valid")
		}
	}
}

type publicTransport func(*http.Request) (*http.Response, error)

func (f publicTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestPublicClientRejectsFailuresAndRedirectsWithoutCredentials(t *testing.T) {
	client := NewClient()
	requests := 0
	client.HTTP.Transport = publicTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		if r.Method != "GET" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Fatal("public source request carried authority")
		}
		return &http.Response{StatusCode: 403, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("blocked")), Request: r}, nil
	})
	_, err := client.Fetch(context.Background(), sourceSpec(t, "bls-calendar"), time.Now())
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") || requests != 1 {
		t.Fatal("source failure was hidden or retried without bounds")
	}
	client.HTTP.Transport = publicTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"http://127.0.0.1/private"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})
	if _, err = client.Fetch(context.Background(), sourceSpec(t, "bls-calendar"), time.Now()); err == nil {
		t.Fatal("official feed redirected into a private endpoint")
	}
	client.HTTP.Transport = publicTransport(func(*http.Request) (*http.Response, error) { return nil, errors.New("private diagnostic") })
	_, err = client.Fetch(context.Background(), sourceSpec(t, "bls-calendar"), time.Now())
	if err == nil || strings.Contains(err.Error(), "private diagnostic") {
		t.Fatal("raw transport details escaped public source status")
	}
}
