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

func TestNormalizeGLMOpenAIReasoningEffort(t *testing.T) {
	tests := []struct {
		name          string
		model         string
		input         string
		wantApplied   bool
		wantPath      string
		wantValue     string
		wantUnchanged bool
	}{
		{name: "flat xhigh maps to max", model: "glm-5.2", input: `{"model":"glm-5.2","reasoning_effort":"xhigh","messages":[]}`, wantApplied: true, wantPath: "reasoning_effort", wantValue: "max"},
		{name: "flat x-high maps to max", model: "GLM-5.2", input: `{"model":"glm-5.2","reasoning_effort":"x-high","messages":[]}`, wantApplied: true, wantPath: "reasoning_effort", wantValue: "max"},
		{name: "flat ultracode maps to max", model: "glm-5.2", input: `{"model":"glm-5.2","reasoning_effort":"ultracode","messages":[]}`, wantApplied: true, wantPath: "reasoning_effort", wantValue: "max"},
		{name: "flat medium maps to high", model: "glm-5.2", input: `{"model":"glm-5.2","reasoning_effort":"medium","messages":[]}`, wantApplied: true, wantPath: "reasoning_effort", wantValue: "high"},
		{name: "glm 5.2 low maps to high", model: "glm-5.2", input: `{"model":"glm-5.2","reasoning_effort":"low","messages":[]}`, wantApplied: true, wantPath: "reasoning_effort", wantValue: "high"},
		{name: "nested high case-normalizes", model: "glm-5.2", input: `{"model":"glm-5.2","reasoning":{"effort":"HIGH"},"messages":[]}`, wantApplied: true, wantPath: "reasoning.effort", wantValue: "high"},
		{name: "glm 5.3 native low unchanged", model: "glm-5.3", input: `{"model":"glm-5.3","reasoning_effort":"low","messages":[]}`, wantApplied: false, wantUnchanged: true},
		{name: "native max unchanged", model: "glm-5.2", input: `{"model":"glm-5.2","reasoning_effort":"max","messages":[]}`, wantApplied: false, wantUnchanged: true},
		{name: "non glm unchanged", model: "deepseek-v4-pro", input: `{"model":"deepseek-v4-pro","reasoning_effort":"xhigh","messages":[]}`, wantApplied: false, wantUnchanged: true},
		{name: "missing effort unchanged", model: "glm-5.2", input: `{"model":"glm-5.2","messages":[]}`, wantApplied: false, wantUnchanged: true},
		{name: "unknown effort unchanged", model: "glm-5.2", input: `{"model":"glm-5.2","reasoning_effort":"banana","messages":[]}`, wantApplied: false, wantUnchanged: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, applied := NormalizeGLMOpenAIReasoningEffort([]byte(tt.input), tt.model)
			require.Equal(t, tt.wantApplied, applied)
			if tt.wantUnchanged {
				require.Equal(t, tt.input, string(got))
				return
			}
			require.Equal(t, tt.wantValue, gjson.GetBytes(got, tt.wantPath).String())
		})
	}
}

