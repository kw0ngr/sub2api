//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAccountIsSchedulable_QuotaExceeded(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		account *Account
		want    bool
	}{
		{
			name: "apikey daily quota exceeded",
			account: &Account{
				Status:      StatusActive,
				Schedulable: true,
				Type:        AccountTypeAPIKey,
				Extra: map[string]any{
					"quota_daily_limit": 10.0,
					"quota_daily_used":  10.0,
					"quota_daily_start": now.Add(-1 * time.Hour).Format(time.RFC3339),
				},
			},
			want: false,
		},
		{
			name: "apikey weekly quota exceeded",
			account: &Account{
				Status:      StatusActive,
				Schedulable: true,
				Type:        AccountTypeAPIKey,
				Extra: map[string]any{
					"quota_weekly_limit": 20.0,
					"quota_weekly_used":  20.0,
					"quota_weekly_start": now.Add(-24 * time.Hour).Format(time.RFC3339),
				},
			},
			want: false,
		},
		{
			name: "bedrock total quota exceeded",
			account: &Account{
				Status:      StatusActive,
				Schedulable: true,
				Type:        AccountTypeBedrock,
				Extra: map[string]any{
					"quota_limit": 5.0,
					"quota_used":  5.0,
				},
			},
			want: false,
		},
		{
			name: "oauth account ignores quota exceeded",
			account: &Account{
				Status:      StatusActive,
				Schedulable: true,
				Type:        AccountTypeOAuth,
				Extra: map[string]any{
					"quota_limit": 5.0,
					"quota_used":  5.0,
				},
			},
			want: true,
		},
		{
			name: "apikey expired daily window remains schedulable",
			account: &Account{
				Status:      StatusActive,
				Schedulable: true,
				Type:        AccountTypeAPIKey,
				Extra: map[string]any{
					"quota_daily_limit": 10.0,
					"quota_daily_used":  10.0,
					"quota_daily_start": now.Add(-48 * time.Hour).Format(time.RFC3339),
				},
			},
			want: true,
		},
		{
			name: "grok request quota exhausted before reset",
			account: &Account{
				Status:      StatusActive,
				Schedulable: true,
				Platform:    PlatformGrok,
				Type:        AccountTypeOAuth,
				Extra: map[string]any{
					"grok_usage_snapshot": map[string]any{
						"requests": map[string]any{
							"remaining":  0,
							"limit":      60,
							"reset_unix": now.Add(30 * time.Minute).Unix(),
						},
						"headers_observed": true,
						"updated_at":       now.Format(time.RFC3339),
					},
				},
			},
			want: false,
		},
		{
			name: "grok request quota recovered after reset",
			account: &Account{
				Status:      StatusActive,
				Schedulable: true,
				Platform:    PlatformGrok,
				Type:        AccountTypeOAuth,
				Extra: map[string]any{
					"grok_usage_snapshot": map[string]any{
						"requests": map[string]any{
							"remaining":  0,
							"limit":      60,
							"reset_unix": now.Add(-5 * time.Minute).Unix(),
						},
						"headers_observed": true,
						"updated_at":       now.Format(time.RFC3339),
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.account.IsSchedulable())
		})
	}
}
