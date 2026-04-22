//go:build unit

package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepareDebugTraceBodyPreview_TruncatesChatFields(t *testing.T) {
	longText := strings.Repeat("x", debugTracePreviewChatStringBytes+128)
	raw := []byte(`{
		"model":"gpt-5.4",
		"stream":true,
		"messages":[
			{"role":"user","content":[{"type":"input_text","text":"` + longText + `"}]}
		],
		"metadata":{"request_source":"test"}
	}`)

	previewJSON, truncated, bodyBytes, truncatedPaths, strategy := PrepareDebugTraceBodyPreview(raw)
	require.NotNil(t, previewJSON)
	require.True(t, truncated)
	require.NotNil(t, bodyBytes)
	require.Equal(t, len(raw), *bodyBytes)
	require.Equal(t, "structured_preview_v1", strategy)
	require.Contains(t, *previewJSON, `"model":"gpt-5.4"`)
	require.Contains(t, *previewJSON, `"request_source":"test"`)
	require.Contains(t, *previewJSON, "truncated 128 bytes")
	require.Contains(t, truncatedPaths, "messages[0].content[0].text")
}

func TestPrepareDebugTraceBodyPreview_InvalidJSONFallsBackToRawPreview(t *testing.T) {
	raw := []byte(`{"model":"gpt-5.4"`)

	previewJSON, truncated, bodyBytes, truncatedPaths, strategy := PrepareDebugTraceBodyPreview(raw)
	require.NotNil(t, previewJSON)
	require.NotNil(t, bodyBytes)
	require.Equal(t, len(raw), *bodyBytes)
	require.Equal(t, "raw_preview_v1", strategy)
	require.Empty(t, truncatedPaths)
	require.Contains(t, *previewJSON, `_trace_invalid_json`)
	require.False(t, truncated)
}
