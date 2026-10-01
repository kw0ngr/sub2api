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

func TestGPT61SolRejectsExplicitNoReasoningBeforeUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name, path, body string
		forward          func(*OpenAIGatewayService, *gin.Context, *Account, []byte) (*OpenAIForwardResult, error)
	}{
		{"responses nested none", "/v1/responses", `{"model":"gpt-6.1-sol","input":"hi","reasoning":{"effort":"none"}}`,
			func(s *OpenAIGatewayService, c *gin.Context, a *Account, b []byte) (*OpenAIForwardResult, error) {
				return s.Forward(context.Background(), c, a, b)
			}},
		{"responses flat minimal", "/v1/responses", `{"model":"gpt-6.1-sol","input":"hi","reasoning_effort":"minimal"}`,
			func(s *OpenAIGatewayService, c *gin.Context, a *Account, b []byte) (*OpenAIForwardResult, error) {
				return s.Forward(context.Background(), c, a, b)
			}},
		{"chat mapped alias none", "/v1/chat/completions", `{"model":"my-sol","messages":[{"role":"user","content":"hi"}],"reasoning_effort":"none"}`,
			func(s *OpenAIGatewayService, c *gin.Context, a *Account, b []byte) (*OpenAIForwardResult, error) {
				return s.ForwardAsChatCompletions(context.Background(), c, a, b, "", "")
			}},
		{"messages suffix none", "/v1/messages", `{"model":"gpt-6.1-sol-none","max_tokens":256,"messages":[{"role":"user","content":"hi"}]}`,
			func(s *OpenAIGatewayService, c *gin.Context, a *Account, b []byte) (*OpenAIForwardResult, error) {
				return s.ForwardAsAnthropic(context.Background(), c, a, b, "", "")
			}},
		{"messages thinking disabled", "/v1/messages", `{"model":"gpt-6.1-sol","max_tokens":256,"thinking":{"type":"disabled"},"messages":[{"role":"user","content":"hi"}]}`,
			func(s *OpenAIGatewayService, c *gin.Context, a *Account, b []byte) (*OpenAIForwardResult, error) {
				return s.ForwardAsAnthropic(context.Background(), c, a, b, "", "")
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream := &httpUpstreamRecorder{}
			svc := &OpenAIGatewayService{httpUpstream: upstream, cfg: &config.Config{}}
			account := &Account{ID: 112, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1,
				Credentials: map[string]any{"api_key": "sk-test", "model_mapping": map[string]any{"gpt-6.1-sol": "gpt-6.1-sol", "my-sol": "gpt-6.1-sol"}}}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			body := []byte(tc.body)
			c.Request = httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)
			result, err := tc.forward(svc, c, account, body)
			require.ErrorContains(t, err, "does not support reasoning effort")
			require.Nil(t, result)
			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.Nil(t, upstream.lastReq)
		})
	}
}

func TestGPT61SolRejectsToolsWhenAccountOnlySupportsChat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := &Account{ID: 113, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1,
		Extra:       map[string]any{"openai_responses_supported": false},
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://example.com/v1"}}
	upstream := &httpUpstreamRecorder{}
	svc := &OpenAIGatewayService{httpUpstream: upstream, cfg: &config.Config{}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-6.1-sol","input":"hi","tools":[{"type":"function","name":"echo","parameters":{"type":"object","properties":{}}}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(string(body)))
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.ErrorContains(t, err, "requires Responses for tool calls")
	require.Nil(t, result)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Nil(t, upstream.lastReq)

	chatBody := []byte(`{"model":"gpt-6.1-sol","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function","function":{"name":"echo","parameters":{"type":"object","properties":{}}}}]}`)
	chatRec := httptest.NewRecorder()
	chatCtx, _ := gin.CreateTestContext(chatRec)
	chatCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(chatBody)))
	_, err = svc.ForwardAsChatCompletions(context.Background(), chatCtx, account, chatBody, "", "")
	require.ErrorContains(t, err, "requires Responses for tool calls")
	require.Equal(t, http.StatusBadRequest, chatRec.Code)
	require.Nil(t, upstream.lastReq)
}

func TestGPT61SolResponsesStripReasoningSampling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK,
		Header: http.Header{"Content-Type": []string{"application/json"}},
		Body:   io.NopCloser(strings.NewReader(`{"usage":{"input_tokens":1,"output_tokens":2}}`))}}
	svc := &OpenAIGatewayService{httpUpstream: upstream, cfg: &config.Config{}}
	account := &Account{ID: 114, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://example.com/v1"}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-6.1-sol","input":"hi","reasoning":{"effort":"max"},"temperature":0.7,"top_p":0.9,"logprobs":true,"include":["message.output_text.logprobs","reasoning.encrypted_content"]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(string(body)))
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)
	_, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.Equal(t, "max", gjson.GetBytes(upstream.lastBody, "reasoning.effort").String())
	for _, field := range []string{"temperature", "top_p", "logprobs"} {
		require.False(t, gjson.GetBytes(upstream.lastBody, field).Exists(), field)
	}
	require.NotContains(t, string(upstream.lastBody), "message.output_text.logprobs")
	require.Contains(t, string(upstream.lastBody), "reasoning.encrypted_content")
}

func TestGPT61SolMappedAliasUsesEffectiveModelRulesDuringConversion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, path, body string
		forward          func(*OpenAIGatewayService, *gin.Context, *Account, []byte) (*OpenAIForwardResult, error)
	}{
		{"chat completions", "/v1/chat/completions", `{"model":"my-sol","messages":[{"role":"user","content":"hi"}],"reasoning_effort":"max","temperature":0.7,"top_p":0.8}`,
			func(s *OpenAIGatewayService, c *gin.Context, a *Account, b []byte) (*OpenAIForwardResult, error) {
				return s.ForwardAsChatCompletions(context.Background(), c, a, b, "", "")
			}},
		{"anthropic messages", "/v1/messages", `{"model":"my-sol","max_tokens":256,"messages":[{"role":"user","content":"hi"}],"output_config":{"effort":"max"},"temperature":0.7,"top_p":0.8}`,
			func(s *OpenAIGatewayService, c *gin.Context, a *Account, b []byte) (*OpenAIForwardResult, error) {
				return s.ForwardAsAnthropic(context.Background(), c, a, b, "", "")
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusBadRequest,
				Body: io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"stop after request capture"}}`))}}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			account := &Account{ID: 116, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1,
				Credentials: map[string]any{"api_key": "sk-test", "model_mapping": map[string]any{"my-sol": "gpt-6.1-sol"}}}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			body := []byte(tc.body)
			c.Request = httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			_, err := tc.forward(svc, c, account, body)
			require.Error(t, err)
			require.NotNil(t, upstream.lastReq)
			require.Equal(t, "gpt-6.1-sol", gjson.GetBytes(upstream.lastBody, "model").String())
			require.Equal(t, "max", gjson.GetBytes(upstream.lastBody, "reasoning.effort").String())
			require.False(t, gjson.GetBytes(upstream.lastBody, "temperature").Exists())
			require.False(t, gjson.GetBytes(upstream.lastBody, "top_p").Exists())
		})
	}
}
