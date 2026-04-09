package sub2clash

import "time"

type Profile struct {
	ID                  string
	Name                string
	SourceURLCiphertext string
	RefreshInterval     time.Duration
	LastVariant         string
	LastUserAgent       string
	LastRefreshAt       time.Time
	LastSuccessAt       time.Time
	LastError           string
	SubscriptionInfo    string
	YAMLRelPath         string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type ProfileSummary struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	ManagedURL       string    `json:"managed_url,omitempty"`
	DownloadURL      string    `json:"download_url,omitempty"`
	RefreshInterval  string    `json:"refresh_interval"`
	LastVariant      string    `json:"last_variant,omitempty"`
	LastUserAgent    string    `json:"last_user_agent,omitempty"`
	LastRefreshAt    time.Time `json:"last_refresh_at,omitempty"`
	LastSuccessAt    time.Time `json:"last_success_at,omitempty"`
	LastError        string    `json:"last_error,omitempty"`
	SubscriptionInfo string    `json:"subscription_userinfo,omitempty"`
}

type ManagedProfile struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	ManagedURL       string   `json:"managed_url"`
	DownloadURL      string   `json:"download_url"`
	RefreshInterval  string   `json:"refresh_interval"`
	Warnings         []string `json:"warnings,omitempty"`
	SubscriptionInfo string   `json:"subscription_userinfo,omitempty"`
}

type FetchStrategy struct {
	Variant   string
	UserAgent string
}

type ConvertResult struct {
	YAML             []byte
	Warnings         []string
	SubscriptionInfo string
	Strategy         FetchStrategy
}

type AdminCreateProfileRequest struct {
	Name            string `json:"name"`
	URL             string `json:"url"`
	RefreshInterval string `json:"refresh_interval,omitempty"`
}

type AdminRefreshResponse struct {
	ID               string   `json:"id"`
	Warnings         []string `json:"warnings,omitempty"`
	SubscriptionInfo string   `json:"subscription_userinfo,omitempty"`
}
