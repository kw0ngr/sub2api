package service

import (
	"bytes"
	"context"
	"fmt"
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

func TestGeminiResponsesProtocolRoutingAndEffort(t *testing.T) {
	for _, platform := range []string{PlatformGemini, PlatformOpenAI, PlatformOpenRouter, PlatformDeepSeek, PlatformGrok} {
		require.True(t, IsOpenAIResponsesCompatiblePlatform(platform))
	}
	for _, platform := range []string{PlatformAnthropic, PlatformGLM, PlatformAntigravity} {
		require.False(t, IsOpenAIResponsesCompatiblePlatform(platform))
	}
	for _, tc := range []struct{ model, want string }{{"gemini-4-argon", "high"}, {"gemini-4-carbon", "high"}, {"gemini-4-hydrogen", "xhigh"}} {
		gin.SetMode(gin.TestMode)
		body := []byte(fmt.Sprintf(`{"model":"%s","input":"hi","reasoning":{"effort":"max"}}`, tc.model))
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
		upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"id":"chatcmpl_gemini","model":"gemini-4-argon","choices":[{"index":0,"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))}}
		svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
		account := &Account{ID: 1, Platform: PlatformGemini, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{"api_key": "test-key"}}
		result, err := svc.Forward(context.Background(), c, account, body)
		require.NoError(t, err)
		require.Equal(t, "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions", upstream.lastReq.URL.String())
		require.Equal(t, tc.want, gjson.GetBytes(upstream.lastBody, "reasoning_effort").String())
		require.NotNil(t, result.ReasoningEffort)
		require.Equal(t, tc.want, *result.ReasoningEffort)
		require.Equal(t, "completed", gjson.Get(recorder.Body.String(), "status").String())
	}
}

func TestGeminiNewModelsChatEffortUsesMappedModel(t *testing.T) {
	for _, tc := range []struct{ model, want string }{{"gemini-4-carbon", "high"}, {"gemini-4-hydrogen", "xhigh"}} {
		body := []byte(`{"model":"client-model","messages":[{"role":"user","content":"hi"}],"reasoning_effort":"max","stream":false}`)
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		upstream := &httpUpstreamRecorder{resp: &http.Response{
			StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{"id":"chatcmpl_test","model":"` + tc.model + `","choices":[{"index":0,"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`)),
		}}
		svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
		account := &Account{ID: 1, Platform: PlatformGemini, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{
			"api_key": "test-key", "model_mapping": map[string]any{"client-model": tc.model},
		}}
		result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
		require.NoError(t, err)
		require.Equal(t, tc.model, gjson.GetBytes(upstream.lastBody, "model").String())
		require.Equal(t, tc.want, gjson.GetBytes(upstream.lastBody, "reasoning_effort").String())
		require.NotNil(t, result.ReasoningEffort)
		require.Equal(t, tc.want, *result.ReasoningEffort)
	}
}

func TestGeminiChatStripsZCodeThinkingAliasesBeforeUpstream(t *testing.T) {
	body := []byte(`{"model":"gemini-4-hydrogen","stream":true,"messages":[{"role":"system","content":"You are ZCode connectivity probe."},{"role":"user","content":"hi"}],"thinking":{"type":"disabled"},"reasoning":{"effort":"none"},"stream_options":{"include_usage":true},"enable_thinking":false,"reasoning_effort":"none","max_completion_tokens":512}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader("data: {\"id\":\"chatcmpl_test\",\"model\":\"gemini-4-hydrogen\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"OK\"},\"finish_reason\":null}]}\n\ndata: {\"id\":\"chatcmpl_test\",\"model\":\"gemini-4-hydrogen\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 1, Platform: PlatformGemini, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{"api_key": "test-key"}}
	_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
	require.NoError(t, err)
	require.Equal(t, "none", gjson.GetBytes(upstream.lastBody, "reasoning_effort").String())
	for _, field := range []string{"reasoning", "thinking", "enable_thinking"} {
		require.False(t, gjson.GetBytes(upstream.lastBody, field).Exists(), field)
	}
}
