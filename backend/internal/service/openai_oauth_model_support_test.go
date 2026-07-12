package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsModelSupported_OpenAIOAuthEmptyMappingRejectsForeignModels(t *testing.T) {
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	for _, model := range []string{
		"deepseek-v4",
		"glm-4.7",
		"kimi-k2",
		"gemini-3.0-pro",
		"grok-4",
		"qwen3-max",
		"claude-unknown-family",
	} {
		require.False(t, account.IsModelSupported(model), "model %q should not be served by empty-mapping OpenAI OAuth", model)
	}
}

func TestIsModelSupported_OpenAIOAuthEmptyMappingAllowsCodexAndClaudeDispatch(t *testing.T) {
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	for _, model := range []string{
		"",
		"gpt-5.6-sol-max",
		"gpt-5.4-high",
		"gpt-5.3-codex-xhigh",
		"gpt5.3codexspark",
		"gpt-image-1",
		"claude-sonnet-4-6",
		"claude-3-opus-20240229",
	} {
		require.True(t, account.IsModelSupported(model), "model %q should be served by empty-mapping OpenAI OAuth", model)
	}
}

func TestIsModelSupported_OpenAIOAuthExplicitMappingAndPassthroughUnchanged(t *testing.T) {
	mapped := &Account{
		ID:       1,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"deepseek-v4": "gpt-5.6-sol"},
		},
	}
	require.True(t, mapped.IsModelSupported("deepseek-v4"))
	require.False(t, mapped.IsModelSupported("glm-4.7"))

	passthrough := &Account{
		ID:       2,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra:    map[string]any{"openai_passthrough": true},
	}
	require.True(t, passthrough.IsModelSupported("deepseek-v4"))
}
