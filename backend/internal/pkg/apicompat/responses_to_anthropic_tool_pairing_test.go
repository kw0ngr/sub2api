package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResponsesToAnthropic_DropsOrphanToolResult(t *testing.T) {
	orphanID := "call_orphan"
	anthReq, err := ResponsesToAnthropicRequest(&ResponsesRequest{
		Model: "claude-sonnet-4-5",
		Input: json.RawMessage(`[
			{"role":"user","content":"search"},
			{"type":"function_call_output","call_id":"call_orphan","output":"stale"},
			{"role":"user","content":"next"}
		]`),
	})
	require.NoError(t, err)
	assertAnthropicToolPairing(t, anthReq.Messages)
	for _, msg := range anthReq.Messages {
		require.False(t, anthropicBlocksHaveToolResult(parseContentBlocks(msg.Content), orphanID))
	}
}

func TestResponsesToAnthropic_DropsUnansweredParallelToolUse(t *testing.T) {
	anthReq, err := ResponsesToAnthropicRequest(&ResponsesRequest{
		Model: "claude-sonnet-4-5",
		Input: json.RawMessage(`[
			{"role":"user","content":"run both"},
			{"type":"function_call","call_id":"call_a","name":"exec","arguments":"{\"cmd\":\"A\"}"},
			{"type":"function_call","call_id":"call_b","name":"exec","arguments":"{\"cmd\":\"B\"}"},
			{"type":"function_call_output","call_id":"call_a","output":"A ok"}
		]`),
	})
	require.NoError(t, err)
	assertAnthropicToolPairing(t, anthReq.Messages)
	var sawA bool
	for _, msg := range anthReq.Messages {
		blocks := parseContentBlocks(msg.Content)
		sawA = sawA || anthropicBlocksHaveToolUse(blocks, "call_a")
		require.False(t, anthropicBlocksHaveToolUse(blocks, "call_b"))
	}
	require.True(t, sawA)
}

func assertAnthropicToolPairing(t *testing.T, messages []AnthropicMessage) {
	t.Helper()
	for i, msg := range messages {
		if i > 0 {
			require.NotEqual(t, messages[i-1].Role, msg.Role)
		}
		blocks := parseContentBlocks(msg.Content)
		for _, block := range blocks {
			switch block.Type {
			case "tool_result":
				require.Positive(t, i)
				require.True(t, anthropicBlocksHaveToolUse(parseContentBlocks(messages[i-1].Content), block.ToolUseID))
			case "tool_use":
				require.Less(t, i+1, len(messages))
				require.True(t, anthropicBlocksHaveToolResult(parseContentBlocks(messages[i+1].Content), block.ID))
			}
		}
	}
}

func anthropicBlocksHaveToolUse(blocks []AnthropicContentBlock, id string) bool {
	for _, block := range blocks {
		if block.Type == "tool_use" && block.ID == id {
			return true
		}
	}
	return false
}

func anthropicBlocksHaveToolResult(blocks []AnthropicContentBlock, id string) bool {
	for _, block := range blocks {
		if block.Type == "tool_result" && block.ToolUseID == id {
			return true
		}
	}
	return false
}
