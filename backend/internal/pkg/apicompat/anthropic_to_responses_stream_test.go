package apicompat

import "testing"

func TestAnthropicEventToResponses_TextEmitsContentPart(t *testing.T) {
	state := NewAnthropicEventToResponsesState()
	state.Model = "claude-sonnet-4-5"

	var types []string
	feed := func(evt *AnthropicStreamEvent) {
		for _, out := range AnthropicEventToResponsesEvents(evt, state) {
			types = append(types, out.Type)
		}
	}

	idx := 0
	feed(&AnthropicStreamEvent{Type: "message_start", Message: &AnthropicResponse{ID: "msg_1", Model: "claude-sonnet-4-5"}})
	feed(&AnthropicStreamEvent{Type: "content_block_start", Index: &idx, ContentBlock: &AnthropicContentBlock{Type: "text"}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &AnthropicDelta{Type: "text_delta", Text: "Hel"}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &AnthropicDelta{Type: "text_delta", Text: "lo"}})
	feed(&AnthropicStreamEvent{Type: "content_block_stop", Index: &idx})
	feed(&AnthropicStreamEvent{Type: "message_stop"})

	posOf := func(target string) int {
		for i, eventType := range types {
			if eventType == target {
				return i
			}
		}
		return -1
	}

	partAdded := posOf("response.content_part.added")
	firstDelta := posOf("response.output_text.delta")
	if partAdded < 0 {
		t.Fatalf("response.content_part.added was not emitted; got %v", types)
	}
	if firstDelta < 0 {
		t.Fatalf("response.output_text.delta was not emitted; got %v", types)
	}
	if partAdded > firstDelta {
		t.Errorf("content_part.added must precede the first output_text.delta; got %v", types)
	}
	if posOf("response.content_part.done") < 0 {
		t.Errorf("response.content_part.done was not emitted; got %v", types)
	}
}

func TestAnthropicEventToResponses_DoneEventsCarryFullText(t *testing.T) {
	state := NewAnthropicEventToResponsesState()
	state.Model = "claude-sonnet-4-5"

	var events []ResponsesStreamEvent
	feed := func(evt *AnthropicStreamEvent) {
		events = append(events, AnthropicEventToResponsesEvents(evt, state)...)
	}

	idx := 0
	feed(&AnthropicStreamEvent{Type: "message_start", Message: &AnthropicResponse{ID: "msg_1"}})
	feed(&AnthropicStreamEvent{Type: "content_block_start", Index: &idx, ContentBlock: &AnthropicContentBlock{Type: "text"}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &AnthropicDelta{Type: "text_delta", Text: "Hello "}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &AnthropicDelta{Type: "text_delta", Text: "world"}})
	feed(&AnthropicStreamEvent{Type: "content_block_stop", Index: &idx})

	const want = "Hello world"
	var sawTextDone, sawPartDone bool
	for _, event := range events {
		switch event.Type {
		case "response.output_text.done":
			sawTextDone = true
			if event.Text != want {
				t.Errorf("output_text.done text = %q, want %q", event.Text, want)
			}
		case "response.content_part.done":
			sawPartDone = true
			if event.Part == nil || event.Part.Text != want {
				t.Errorf("content_part.done part = %+v, want text %q", event.Part, want)
			}
		}
	}
	if !sawTextDone || !sawPartDone {
		t.Errorf("missing done events: output_text.done=%v content_part.done=%v", sawTextDone, sawPartDone)
	}
}

