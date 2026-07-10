package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChatCompletionsToResponses_preserves_json_schema_response_format(t *testing.T) {
	req := &ChatCompletionsRequest{
		Model: "gpt-5.6",
		Messages: []ChatMessage{{
			Role:    "user",
			Content: json.RawMessage(`"hi"`),
		}},
		ResponseFormat: json.RawMessage(`{"type":"json_schema","json_schema":{"name":"answer","schema":{"type":"object"},"strict":true}}`),
	}

	got, err := ChatCompletionsToResponses(req)

	require.NoError(t, err)
	require.NotNil(t, got.Text)
	require.JSONEq(t, `{"type":"json_schema","name":"answer","schema":{"type":"object"},"strict":true}`, string(got.Text.Format))
}

func TestResponsesToChatCompletionsRequest_preserves_json_schema_text_format(t *testing.T) {
	req := &ResponsesRequest{
		Model: "gpt-5.6",
		Input: json.RawMessage(`[{"role":"user","content":[{"type":"input_text","text":"hi"}]}]`),
		Text: &ResponsesText{
			Format: json.RawMessage(`{"type":"json_schema","name":"answer","schema":{"type":"object"},"strict":true}`),
		},
	}

	got, err := ResponsesToChatCompletionsRequest(req)

	require.NoError(t, err)
	require.JSONEq(t, `{"type":"json_schema","json_schema":{"name":"answer","schema":{"type":"object"},"strict":true}}`, string(got.ResponseFormat))
}

func TestResponsesToAnthropicRequest_maps_instructions_and_developer_role_to_system(t *testing.T) {
	req := &ResponsesRequest{
		Model:        "claude-sonnet-4-6",
		Instructions: "top instruction",
		Input: json.RawMessage(`[
			{"role":"developer","content":[{"type":"input_text","text":"developer note"}]},
			{"role":"system","content":"system note"},
			{"role":"user","content":[{"type":"input_text","text":"hi"}]}
		]`),
	}

	got, err := ResponsesToAnthropicRequest(req)

	require.NoError(t, err)
	var system string
	require.NoError(t, json.Unmarshal(got.System, &system))
	require.Equal(t, "top instruction\n\ndeveloper note\n\nsystem note", system)
	require.Len(t, got.Messages, 1)
	require.Equal(t, "user", got.Messages[0].Role)
}
