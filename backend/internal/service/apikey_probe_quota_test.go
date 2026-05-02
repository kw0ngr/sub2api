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
	require.Equal(t, http.StatusOK, snapshot.StatusCode)
	require.NotEmpty(t, snapshot.Note)
}

func TestAPIKeyProbeQuotaExtraRoundTrip(t *testing.T) {
	snapshot := &APIKeyProbeQuotaSnapshot{
		Provider:          PlatformAnthropic,
		Supported:         true,
		Source:            "headers",
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
