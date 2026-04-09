package sub2clash

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetcherFallsBackToShadowrocketUserAgent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.UserAgent(), "Shadowrocket") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ss://YWVzLTEyOC1nY206c2VjcmV0@ss.example.com:8388#Good"))
			return
		}
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer server.Close()

	fetcher := &Fetcher{timeout: 5 * time.Second}
	body, _, strategy, err := fetcher.Fetch(context.Background(), server.URL+"?token=abc123", FetchStrategy{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if strategy.UserAgent != "Shadowrocket" {
		t.Fatalf("strategy user agent = %q, want Shadowrocket", strategy.UserAgent)
	}
	if !strings.Contains(string(body), "ss://") {
		t.Fatalf("expected subscription body, got %q", string(body))
	}
}
