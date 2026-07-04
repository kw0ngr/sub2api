//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestDisableDeepSeekAnthropicThinkingForForcedToolChoice_sets_disabled_when_named_tool_choice_present(t *testing.T) {
	// Given
	body := []byte(`{"model":"deepseek-v4-flash","tool_choice":{"type":"tool","name":"_set_title"},"messages":[{"role":"user","content":"hi"}]}`)

	// When
	got := disableDeepSeekAnthropicThinkingForForcedToolChoice(body)

	// Then
	require.Equal(t, "disabled", gjson.GetBytes(got, "thinking.type").String())
	require.Equal(t, "_set_title", gjson.GetBytes(got, "tool_choice.name").String())
}

func TestDisableDeepSeekAnthropicThinkingForForcedToolChoice_overwrites_enabled_thinking_when_forced(t *testing.T) {
	// Given
	body := []byte(`{"thinking":{"type":"enabled","budget_tokens":1024},"tool_choice":{"type":"any"},"messages":[{"role":"user","content":"hi"}]}`)

	// When
	got := disableDeepSeekAnthropicThinkingForForcedToolChoice(body)

	// Then
	require.Equal(t, "disabled", gjson.GetBytes(got, "thinking.type").String())
	require.False(t, gjson.GetBytes(got, "thinking.budget_tokens").Exists())
}

func TestDisableDeepSeekAnthropicThinkingForForcedToolChoice_preserves_auto_tool_choice(t *testing.T) {
	// Given
	body := []byte(`{"thinking":{"type":"enabled","budget_tokens":1024},"tool_choice":{"type":"auto"},"messages":[{"role":"user","content":"hi"}]}`)

	// When
	got := disableDeepSeekAnthropicThinkingForForcedToolChoice(body)

	// Then
	require.Equal(t, string(body), string(got))
}

func TestDisableDeepSeekAnthropicThinkingForForcedToolChoice_preserves_body_without_tool_choice(t *testing.T) {
	// Given
	body := []byte(`{"thinking":{"type":"enabled","budget_tokens":1024},"messages":[{"role":"user","content":"hi"}]}`)

	// When
	got := disableDeepSeekAnthropicThinkingForForcedToolChoice(body)

	// Then
	require.Equal(t, string(body), string(got))
}
