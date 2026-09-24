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

func TestNormalizeOpenAIPassthroughOAuthBody_RemovesUnsupportedUser(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","input":"hello","user":"user_123","metadata":{"user_id":"user_123"},"prompt_cache_retention":"24h","safety_identifier":"sid","stream_options":{"include_usage":true}}`)

	normalized, changed, err := normalizeOpenAIPassthroughOAuthBody(body, false)
	require.NoError(t, err)
	require.True(t, changed)
	for _, field := range openAIChatGPTInternalUnsupportedFields {
		require.False(t, gjson.GetBytes(normalized, field).Exists(), "%s should be stripped", field)
	}
	require.True(t, gjson.GetBytes(normalized, "stream").Bool())
	require.False(t, gjson.GetBytes(normalized, "store").Bool())
}

func TestNormalizeOpenAIPassthroughOAuthBody_CompactRemovesUnsupportedUser(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","input":"hello","user":"user_123","metadata":{"user_id":"user_123"},"stream":true,"store":true}`)

	normalized, changed, err := normalizeOpenAIPassthroughOAuthBody(body, true)
	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(normalized, "user").Exists())
	require.False(t, gjson.GetBytes(normalized, "metadata").Exists())
	require.False(t, gjson.GetBytes(normalized, "stream").Exists())
	require.False(t, gjson.GetBytes(normalized, "store").Exists())
}

func TestNormalizeOpenAIResponsesReasoningEffortAlias_MovesFlatResponsesAlias(t *testing.T) {
	body := []byte(`{"model":"gpt-5.6-sol","input":"hello","reasoning_effort":"max"}`)

	normalized, changed, err := normalizeOpenAIResponsesReasoningEffortAlias(body, "gpt-5.6-sol")

	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(normalized, "reasoning_effort").Exists())
	require.Equal(t, "max", gjson.GetBytes(normalized, "reasoning.effort").String())
}

func TestNormalizeOpenAIResponsesReasoningEffortAlias_AstraNoneBecomesLow(t *testing.T) {
	body := []byte(`{"model":"gpt-6-astra","input":"hello","reasoning_effort":"none"}`)

	normalized, changed, err := normalizeOpenAIResponsesReasoningEffortAlias(body, "gpt-6-astra")

	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(normalized, "reasoning_effort").Exists())
	require.Equal(t, "low", gjson.GetBytes(normalized, "reasoning.effort").String())
}

func TestGPT6SolAndLunaSamplingDependsOnReasoning(t *testing.T) {
	for _, model := range []string{"gpt-6-sol", "gpt-6-luna"} {
		for _, effort := range []string{"none", "max"} {
			t.Run(model+"/"+effort, func(t *testing.T) {
				body := []byte(`{"model":"` + model + `","input":"hi","reasoning_effort":"` + effort + `","temperature":0.7,"top_p":0.8,"logprobs":true,"include":["message.output_text.logprobs","reasoning.encrypted_content"]}`)
				normalized, _, err := normalizeOpenAIResponsesReasoningEffortAlias(body, model)
				require.NoError(t, err)
				forwarded, _, err := stripOpenAIResponsesReasoningUnsupportedFieldsBytes(normalized, model)
				require.NoError(t, err)
				require.Equal(t, effort, gjson.GetBytes(forwarded, "reasoning.effort").String())
				require.Equal(t, effort == "none", gjson.GetBytes(forwarded, "temperature").Exists())
				require.Equal(t, effort == "none", gjson.GetBytes(forwarded, "top_p").Exists())
				require.Equal(t, effort == "none", gjson.GetBytes(forwarded, "logprobs").Exists())
				require.Equal(t, effort == "none", strings.Contains(string(forwarded), "message.output_text.logprobs"))
				require.Contains(t, string(forwarded), "reasoning.encrypted_content")
			})
		}
	}
}

func TestOpenAIGatewayServiceForward_AstraStripsForbiddenSamplingFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"usage":{"input_tokens":1,"output_tokens":2}}`)),
	}}
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}
	account := &Account{
		ID: 31, Name: "openai-apikey", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://example.com"},
		Status:      StatusActive, Schedulable: true,
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)
	body := []byte(`{"model":"gpt-6-astra","stream":false,"input":"hello","reasoning":{"effort":"max"},"temperature":0.7,"top_p":0.8,"top_logprobs":2,"logprobs":true,"include":["message.output_text.logprobs","reasoning.encrypted_content"]}`)

	result, err := svc.Forward(context.Background(), c, account, body)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "max", gjson.GetBytes(upstream.lastBody, "reasoning.effort").String())
	for _, field := range []string{"temperature", "top_p", "top_logprobs", "logprobs"} {
		require.False(t, gjson.GetBytes(upstream.lastBody, field).Exists(), "%s should be stripped for Astra", field)
	}
	require.NotContains(t, string(upstream.lastBody), "message.output_text.logprobs")
	require.Contains(t, string(upstream.lastBody), "reasoning.encrypted_content")
}

func TestOpenAIGatewayServiceForward_NonAstraSamplingFieldsRemain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"usage":{"input_tokens":1,"output_tokens":2}}`)),
	}}
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}
	account := &Account{
		ID: 32, Name: "openai-apikey", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://example.com"},
		Status:      StatusActive, Schedulable: true,
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)
	body := []byte(`{"model":"gpt-4o","stream":false,"input":"hello","temperature":0.7,"top_p":0.8,"top_logprobs":2,"logprobs":true,"include":["message.output_text.logprobs"]}`)

	result, err := svc.Forward(context.Background(), c, account, body)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 0.7, gjson.GetBytes(upstream.lastBody, "temperature").Float())
	require.Equal(t, 0.8, gjson.GetBytes(upstream.lastBody, "top_p").Float())
	require.True(t, gjson.GetBytes(upstream.lastBody, "logprobs").Bool())
	require.Contains(t, string(upstream.lastBody), "message.output_text.logprobs")
}