func TestAnthropicEventToResponses_CompletedCarriesOutput(t *testing.T) {
	state := NewAnthropicEventToResponsesState()
	state.Model = "claude-sonnet-4-5"

	var events []ResponsesStreamEvent
	feed := func(evt *AnthropicStreamEvent) {
		events = append(events, AnthropicEventToResponsesEvents(evt, state)...)
	}

	idx := 0
	feed(&AnthropicStreamEvent{Type: "message_start", Message: &AnthropicResponse{ID: "msg_1"}})
	feed(&AnthropicStreamEvent{Type: "content_block_start", Index: &idx, ContentBlock: &AnthropicContentBlock{Type: "text"}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &AnthropicDelta{Type: "text_delta", Text: "4826"}})
	feed(&AnthropicStreamEvent{Type: "content_block_stop", Index: &idx})
	feed(&AnthropicStreamEvent{Type: "message_stop"})

	var completed *ResponsesStreamEvent
	for i := range events {
		if events[i].Type == "response.completed" {
			completed = &events[i]
		}
	}
	if completed == nil || completed.Response == nil {
		t.Fatal("response.completed was not emitted")
	}
	if len(completed.Response.Output) == 0 {
		t.Fatal("response.completed carries an empty output; clients would see no result")
	}
	message := completed.Response.Output[0]
	if message.Type != "message" || len(message.Content) == 0 {
		t.Fatalf("output[0] = %+v, want a message with content", message)
	}
	if message.Content[0].Text != "4826" {
		t.Errorf("output[0].content[0].text = %q, want %q", message.Content[0].Text, "4826")
	}
}

func TestAnthropicEventToResponses_ToolCallCompletedCarriesArguments(t *testing.T) {
	state := NewAnthropicEventToResponsesState()
	state.Model = "claude-sonnet-4-5"

	var events []ResponsesStreamEvent
	feed := func(evt *AnthropicStreamEvent) {
		events = append(events, AnthropicEventToResponsesEvents(evt, state)...)
	}

	idx := 0
	feed(&AnthropicStreamEvent{Type: "message_start", Message: &AnthropicResponse{ID: "msg_1"}})
	feed(&AnthropicStreamEvent{Type: "content_block_start", Index: &idx, ContentBlock: &AnthropicContentBlock{
		Type: "tool_use", ID: "toolu_1", Name: "get_weather",
	}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &AnthropicDelta{
		Type: "input_json_delta", PartialJSON: `{"city":`,
	}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &AnthropicDelta{
		Type: "input_json_delta", PartialJSON: `"SH"}`,
	}})
	feed(&AnthropicStreamEvent{Type: "content_block_stop", Index: &idx})
	feed(&AnthropicStreamEvent{Type: "message_stop"})

	var completed *ResponsesStreamEvent
	for i := range events {
		if events[i].Type == "response.completed" {
			completed = &events[i]
		}
	}
	if completed == nil || completed.Response == nil || len(completed.Response.Output) == 0 {
		t.Fatal("response.completed carries no output")
	}
	functionCall := completed.Response.Output[0]
	if functionCall.Type != "function_call" {
		t.Fatalf("output[0].type = %q, want function_call", functionCall.Type)
	}
	if functionCall.Arguments != `{"city":"SH"}` {
		t.Errorf("arguments = %q, want %q", functionCall.Arguments, `{"city":"SH"}`)
	}
	if functionCall.Name != "get_weather" {
		t.Errorf("name = %q, want get_weather", functionCall.Name)
	}
}

func TestAnthropicEventToResponses_MultipleTextPartsUseDistinctIndexes(t *testing.T) {
	state := NewAnthropicEventToResponsesState()
	state.Model = "claude-sonnet-4-5"

	var addedIndexes []int
	feed := func(evt *AnthropicStreamEvent) {
		for _, event := range AnthropicEventToResponsesEvents(evt, state) {
			if event.Type == "response.content_part.added" {
				addedIndexes = append(addedIndexes, event.ContentIndex)
			}
		}
	}

	first, second := 0, 1
	feed(&AnthropicStreamEvent{Type: "message_start", Message: &AnthropicResponse{ID: "msg_1"}})
	feed(&AnthropicStreamEvent{Type: "content_block_start", Index: &first, ContentBlock: &AnthropicContentBlock{Type: "text"}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &first, Delta: &AnthropicDelta{Type: "text_delta", Text: "first"}})
	feed(&AnthropicStreamEvent{Type: "content_block_stop", Index: &first})
	feed(&AnthropicStreamEvent{Type: "content_block_start", Index: &second, ContentBlock: &AnthropicContentBlock{Type: "text"}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &second, Delta: &AnthropicDelta{Type: "text_delta", Text: "second"}})
	feed(&AnthropicStreamEvent{Type: "content_block_stop", Index: &second})

	if len(addedIndexes) != 2 || addedIndexes[0] != 0 || addedIndexes[1] != 1 {
		t.Fatalf("content_part.added indexes = %v, want [0 1]", addedIndexes)
	}
}
