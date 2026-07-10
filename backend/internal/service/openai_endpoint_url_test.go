package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildOpenAIEndpointURL_handles_version_suffixes(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		endpoint string
		want     string
	}{
		{"root", "https://api.openai.com", "/v1/responses", "https://api.openai.com/v1/responses"},
		{"v1", "https://proxy.example.com/v1", "/v1/responses", "https://proxy.example.com/v1/responses"},
		{"v1 models", "https://proxy.example.com/v1", "/v1/models", "https://proxy.example.com/v1/models"},
		{"already endpoint", "https://proxy.example.com/v1/responses", "/v1/responses", "https://proxy.example.com/v1/responses"},
		{"version beta", "https://proxy.example.com/v1beta", "/v1/responses", "https://proxy.example.com/v1beta/responses"},
		{"responses suffix", "https://proxy.example.com/responses", "/v1/responses", "https://proxy.example.com/responses"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildOpenAIEndpointURL(tt.base, tt.endpoint)

			require.Equal(t, tt.want, got)
		})
	}
}

func TestBodyHasSSEFraming_only_matches_line_prefix(t *testing.T) {
	require.False(t, bodyHasSSEFraming([]byte(`{"output_text":"contains data: but it is JSON"}`)))
	require.True(t, bodyHasSSEFraming([]byte("event: response.completed\ndata: {}\n\n")))
	require.True(t, bodyHasSSEFraming([]byte("x: 1\r\ndata: {}\r\n")))
}

func TestIsOpenAITransientProcessingError_matches_overload_codes_and_capacity(t *testing.T) {
	require.True(t, isOpenAITransientProcessingError(
		http.StatusServiceUnavailable,
		"",
		[]byte(`{"error":{"code":"server_is_overloaded","message":"try later"}}`),
	))
	require.True(t, isOpenAITransientProcessingError(
		http.StatusServiceUnavailable,
		"",
		[]byte(`{"response":{"error":{"code":"slow_down","message":"try later"}}}`),
	))
	require.True(t, isOpenAITransientProcessingError(
		http.StatusBadRequest,
		"selected model is at capacity",
		nil,
	))
	require.False(t, isOpenAITransientProcessingError(
		http.StatusServiceUnavailable,
		"generic outage",
		[]byte(`{"error":{"code":"unknown"}}`),
	))
}
