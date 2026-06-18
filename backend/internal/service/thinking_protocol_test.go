//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveThinkingProtocol(t *testing.T) {
	tests := []struct {
		model string
		want  ThinkingProtocol
	}{
		{"claude-sonnet-4-6", ThinkingProtocolAnthropicStrict},
		{"anthropic/claude-sonnet-4-6", ThinkingProtocolAnthropicStrict},
		{"opus-4-8", ThinkingProtocolAnthropicStrict},
		{"deepseek-v4-pro", ThinkingProtocolPassbackRequired},
		{"openrouter/deepseek-v4-pro", ThinkingProtocolPassbackRequired},
		{"kimi-for-coding", ThinkingProtocolPassbackRequired},
		{"MiniMax-M2.1", ThinkingProtocolPassbackRequired},
		{"qwen3-235b-a22b-thinking", ThinkingProtocolPassbackRequired},
		{"qwen3-235b-a22b", ThinkingProtocolUnknown},
		{"", ThinkingProtocolUnknown},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, ResolveThinkingProtocol(tt.model), tt.model)
	}
}

func TestThinkingFiltersRespectProtocol(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-6","thinking":{"type":"enabled"},"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"hidden"},{"type":"text","text":"ok"}]}]}`)

	for _, model := range []string{"deepseek-v4-pro", "kimi-for-coding", "unknown-model"} {
		require.Equal(t, body, FilterThinkingBlocksForModel(body, model), model)
		require.Equal(t, body, FilterThinkingBlocksForRetryModel(body, model), model)
		require.Equal(t, body, FilterSignatureSensitiveBlocksForRetryModel(body, model), model)
	}

	require.NotEqual(t, body, FilterThinkingBlocksForModel(body, "claude-sonnet-4-6"))
	require.NotEqual(t, body, FilterThinkingBlocksForRetryModel(body, "claude-sonnet-4-6"))
	require.NotEqual(t, body, FilterSignatureSensitiveBlocksForRetryModel(body, "claude-sonnet-4-6"))
}
