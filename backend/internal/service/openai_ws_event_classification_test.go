package service

import "testing"

func TestIsOpenAIWSTokenEvent_TerminalEventsExcluded(t *testing.T) {
	for _, eventType := range []string{
		"response.completed",
		"response.done",
		"response.failed",
		"response.incomplete",
		"response.cancelled",
		"response.canceled",
	} {
		t.Run(eventType, func(t *testing.T) {
			if isOpenAIWSTokenEvent(eventType) {
				t.Fatalf("terminal event %q must not be classified as token event", eventType)
			}
			if !isOpenAIWSTerminalEvent(eventType) {
				t.Fatalf("expected terminal event %q", eventType)
			}
			if !openAIWSEventShouldParseUsage(eventType) {
				t.Fatalf("terminal event %q should be eligible for usage parsing", eventType)
			}
		})
	}
}

func TestIsOpenAIWSTokenEvent_DeltaStillTokenEvent(t *testing.T) {
	for _, eventType := range []string{
		"response.output_text.delta",
		"response.function_call_arguments.delta",
		"response.output_item.delta",
	} {
		t.Run(eventType, func(t *testing.T) {
			if !isOpenAIWSTokenEvent(eventType) {
				t.Fatalf("delta event %q should be classified as token event", eventType)
			}
		})
	}
}
