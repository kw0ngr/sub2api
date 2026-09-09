package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrokQuotaHeaderTierHintRespectsUserPrecedence(t *testing.T) {
	tests := []struct {
		name            string
		extra           map[string]any
		wantTierUpdate  string
		wantTierPresent bool
	}{
		{name: "does not overwrite existing user tier", extra: map[string]any{"subscription_tier": "SuperGrok Lite"}},
		{name: "fills missing user tier", wantTierUpdate: "supergrok_heavy", wantTierPresent: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			account := &Account{
				ID:          1917,
				Platform:    PlatformGrok,
				Type:        AccountTypeOAuth,
				Status:      StatusActive,
				Schedulable: true,
				Credentials: map[string]any{"access_token": "token"},
				Extra:       tt.extra,
				Concurrency: 1,
			}
			repo := &grokQuotaAccountRepoStub{account: account}
			headers := http.Header{}
			headers.Set("x-ratelimit-limit-requests", "60")
			headers.Set("xai-subscription-tier", "5")
			headers.Set("xai-entitlement-status", "eligible")
			svc := NewGrokQuotaService(repo, nil, nil, &grokQuotaHTTPUpstreamStub{
				response: &http.Response{
					StatusCode: http.StatusOK,
					Header:     headers,
					Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
				},
			})

			// When
			_, err := svc.ProbeHeaders(context.Background(), account.ID)

			// Then
			require.NoError(t, err)
			gotTier, hasTier := repo.extraUpdates["subscription_tier"]
			require.Equal(t, tt.wantTierPresent, hasTier)
			if tt.wantTierPresent {
				require.Equal(t, tt.wantTierUpdate, gotTier)
			}
			require.Equal(t, "eligible", repo.extraUpdates["entitlement_status"])
		})
	}
}
