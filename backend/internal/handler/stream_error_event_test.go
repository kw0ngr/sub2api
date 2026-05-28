package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newResponsesStreamContext(t *testing.T, path string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	return c, rec
}

func parseResponsesFailedEvent(t *testing.T, body string) (map[string]any, map[string]any) {
	t.Helper()
	require.Contains(t, body, "event: response.failed\n")
	dataAt := strings.LastIndex(body, "data: ")
	require.NotEqual(t, -1, dataAt)
	raw := strings.TrimSpace(body[dataAt+len("data: "):])

	var event map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &event))
	assert.Equal(t, "response.failed", event["type"])
	responseObject, ok := event["response"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "response", responseObject["object"])
	assert.Equal(t, "failed", responseObject["status"])
	errorObject, ok := responseObject["error"].(map[string]any)
	require.True(t, ok)
	return responseObject, errorObject
}

func TestOpenAIHandleStreamingAwareError_ResponsesStreamingEmitsResponseFailed(t *testing.T) {
	c, rec := newResponsesStreamContext(t, EndpointResponses)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.RequestID, "15cc0881-adc2-4022-91bd-9e658e923843"))
	setOpsRequestContext(c, "gpt-5.3-codex", true, nil)

	h := &OpenAIGatewayHandler{}
	h.handleStreamingAwareError(c, http.StatusTooManyRequests, "rate_limit_error", "Upstream rate limit exceeded", true)

	responseObject, errorObject := parseResponsesFailedEvent(t, rec.Body.String())
	assert.Equal(t, "resp_15cc0881adc2402291bd9e658e923843", responseObject["id"])
	assert.Equal(t, "gpt-5.3-codex", responseObject["model"])
	assert.Equal(t, "rate_limit_exceeded", errorObject["code"])
	assert.Equal(t, "Upstream rate limit exceeded", errorObject["message"])
}

func TestGatewayHandleStreamingAwareError_ResponsesStreamingEmitsResponseFailed(t *testing.T) {
	c, rec := newResponsesStreamContext(t, "/responses")

	h := &GatewayHandler{}
	h.handleStreamingAwareError(c, http.StatusBadGateway, "upstream_error", "upstream gone", true)

	_, errorObject := parseResponsesFailedEvent(t, rec.Body.String())
	assert.Equal(t, "upstream_error", errorObject["code"])
}

func TestInboundIsResponses_CoversRegisteredRoutes(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/v1/responses", true},
		{"/v1/responses/compact", true},
		{"/responses", true},
		{"/responses/compact", true},
		{"/backend-api/codex/responses", true},
		{"/backend-api/codex/responses/compact", true},
		{"/v1/chat/completions", false},
		{"/responses-fake", false},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			c, _ := newResponsesStreamContext(t, test.path)
			assert.Equal(t, test.want, inboundIsResponses(c))
		})
	}
}

func TestOpenAIEnsureForwardErrorResponse_ResponsesAfterWrittenEmitsResponseFailed(t *testing.T) {
	c, rec := newResponsesStreamContext(t, EndpointResponses)
	_, _ = c.Writer.WriteString(":\n\n")

	h := &OpenAIGatewayHandler{}
	require.True(t, h.ensureForwardErrorResponse(c, false))
	require.Contains(t, rec.Body.String(), ":\n\n")
	parseResponsesFailedEvent(t, rec.Body.String())
}
