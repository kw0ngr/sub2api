package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIGatewayService_Forward_GrokOAuthPreservesNativeResponsesFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{
		"model":"grok-4.5",
		"input":"Reply exactly OK",
		"stream":false,
		"store":false,
		"max_output_tokens":128,
		"temperature":0.2,
		"top_p":0.9,
		"metadata":{"audit":"grok-native"},
		"reasoning":{"effort":"high","summary":"concise"}
	}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "Codex Desktop/1.2.3")
	c.Request.Header.Set("originator", "Codex Desktop")
	c.Request.Header.Set("session_id", "codex-session")
	c.Request.Header.Set("conversation_id", "codex-conversation")
	c.Request.Header.Set("x-codex-turn-state", "codex-state")
	c.Request.Header.Set("x-codex-turn-metadata", "codex-metadata")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"id":"resp_grok","object":"response","model":"grok-4.5","status":"completed","output":[],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`,
		)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{
		ID:          2000,
		Name:        "grok-native",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "xai-token",
			"base_url":     "https://api.x.ai/v1",
		},
	}

	result, err := svc.Forward(context.Background(), c, account, body)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://api.x.ai/v1/responses", upstream.lastReq.URL.String())
	require.Equal(t, "grok-shell/0.2.111 (linux; x86_64)", upstream.lastReq.Header.Get("User-Agent"))
	require.Equal(t, "0.2.111", upstream.lastReq.Header.Get("x-grok-client-version"))
	for _, header := range []string{"originator", "session_id", "conversation_id", "x-codex-turn-state", "x-codex-turn-metadata"} {
		require.Empty(t, upstream.lastReq.Header.Get(header), header)
	}
	require.Equal(t, int64(128), gjson.GetBytes(upstream.lastBody, "max_output_tokens").Int())
	require.InDelta(t, 0.2, gjson.GetBytes(upstream.lastBody, "temperature").Float(), 0.0001)
	require.InDelta(t, 0.9, gjson.GetBytes(upstream.lastBody, "top_p").Float(), 0.0001)
	require.True(t, gjson.GetBytes(upstream.lastBody, "store").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "store").Bool())
	require.Equal(t, "high", gjson.GetBytes(upstream.lastBody, "reasoning.effort").String())
	require.Equal(t, "concise", gjson.GetBytes(upstream.lastBody, "reasoning.summary").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "metadata").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "instructions").Exists())
}

func TestOpenAIGatewayService_ForwardAsAnthropic_GrokOAuthSkipsCodexTransform(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"grok-4.5","max_tokens":128,"temperature":0.2,"top_p":0.9,"messages":[{"role":"user","content":"hi"}],"stream":false}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"type":"response.completed","response":{"id":"resp_grok_messages","object":"response","model":"grok-4.5","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{
		ID:          2000,
		Name:        "grok-native",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "xai-token",
			"base_url":     "https://api.x.ai/v1",
		},
	}

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "")

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "https://api.x.ai/v1/responses", upstream.lastReq.URL.String())
	require.Equal(t, int64(128), gjson.GetBytes(upstream.lastBody, "max_output_tokens").Int())
	require.InDelta(t, 0.2, gjson.GetBytes(upstream.lastBody, "temperature").Float(), 0.0001)
	require.InDelta(t, 0.9, gjson.GetBytes(upstream.lastBody, "top_p").Float(), 0.0001)
	require.False(t, gjson.GetBytes(upstream.lastBody, "instructions").Exists())
	require.Empty(t, upstream.lastReq.Header.Get("OpenAI-Beta"))
	require.Empty(t, upstream.lastReq.Header.Get("originator"))
}

func TestOpenAIGatewayService_GrokStreamingResponseKeepsNativeToolNames(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	stream := strings.Join([]string{
		`data: {"type":"response.output_item.done","tool_calls":[{"function":{"name":"apply_patch","arguments":"{}"}}]}`,
		"",
		`data: {"type":"response.completed","response":{"id":"resp_grok_tools","object":"response","model":"grok-4.5","status":"completed","output":[],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(stream)),
	}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, toolCorrector: NewCodexToolCorrector()}

	_, err := svc.handleStreamingResponse(
		context.Background(), resp, c,
		&Account{ID: 2000, Platform: PlatformGrok, Type: AccountTypeOAuth},
		time.Now(), "grok-4.5", "grok-4.5",
	)

	require.NoError(t, err)
	require.Contains(t, rec.Body.String(), `"name":"apply_patch"`)
	require.NotContains(t, rec.Body.String(), `"name":"edit"`)
}
