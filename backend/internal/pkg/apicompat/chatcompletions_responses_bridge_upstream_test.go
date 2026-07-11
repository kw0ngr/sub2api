package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResponsesToChatCompletionsRequest_PreservesParallelToolContextAndReasoning(t *testing.T) {
	req := &ResponsesRequest{
		Model:        "gpt-5.4",
		Instructions: "system note",
		Input: json.RawMessage(`[
			{"role":"user","content":[{"type":"input_text","text":"run both tools"}]},
			{"type":"reasoning","summary":[{"type":"summary_text","text":"call both tools together"}]},
			{"type":"function_call","call_id":"call_a","name":"alpha","arguments":"{\"a\":1}"},
			{"type":"function_call","call_id":"call_b","name":"beta","arguments":""},
			{"type":"function_call_output","call_id":"call_a","output":"alpha ok"},
			{"type":"function_call_output","call_id":"call_b","output":"beta ok"}
		]`),
	}

	out, err := ResponsesToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, out.Messages, 5)
	require.Equal(t, "system", out.Messages[0].Role)
	require.Equal(t, "user", out.Messages[1].Role)

	assistant := out.Messages[2]
	require.Equal(t, "assistant", assistant.Role)
	require.Equal(t, "call both tools together", assistant.ReasoningContent)
	require.Len(t, assistant.ToolCalls, 2)
	require.Equal(t, "call_a", assistant.ToolCalls[0].ID)
	require.Equal(t, "alpha", assistant.ToolCalls[0].Function.Name)
	require.Equal(t, `{"a":1}`, assistant.ToolCalls[0].Function.Arguments)
	require.Equal(t, "call_b", assistant.ToolCalls[1].ID)
	require.Equal(t, "beta", assistant.ToolCalls[1].Function.Name)
	require.Equal(t, "{}", assistant.ToolCalls[1].Function.Arguments)

	require.Equal(t, "tool", out.Messages[3].Role)
	require.Equal(t, "call_a", out.Messages[3].ToolCallID)
	require.Equal(t, "tool", out.Messages[4].Role)
	require.Equal(t, "call_b", out.Messages[4].ToolCallID)
}

func TestResponsesToChatCompletionsRequest_DropsDanglingAndOrphanToolContext(t *testing.T) {
	req := &ResponsesRequest{
		Model: "gpt-5.4",
		Input: json.RawMessage(`[
			{"role":"user","content":"use one tool"},
			{"type":"function_call","call_id":"call_keep","name":"kept","arguments":"{}"},
			{"type":"local_shell_call","call_id":"ignored","name":"shell","arguments":"{}"},
			{"type":"function_call_output","call_id":"call_keep","output":"kept ok"},
			{"type":"function_call","call_id":"call_dangling","name":"dangling","arguments":"{}"},
			{"type":"function_call_output","call_id":"call_orphan","output":"orphan"}
		]`),
	}

	out, err := ResponsesToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, out.Messages, 3)
	require.Equal(t, "user", out.Messages[0].Role)
	require.Equal(t, "assistant", out.Messages[1].Role)
	require.Len(t, out.Messages[1].ToolCalls, 1)
	require.Equal(t, "call_keep", out.Messages[1].ToolCalls[0].ID)
	require.Equal(t, "tool", out.Messages[2].Role)
	require.Equal(t, "call_keep", out.Messages[2].ToolCallID)

	encoded, err := json.Marshal(out.Messages)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "call_dangling")
	require.NotContains(t, string(encoded), "call_orphan")
	require.NotContains(t, string(encoded), "local_shell_call")
}

