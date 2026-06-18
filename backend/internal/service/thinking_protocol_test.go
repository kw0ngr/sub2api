//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
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

func TestNormalizeChineseLLMThinkingForMiniMax(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		body      string
		wantType  string
		wantApply bool
	}{
		{
			name:      "minimax enabled becomes adaptive",
			model:     "MiniMax-M3",
			body:      `{"thinking":{"type":"enabled"},"messages":[]}`,
			wantType:  "adaptive",
			wantApply: true,
		},
		{
			name:      "provider prefix minimax enabled becomes adaptive",
			model:     "openrouter/MiniMax-M2.7",
			body:      `{"thinking":{"type":"enabled"},"messages":[]}`,
			wantType:  "adaptive",
			wantApply: true,
		},
		{
			name:      "minimax adaptive stays adaptive",
			model:     "MiniMax-M3",
			body:      `{"thinking":{"type":"adaptive"},"messages":[]}`,
			wantType:  "adaptive",
			wantApply: false,
		},
		{
			name:      "deepseek enabled is unchanged",
			model:     "deepseek-v4-pro",
			body:      `{"thinking":{"type":"enabled"},"messages":[]}`,
			wantType:  "enabled",
			wantApply: false,
		},
		{
			name:      "claude enabled is unchanged",
			model:     "claude-sonnet-4-6",
			body:      `{"thinking":{"type":"enabled"},"messages":[]}`,
			wantType:  "enabled",
			wantApply: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, applied := NormalizeChineseLLMThinkingForModel([]byte(tt.body), tt.model)
			require.Equal(t, tt.wantApply, applied)
			require.Equal(t, tt.wantType, gjson.GetBytes(got, "thinking.type").String())
		})
	}
}

func TestNormalizeChineseLLMThinkingInvalidJSON(t *testing.T) {
	body := []byte(`{"thinking":`)
	got, applied := NormalizeChineseLLMThinkingForModel(body, "MiniMax-M3")
	require.False(t, applied)
	require.Equal(t, body, got)
}
