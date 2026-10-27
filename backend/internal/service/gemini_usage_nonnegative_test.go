package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeminiUsageCacheOnlyNeverProducesNegativeInput(t *testing.T) {
	for _, raw := range []string{
		`{"usageMetadata":{"cachedContentTokenCount":30}}`,
		`{"usageMetadata":{"promptTokenCount":10,"cachedContentTokenCount":30}}`,
	} {
		usage := extractGeminiUsage([]byte(raw))
		require.NotNil(t, usage)
		require.Zero(t, usage.InputTokens)
		require.Equal(t, 30, usage.CacheReadInputTokens)
	}
	usage := extractGeminiUsage([]byte(`{"usageMetadata":{"promptTokenCount":50,"cachedContentTokenCount":30,"candidatesTokenCount":5}}`))
	require.Equal(t, 20, usage.InputTokens)
	require.Equal(t, 5, usage.OutputTokens)
}
