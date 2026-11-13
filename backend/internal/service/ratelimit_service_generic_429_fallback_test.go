//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestRateLimitService_Generic429FallbackHonorsSettings(t *testing.T) {
	for _, platform := range []string{PlatformGrok, PlatformGemini, PlatformOpenAI} {
		t.Run(platform, func(t *testing.T) {
			for _, resetHeader := range []string{"", "not-a-timestamp"} {
				name := "missing_reset"
				if resetHeader != "" {
					name = "invalid_reset"
				}
				t.Run(name, func(t *testing.T) {
					for _, tc := range []struct {
						name     string
						settings *RateLimit429CooldownSettings
						cooldown time.Duration
					}{
						{name: "default", cooldown: 5 * time.Second},
						{name: "configured", settings: &RateLimit429CooldownSettings{Enabled: true, CooldownSeconds: 12}, cooldown: 12 * time.Second},
						{name: "disabled", settings: &RateLimit429CooldownSettings{Enabled: false, CooldownSeconds: 12}},
					} {
						t.Run(tc.name, func(t *testing.T) {
							repo := &rateLimitAccountRepoStub{}
							svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
							if tc.settings != nil {
								data, err := json.Marshal(tc.settings)
								require.NoError(t, err)
								settingsRepo := &rateLimit429SettingRepo{data: map[string]string{
									SettingKeyRateLimit429CooldownSettings: string(data),
								}}
								svc.SetSettingService(NewSettingService(settingsRepo, &config.Config{}))
							}
							account := &Account{ID: 4701, Platform: platform, Type: AccountTypeOAuth}
							headers := http.Header{}
							if resetHeader != "" {
								headers.Set("anthropic-ratelimit-unified-reset", resetHeader)
							}

							before := time.Now()
							svc.HandleUpstreamError(context.Background(), account, http.StatusTooManyRequests, headers,
								[]byte(`{"error":{"message":"Too many requests"}}`), "inference-model")
							after := time.Now()

							if tc.cooldown == 0 {
								require.Zero(t, repo.rateLimitedCalls, "disabled fallback must not mark the account")
							} else {
								require.Equal(t, 1, repo.rateLimitedCalls)
								require.NotNil(t, repo.lastRateLimitResetAt)
								require.False(t, repo.lastRateLimitResetAt.Before(before.Add(tc.cooldown)))
								require.False(t, repo.lastRateLimitResetAt.After(after.Add(tc.cooldown)))
							}
							require.Zero(t, repo.setErrorCalls)
							require.Zero(t, repo.tempCalls)
							require.Empty(t, repo.modelRateLimitScopes)
						})
					}
				})
			}
		})
	}
}
