package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChatChunkToSSE_ArgumentDeltaOmitsEmptyFunctionName(t *testing.T) {
	t.Parallel()

	index := 0
	wire, err := ChatChunkToSSE(ChatCompletionsChunk{
		Choices: []ChatChunkChoice{{
			Delta: ChatDelta{ToolCalls: []ChatToolCall{{
				Index:    &index,
				Function: ChatFunctionCall{Arguments: `{"command":"echo ok"}`},
			}}},
		}},
	})

	require.NoError(t, err)
	require.Contains(t, wire, `"arguments":"{\"command\":\"echo ok\"}"`)
	require.NotContains(t, wire, `"name":""`, "an argument-only delta must not erase the name from an earlier chunk")
}

func TestChatCompletionsToAnthropic_DropsNamelessToolHistory(t *testing.T) {
	t.Parallel()

	var req ChatCompletionsRequest
	require.NoError(t, json.Unmarshal([]byte(`{
		"model":"deepseek-v4-flash",
		"messages":[
			{"role":"user","content":"run it"},
			{"role":"assistant","tool_calls":[{"id":"call_bad","type":"function","function":{"name":"","arguments":"{\"command\":\"echo ok\"}"}}]},
			{"role":"tool","tool_call_id":"call_bad","content":"Tool not found"},
			{"role":"user","content":"continue"}
		]
	}`), &req))

	responsesReq, err := ChatCompletionsToResponses(&req)
	require.NoError(t, err)
	anthropicReq, err := ResponsesToAnthropicRequest(responsesReq)
	require.NoError(t, err)
	body, err := json.Marshal(anthropicReq)
	require.NoError(t, err)
	require.NotContains(t, string(body), `"type":"tool_use"`, "invalid nameless tool history must not reach strict Anthropic-compatible upstreams")
	require.NotContains(t, string(body), `"type":"tool_result"`)
}
