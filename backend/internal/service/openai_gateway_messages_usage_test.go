package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestHandleAnthropicStreamingResponse_TopLevelTerminalUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_messages_top_level_usage"}},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"resp_messages_top","model":"gpt-5.4","status":"in_progress","output":[]}}`,
			"",
			`data: {"type":"response.output_text.delta","delta":"ok"}`,
			"",
			`data: {"type":"response.completed","response":{"id":"resp_messages_top","object":"response","model":"gpt-5.4","status":"completed","output":[]},"usage":{"input_tokens":20,"output_tokens":6,"total_tokens":26,"input_tokens_details":{"cached_tokens":5}}}`,
			"",
		}, "\n"))),
	}

	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	result, err := svc.handleAnthropicStreamingResponse(resp, c, &Account{ID: 1, Platform: PlatformOpenAI, Name: "openai"}, "claude-sonnet-4", "gpt-5.4", "gpt-5.4", time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 20, result.Usage.InputTokens)
	require.Equal(t, 6, result.Usage.OutputTokens)
	require.Equal(t, 5, result.Usage.CacheReadInputTokens)
	require.Contains(t, rec.Body.String(), `"cache_read_input_tokens":5`)
}

func TestHandleAnthropicBufferedStreamingResponse_TopLevelTerminalUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_messages_buffered_top_level_usage"}},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"response.completed","response":{"id":"resp_messages_buffered","object":"response","model":"gpt-5.4","status":"completed","output":[]},"usage":{"input_tokens":18,"output_tokens":7,"total_tokens":25,"input_tokens_details":{"cached_tokens":4}}}`,
			"",
		}, "\n"))),
	}

	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	result, err := svc.handleAnthropicBufferedStreamingResponse(resp, c, "claude-sonnet-4", "gpt-5.4", "gpt-5.4", time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 18, result.Usage.InputTokens)
	require.Equal(t, 7, result.Usage.OutputTokens)
	require.Equal(t, 4, result.Usage.CacheReadInputTokens)
	require.Contains(t, rec.Body.String(), `"cache_read_input_tokens":4`)
}
