package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepareOpenAIInputTokensCountRequest_converts_anthropic_to_responses_input(t *testing.T) {
	account := &Account{Type: AccountTypeAPIKey, Platform: PlatformOpenAI}
	body := []byte(`{
		"model":"gpt-5.6",
		"system":"system note",
		"messages":[{"role":"user","content":"hi"}],
		"tools":[{"name":"lookup","description":"desc","input_schema":{"type":"object"}}],
		"tool_choice":{"type":"tool","name":"lookup"}
	}`)

	got, err := prepareOpenAIInputTokensCountRequest(body, account, "")

	require.NoError(t, err)
	require.Equal(t, "gpt-5.6", got.OriginalModel)
	require.Equal(t, "gpt-5.6", got.NormalizedModel)
	require.Equal(t, "gpt-5.6", got.UpstreamModel)
	require.Equal(t, "gpt-5.6", got.Request.Model)
	require.NotEmpty(t, got.Request.Input)
	require.Len(t, got.Request.Tools, 1)
	require.NotEmpty(t, got.Request.ToolChoice)
}

func TestOpenAIOAuthInputTokensUnsupported_detects_scope_and_missing_endpoint(t *testing.T) {
	require.True(t, isOpenAIOAuthInputTokensUnsupported(
		http.StatusForbidden,
		[]byte(`{"error":{"code":"missing_scope","message":"missing scopes: api.responses.write"}}`),
	))
	require.True(t, isOpenAIOAuthInputTokensUnsupported(
		http.StatusNotFound,
		[]byte(`{"error":{"message":"input_tokens not found"}}`),
	))
	require.False(t, isOpenAIOAuthInputTokensUnsupported(
		http.StatusTooManyRequests,
		[]byte(`{"error":{"message":"rate limited"}}`),
	))
}

func TestEstimateOpenAIInputTokens_returns_minimum_positive_estimate(t *testing.T) {
	got := estimateOpenAIInputTokens(openAIInputTokensCountRequest{
		Model:        "gpt-5.6",
		Instructions: "system",
		Input:        []byte(`"hello world"`),
	})

	require.Greater(t, got, 0)
}
