package service

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestTrimAnthropicCompatResponsesInputToLatestTurnKeepsMultiToolOutputs(t *testing.T) {
	req := &apicompat.ResponsesRequest{
		Input: json.RawMessage(`[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"inspect"}]},
			{"type":"function_call","call_id":"call_one","name":"Read","arguments":"{}"},
			{"type":"function_call","call_id":"call_two","name":"Read","arguments":"{}"},
			{"type":"function_call_output","call_id":"call_one","output":"package a"},
			{"type":"function_call_output","call_id":"call_two","output":"package b"}
		]`),
	}

	trimAnthropicCompatResponsesInputToLatestTurn(req)

	require.Equal(t, 4, int(gjson.GetBytes(req.Input, "#").Int()))
	require.Equal(t, "function_call", gjson.GetBytes(req.Input, "0.type").String())
	require.Equal(t, "call_one", gjson.GetBytes(req.Input, "0.call_id").String())
	require.Equal(t, "function_call", gjson.GetBytes(req.Input, "1.type").String())
	require.Equal(t, "call_two", gjson.GetBytes(req.Input, "1.call_id").String())
	require.Equal(t, "function_call_output", gjson.GetBytes(req.Input, "2.type").String())
	require.Equal(t, "call_one", gjson.GetBytes(req.Input, "2.call_id").String())
	require.Equal(t, "function_call_output", gjson.GetBytes(req.Input, "3.type").String())
	require.Equal(t, "call_two", gjson.GetBytes(req.Input, "3.call_id").String())
}

func TestTrimAnthropicCompatResponsesInputToLatestTurnKeepsToolOutputsBeforeUserMessage(t *testing.T) {
	req := &apicompat.ResponsesRequest{
		Input: json.RawMessage(`[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"inspect"}]},
			{"type":"function_call","call_id":"call_one","name":"Read","arguments":"{}"},
			{"type":"function_call","call_id":"call_two","name":"Read","arguments":"{}"},
			{"type":"function_call_output","call_id":"call_one","output":"package a"},
			{"type":"function_call_output","call_id":"call_two","output":"package b"},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}
		]`),
	}

	trimAnthropicCompatResponsesInputToLatestTurn(req)

	require.Equal(t, 5, int(gjson.GetBytes(req.Input, "#").Int()))
	require.Equal(t, "function_call", gjson.GetBytes(req.Input, "0.type").String())
	require.Equal(t, "call_one", gjson.GetBytes(req.Input, "0.call_id").String())
	require.Equal(t, "function_call", gjson.GetBytes(req.Input, "1.type").String())
	require.Equal(t, "call_two", gjson.GetBytes(req.Input, "1.call_id").String())
	require.Equal(t, "function_call_output", gjson.GetBytes(req.Input, "2.type").String())
	require.Equal(t, "function_call_output", gjson.GetBytes(req.Input, "3.type").String())
	require.Equal(t, "message", gjson.GetBytes(req.Input, "4.type").String())
	require.Equal(t, "continue", gjson.GetBytes(req.Input, "4.content.0.text").String())
}
