package auth

import (
	"testing"
	"time"
)

func TestCredentials_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		creds     *Credentials
		expected  bool
	}{
		{"nil credentials", nil, true},
		{"zero expiry", &Credentials{}, true},
		{"expired", &Credentials{ExpiresAt: time.Now().Add(-1 * time.Hour)}, true},
		{"expires in 1 minute", &Credentials{ExpiresAt: time.Now().Add(1 * time.Minute)}, true},   // Within 5-minute buffer
		{"expires in 4 minutes", &Credentials{ExpiresAt: time.Now().Add(4 * time.Minute)}, true}, // Within 5-minute buffer
		{"expires in 6 minutes", &Credentials{ExpiresAt: time.Now().Add(6 * time.Minute)}, false},
		{"expires in 1 hour", &Credentials{ExpiresAt: time.Now().Add(1 * time.Hour)}, false},
		{"expires in 1 day", &Credentials{ExpiresAt: time.Now().Add(24 * time.Hour)}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.creds.IsExpired()
			if got != tt.expected {
				t.Errorf("IsExpired() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCredentials_HasValidToken(t *testing.T) {
	tests := []struct {
		name     string
		creds    *Credentials
		expected bool
	}{
		{"nil credentials", nil, false},
		{"empty access token", &Credentials{AccessToken: "", ExpiresAt: time.Now().Add(1 * time.Hour)}, false},
		{"expired token", &Credentials{AccessToken: "valid-token", ExpiresAt: time.Now().Add(-1 * time.Hour)}, false},
		{"valid token", &Credentials{AccessToken: "valid-token", ExpiresAt: time.Now().Add(1 * time.Hour)}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.creds.HasValidToken()
			if got != tt.expected {
				t.Errorf("HasValidToken() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCredentials_HasProxyCredentials(t *testing.T) {
	tests := []struct {
		name     string
		creds    *Credentials
		expected bool
	}{
		{"nil credentials", nil, false},
		{"empty credentials", &Credentials{}, false},
		{"only proxy api key", &Credentials{ProxyAPIKey: "key"}, false},
		{"only proxy id", &Credentials{ProxyID: "id"}, false},
		{"both present", &Credentials{ProxyAPIKey: "key", ProxyID: "id"}, true},
		{"full credentials", &Credentials{
			AccessToken:  "token",
			RefreshToken: "refresh",
			ExpiresAt:    time.Now().Add(1 * time.Hour),
			ProxyAPIKey:  "key",
			ProxyID:      "id",
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.creds.HasProxyCredentials()
			if got != tt.expected {
				t.Errorf("HasProxyCredentials() = %v, want %v", got, tt.expected)
			}
		})
	}
}
