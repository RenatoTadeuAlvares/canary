package publichttp

import (
	"bufio"
	"bytes"
	"net/http"
	"strings"
	"testing"
)

func TestAnonymousIdentityProfilesOnWire(t *testing.T) {
	for _, tc := range []struct{ host, want string }{
		{"www.bls.gov", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/153.0.0.0 Safari/537.36"},
		{"WWW.BLS.GOV", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/153.0.0.0 Safari/537.36"},
		{"en.wikipedia.org", "Canary-public-feeds/1.0"},
		{"api.nasdaq.com", ""},
		{"fred.stlouisfed.org", "Go-http-client/1.1"},
		{"cdn.cboe.com", "Go-http-client/1.1"},
		{"www.nasdaqtrader.com", "Go-http-client/1.1"},
		{"apps.bea.gov", "Go-http-client/1.1"},
		{"www.newyorkfed.org", "Go-http-client/1.1"},
		{"www.federalreserve.gov", "Go-http-client/1.1"},
		{"home.treasury.gov", "Go-http-client/1.1"},
		{"www.ecb.europa.eu", "Go-http-client/1.1"},
		{"www.bls.gov.example.invalid", "Go-http-client/1.1"},
		{"api.nasdaq.com.example.invalid", "Go-http-client/1.1"},
	} {
		t.Run(tc.host, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "https://"+tc.host+"/public-data", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("User-Agent", "fixture/1.0 (+https://operator.example)")
			req.Header.Set("Accept", "text/xml")
			req.Header.Set("Accept-Language", "en-US,en;q=0.9")
			SetUserAgent(req)
			var wire bytes.Buffer
			if err := req.Write(&wire); err != nil {
				t.Fatal(err)
			}
			got, err := http.ReadRequest(bufio.NewReader(&wire))
			if err != nil {
				t.Fatal(err)
			}
			defer got.Body.Close()
			if got.UserAgent() != tc.want {
				t.Fatalf("wire User-Agent = %q, want %q", got.UserAgent(), tc.want)
			}
			if tc.want == "" && got.Header.Values("User-Agent") != nil {
				t.Fatal("Nasdaq suppression leaked an empty header or Go's default")
			}
			if got.Header.Get("Accept") != "text/xml" || got.Header.Get("Accept-Language") != "en-US,en;q=0.9" {
				t.Fatal("source-specific content negotiation changed")
			}
			for _, key := range []string{"Cookie", "Authorization", "Origin", "Referer"} {
				if got.Header.Get(key) != "" {
					t.Fatalf("request identity added %s", key)
				}
			}
			if strings.Contains(got.UserAgent(), "operator.example") {
				t.Fatal("prior personal attribution survived header selection")
			}
		})
	}
}
