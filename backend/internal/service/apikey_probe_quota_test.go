package service

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuildAPIKeyProbeQuotaSnapshot_AnthropicHeaders(t *testing.T) {
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-requests-limit", "1000")
	headers.Set("anthropic-ratelimit-requests-remaining", "998")
	headers.Set("anthropic-ratelimit-requests-reset", "2026-05-02T10:00:00Z")
	headers.Set("anthropic-ratelimit-tokens-limit", "200000")
	headers.Set("anthropic-ratelimit-tokens-remaining", "199500")
	headers.Set("anthropic-ratelimit-tokens-reset", "2026-05-02T10:00:00Z")

	now := time.Date(2026, 5, 2, 10, 30, 0, 0, time.UTC)
	snapshot := BuildAPIKeyProbeQuotaSnapshot(PlatformAnthropic, http.StatusOK, "claude-sonnet-4-5", headers, now)

	require.NotNil(t, snapshot)
	require.Equal(t, PlatformAnthropic, snapshot.Provider)
	require.True(t, snapshot.Supported)
	require.Equal(t, http.StatusOK, snapshot.StatusCode)
	require.Equal(t, "claude-sonnet-4-5", snapshot.Model)
	require.Equal(t, "1000", snapshot.RequestsLimit)
	require.Equal(t, "998", snapshot.RequestsRemaining)
	require.Equal(t, "200000", snapshot.TokensLimit)
	require.Equal(t, "199500", snapshot.TokensRemaining)
	require.Equal(t, now.Format(time.RFC3339), snapshot.UpdatedAt)
}

func TestBuildAPIKeyProbeQuotaSnapshot_OpenAIHeaders(t *testing.T) {
	headers := http.Header{}
	headers.Set("x-ratelimit-limit-requests", "500")
	headers.Set("x-ratelimit-remaining-requests", "499")
	headers.Set("x-ratelimit-reset-requests", "1m0s")
	headers.Set("x-ratelimit-limit-tokens", "30000")
	headers.Set("x-ratelimit-remaining-tokens", "29900")
	headers.Set("x-ratelimit-reset-tokens", "1s")

	snapshot := BuildAPIKeyProbeQuotaSnapshot(PlatformOpenAI, http.StatusOK, "gpt-5.4", headers, time.Unix(0, 0).UTC())

	require.NotNil(t, snapshot)
	require.Equal(t, PlatformOpenAI, snapshot.Provider)
	require.True(t, snapshot.Supported)
	require.Equal(t, "500", snapshot.RequestsLimit)
	require.Equal(t, "499", snapshot.RequestsRemaining)
	require.Equal(t, "30000", snapshot.TokensLimit)
	require.Equal(t, "29900", snapshot.TokensRemaining)
	require.Equal(t, "headers", snapshot.Source)
}

func TestBuildAPIKeyProbeQuotaSnapshot_GeminiNoHeadersKeepsProbeContext(t *testing.T) {
	snapshot := BuildAPIKeyProbeQuotaSnapshot(PlatformGemini, http.StatusOK, "gemini-2.5-flash", http.Header{}, time.Unix(0, 0).UTC())

	require.NotNil(t, snapshot)
	require.Equal(t, PlatformGemini, snapshot.Provider)
	require.False(t, snapshot.Supported)
	require.Equal(t, "project", snapshot.Scope)
	require.Equal(t, "project_quota", snapshot.Source)
	require.False(t, snapshot.HasRateLimitHeaderSignal)
	require.Equal(t, http.StatusOK, snapshot.StatusCode)
	require.NotEmpty(t, snapshot.Note)
}

func TestBuildAPIKeyProbeQuotaSnapshot_GeminiHeadersAreRealSignal(t *testing.T) {
	headers := http.Header{}
	headers.Set("x-ratelimit-limit-requests", "60")
	headers.Set("x-ratelimit-remaining-requests", "42")
	headers.Set("x-ratelimit-reset-requests", "30s")
	headers.Set("x-goog-quota-project", "team-project")

	snapshot := BuildAPIKeyProbeQuotaSnapshot(PlatformGemini, http.StatusOK, "gemini-2.0-flash", headers, time.Unix(0, 0).UTC())

	require.NotNil(t, snapshot)
	require.Equal(t, PlatformGemini, snapshot.Provider)
	require.True(t, snapshot.Supported)
	require.Equal(t, "project", snapshot.Scope)
	require.Equal(t, "headers", snapshot.Source)
	require.True(t, snapshot.HasRateLimitHeaderSignal)
	require.Equal(t, "60", snapshot.RequestsLimit)
	require.Equal(t, "42", snapshot.RequestsRemaining)
	require.Equal(t, "team-project", snapshot.QuotaProject)
}