func TestChatCompletionsChunkToResponsesEvents_StrictLifecycle(t *testing.T) {
	state := NewChatCompletionsToResponsesStreamState("gpt-5.4")
	content := "answer"
	reasoning := "plan"
	index := 0
	finishReason := "tool_calls"
	chunk := &ChatCompletionsChunk{
		ID:    "chatcmpl-test",
		Model: "gpt-5.4",
		Choices: []ChatChunkChoice{{
			Delta: ChatDelta{
				ReasoningContent: &reasoning,
				Content:          &content,
				ToolCalls: []ChatToolCall{{
					Index: &index,
					ID:    "call_1",
					Type:  "function",
					Function: ChatFunctionCall{
						Name:      "lookup",
						Arguments: `{"q":"x"}`,
					},
				}},
			},
			FinishReason: &finishReason,
		}},
		Usage: &ChatUsage{PromptTokens: 11, CompletionTokens: 7},
	}

	events := ChatCompletionsChunkToResponsesEvents(chunk, state)
	events = append(events, FinalizeChatCompletionsResponsesStream(state)...)
	var types []string
	for _, evt := range events {
		types = append(types, evt.Type)
	}

	require.Equal(t, []string{
		"response.created",
		"response.output_item.added",
		"response.reasoning_summary_part.added",
		"response.reasoning_summary_text.delta",
		"response.reasoning_summary_text.done",
		"response.reasoning_summary_part.done",
		"response.output_item.done",
		"response.output_item.added",
		"response.content_part.added",
		"response.output_text.delta",
		"response.output_item.added",
		"response.function_call_arguments.delta",
		"response.output_text.done",
		"response.content_part.done",
		"response.output_item.done",
		"response.function_call_arguments.done",
		"response.output_item.done",
		"response.completed",
	}, types)

	terminal := events[len(events)-1]
	require.NotNil(t, terminal.Response)
	require.Equal(t, "completed", terminal.Response.Status)
	require.NotNil(t, terminal.Response.Usage)
	require.Equal(t, 11, terminal.Response.Usage.InputTokens)
	require.Equal(t, 7, terminal.Response.Usage.OutputTokens)
	require.Len(t, terminal.Response.Output, 3)
	require.Equal(t, "reasoning", terminal.Response.Output[0].Type)
	require.Equal(t, "message", terminal.Response.Output[1].Type)
	require.Equal(t, "function_call", terminal.Response.Output[2].Type)
}

func TestChatCompletionsChunkToResponsesEvents_FirstToolCallArgumentsNotDoubled(t *testing.T) {
	state := NewChatCompletionsToResponsesStreamState("glm-5.2")
	index := 0
	finishReason := "tool_calls"
	chunk := &ChatCompletionsChunk{
		ID:    "chatcmpl-glm",
		Model: "glm-5.2",
		Choices: []ChatChunkChoice{{
			Delta: ChatDelta{
				ToolCalls: []ChatToolCall{{
					Index: &index,
					ID:    "call_a",
					Type:  "function",
					Function: ChatFunctionCall{
						Name:      "exec",
						Arguments: `{"cmd":"ls"}`,
					},
				}},
			},
			FinishReason: &finishReason,
		}},
	}

	events := ChatCompletionsChunkToResponsesEvents(chunk, state)
	events = append(events, FinalizeChatCompletionsResponsesStream(state)...)
	argsDelta := ""
	var sawArgsDone bool
	for _, evt := range events {
		if evt.Type == "response.function_call_arguments.delta" {
			argsDelta += evt.Delta
		}
		if evt.Type == "response.function_call_arguments.done" {
			sawArgsDone = true
			require.Equal(t, `{"cmd":"ls"}`, evt.Arguments)
		}
	}

	require.True(t, sawArgsDone)
	require.Equal(t, `{"cmd":"ls"}`, argsDelta)
}

