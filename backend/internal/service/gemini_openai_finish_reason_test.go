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
	normalized := normalizeGeminiOpenAIFinishReasons(account, body)

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