func TestBuildGLMAPIKeyProbeQuotaSnapshot_QuotaLimitFiveHourWindow(t *testing.T) {
	body := []byte(`{
		"code": 200,
		"data": {
			"level": "team",
			"limits": [
				{
					"type": "TOKENS_LIMIT",
					"unit": 3,
					"number": 5,
					"usage": 40000000,
					"currentValue": 10261098,
					"remaining": 29738902,
					"percentage": 25,
					"nextResetTime": 1767373239187
				}
			]
		}
	}`)
	now := time.Date(2026, 6, 24, 10, 30, 0, 0, time.UTC)

	snapshot := BuildGLMAPIKeyProbeQuotaSnapshot(http.StatusOK, body, now)

	require.NotNil(t, snapshot)
	require.Equal(t, PlatformGLM, snapshot.Provider)
	require.True(t, snapshot.Supported)
	require.Equal(t, "glm_quota_api", snapshot.Source)
	require.Equal(t, "account", snapshot.Scope)
	require.Equal(t, http.StatusOK, snapshot.StatusCode)
	require.Equal(t, "40000000", snapshot.TokensLimit)
	require.Equal(t, "29738902", snapshot.TokensRemaining)
	require.Equal(t, "2026-01-02T17:00:39Z", snapshot.TokensReset)
	require.Equal(t, "Token usage(5 Hour)", snapshot.RateLimitPolicy)
	require.True(t, snapshot.HasRateLimitHeaderSignal)
	require.Contains(t, snapshot.Note, "used=10261098")
	require.Contains(t, snapshot.Note, "25%")

	roundTrip := APIKeyProbeQuotaSnapshotFromExtra(BuildAPIKeyProbeQuotaExtraUpdates(snapshot))
	require.NotNil(t, roundTrip)
	require.Equal(t, "glm_quota_api", roundTrip.Source)
	require.Equal(t, "29738902", roundTrip.TokensRemaining)
	require.Equal(t, "Token usage(5 Hour)", roundTrip.RateLimitPolicy)
}

func TestAPIKeyProbeQuotaExtraRoundTrip(t *testing.T) {
	snapshot := &APIKeyProbeQuotaSnapshot{
		Provider:          PlatformAnthropic,
		Supported:         true,
		Source:            "headers",
		Scope:             "response_headers",
		UpdatedAt:         "2026-05-02T10:30:00Z",
		StatusCode:        http.StatusOK,
		Model:             "claude-sonnet-4-5",
		RequestsLimit:     "1000",
		RequestsRemaining: "998",
		TokensLimit:       "200000",
		TokensRemaining:   "199500",
	}

	updates := BuildAPIKeyProbeQuotaExtraUpdates(snapshot)
	require.Contains(t, updates, APIKeyProbeQuotaExtraKey)
	require.Equal(t, "2026-05-02T10:30:00Z", updates[APIKeyProbeQuotaUpdatedAtExtraKey])

	roundTrip := APIKeyProbeQuotaSnapshotFromExtra(updates)
	require.NotNil(t, roundTrip)
	require.Equal(t, snapshot.Provider, roundTrip.Provider)
	require.Equal(t, snapshot.TokensRemaining, roundTrip.TokensRemaining)
	require.Equal(t, snapshot.RequestsRemaining, roundTrip.RequestsRemaining)
}

func TestBuildAPIKeyProbeBalanceSnapshotRoundTrip(t *testing.T) {
	now := time.Date(2026, 5, 2, 10, 30, 0, 0, time.UTC)
	snapshot := BuildAPIKeyProbeBalanceSnapshot(PlatformOpenRouter, http.StatusOK, "$12.3400 remaining", "20", "7.66", "12.3400", "", now)

	require.NotNil(t, snapshot)
	require.Equal(t, PlatformOpenRouter, snapshot.Provider)
	require.Equal(t, "balance_api", snapshot.Source)
	require.Equal(t, "account", snapshot.Scope)
	require.True(t, snapshot.HasBalanceSignal)
	require.Equal(t, "$12.3400 remaining", snapshot.Balance)

	roundTrip := APIKeyProbeQuotaSnapshotFromExtra(BuildAPIKeyProbeQuotaExtraUpdates(snapshot))
	require.NotNil(t, roundTrip)
	require.True(t, roundTrip.HasBalanceSignal)
	require.Equal(t, "12.3400", roundTrip.CreditsRemaining)
}
