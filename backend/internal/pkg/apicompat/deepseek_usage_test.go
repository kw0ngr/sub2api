package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeepSeekChatCacheUsageSurvivesResponsesBridge(t *testing.T) {
	for _, tc := range []struct {
		name, fields string
		want         int
	}{
		{"deepseek", `"prompt_cache_hit_tokens":80`, 80},
		{"nested", `"prompt_cache_hit_tokens":80,"prompt_tokens_details":{"cached_tokens":64,"audio_tokens":2}`, 64},
		{"zero", `"prompt_cache_hit_tokens":80,"prompt_tokens_details":{"cached_tokens":0}`, 0},
		{"missing_nested_cache", `"prompt_cache_hit_tokens":80,"prompt_tokens_details":{"audio_tokens":2}`, 80},
		{"negative", `"prompt_cache_hit_tokens":-1`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := `{"prompt_tokens":100,"completion_tokens":5,` + tc.fields + `}`
			var chunk ChatCompletionsChunk
			require.NoError(t, json.Unmarshal([]byte(`{"usage":`+raw+`}`), &chunk))
			usage := ChatUsageToResponsesUsage(chunk.Usage)
			require.Equal(t, 100, usage.InputTokens)
			require.Equal(t, 105, usage.TotalTokens)
			cached := 0
			if usage.InputTokensDetails != nil {
				cached = usage.InputTokensDetails.CachedTokens
			}
			require.Equal(t, tc.want, cached)
		})
	}
}
