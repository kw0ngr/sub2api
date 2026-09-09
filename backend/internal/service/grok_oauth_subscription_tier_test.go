package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type grokSubscriptionTokenService struct {
	info *GrokTokenInfo
}

func (s *grokSubscriptionTokenService) RefreshAccountToken(context.Context, *Account) (*GrokTokenInfo, error) {
	return s.info, nil
}

func (s *grokSubscriptionTokenService) BuildAccountCredentials(info *GrokTokenInfo) map[string]any {
	return (&GrokOAuthService{}).BuildAccountCredentials(info)
}

type grokSubscriptionAccountRepo struct {
	AccountRepository
	account            *Account
	updatedCredentials map[string]any
	setSchedulableN    int
}

func (r *grokSubscriptionAccountRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	if r.account == nil || r.account.ID != id {
		return nil, fmt.Errorf("account %d not found", id)
	}
	return r.account, nil
}

func (r *grokSubscriptionAccountRepo) UpdateCredentials(_ context.Context, id int64, credentials map[string]any) error {
	if r.account == nil || r.account.ID != id {
		return fmt.Errorf("account %d not found", id)
	}
	r.updatedCredentials = cloneCredentials(credentials)
	r.account.Credentials = cloneCredentials(credentials)
	return nil
}

func (r *grokSubscriptionAccountRepo) SetSchedulable(context.Context, int64, bool) error {
	r.setSchedulableN++
	return nil
}

func (r *grokSubscriptionAccountRepo) SetTempUnschedulable(context.Context, int64, time.Time, string) error {
	return nil
}

func TestSubscriptionTierFromJWTUsesNumericClaim(t *testing.T) {
	tests := []struct {
		name        string
		accessToken string
		want        string
	}{
		{name: "numeric tier 5", accessToken: task17JWTWithClaims(t, map[string]any{"tier": 5}), want: "supergrok_heavy"},
		{name: "string alias", accessToken: task17JWTWithClaims(t, map[string]any{"tier": "SuperGrok Lite"}), want: "supergrok_lite"},
		{name: "missing claim", accessToken: task17JWTWithClaims(t, map[string]any{"sub": "user"}), want: ""},
		{name: "negative claim", accessToken: task17JWTWithClaims(t, map[string]any{"tier": -1}), want: ""},
		{name: "unknown claim", accessToken: task17JWTWithClaims(t, map[string]any{"tier": 8}), want: ""},
		{name: "malformed token", accessToken: "not-a-jwt", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			refresher := NewGrokTokenRefresher(&grokSubscriptionTokenService{info: &GrokTokenInfo{
				AccessToken:  tt.accessToken,
				RefreshToken: "new-refresh",
				ExpiresAt:    time.Now().Add(time.Hour).Unix(),
			}})

			// When
			credentials, err := refresher.Refresh(context.Background(), &Account{Platform: PlatformGrok, Type: AccountTypeOAuth})

			// Then
			require.NoError(t, err)
			got, _ := credentials["subscription_tier"].(string)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestGrokJWTTierDoesNotAffectSchedulability(t *testing.T) {
	// Given
	account := &Account{
		ID:           1717,
		Platform:     PlatformGrok,
		Type:         AccountTypeOAuth,
		Status:       StatusError,
		Schedulable:  false,
		ErrorMessage: "grok probe forbidden",
		Credentials: map[string]any{
			"access_token":  "expired-token",
			"refresh_token": "refresh-token",
			"expires_at":    time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
		},
	}
	repo := &grokSubscriptionAccountRepo{account: account}
	provider := NewGrokTokenProvider(repo, nil)
	provider.SetRefreshAPI(NewOAuthRefreshAPI(repo, nil), NewGrokTokenRefresher(&grokSubscriptionTokenService{info: &GrokTokenInfo{
		AccessToken:  task17JWTWithClaims(t, map[string]any{"tier": 7}),
		RefreshToken: "new-refresh",
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
	}}))

	// When
	token, err := provider.GetAccessToken(context.Background(), account)

	// Then
	require.NoError(t, err)
	require.Equal(t, repo.updatedCredentials["access_token"], token)
	require.Equal(t, "supergrok_plus", repo.updatedCredentials["subscription_tier"])
	require.Equal(t, StatusError, account.Status)
	require.False(t, account.Schedulable)
	require.Zero(t, repo.setSchedulableN)

	// Given: a forged tier in an expired access-token-only account.
	expiredOnly := &Account{
		ID:          1719,
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"access_token": task17JWTWithClaims(t, map[string]any{"tier": 7}),
			"expires_at":   time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
		},
	}

	// When
	_, err = NewGrokTokenProvider(&grokSubscriptionAccountRepo{account: expiredOnly}, nil).GetAccessToken(context.Background(), expiredOnly)

	// Then
	require.Error(t, err)
	require.False(t, expiredOnly.IsSchedulable())
}

func TestGrokRefreshedTokenTierRespectsUserPrecedence(t *testing.T) {
	tests := []struct {
		name  string
		extra map[string]any
		want  string
	}{
		{name: "jwt tier fills missing display metadata", want: "supergrok_heavy"},
		{name: "existing upstream user tier wins", extra: map[string]any{"subscription_tier": "SuperGrok Lite"}, want: "supergrok_lite"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			account := &Account{
				Platform:    PlatformGrok,
				Type:        AccountTypeOAuth,
				Credentials: map[string]any{"refresh_token": "refresh-token"},
				Extra:       tt.extra,
			}
			refresher := NewGrokTokenRefresher(&grokSubscriptionTokenService{info: &GrokTokenInfo{
				AccessToken: task17JWTWithClaims(t, map[string]any{"tier": 5}),
				ExpiresAt:   time.Now().Add(time.Hour).Unix(),
			}})

			// When
			credentials, err := refresher.Refresh(context.Background(), account)

			// Then
			require.NoError(t, err)
			require.Equal(t, tt.want, credentials["subscription_tier"])
		})
	}
}

func task17JWTWithClaims(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}
