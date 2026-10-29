package service

import (
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

func TestNormalizeGeminiOpenAIFinishReasons_maps_extended_content_filter(t *testing.T) {
	// Given
	account := &Account{Platform: PlatformGemini}
	body := []byte(`{"choices":[{"finish_reason":"content_filter: PROHIBITED_CONTENT"}]}`)

	// When
	normalized := normalizeGeminiOpenAIFinishReasons(account, body, nil)

	// Then
	require.Equal(t, "content_filter", gjson.GetBytes(normalized, "choices.0.finish_reason").String())
}

func TestOpenAIGatewayGemini_non_stream_normalizes_extended_content_filter(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	response := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"choices":[{"finish_reason":"content_filter: PROHIBITED_CONTENT"}],"usage":{"prompt_tokens":5,"completion_tokens":0}}`,
		)),
	}
	service := &OpenAIGatewayService{cfg: &config.Config{}}
	account := &Account{Platform: PlatformGemini}

	// When
	_, err := service.handleNonStreamingResponse(context.Background(), response, c, account, "gemini-3.6-flash", "gemini-3.6-flash")

	// Then
	require.NoError(t, err)
	require.Equal(t, "content_filter", gjson.GetBytes(recorder.Body.Bytes(), "choices.0.finish_reason").String())
}

func TestOpenAIGatewayGemini_stream_normalizes_extended_content_filter(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	response := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"data: {\"choices\":[{\"finish_reason\":\"content_filter: PROHIBITED_CONTENT\"}],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":0}}\n\ndata: [DONE]\n\n",
		)),
	}
	service := &OpenAIGatewayService{cfg: &config.Config{}}
	account := &Account{Platform: PlatformGemini}

	// When
	_, err := service.handleStreamingResponse(context.Background(), response, c, account, time.Now(), "gemini-3.6-flash", "gemini-3.6-flash")

	// Then
	require.NoError(t, err)
	require.Contains(t, recorder.Body.String(), `"finish_reason":"content_filter"`)
	require.NotContains(t, recorder.Body.String(), "PROHIBITED_CONTENT")
}

func TestGeminiOpenAIThoughtSignaturesSurviveResponseForwarding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	payload := `{"id":"chatcmpl-test","choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup_test_value","arguments":"{}"},"extra_content":{"google":{"thought_signature":"signed-context"}}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":5,"completion_tokens":2}}`
	for _, stream := range []bool{false, true} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		body, contentType := payload, "application/json"
		if stream {
			body, contentType = "data: "+payload+"\n\ndata: [DONE]\n\n", "text/event-stream"
		}
		resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{contentType}}, Body: io.NopCloser(strings.NewReader(body))}
		svc := &OpenAIGatewayService{cfg: &config.Config{}}
		account := &Account{Platform: PlatformGemini, Type: AccountTypeAPIKey}
		if stream {
			_, err := svc.handleStreamingResponse(context.Background(), resp, c, account, time.Now(), "gemini-4-argon", "gemini-4-argon")
			require.NoError(t, err)
		} else {
			_, err := svc.handleNonStreamingResponse(context.Background(), resp, c, account, "gemini-4-argon", "gemini-4-argon")
			require.NoError(t, err)
		}
		require.Contains(t, recorder.Body.String(), `"thought_signature":"signed-context"`)
	}
}

func TestGeminiOpenAIToolCallsNormalizeStopFinishReason(t *testing.T) {
	payload := `{"id":"chatcmpl-test","choices":[{"index":0,"message":{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup_test_value","arguments":"{}"},"extra_content":{"google":{"thought_signature":"signed-context"}}}]},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2}}`
	got := normalizeGeminiOpenAIFinishReasons(&Account{Platform: PlatformGemini}, []byte(payload), nil)
	require.Equal(t, "tool_calls", gjson.GetBytes(got, "choices.0.finish_reason").String())
	require.Equal(t, "signed-context", gjson.GetBytes(got, "choices.0.message.tool_calls.0.extra_content.google.thought_signature").String())
}

