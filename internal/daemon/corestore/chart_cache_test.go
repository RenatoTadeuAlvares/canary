package corestore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
)

func TestChartCacheBoundCannotDeleteOtherAuthority(t *testing.T) {
	s, _ := openTestStore(t)
	_, err := s.CompareAndSwapStateDocument(t.Context(), StateDocumentCAS{ScopeKey: "fixture", Kind: "unrelated", JSON: json.RawMessage(`{"kept":true}`)})
	if err != nil {
		t.Fatal(err)
	}
	var oldest, newest string
	for i := range 752 {
		hash := sha256.Sum256([]byte(fmt.Sprint(i)))
		key := hex.EncodeToString(hash[:])
		if i == 0 {
			oldest = key
		}
		newest = key
		if _, err := s.SaveChartCache(t.Context(), key, []byte(`{"fixture":true}`)); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok, err := s.LoadChartCache(t.Context(), oldest); err != nil || ok {
		t.Fatal("oldest cache was not bounded", err)
	}
	if _, ok, err := s.LoadChartCache(t.Context(), newest); err != nil || !ok {
		t.Fatal("newest cache lost", err)
	}
	if _, ok, err := s.GetStateDocument(t.Context(), "fixture", "unrelated"); err != nil || !ok {
		t.Fatal("cache eviction touched other authority", err)
	}
	if _, err := s.SaveChartCache(t.Context(), newest, make([]byte, 2<<20+1)); err == nil {
		t.Fatal("oversized cache accepted")
	}
}
