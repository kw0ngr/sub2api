package service

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilterCodexInput_MessageItemID(t *testing.T) {
	for _, id := range []string{"item_replayed", "fc_not_a_message", "msg_valid", "msgWithoutUnderscore", ""} {
		for _, preserveReferences := range []bool{false, true} {
			for _, preserveCallIDs := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/references=%v/callIDs=%v", id, preserveReferences, preserveCallIDs), func(t *testing.T) {
					original := map[string]any{
						"type": "message", "id": id, "role": "assistant",
						"content": []any{map[string]any{"type": "output_text", "text": "hello"}},
					}
					filtered := filterCodexInputWithOptions([]any{original}, codexInputFilterOptions{
						PreserveReferences: preserveReferences,
						PreserveCallIDs:    preserveCallIDs,
					})
					require.Len(t, filtered, 1)
					got := filtered[0].(map[string]any)
					if preserveReferences && (id == "" || id == "msg_valid" || id == "msgWithoutUnderscore") {
						require.Equal(t, id, got["id"])
					} else {
						require.NotContains(t, got, "id")
					}
					require.Equal(t, original["type"], got["type"])
					require.Equal(t, original["role"], got["role"])
					require.Equal(t, original["content"], got["content"])
					require.Equal(t, id, original["id"], "must not mutate the replayed input")
				})
			}
		}
	}
}

func TestFilterCodexInput_MessageIDCleanupPreservesToolLoop(t *testing.T) {
	input := []any{
		map[string]any{"type": "message", "id": "item_replayed", "role": "assistant"},
		map[string]any{"type": "function_call", "id": "item_call", "call_id": "fc_tool", "name": "bash"},
		map[string]any{"type": "function_call_output", "id": "output_id", "call_id": "fc_tool", "output": "done"},
		map[string]any{"type": "item_reference", "id": "msg_previous"},
		map[string]any{"type": "web_search_call", "id": "ws_search"},
	}
	filtered := filterCodexInputWithOptions(input, codexInputFilterOptions{PreserveReferences: true})

	require.Len(t, filtered, len(input))
	require.NotContains(t, filtered[0], "id")
	require.NotContains(t, filtered[1], "id")
	require.Equal(t, "fc_tool", filtered[1].(map[string]any)["call_id"])
	require.Equal(t, input[2:], filtered[2:])
	require.Equal(t, "item_replayed", input[0].(map[string]any)["id"])
	require.Equal(t, "item_call", input[1].(map[string]any)["id"])
}

func TestApplyCodexOAuthTransform_ToolContinuationStripsReplayedMessageID(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.2",
		"input": []any{
			map[string]any{"type": "message", "id": "item_replayed", "role": "user", "content": "continue"},
			map[string]any{"type": "function_call", "id": "fc_item", "call_id": "fc_tool", "name": "bash", "arguments": "{}"},
			map[string]any{"type": "function_call_output", "call_id": "fc_tool", "output": "done"},
		},
	}

	applyCodexOAuthTransform(reqBody, false, false)

	input := reqBody["input"].([]any)
	require.Len(t, input, 3)
	require.NotContains(t, input[0], "id")
	require.Equal(t, "fc_item", input[1].(map[string]any)["id"])
	require.Equal(t, "fc_tool", input[1].(map[string]any)["call_id"])
	require.Equal(t, "fc_tool", input[2].(map[string]any)["call_id"])
}
