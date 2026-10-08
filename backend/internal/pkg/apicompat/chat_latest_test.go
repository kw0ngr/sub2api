package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnthropicChatLatestUsesFixedMediumAndNoSampling(t *testing.T) {
	temp := 0.2
	req, err := AnthropicToResponses(&AnthropicRequest{
		Model: "chat-latest", MaxTokens: 128,
		Messages:    []AnthropicMessage{{Role: "user", Content: json.RawMessage(`"hi"`)}},
		Temperature: &temp,
	})
	require.NoError(t, err)
	require.Equal(t, "medium", req.Reasoning.Effort)
	require.Nil(t, req.Temperature)
	require.Nil(t, req.Text)
}

func TestChatLatestRejectsUnsupportedReasoningBeforeConversion(t *testing.T) {
	for _, effort := range []string{"none", "low", "high", "xhigh", "max"} {
		_, err := ChatCompletionsToResponses(&ChatCompletionsRequest{
			Model: "chat-latest", ReasoningEffort: effort,
		})
		require.ErrorContains(t, err, "only supports")
	}
}
