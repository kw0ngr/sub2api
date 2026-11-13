package service

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestConvertGeminiToClaudeMessageOmitsThoughtParts(t *testing.T) {
	geminiResp := map[string]any{
		"candidates": []any{map[string]any{
			"content": map[string]any{"parts": []any{
				map[string]any{"text": "planning the call", "thought": true},
				map[string]any{"text": "checking", "thought": false},
				map[string]any{"functionCall": map[string]any{"name": "get_weather", "args": map[string]any{"city": "Paris"}}, "thoughtSignature": "sig_1"},
			}},
			"finishReason": "STOP",
		}},
	}
	rawData, err := json.Marshal(geminiResp)
	require.NoError(t, err)

	got, _ := convertGeminiToClaudeMessage(geminiResp, "gemini-test", rawData)
	content, ok := got["content"].([]any)
	require.True(t, ok)
	require.Len(t, content, 2)
	require.Equal(t, map[string]any{"type": "text", "text": "checking"}, content[0])
	toolUse, ok := content[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "tool_use", toolUse["type"])
	require.Equal(t, "get_weather", toolUse["name"])
	require.Equal(t, "tool_use", got["stop_reason"])
}