func TestNativeAnthropicPassthroughNormalizesGLM53Thinking(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		stream     bool
		preference string
		wantEffort string
	}{
		{name: "disabled buffered", preference: `"thinking":{"type":"disabled"},`, wantEffort: "low"},
		{name: "off buffered", preference: `"thinking":{"type":"off"},`, wantEffort: "low"},
		{name: "none buffered", preference: `"thinking":{"type":"none"},`, wantEffort: "low"},
		{name: "enabled buffered", preference: `"thinking":{"type":"enabled"},`, wantEffort: "high"},
		{name: "enabled streaming", stream: true, preference: `"thinking":{"type":"enabled"},`, wantEffort: "high"},
		{name: "adaptive buffered", preference: `"thinking":{"type":"adaptive"},`, wantEffort: "high"},
		{name: "adaptive streaming", stream: true, preference: `"thinking":{"type":"adaptive"},`, wantEffort: "high"},
		{name: "minimal buffered", preference: `"output_config":{"effort":"minimal"},`, wantEffort: "low"},
		{name: "low buffered", preference: `"output_config":{"effort":"low"},`, wantEffort: "low"},
		{name: "medium streaming", stream: true, preference: `"output_config":{"effort":"medium"},`, wantEffort: "high"},
		{name: "high buffered", preference: `"output_config":{"effort":"high"},`, wantEffort: "high"},
		{name: "xhigh buffered", preference: `"output_config":{"effort":"xhigh"},`, wantEffort: "max"},
		{name: "max buffered", preference: `"output_config":{"effort":"max"},`, wantEffort: "max"},
		{name: "ultra buffered", preference: `"output_config":{"effort":"ultra"},`, wantEffort: "max"},
		{name: "output effort wins over thinking", preference: `"thinking":{"type":"adaptive"},"output_config":{"effort":"low"},`, wantEffort: "low"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"model":"glm-5.3","max_tokens":32,"stream":%t,%s"messages":[{"role":"user","content":"hi"}]}`, tt.stream, tt.preference)
			upstreamBody, result, err := forwardGLM53NativeAnthropicForTest(t, body, tt.stream)

			require.Equal(t, "enabled", gjson.GetBytes(upstreamBody, "thinking.type").String())
			require.Equal(t, tt.wantEffort, gjson.GetBytes(upstreamBody, "output_config.effort").String())
			if tt.stream {
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, result.ReasoningEffort)
			require.Equal(t, tt.wantEffort, *result.ReasoningEffort)
		})
	}
}

func TestNativeAnthropicPassthroughLeavesOtherThinkingUntouched(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name     string
		body     string
		wantBody string
	}{
		{name: "glm 5.3 unspecified", body: `{"model":"glm-5.3","max_tokens":32,"stream":false,"messages":[]}`},
		{name: "glm 5.2 disabled", body: `{"model":"glm-5.2","max_tokens":32,"stream":false,"thinking":{"type":"disabled"},"messages":[]}`},
		{
			name:     "glm 5.2 output config effort low",
			body:     `{"model":"glm-5.2","max_tokens":32,"stream":false,"output_config":{"effort":"low"},"messages":[]}`,
			wantBody: `{"model":"glm-5.2","max_tokens":32,"stream":false,"output_config":{"effort":"low"},"messages":[],"thinking":{"type":"disabled"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstreamBody, result, err := forwardGLM53NativeAnthropicForTest(t, tt.body, false)
			wantBody := tt.body
			if tt.wantBody != "" {
				wantBody = tt.wantBody
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			require.JSONEq(t, wantBody, string(upstreamBody))
			require.Nil(t, result.ReasoningEffort)
		})
	}
}

func forwardGLM53NativeAnthropicForTest(t *testing.T, body string, stream bool) ([]byte, *ForwardResult, error) {
	t.Helper()
	bodyBytes := []byte(body)
	upstream := &httpUpstreamRecorder{resp: nativeAnthropicGLM53Response(stream)}
	svc := &GatewayService{cfg: &config.Config{}, httpUpstream: upstream, rateLimitService: &RateLimitService{}}
	result, err := svc.Forward(context.Background(), glm53AnthropicContext(bodyBytes), nativeAnthropicGLM53Account(), &ParsedRequest{
		Body:   bodyBytes,
		Model:  gjson.GetBytes(bodyBytes, "model").String(),
		Stream: gjson.GetBytes(bodyBytes, "stream").Bool(),
	})
	require.NotNil(t, upstream.lastReq)
	return upstream.lastBody, result, err
}

func nativeAnthropicGLM53Account() *Account {
	return &Account{
		ID:       702,
		Name:     "glm-native",
		Platform: PlatformGLM,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":     "sk-glm-test",
			"base_url":    "https://open.bigmodel.cn/api/anthropic",
			"compat_mode": GLMCompatModeAnthropic,
		},
	}
}

func glm53AnthropicContext(body []byte) *gin.Context {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func nativeAnthropicGLM53Response(stream bool) *http.Response {
	if stream {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader("data: [DONE]\n\n"))}
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"msg_1","type":"message","role":"assistant","model":"glm-5.3","content":[{"type":"text","text":"pong"}],"stop_reason":"end_turn","usage":{"input_tokens":93,"output_tokens":16}}`)),
	}
}
