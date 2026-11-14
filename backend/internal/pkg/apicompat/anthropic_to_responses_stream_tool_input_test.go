package apicompat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnthropicEventToResponses_ToolInputStreams(t *testing.T) {
	const args = `{"city":"上海"}`
	tests := []struct {
		name     string
		input    string
		partials []string
		deltas   []string
	}{
		{name: "inline", input: " \n" + args + "\t ", deltas: []string{args}},
		{name: "inline_with_empty_delta", input: args, partials: []string{""}, deltas: []string{args}},
		{name: "deltas_override_seed", input: `{"city":"seed"}`, partials: []string{"", `{"city":`, `"上海"}`, ""}, deltas: []string{`{"city":`, `"上海"}`}},
		{name: "canonical", input: `{}`, partials: []string{`{"city":`, `"上海"}`}, deltas: []string{`{"city":`, `"上海"}`}},
		{name: "delta_empty_object_overrides_seed", input: args, partials: []string{`{`, `}`}, deltas: []string{`{`, `}`}},
		{name: "absent", deltas: []string{`{}`}},
		{name: "empty_object", input: `{}`, deltas: []string{`{}`}},
		{name: "null", input: " null ", deltas: []string{`{}`}},
		{name: "whitespace", input: " \n\t ", deltas: []string{`{}`}},
	}
	for _, model := range []string{
		"claude-sonnet-4-5", "claude-fable-5-2", "claude-mythos-5-2",
		"claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5",
		"deepseek-reasoner", "glm-4.7",
	} {
		for _, tt := range tests {
			t.Run(model+"/"+tt.name, func(t *testing.T) {
				state := NewAnthropicEventToResponsesState()
				var events []ResponsesStreamEvent
				feed := func(evt *AnthropicStreamEvent) {
					events = append(events, AnthropicEventToResponsesEvents(evt, state)...)
				}
				feed(&AnthropicStreamEvent{
					Type: "message_start", Message: &AnthropicResponse{ID: "msg_inline", Model: model},
				})
				feed(&AnthropicStreamEvent{
					Type: "content_block_start",
					ContentBlock: &AnthropicContentBlock{
						Type: "tool_use", ID: "toolu_weather", Name: "lookup", Input: json.RawMessage(tt.input),
					},
				})
				require.Len(t, events, 2, "inline input must wait for possible authoritative deltas")
				for _, partial := range tt.partials {
					feed(&AnthropicStreamEvent{
						Type: "content_block_delta", Delta: &AnthropicDelta{Type: "input_json_delta", PartialJSON: partial},
					})
				}
				feed(&AnthropicStreamEvent{Type: "content_block_stop"})
				feed(&AnthropicStreamEvent{Type: "message_stop"})

				var types, deltas []string
				var done *ResponsesStreamEvent
				var item *ResponsesOutput
				var completed *ResponsesResponse
				for i, event := range events {
					require.Equal(t, i, event.SequenceNumber)
					raw, err := json.Marshal(event)
					require.NoError(t, err)
					var wire ResponsesStreamEvent
					require.NoError(t, json.Unmarshal(raw, &wire))
					types = append(types, wire.Type)
					switch wire.Type {
					case "response.function_call_arguments.delta":
						require.Equal(t, 0, wire.OutputIndex)
						require.Equal(t, toResponsesCallID("toolu_weather"), wire.CallID)
						require.Equal(t, "lookup", wire.Name)
						require.Equal(t, events[1].Item.ID, wire.ItemID)
						deltas = append(deltas, wire.Delta)
					case "response.function_call_arguments.done":
						done = &wire
					case "response.output_item.done":
						item = wire.Item
					case "response.completed":
						completed = wire.Response
					}
				}
				wantTypes := []string{"response.created", "response.output_item.added"}
				for range tt.deltas {
					wantTypes = append(wantTypes, "response.function_call_arguments.delta")
				}
				wantTypes = append(wantTypes, "response.function_call_arguments.done", "response.output_item.done", "response.completed")
				require.Equal(t, wantTypes, types)
				require.Equal(t, tt.deltas, deltas)
				want := strings.Join(tt.deltas, "")
				require.True(t, json.Valid([]byte(want)))
				require.NotNil(t, done)
				require.Equal(t, want, done.Arguments)
				require.Equal(t, 0, done.OutputIndex)
				require.Equal(t, toResponsesCallID("toolu_weather"), done.CallID)
				require.Equal(t, "lookup", done.Name)
				require.NotNil(t, item)
				require.Equal(t, item.ID, done.ItemID)
				require.Equal(t, want, item.Arguments)
				require.NotNil(t, completed)
				require.Len(t, completed.Output, 1)
				require.Equal(t, *item, completed.Output[0])
				require.Empty(t, FinalizeAnthropicResponsesStream(state))
			})
		}
	}
}

func TestAnthropicEventToResponses_ToolInputClosedWithoutBlockStop(t *testing.T) {
	for _, ending := range []string{"message_stop", "finalize", "next_tool"} {
		t.Run(ending, func(t *testing.T) {
			const args = `{"city":"上海"}`
			state := NewAnthropicEventToResponsesState()
			AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
				Type: "message_start", Message: &AnthropicResponse{ID: "msg_inline", Model: "glm-4.7"},
			}, state)
			AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
				Type:         "content_block_start",
				ContentBlock: &AnthropicContentBlock{Type: "tool_use", ID: "toolu_weather", Name: "lookup", Input: json.RawMessage(args)},
			}, state)
			var events []ResponsesStreamEvent
			switch ending {
			case "message_stop":
				events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{Type: "message_stop"}, state)
			case "finalize":
				events = FinalizeAnthropicResponsesStream(state)
			case "next_tool":
				events = AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
					Type:         "content_block_start",
					ContentBlock: &AnthropicContentBlock{Type: "tool_use", ID: "toolu_refresh", Name: "refresh"},
				}, state)
				events = append(events, AnthropicEventToResponsesEvents(&AnthropicStreamEvent{Type: "content_block_stop"}, state)...)
				events = append(events, AnthropicEventToResponsesEvents(&AnthropicStreamEvent{Type: "message_stop"}, state)...)
			}
			require.GreaterOrEqual(t, len(events), 4)
			require.Equal(t, "response.function_call_arguments.delta", events[0].Type)
			require.Equal(t, args, events[0].Delta)
			require.Equal(t, "response.function_call_arguments.done", events[1].Type)
			require.Equal(t, args, events[1].Arguments)
			require.Equal(t, "response.output_item.done", events[2].Type)
			require.Equal(t, args, events[2].Item.Arguments)
			completed := events[len(events)-1]
			require.Equal(t, "response.completed", completed.Type)
			require.Equal(t, *events[2].Item, completed.Response.Output[0])
			if ending == "next_tool" {
				require.Len(t, completed.Response.Output, 2)
				require.Equal(t, `{}`, completed.Response.Output[1].Arguments, "seed must not leak to the next call")
			}
			require.Empty(t, FinalizeAnthropicResponsesStream(state))
			require.Empty(t, AnthropicEventToResponsesEvents(&AnthropicStreamEvent{Type: "message_stop"}, state))
		})
	}
}
