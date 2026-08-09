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
