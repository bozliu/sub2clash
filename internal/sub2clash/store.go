package sub2clash

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db      *sql.DB
	dataDir string
}

func OpenStore(ctx context.Context, dataDir string) (*Store, error) {
	if err := os.MkdirAll(filepath.Join(dataDir, "profiles"), 0o755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dataDir, "sub2clash.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db, dataDir: dataDir}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS profiles (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	source_url_ciphertext TEXT NOT NULL,
	refresh_interval_seconds INTEGER NOT NULL,
	last_variant TEXT NOT NULL DEFAULT '',
	last_user_agent TEXT NOT NULL DEFAULT '',
	last_refresh_at INTEGER NOT NULL DEFAULT 0,
	last_success_at INTEGER NOT NULL DEFAULT 0,
	last_error TEXT NOT NULL DEFAULT '',
	subscription_info TEXT NOT NULL DEFAULT '',
	yaml_rel_path TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);
`
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

func (s *Store) CreateProfile(ctx context.Context, profile *Profile) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO profiles (
			id, name, source_url_ciphertext, refresh_interval_seconds, last_variant, last_user_agent,
			last_refresh_at, last_success_at, last_error, subscription_info, yaml_rel_path, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		profile.ID,
		profile.Name,
		profile.SourceURLCiphertext,
		int64(profile.RefreshInterval.Seconds()),
		profile.LastVariant,
		profile.LastUserAgent,
		toUnix(profile.LastRefreshAt),
		toUnix(profile.LastSuccessAt),
		profile.LastError,
		profile.SubscriptionInfo,
		profile.YAMLRelPath,
		toUnix(profile.CreatedAt),
		toUnix(profile.UpdatedAt),
	)
	return err
}

func (s *Store) UpdateProfileState(ctx context.Context, profile *Profile) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE profiles SET
			name = ?,
			refresh_interval_seconds = ?,
			last_variant = ?,
			last_user_agent = ?,
			last_refresh_at = ?,
			last_success_at = ?,
			last_error = ?,
			subscription_info = ?,
			updated_at = ?
		WHERE id = ?`,
		profile.Name,
		int64(profile.RefreshInterval.Seconds()),
		profile.LastVariant,
		profile.LastUserAgent,
		toUnix(profile.LastRefreshAt),
		toUnix(profile.LastSuccessAt),
		profile.LastError,
		profile.SubscriptionInfo,
		toUnix(profile.UpdatedAt),
		profile.ID,
	)
	return err
}

func (s *Store) GetProfile(ctx context.Context, id string) (*Profile, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, source_url_ciphertext, refresh_interval_seconds, last_variant, last_user_agent,
       last_refresh_at, last_success_at, last_error, subscription_info, yaml_rel_path, created_at, updated_at
FROM profiles WHERE id = ?`, id)

	profile, err := scanProfile(row)
	if err != nil {
		return nil, err
	}
	return profile, nil
}

func (s *Store) ListProfiles(ctx context.Context) ([]*Profile, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, source_url_ciphertext, refresh_interval_seconds, last_variant, last_user_agent,
       last_refresh_at, last_success_at, last_error, subscription_info, yaml_rel_path, created_at, updated_at
FROM profiles ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []*Profile
	for rows.Next() {
		profile, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, rows.Err()
}

func (s *Store) DueProfiles(ctx context.Context, now time.Time) ([]*Profile, error) {
	profiles, err := s.ListProfiles(ctx)
	if err != nil {
		return nil, err
	}

	var due []*Profile
	for _, profile := range profiles {
		anchor := profile.LastRefreshAt
		if anchor.IsZero() {
			anchor = profile.CreatedAt
		}
		if anchor.IsZero() || anchor.Add(profile.RefreshInterval).Before(now) || anchor.Add(profile.RefreshInterval).Equal(now) {
			due = append(due, profile)
		}
	}
	return due, nil
}

func (s *Store) ProfilePath(profile *Profile) string {
	return filepath.Join(s.dataDir, profile.YAMLRelPath)
}

func scanProfile(scanner interface {
	Scan(dest ...any) error
}) (*Profile, error) {
	var (
		profile              Profile
		refreshSeconds       int64
		lastRefreshUnix      int64
		lastSuccessUnix      int64
		createdUnix, updated int64
	)

	if err := scanner.Scan(
		&profile.ID,
		&profile.Name,
		&profile.SourceURLCiphertext,
		&refreshSeconds,
		&profile.LastVariant,
		&profile.LastUserAgent,
		&lastRefreshUnix,
		&lastSuccessUnix,
		&profile.LastError,
		&profile.SubscriptionInfo,
		&profile.YAMLRelPath,
		&createdUnix,
		&updated,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("profile not found")
		}
		return nil, err
	}

	profile.RefreshInterval = time.Duration(refreshSeconds) * time.Second
	profile.LastRefreshAt = fromUnix(lastRefreshUnix)
	profile.LastSuccessAt = fromUnix(lastSuccessUnix)
	profile.CreatedAt = fromUnix(createdUnix)
	profile.UpdatedAt = fromUnix(updated)
	return &profile, nil
}

func toUnix(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.UTC().Unix()
}

func fromUnix(value int64) time.Time {
	if value == 0 {
		return time.Time{}
	}
	return time.Unix(value, 0).UTC()
}
