package sub2clash

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestServiceAddProfileAndServeManagedYAML(t *testing.T) {
	payload := base64.StdEncoding.EncodeToString([]byte(strings.Join([]string{
		"STATUS=🚀↑:3GB,↓:4GB,TOT:20GB💒Expires:2026-06-26",
		"ss://" + base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:secret")) + "@ss.example.com:8388#SS%20Node",
	}, "\n")))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.UserAgent(), "Shadowrocket") {
			_, _ = w.Write([]byte(payload))
			return
		}
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	cfg, err := NewConfig(tempDir, "", "http://127.0.0.1:8080")
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}
	cfg.EncryptionKey = []byte("0123456789abcdef0123456789abcdef")
	cfg.AdminToken = "test-token"
	cfg.HTTPRequestTimout = 5 * time.Second

	store, err := OpenStore(context.Background(), cfg.DataDir)
	if err != nil {
		t.Fatalf("OpenStore() error = %v", err)
	}
	defer store.Close()

	service, err := NewService(cfg, store)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	managed, err := service.AddProfile(context.Background(), "Demo", server.URL+"?token=abc123", 24*time.Hour)
	if err != nil {
		t.Fatalf("AddProfile() error = %v", err)
	}
	if managed.ManagedURL == "" || managed.DownloadURL == "" {
		t.Fatalf("expected managed urls, got %#v", managed)
	}

	body, info, err := service.GetManagedYAML(context.Background(), managed.ID)
	if err != nil {
		t.Fatalf("GetManagedYAML() error = %v", err)
	}
	if !strings.Contains(string(body), "SS Node") {
		t.Fatalf("expected generated yaml, got:\n%s", string(body))
	}
	if !strings.Contains(info, "total=") {
		t.Fatalf("expected subscription info, got %q", info)
	}
}
