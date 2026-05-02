package admin

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBuildAccountPoolMapAccountIncludesAPIKeyProbeQuota(t *testing.T) {
	account := AccountWithConcurrency{
		Account: &dto.Account{
			ID:          42,
			Name:        "anthropic-apikey",
			Platform:    service.PlatformAnthropic,
			Type:        service.AccountTypeAPIKey,
			Status:      service.StatusActive,
			Schedulable: true,
			Extra: map[string]any{
				service.APIKeyProbeQuotaExtraKey: map[string]any{
					"provider":           service.PlatformAnthropic,
					"supported":          true,
					"source":             "headers",
					"updated_at":         "2026-05-02T10:30:00Z",
					"status_code":        200,
					"model":              "claude-sonnet-4-5",
					"tokens_limit":       "200000",
					"tokens_remaining":   "199500",
					"requests_limit":     "1000",
					"requests_remaining": "998",
				},
			},
		},
	}

	mapped := buildAccountPoolMapAccount(account, time.Date(2026, 5, 2, 10, 31, 0, 0, time.UTC))

	require.NotNil(t, mapped.APIKeyProbeQuota)
	require.Equal(t, "199500", mapped.APIKeyProbeQuota.TokensRemaining)
	require.Equal(t, "998", mapped.APIKeyProbeQuota.RequestsRemaining)

	var summary AccountPoolMapSummary
	accountPoolAddSummary(&summary, mapped)
	require.Equal(t, 1, summary.QuotaSignals)
}