func TestChatCompletionsChunkToResponsesEvents_ReasoningOnlyFallbackMessage(t *testing.T) {
	state := NewChatCompletionsToResponsesStreamState("deepseek-reasoner")
	reasoning := "visible fallback"
	finishReason := "length"
	chunk := &ChatCompletionsChunk{
		ID:    "chatcmpl-reasoning",
		Model: "deepseek-reasoner",
		Choices: []ChatChunkChoice{{
			Delta:        ChatDelta{ReasoningContent: &reasoning},
			FinishReason: &finishReason,
		}},
		Usage: &ChatUsage{PromptTokens: 4, CompletionTokens: 3},
	}

	events := ChatCompletionsChunkToResponsesEvents(chunk, state)
	events = append(events, FinalizeChatCompletionsResponsesStream(state)...)

	var textDeltaFound bool
	for _, evt := range events {
		if evt.Type == "response.output_text.delta" && evt.Delta == reasoning {
			textDeltaFound = true
			break
		}
	}
	require.True(t, textDeltaFound)
	terminal := events[len(events)-1]
	require.NotNil(t, terminal.Response)
	require.Equal(t, "incomplete", terminal.Response.Status)
	require.NotNil(t, terminal.Response.IncompleteDetails)
	require.Equal(t, "max_output_tokens", terminal.Response.IncompleteDetails.Reason)
	require.Len(t, terminal.Response.Output, 2)
	require.Equal(t, "reasoning", terminal.Response.Output[0].Type)
	require.Equal(t, "message", terminal.Response.Output[1].Type)
	require.Equal(t, reasoning, terminal.Response.Output[1].Content[0].Text)
}

func TestResponsesStreamEventWire_RequiredZeroValueFields(t *testing.T) {
	payload, err := json.Marshal(ResponsesStreamEvent{
		Type:           "response.output_text.delta",
		SequenceNumber: 0,
		OutputIndex:    0,
		ContentIndex:   0,
		ItemID:         "msg_1",
		Delta:          "",
	})
	require.NoError(t, err)
	require.JSONEq(t, `{
		"type":"response.output_text.delta",
		"sequence_number":0,
		"output_index":0,
		"content_index":0,
		"item_id":"msg_1",
		"delta":""
	}`, string(payload))

	payload, err = json.Marshal(ResponsesStreamEvent{
		Type:        "response.function_call_arguments.done",
		OutputIndex: 0,
		ItemID:      "call_item",
		CallID:      "call_1",
		Name:        "lookup",
		Arguments:   "",
	})
	require.NoError(t, err)
	require.JSONEq(t, `{
		"type":"response.function_call_arguments.done",
		"sequence_number":0,
		"output_index":0,
		"item_id":"call_item",
		"call_id":"call_1",
		"name":"lookup",
		"arguments":""
	}`, string(payload))
}

func TestResponsesUsageUnmarshalAcceptsChatUsageShape(t *testing.T) {
	var usage ResponsesUsage
	require.NoError(t, json.Unmarshal([]byte(`{
		"prompt_tokens":12,
		"completion_tokens":5,
		"prompt_tokens_details":{"cached_tokens":9},
		"completion_tokens_details":{"reasoning_tokens":3}
	}`), &usage))

	require.Equal(t, 12, usage.InputTokens)
	require.Equal(t, 5, usage.OutputTokens)
	require.Equal(t, 17, usage.TotalTokens)
	require.NotNil(t, usage.InputTokensDetails)
	require.Equal(t, 9, usage.InputTokensDetails.CachedTokens)
	require.NotNil(t, usage.OutputTokensDetails)
	require.Equal(t, 3, usage.OutputTokensDetails.ReasoningTokens)
}

func TestResponsesToChatCompletionsRequest_DropsToolChoiceForDroppedTool(t *testing.T) {
	req := &ResponsesRequest{
		Model: "gpt-5.4",
		Tools: []ResponsesTool{{
			Type: "custom",
			Name: "exec",
		}},
		ToolChoice: json.RawMessage(`{"type":"function","name":"exec"}`),
	}

	out, err := ResponsesToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Empty(t, out.Tools)
	require.Empty(t, out.ToolChoice)
}

func TestResponsesToChatCompletionsRequest_KeepsToolChoiceForDeclaredFunction(t *testing.T) {
	req := &ResponsesRequest{
		Model: "gpt-5.4",
		Tools: []ResponsesTool{{
			Type: "function",
			Name: "lookup",
		}},
		ToolChoice: json.RawMessage(`{"type":"function","name":"lookup"}`),
	}

	out, err := ResponsesToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, out.Tools, 1)
	require.NotEmpty(t, out.ToolChoice)
}
