package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const openAICompatibleFakeRateLimitJSON = `{"id":"chatcmpl-freeipblk000000000000000","object":"chat.completion","created":1,"model":"gpt-5.5","choices":[{"index":0,"message":{"role":"assistant","content":"当前网络的请求存在异常，已被临时限制访问，请稍后再试。"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`

const openAICompatibleFakeRateLimitSSE = "data: {\"id\":\"chatcmpl-freeblock000\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"gpt-5.5\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"当前网络的请求存在异常，已被临时限制访问，请稍后再试。\"},\"finish_reason\":null}]}\n\ndata: [DONE]\n\n"

func TestIsOpenAICompatibleFakeRateLimitBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want bool
	}{
		{name: "json_freeipblk", body: openAICompatibleFakeRateLimitJSON, want: true},
		{name: "json_freeblock_id_only", body: `{"id":"chatcmpl-freeblock000","choices":[{"message":{"content":"ok"}}]}`, want: true},
		{name: "sse_freeblock", body: openAICompatibleFakeRateLimitSSE, want: true},
		{name: "text_only", body: `{"id":"chatcmpl-abc","choices":[{"message":{"content":"当前网络的请求存在异常，已被临时限制访问，请稍后再试。"}}]}`, want: true},
		{name: "normal_json", body: `{"id":"chatcmpl_1","model":"deepseek-v4.1-flash","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`, want: false},
		{name: "ordinary_rate_limit_text", body: `{"error":{"message":"您的请求过快，请稍后重试。"}}`, want: false},
		{name: "empty", body: "", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isOpenAICompatibleFakeRateLimitBody([]byte(tc.body)))
		})
	}
}

func TestPeekHTTPResponseBodyRestoresUnreadTail(t *testing.T) {
	payload := bytes.Repeat([]byte("a"), 100)
	payload = append(payload, []byte("TAIL")...)
	resp := &http.Response{Body: io.NopCloser(bytes.NewReader(payload))}

	prefix, err := peekHTTPResponseBody(resp, 100)
	require.NoError(t, err)
	require.Equal(t, bytes.Repeat([]byte("a"), 100), prefix)

	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, payload, got)
}

func TestForwardAsChatCompletions_FakeRateLimitJSONFailoversWithoutWriting(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"deepseek-v4.1-flash","messages":[{"role":"user","content":"hello"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_fake_rl"}},
		Body:       io.NopCloser(strings.NewReader(openAICompatibleFakeRateLimitJSON)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream, cfg: &config.Config{}}
	account := &Account{
		ID:          4578,
		Name:        "hiyo-deepseek-flash-1",
		Platform:    PlatformDeepSeek,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-ds-test",
			"base_url": "https://codexfree.xiaotuxp.com/v1",
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
	require.Nil(t, result)
	var failover *UpstreamFailoverError
	require.ErrorAs(t, err, &failover)
	require.Equal(t, http.StatusTooManyRequests, failover.StatusCode)
	require.False(t, c.Writer.Written(), "handler must be able to retry another account")
	require.Empty(t, rec.Body.String())
	require.NotContains(t, rec.Body.String(), "chatcmpl-freeipblk")
}

func TestForwardAsChatCompletions_FakeRateLimitSSEFailoversBeforeStream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"deepseek-v4.1-flash","stream":true,"messages":[{"role":"user","content":"hello"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_fake_sse"}},
		Body:       io.NopCloser(strings.NewReader(openAICompatibleFakeRateLimitSSE)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream, cfg: &config.Config{}}
	account := &Account{
		ID:          4579,
		Platform:    PlatformDeepSeek,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-ds-test", "base_url": "https://codexfree.xiaotuxp.com/v1"},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
	require.Nil(t, result)
	var failover *UpstreamFailoverError
	require.ErrorAs(t, err, &failover)
	require.Equal(t, http.StatusTooManyRequests, failover.StatusCode)
	require.False(t, c.Writer.Written())
	require.Empty(t, rec.Body.String())
}

func TestForwardResponses_FakeRateLimitChatFallbackFailovers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"deepseek-v4.1-flash","input":"hello"}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_fake_resp"}},
		Body:       io.NopCloser(strings.NewReader(openAICompatibleFakeRateLimitJSON)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{
		ID:          4580,
		Platform:    PlatformDeepSeek,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-ds-test"},
	}

	result, err := svc.Forward(context.Background(), c, account, body)
	require.Nil(t, result)
	var failover *UpstreamFailoverError
	require.True(t, errors.As(err, &failover))
	require.Equal(t, http.StatusTooManyRequests, failover.StatusCode)
	require.False(t, c.Writer.Written())
	require.Empty(t, rec.Body.String())
}