func TestGeminiOpenAIStreamToolFinishReasonAcrossChunks(t *testing.T) {
	for _, platform := range []string{PlatformGemini, PlatformOpenAI} {
		t.Run(platform, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			body := "data: " + `{"id":"chatcmpl-test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup_test_value","arguments":"{}"},"extra_content":{"google":{"thought_signature":"signed-context"}}}]},"finish_reason":null}]}` + "\n\n" +
				"data: " + `{"choices":[{"index":1,"delta":{"content":"ordinary text"},"finish_reason":null}]}` + "\n\n" +
				"data: " + `{"choices":[{"index":0,"delta":{},"finish_reason":"stop"},{"index":1,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2}}` + "\n\ndata: [DONE]\n\n"
			resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))}
			svc := &OpenAIGatewayService{cfg: &config.Config{}}
			_, err := svc.handleStreamingResponse(context.Background(), resp, c, &Account{Platform: platform, Type: AccountTypeAPIKey}, time.Now(), "gemini-3.8-flash", "gemini-3.8-flash")
			require.NoError(t, err)
			var terminal gjson.Result
			for _, line := range strings.Split(recorder.Body.String(), "\n") {
				if !strings.HasPrefix(line, "data:") {
					continue
				}
				p := gjson.Parse(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
				if p.Get("usage").Exists() {
					terminal = p
				}
			}
			expected := "stop"
			if platform == PlatformGemini {
				expected = "tool_calls"
			}
			require.Equal(t, expected, terminal.Get("choices.0.finish_reason").String())
			require.Equal(t, "stop", terminal.Get("choices.1.finish_reason").String())
			require.Contains(t, recorder.Body.String(), `"thought_signature":"signed-context"`)
		})
	}
}

func TestGeminiOpenAIToolFinishReasonKeepsFailureSemantics(t *testing.T) {
	for _, reason := range []string{"length", "content_filter", "content_filter: SAFETY", "tool_calls"} {
		body := []byte(`{"choices":[{"message":{"tool_calls":[{"id":"call_1"}]},"finish_reason":"` + reason + `"}]}`)
		got := normalizeGeminiOpenAIFinishReasons(&Account{Platform: PlatformGemini}, body, nil)
		expected := reason
		if strings.HasPrefix(reason, "content_filter:") {
			expected = "content_filter"
		}
		require.Equal(t, expected, gjson.GetBytes(got, "choices.0.finish_reason").String())
	}
}

func TestGeminiOpenAIRequestKeepsOriginalToolSignatures(t *testing.T) {
	for _, signature := range []string{"signed-context", ""} {
		t.Run(signature, func(t *testing.T) {
			body := `{"model":"gemini-3.8-flash","messages":[{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup_test_value","arguments":"{}"},"extra_content":{"google":{"thought_signature":"` + signature + `"},"provider_extension":{"keep":true}}}]},{"role":"tool","tool_call_id":"call_1","content":"OK"}],"stream":true}`
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 400,
				Header: http.Header{"Content-Type": {"application/json"}},
				Body:   io.NopCloser(strings.NewReader(`{"error":{"message":"stop after forwarding"}}`))}}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			account := &Account{Platform: PlatformGemini, Type: AccountTypeAPIKey,
				Credentials: map[string]any{"api_key": "test", "base_url": "https://generativelanguage.googleapis.com"}}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
			_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, []byte(body), "", "")
			require.Error(t, err)
			require.NotNil(t, upstream.lastReq)
			require.Equal(t, signature, gjson.GetBytes(upstream.lastBody, "messages.0.tool_calls.0.extra_content.google.thought_signature").String())
			require.True(t, gjson.GetBytes(upstream.lastBody, "messages.0.tool_calls.0.extra_content.provider_extension.keep").Bool())
			require.Equal(t, "call_1", gjson.GetBytes(upstream.lastBody, "messages.1.tool_call_id").String())
		})
	}
}
