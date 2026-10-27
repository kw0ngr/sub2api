package service

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeepSeekChatCacheUsageStreamingAndJSON(t *testing.T) {
	for _, tc := range []struct {
		name, fields string
		want         int
	}{
		{"deepseek", `"prompt_cache_hit_tokens":80`, 80},
		{"nested_precedence", `"prompt_cache_hit_tokens":80,"prompt_tokens_details":{"cached_tokens":64}`, 64},
		{"explicit_zero", `"prompt_cache_hit_tokens":80,"prompt_tokens_details":{"cached_tokens":0}`, 0},
		{"null_fallback", `"prompt_cache_hit_tokens":80,"prompt_tokens_details":{"cached_tokens":null}`, 80},
		{"negative", `"prompt_cache_hit_tokens":-80`, 0},
		{"no_cache", `"total_tokens":105`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			usageJSON := fmt.Sprintf(`{"prompt_tokens":100,"completion_tokens":5,%s}`, tc.fields)
			for _, body := range []string{`{"usage":` + usageJSON + `}`, `{"response":{"usage":` + usageJSON + `}}`} {
				usage, ok := extractOpenAIUsageFromJSONBytes([]byte(body))
				require.True(t, ok)
				require.Equal(t, 100, usage.InputTokens)
				require.Equal(t, tc.want, usage.CacheReadInputTokens)
			}
			var usage OpenAIUsage
			(&OpenAIGatewayService{}).parseSSEUsage(`{"usage":`+usageJSON+`}`, &usage)
			require.Equal(t, 100, usage.InputTokens)
			require.Equal(t, tc.want, usage.CacheReadInputTokens)
		})
	}
}
