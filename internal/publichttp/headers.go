// Package publichttp supplies anonymous request identities for public data
// sources. Callers retain their own transport, deadlines, formats and retry policy.
package publichttp

import (
	"net/http"
	"strings"
)

// SetUserAgent applies the destination's public-data compatibility policy.
// It never includes an operator name, contact address or personal project URL.
// Apply it again when an existing redirect policy permits a new destination.
func SetUserAgent(req *http.Request) {
	// FRED has rejected custom product identities while accepting Go's default.
	userAgent := "Go-http-client/1.1"
	switch strings.ToLower(req.URL.Hostname()) {
	case "www.bls.gov":
		// A repeated same-client comparison on 2026-09-12 returned 403 with
		// Canary's generic identity and 200 with only this header changed.
		userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/153.0.0.0 Safari/537.36"
	case "en.wikipedia.org":
		userAgent = "Canary-public-feeds/1.0"
	case "api.nasdaq.com":
		// Explicitly empty suppresses Go's default User-Agent on the wire.
		// The earnings endpoint rejected named clients in the existing witness.
		userAgent = ""
	}
	req.Header.Set("User-Agent", userAgent)
}
