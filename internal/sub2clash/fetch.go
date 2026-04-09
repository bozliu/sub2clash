package sub2clash

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Fetcher struct {
	timeout       time.Duration
	upstreamProxy string
}

func NewFetcher(cfg *Config) *Fetcher {
	return &Fetcher{
		timeout:       cfg.HTTPRequestTimout,
		upstreamProxy: strings.TrimSpace(cfg.UpstreamProxy),
	}
}

type fetchAttempt struct {
	URL       string
	Variant   string
	UserAgent string
}

func (f *Fetcher) Fetch(ctx context.Context, sourceURL string, preferred FetchStrategy) ([]byte, http.Header, FetchStrategy, error) {
	candidates := buildFetchAttempts(sourceURL, preferred)
	var errorsSeen []string

	for _, attempt := range candidates {
		body, headers, err := f.doRequest(ctx, attempt, false)
		if err == nil {
			return body, headers, FetchStrategy{Variant: attempt.Variant, UserAgent: attempt.UserAgent}, nil
		}
		errorsSeen = append(errorsSeen, fmt.Sprintf("%s/%s: %v", attempt.Variant, attempt.UserAgent, err))

		if f.upstreamProxy != "" {
			body, headers, err = f.doRequest(ctx, attempt, true)
			if err == nil {
				return body, headers, FetchStrategy{Variant: attempt.Variant, UserAgent: attempt.UserAgent}, nil
			}
			errorsSeen = append(errorsSeen, fmt.Sprintf("%s/%s via proxy: %v", attempt.Variant, attempt.UserAgent, err))
		}
	}

	return nil, nil, FetchStrategy{}, fmt.Errorf("fetch failed after %d attempts: %s", len(candidates), strings.Join(errorsSeen, "; "))
}

func (f *Fetcher) doRequest(ctx context.Context, attempt fetchAttempt, useProxy bool) ([]byte, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, attempt.URL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", attempt.UserAgent)
	req.Header.Set("Accept", "*/*")

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}
	if useProxy {
		parsed, err := url.Parse(f.upstreamProxy)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid SUB2CLASH_UPSTREAM_PROXY: %w", err)
		}
		transport.Proxy = http.ProxyURL(parsed)
	}
	client := &http.Client{
		Timeout:   f.timeout,
		Transport: transport,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.Header.Clone(), fmt.Errorf("upstream status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, nil, err
	}
	return body, resp.Header.Clone(), nil
}

func buildFetchAttempts(sourceURL string, preferred FetchStrategy) []fetchAttempt {
	var attempts []fetchAttempt
	seen := map[string]bool{}

	uaCandidates := []string{"sub2clash/1.0"}
	if strings.Contains(sourceURL, "token=") {
		uaCandidates = append(uaCandidates, "Shadowrocket")
	}

	variantCandidates := []struct {
		label  string
		suffix string
	}{
		{label: "base", suffix: ""},
		{label: "flag=clash", suffix: "flag=clash"},
		{label: "flag=shadowrocket", suffix: "flag=shadowrocket"},
		{label: "flag=stash", suffix: "flag=stash"},
		{label: "flag=surge", suffix: "flag=surge"},
		{label: "sub=1", suffix: "sub=1"},
		{label: "clash=1", suffix: "clash=1"},
	}

	queue := func(attempt fetchAttempt) {
		key := attempt.URL + "\x00" + attempt.UserAgent
		if seen[key] {
			return
		}
		seen[key] = true
		attempts = append(attempts, attempt)
	}

	if preferred.Variant != "" || preferred.UserAgent != "" {
		for _, variant := range variantCandidates {
			if preferred.Variant != "" && variant.label != preferred.Variant {
				continue
			}
			targetURL := withSuffix(sourceURL, variant.suffix)
			if preferred.UserAgent != "" {
				queue(fetchAttempt{URL: targetURL, Variant: variant.label, UserAgent: preferred.UserAgent})
			}
		}
	}

	for _, variant := range variantCandidates {
		targetURL := withSuffix(sourceURL, variant.suffix)
		for _, userAgent := range uaCandidates {
			queue(fetchAttempt{URL: targetURL, Variant: variant.label, UserAgent: userAgent})
		}
	}
	return attempts
}

func withSuffix(rawURL, suffix string) string {
	if suffix == "" {
		return rawURL
	}
	if strings.Contains(rawURL, suffix) {
		return rawURL
	}
	sep := "&"
	if !strings.Contains(rawURL, "?") {
		sep = "?"
	}
	return rawURL + sep + suffix
}
