package sub2clash

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Service struct {
	cfg     *Config
	store   *Store
	secrets *SecretBox
	fetcher *Fetcher
}

func NewService(cfg *Config, store *Store) (*Service, error) {
	if err := cfg.ValidateForStore(); err != nil {
		return nil, err
	}
	secrets, err := NewSecretBox(cfg.EncryptionKey)
	if err != nil {
		return nil, err
	}
	return &Service{
		cfg:     cfg,
		store:   store,
		secrets: secrets,
		fetcher: NewFetcher(cfg),
	}, nil
}

func (s *Service) ConvertURL(ctx context.Context, sourceURL string) (*ConvertResult, error) {
	body, headers, strategy, err := s.fetcher.Fetch(ctx, sourceURL, FetchStrategy{})
	if err != nil {
		return nil, err
	}
	result, err := ConvertRawSubscription(body, headers, s.cfg.AutoTestURL)
	if err != nil {
		return nil, err
	}
	result.Strategy = strategy
	return result, nil
}

func (s *Service) AddProfile(ctx context.Context, name, sourceURL string, refreshInterval time.Duration) (*ManagedProfile, error) {
	if strings.TrimSpace(sourceURL) == "" {
		return nil, errors.New("source url is required")
	}
	if strings.TrimSpace(name) == "" {
		name = "Imported Profile"
	}

	ciphertext, err := s.secrets.EncryptString(sourceURL)
	if err != nil {
		return nil, err
	}

	id := NewProfileID()
	profile := &Profile{
		ID:                  id,
		Name:                name,
		SourceURLCiphertext: ciphertext,
		RefreshInterval:     refreshInterval,
		YAMLRelPath:         filepath.ToSlash(filepath.Join("profiles", id, "clash.yaml")),
		CreatedAt:           time.Now().UTC(),
		UpdatedAt:           time.Now().UTC(),
	}
	if err := s.store.CreateProfile(ctx, profile); err != nil {
		return nil, err
	}
	refreshResult, err := s.RefreshProfile(ctx, id)
	if err != nil {
		return nil, err
	}
	managed := s.publicProfile(profile)
	managed.Warnings = refreshResult.Warnings
	managed.SubscriptionInfo = refreshResult.SubscriptionInfo
	return managed, nil
}

func (s *Service) RefreshProfile(ctx context.Context, id string) (*ConvertResult, error) {
	profile, err := s.store.GetProfile(ctx, id)
	if err != nil {
		return nil, err
	}
	sourceURL, err := s.secrets.DecryptString(profile.SourceURLCiphertext)
	if err != nil {
		return nil, err
	}

	result, err := s.refreshWithProfile(ctx, profile, sourceURL)
	if err != nil {
		profile.LastRefreshAt = time.Now().UTC()
		profile.LastError = err.Error()
		profile.UpdatedAt = time.Now().UTC()
		_ = s.store.UpdateProfileState(ctx, profile)
		return nil, err
	}
	return result, nil
}

func (s *Service) refreshWithProfile(ctx context.Context, profile *Profile, sourceURL string) (*ConvertResult, error) {
	preferred := FetchStrategy{
		Variant:   profile.LastVariant,
		UserAgent: resolveUserAgent(profile.LastUserAgent),
	}
	body, headers, strategy, err := s.fetcher.Fetch(ctx, sourceURL, preferred)
	if err != nil {
		return nil, err
	}

	result, err := ConvertRawSubscription(body, headers, s.cfg.AutoTestURL)
	if err != nil {
		return nil, err
	}
	result.Strategy = strategy

	target := s.store.ProfilePath(profile)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil, err
	}
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, result.YAML, 0o644); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, target); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	profile.LastVariant = strategy.Variant
	profile.LastUserAgent = strategy.UserAgent
	profile.LastRefreshAt = now
	profile.LastSuccessAt = now
	profile.LastError = ""
	profile.SubscriptionInfo = result.SubscriptionInfo
	profile.UpdatedAt = now
	if err := s.store.UpdateProfileState(ctx, profile); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Service) ListProfiles(ctx context.Context) ([]ProfileSummary, error) {
	profiles, err := s.store.ListProfiles(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]ProfileSummary, 0, len(profiles))
	for _, profile := range profiles {
		managed := s.publicProfile(profile)
		items = append(items, ProfileSummary{
			ID:               managed.ID,
			Name:             managed.Name,
			ManagedURL:       managed.ManagedURL,
			DownloadURL:      managed.DownloadURL,
			RefreshInterval:  managed.RefreshInterval,
			LastVariant:      profile.LastVariant,
			LastUserAgent:    profile.LastUserAgent,
			LastRefreshAt:    profile.LastRefreshAt,
			LastSuccessAt:    profile.LastSuccessAt,
			LastError:        profile.LastError,
			SubscriptionInfo: profile.SubscriptionInfo,
		})
	}
	return items, nil
}

func (s *Service) GetManagedYAML(ctx context.Context, id string) ([]byte, string, error) {
	profile, err := s.store.GetProfile(ctx, id)
	if err != nil {
		return nil, "", err
	}
	body, err := os.ReadFile(s.store.ProfilePath(profile))
	if err != nil {
		return nil, "", err
	}
	return body, profile.SubscriptionInfo, nil
}

func (s *Service) RefreshDueProfiles(ctx context.Context) error {
	due, err := s.store.DueProfiles(ctx, time.Now().UTC())
	if err != nil {
		return err
	}
	for _, profile := range due {
		sourceURL, err := s.secrets.DecryptString(profile.SourceURLCiphertext)
		if err != nil {
			continue
		}
		_, _ = s.refreshWithProfile(ctx, profile, sourceURL)
	}
	return nil
}

func (s *Service) publicProfile(profile *Profile) *ManagedProfile {
	base := strings.TrimRight(s.cfg.PublicBaseURL, "/")
	return &ManagedProfile{
		ID:              profile.ID,
		Name:            profile.Name,
		ManagedURL:      fmt.Sprintf("%s/profiles/%s/clash.yaml", base, profile.ID),
		DownloadURL:     fmt.Sprintf("%s/profiles/%s/download", base, profile.ID),
		RefreshInterval: profile.RefreshInterval.String(),
	}
}

func resolveUserAgent(label string) string {
	switch label {
	case "shadowrocket", "Shadowrocket":
		return "Shadowrocket"
	case "", "default":
		return ""
	default:
		return label
	}
}
