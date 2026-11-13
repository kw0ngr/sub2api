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
