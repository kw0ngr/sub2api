//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestDisableGLMAnthropicThinkingByDefault_sets_disabled_when_missing(t *testing.T) {
	// Given
	body := []byte(`{"model":"glm-5.2","max_tokens":32,"messages":[{"role":"user","content":"hi"}],"stream":true}`)

	// When
	got := DisableGLMAnthropicThinkingByDefault(body)

	// Then
	require.Equal(t, "disabled", gjson.GetBytes(got, "thinking.type").String())
}

func TestDisableGLMAnthropicThinkingByDefault_preserves_explicit_thinking(t *testing.T) {
	// Given
	body := []byte(`{"model":"glm-5.2","thinking":{"type":"enabled","budget_tokens":1024},"messages":[]}`)

	// When
	got := DisableGLMAnthropicThinkingByDefault(body)

	// Then
	require.Equal(t, "enabled", gjson.GetBytes(got, "thinking.type").String())
	require.Equal(t, int64(1024), gjson.GetBytes(got, "thinking.budget_tokens").Int())
}
