package sub2clash

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultAddr          = ":8080"
	defaultRefresh       = 24 * time.Hour
	defaultAutoTestURL   = "http://www.gstatic.com/generate_204"
	defaultRefreshTicker = time.Minute
)

type Config struct {
	DataDir           string
	Addr              string
	PublicBaseURL     string
	AdminToken        string
	EncryptionKey     []byte
	UpstreamProxy     string
	AutoTestURL       string
	RefreshTicker     time.Duration
	HTTPRequestTimout time.Duration
}

func DefaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "./data"
	}
	return filepath.Join(home, ".local", "share", "sub2clash")
}

func NewConfig(dataDir, addr, baseURL string) (*Config, error) {
	cfg := &Config{
		DataDir:           firstNonEmpty(dataDir, os.Getenv("SUB2CLASH_DATA_DIR"), DefaultDataDir()),
		Addr:              firstNonEmpty(addr, os.Getenv("SUB2CLASH_ADDR"), defaultAddr),
		PublicBaseURL:     firstNonEmpty(baseURL, os.Getenv("SUB2CLASH_PUBLIC_BASE_URL")),
		AdminToken:        os.Getenv("SUB2CLASH_ADMIN_TOKEN"),
		UpstreamProxy:     os.Getenv("SUB2CLASH_UPSTREAM_PROXY"),
		AutoTestURL:       firstNonEmpty(os.Getenv("SUB2CLASH_AUTO_TEST_URL"), defaultAutoTestURL),
		RefreshTicker:     defaultRefreshTicker,
		HTTPRequestTimout: 30 * time.Second,
	}

	key, err := parseEncryptionKey(os.Getenv("SUB2CLASH_ENCRYPTION_KEY"))
	if err != nil {
		return nil, err
	}
	cfg.EncryptionKey = key

	if cfg.PublicBaseURL == "" {
		cfg.PublicBaseURL = "http://127.0.0.1:8080"
	}
	if _, err := url.Parse(cfg.PublicBaseURL); err != nil {
		return nil, fmt.Errorf("invalid SUB2CLASH_PUBLIC_BASE_URL: %w", err)
	}
	return cfg, nil
}

func (c *Config) ValidateForStore() error {
	if len(c.EncryptionKey) != 32 {
		return errors.New("SUB2CLASH_ENCRYPTION_KEY must resolve to 32 bytes")
	}
	return os.MkdirAll(filepath.Join(c.DataDir, "profiles"), 0o755)
}

func (c *Config) ValidateForServe() error {
	if err := c.ValidateForStore(); err != nil {
		return err
	}
	if c.AdminToken == "" {
		return errors.New("SUB2CLASH_ADMIN_TOKEN is required for serve")
	}
	return nil
}

func parseEncryptionKey(raw string) ([]byte, error) {
	if raw == "" {
		return nil, nil
	}
	if len(raw) == 64 {
		if decoded, err := hex.DecodeString(raw); err == nil && len(decoded) == 32 {
			return decoded, nil
		}
	}
	if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(raw); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if len(raw) == 32 {
		return []byte(raw), nil
	}
	return nil, errors.New("SUB2CLASH_ENCRYPTION_KEY must be a 32-byte raw string, base64, or 64-char hex")
}

func ParseRefreshInterval(raw string) (time.Duration, error) {
	if strings.TrimSpace(raw) == "" {
		return defaultRefresh, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid refresh interval %q: %w", raw, err)
	}
	if value < time.Hour {
		return 0, errors.New("refresh interval must be at least 1h")
	}
	return value, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
