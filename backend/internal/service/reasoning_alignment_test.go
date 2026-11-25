package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestResponsesBridgeSendsPerTurnEffortToGemini(t *testing.T) {
	const request = `{"model":"gemini-4-neon","input":[{"type":"configuration_update","reasoning":{"effort":"max"}},{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}],"reasoning":{"effort":"low"}}`
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(request))
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{"id":"chatcmpl_test","model":"gemini-4-neon","choices":[{"index":0,"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}}
	result, err := svc.Forward(context.Background(), c, account, []byte(request))
	require.NoError(t, err)
	require.Equal(t, "xhigh", gjson.GetBytes(upstream.lastBody, "reasoning_effort").Str)
	require.Equal(t, "xhigh", *result.ReasoningEffort)
}

func TestResponsesPerTurnReasoningEffort(t *testing.T) {
	const body = `{"model":"gpt-5.6-sol","reasoning":{"effort":"low"},"input":[` +
		`{"type":"configuration_update","reasoning":{"effort":"high"}},` +
		`{"type":"configuration\u005fupdate","reasoning":{"effort":"max"}},` +
		`{"type":"configuration_update","reasoning":{"effort":" "}},` +
		`{"type":[],"reasoning":{"effort":"low"}},` +
		`{"type":"message","reasoning":{"effort":"medium"}}]}`
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(body), &parsed))
	for model, want := range map[string]string{"gpt-5.6-sol": "max", "gpt-5.5": "xhigh", "gpt-6.2-astra": "max"} {
		require.Equal(t, want, *extractOpenAIReasoningEffortFromBody([]byte(body), model))
		require.Equal(t, want, *extractOpenAIReasoningEffort(parsed, model))
	}
}

func TestGLM54AnthropicEffort(t *testing.T) {
	for in, want := range map[string]string{"low": "low", "medium": "high", "max": "max", "xhigh": "max"} {
		body := `{"model":"glm-5.4","max_tokens":32,"output_config":{"effort":"` + in + `"},"messages":[]}`
		upstream, result, err := forwardGLM53NativeAnthropicForTest(t, body, false)
		require.NoError(t, err)
		require.Equal(t, want, gjson.GetBytes(upstream, "output_config.effort").Str)
		require.Equal(t, "enabled", gjson.GetBytes(upstream, "thinking.type").Str)
		require.Equal(t, want, *result.ReasoningEffort)
	}
}

func TestGeminiFinalToolResultKeepsTrailingUserTextInSeparateTurn(t *testing.T) {
	body := []byte(`{"messages":[{"role":"assistant","content":[{"type":"tool_use","id":"tool1","name":"lookup","input":{},"signature":"keep"}]},` +
		`{"role":"user","content":[{"type":"tool_result","tool_use_id":"tool1","content":"first"},` +
		`{"type":"tool_result","tool_use_id":"tool1","content":"second"},{"type":"text","text":"continue"}]}]}`)
	out, err := convertClaudeMessagesToGeminiGenerateContent(body, "gemini-4-neon")
	require.NoError(t, err)
	require.Len(t, gjson.GetBytes(out, "contents").Array(), 3)
	require.Equal(t, "keep", gjson.GetBytes(out, "contents.0.parts.0.thoughtSignature").Str)
	require.Equal(t, "second", gjson.GetBytes(out, "contents.1.parts.1.functionResponse.response.content").Str)
	require.Equal(t, "user", gjson.GetBytes(out, "contents.2.role").Str)
	require.Equal(t, "continue", gjson.GetBytes(out, "contents.2.parts.0.text").Str)
}

func TestDeepSeekAnthropicToolHistoryPreservesThinkingAndForcedChoice(t *testing.T) {
	body := []byte(`{"thinking":{"type":"enabled"},"messages":[` +
		`{"role":"assistant","content":[{"type":"tool_use","id":"missing"}]},` +
		`{"role":"assistant","content":[{"type":"thinking","thinking":"keep","signature":"sig"},{"type":"tool_use","id":"present"}]},` +
		`{"role":"assistant","content":[{"type":"text","text":"leave alone"}]}]}`)
	out := repairDeepSeekAnthropicToolHistory(body)
	require.Equal(t, " ", gjson.GetBytes(out, "messages.0.content.0.thinking").Str)
	require.Equal(t, "missing", gjson.GetBytes(out, "messages.0.content.1.id").Str)
	require.Equal(t, "sig", gjson.GetBytes(out, "messages.1.content.0.signature").Str)
	require.Len(t, gjson.GetBytes(out, "messages.2.content").Array(), 1)
	require.Equal(t, out, repairDeepSeekAnthropicToolHistory(out))

	forced := append([]byte(`{"tool_choice":{"type":"any"},`), body[1:]...)
	disabled := disableDeepSeekAnthropicThinkingForForcedToolChoice(forced)
	require.Equal(t, disabled, repairDeepSeekAnthropicToolHistory(disabled))
}
