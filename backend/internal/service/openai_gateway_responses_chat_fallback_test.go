package service

import (
	"bytes"
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

func TestForwardResponses_ChatFallbackRestoresCodexToolSearchAndNamespace(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{
		"model":"deepseek-v4-pro",
		"input":"find repo",
		"stream":false,
		"tools":[
			{"type":"tool_search"},
			{"type":"namespace","name":"mcp","tools":[{"type":"function","name":"grep","description":"grep","parameters":{"type":"object"}}]}
		]
	}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_resp_chat_bridge"}},
		Body: io.NopCloser(strings.NewReader(`{
			"id":"chatcmpl_bridge",
			"object":"chat.completion",
			"model":"deepseek-v4-pro",
			"choices":[{"index":0,"message":{"role":"assistant","tool_calls":[
				{"id":"call_search","type":"function","function":{"name":"tool_search","arguments":"{\"query\":\"repo\"}"}},
				{"id":"call_grep","type":"function","function":{"name":"mcp__grep","arguments":"{\"pattern\":\"TODO\"}"}}
			]},"finish_reason":"tool_calls"}],
			"usage":{"prompt_tokens":5,"completion_tokens":4,"total_tokens":9}
		}`)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          99001,
		Platform:    PlatformDeepSeek,
		Type:        AccountTypeAPIKey,
		Name:        "deepseek-chat-only",
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-test"},
	}

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Stream)
	require.Equal(t, "https://api.deepseek.com/chat/completions", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-test", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "tool_search", gjson.GetBytes(upstream.lastBody, "tools.0.function.name").String())
	require.Equal(t, "mcp__grep", gjson.GetBytes(upstream.lastBody, "tools.1.function.name").String())

	out := rec.Body.String()
	require.Equal(t, "tool_search_call", gjson.Get(out, "output.0.type").String())
	require.Equal(t, "function_call", gjson.Get(out, "output.1.type").String())
	require.Equal(t, "mcp", gjson.Get(out, "output.1.namespace").String())
	require.Equal(t, "grep", gjson.Get(out, "output.1.name").String())
	require.Equal(t, 5, result.Usage.InputTokens)
	require.Equal(t, 4, result.Usage.OutputTokens)
}

func TestShouldForwardResponsesViaChatCompletionsScopesOpenAICompatExtra(t *testing.T) {
	require.True(t, shouldForwardResponsesViaChatCompletions(&Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra:    map[string]any{"openai_responses_supported": false},
	}))
	require.False(t, shouldForwardResponsesViaChatCompletions(&Account{
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Extra:    map[string]any{"openai_responses_supported": false},
	}))
	require.True(t, shouldForwardResponsesViaChatCompletions(&Account{
		Platform: PlatformDeepSeek,
		Type:     AccountTypeAPIKey,
		Extra:    map[string]any{"openai_responses_supported": true},
	}))
}

func TestShouldRetryResponsesViaChatCompletionsAfterHTTPErrorSkipsModel404(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": "https://proxy.example.com/v1",
		},
	}

	modelNotFoundBody := []byte(`{"error":{"code":"model_not_found","message":"Model \"gpt-x\" not found"}}`)
	require.False(t, shouldRetryResponsesViaChatCompletionsAfterHTTPError(account, http.StatusNotFound, "Model \"gpt-x\" not found", modelNotFoundBody))

	endpointNotFoundBody := []byte(`{"error":{"message":"Not found: /v1/responses"}}`)
	require.True(t, shouldRetryResponsesViaChatCompletionsAfterHTTPError(account, http.StatusNotFound, "Not found: /v1/responses", endpointNotFoundBody))
	require.True(t, shouldRetryResponsesViaChatCompletionsAfterHTTPError(account, http.StatusMethodNotAllowed, "method not allowed", nil))
}
