package service

import (
	"context"
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

func TestOpenAIWSHTTPBridgeConfigAndPayload(t *testing.T) {
	cfg := &config.Config{}
	require.Equal(t, openAIWSClientReadLimitBytesDefault, ResolveOpenAIWSClientReadLimitBytes(cfg))
	cfg.Gateway.OpenAIWS.ClientReadLimitBytes = 8 << 20
	require.Equal(t, int64(8<<20), ResolveOpenAIWSClientReadLimitBytes(cfg))

	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	svc.cfg.Gateway.OpenAIWS.HTTPBridgeEnabled = true
	svc.cfg.Gateway.OpenAIWS.HTTPBridgeThresholdBytes = 10
	require.True(t, svc.shouldBridgeOpenAIWSHTTP(10, ""))
	require.False(t, svc.shouldBridgeOpenAIWSHTTP(9, ""))
	require.False(t, svc.shouldBridgeOpenAIWSHTTP(64<<20, "resp_previous"))

	body, err := prepareOpenAIWSHTTPBridgeBody([]byte(`{"type":"response.create","model":"gpt-5.4","input":"hi","generate":true,"previous_response_id":"resp_old","stream":false}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"gpt-5.4","input":"hi","stream":true}`, string(body))
}

func TestProxyOpenAIWSHTTPBridgeTurnStreamsResponsesSSEToClientWS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/openai/v1/responses", nil)

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"text/event-stream"},
				"x-request-id": []string{"upstream-http-bridge"},
			},
			Body: io.NopCloser(strings.NewReader(strings.Join([]string{
				`data: {"type":"response.output_text.delta","item_id":"msg_1","output_index":0,"content_index":0,"delta":"hi"}`,
				`data: {"type":"response.completed","response":{"id":"resp_http_bridge","model":"gpt-5.4","usage":{"input_tokens":3,"output_tokens":2}}}`,
				`data: [DONE]`,
				``,
			}, "\n\n"))),
		},
	}
	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		cfg:          &config.Config{},
	}
	account := &Account{
		ID:          77,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://api.openai.com/v1",
		},
	}

	var clientMessages []string
	result, err := svc.proxyOpenAIWSHTTPBridgeTurn(
		context.Background(),
		c,
		account,
		"sk-test",
		[]byte(`{"type":"response.create","model":"gpt-5.4","input":"hi","stream":false}`),
		64<<20,
		"gpt-5.4",
		1,
		func(message []byte) error {
			clientMessages = append(clientMessages, string(message))
			return nil
		},
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "resp_http_bridge", result.RequestID)
	require.Equal(t, 3, result.Usage.InputTokens)
	require.Equal(t, 2, result.Usage.OutputTokens)
	require.True(t, result.Stream)
	require.True(t, result.OpenAIWSMode)
	require.Len(t, clientMessages, 2)
	require.Contains(t, clientMessages[0], "response.output_text.delta")
	require.Contains(t, clientMessages[1], "response.completed")
	require.JSONEq(t, `{"model":"gpt-5.4","input":"hi","stream":true}`, string(upstream.lastBody))
}

func TestOpenAIWSHTTPBridge_AstraPreservesConfigurationUpdateAndAsyncTools(t *testing.T) {
	// Given
	payload := []byte(`{"type":"response.create","model":"gpt-6-astra","input":[{"type":"configuration_update","session":{"instructions":"keep","tool_choice":"auto"}},{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}],"tools":[{"type":"function","name":"shell","async":true,"parameters":{"type":"object","properties":{"cmd":{"type":"string"}}}}],"generate":true,"previous_response_id":"resp_old","stream":false}`)

	// When
	body, err := prepareOpenAIWSHTTPBridgeBody(payload)

	// Then
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"gpt-6-astra","input":[{"type":"configuration_update","session":{"instructions":"keep","tool_choice":"auto"}},{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}],"tools":[{"type":"function","name":"shell","async":true,"parameters":{"type":"object","properties":{"cmd":{"type":"string"}}}}],"stream":true}`, string(body))
	require.Equal(t, gjson.GetBytes(payload, "input.0").Raw, gjson.GetBytes(body, "input.0").Raw)
	require.Equal(t, gjson.GetBytes(payload, "tools.0").Raw, gjson.GetBytes(body, "tools.0").Raw)
}
