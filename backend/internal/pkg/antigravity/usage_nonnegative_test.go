package antigravity

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeminiUsageCacheOnlyNeverProducesNegativeInput(t *testing.T) {
	const body = `{"candidates":[{"content":{"parts":[{"text":"hi"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"cachedContentTokenCount":30}}`
	_, usage, err := TransformGeminiToClaude([]byte(body), "gemini-3.8-flash")
	require.NoError(t, err)
	require.Zero(t, usage.InputTokens)
	require.Equal(t, 30, usage.CacheReadInputTokens)
	p := NewStreamingProcessor("gemini-3.8-flash")
	out := p.ProcessLine(`data: {"response":` + body + `}`)
	require.NotContains(t, string(out), `"input_tokens":-`)
	_, usage = p.Finish()
	require.NotNil(t, usage)
	require.Zero(t, usage.InputTokens)
	require.Equal(t, 30, usage.CacheReadInputTokens)
}
