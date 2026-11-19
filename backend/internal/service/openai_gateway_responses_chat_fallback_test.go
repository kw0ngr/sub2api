package service

import (
	"bytes"
	"context"
	"fmt"

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

func TestGeminiResponsesBridgeRestoresSignaturesAcrossAccounts(t *testing.T) {
	for _, path := range []string{"/v1/responses", "/responses"} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", path, stream), func(t *testing.T) {
				cache := &geminiSignatureCacheStub{}
				upstream := &httpUpstreamRecorder{}
				svc := &OpenAIGatewayService{cfg: &config.Config{}, cache: cache, httpUpstream: upstream}
				account := signatureTestAccount()
				forward := func(input string, payload string, sse bool) *httptest.ResponseRecorder {
					t.Helper()
					body := []byte(fmt.Sprintf(`{"model":"gemini-3.8-flash","input":%s,"stream":%t,"prompt_cache_key":"codex-session"}`, input, sse))
					rec := httptest.NewRecorder()
					c, _ := gin.CreateTestContext(rec)
					c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
					group := int64(17)
					c.Set("api_key", &APIKey{ID: 33, GroupID: &group})
					upstream.resp = &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(payload))}
					_, err := svc.Forward(context.Background(), c, account, body)
					require.NoError(t, err)
					require.Equal(t, 200, rec.Code)
					return rec
				}
				first := signatureTestResponse
				if stream {
					first = `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"a\":1,\"large\":1234567890123456789}"},"extra_content":{"google":{"thought_signature":"genuine-signature"}}}]}}]}` + "\n\n" +
						`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"id":"call_2","type":"function","function":{"name":"lookup","arguments":"{}"}}]}}]}` + "\n\n" +
						`data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\ndata: [DONE]\n\n"
				}
				rec := forward(`"lookup value"`, first, stream)
				require.Contains(t, rec.Body.String(), `"call_id":"call_1"`)
				require.NotContains(t, rec.Body.String(), "genuine-signature")
				follow := `[{"role":"user","content":"changed user anchor"},{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{\"large\":1234567890123456789,\"a\":1}"}`
				if stream {
					require.Contains(t, rec.Body.String(), `"call_id":"call_2"`)
					follow += `,{"type":"function_call","call_id":"call_2","name":"lookup","arguments":"{}"}`
				}
				follow += `,{"type":"function_call_output","call_id":"call_1","output":"OK"}`
				if stream {
					follow += `,{"type":"function_call_output","call_id":"call_2","output":"OK2"}`
				}
				account = signatureTestAccount()
				account.ID++
				account.Credentials["api_key"] = "other-key"
				forward(follow+`]`, `{"choices":[{"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}]}`, false)
				require.Equal(t, "Bearer other-key", upstream.lastReq.Header.Get("Authorization"))
				require.Equal(t, "genuine-signature", gjson.GetBytes(upstream.lastBody, "messages.1.tool_calls.0.extra_content.google.thought_signature").String())
				require.False(t, gjson.GetBytes(upstream.lastBody, "messages.1.tool_calls.1.extra_content.google.thought_signature").Exists())
			})
		}
	}
}

func TestGeminiResponsesBridgeAnchorsLeadingToolTurn(t *testing.T) {
	body := []byte(`{"model":"gemini-4-neon","instructions":"be brief","input":[{"type":"reasoning","summary":[]},{"type":"function_call","call_id":"call_old","name":"lookup","arguments":"{}"},{"type":"function_call_output","call_id":"call_old","output":"OK"},{"role":"user","content":"what was the result?"}]}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}]}`))}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	_, err := svc.Forward(context.Background(), c, signatureTestAccount(), body)
	require.NoError(t, err)
	require.Equal(t, 200, rec.Code)
	messages := gjson.GetBytes(upstream.lastBody, "messages").Array()
	require.Len(t, messages, 5)
	require.Equal(t, "system", messages[0].Get("role").String())
	require.JSONEq(t, `{"role":"user","content":""}`, messages[1].Raw)
	require.Equal(t, "call_old", messages[2].Get("tool_calls.0.id").String())
	require.Equal(t, "call_old", messages[3].Get("tool_call_id").String())
	require.Equal(t, "what was the result?", messages[4].Get("content").String())
}
