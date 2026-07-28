package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type openAIChatFailingWriter struct {
	gin.ResponseWriter
	failAfter int
	writes    int
}

func (w *openAIChatFailingWriter) Write(p []byte) (int, error) {
	if w.writes >= w.failAfter {
		return 0, errors.New("write failed: client disconnected")
	}
	w.writes++
	return w.ResponseWriter.Write(p)
}

type openAIChatBlockingReadCloser struct {
	data      []byte
	offset    int
	closed    chan struct{}
	closeOnce sync.Once
}

func TestOpenAIGatewayServiceApplyDefaultGPT56SolMaxReasoning(t *testing.T) {
	gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{})
	t.Cleanup(func() {
		gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{})
	})
	settingSvc := NewSettingService(&cacheControlSettingRepoStub{values: map[string]string{
		SettingKeyOpenAIGPT56SolDefaultMaxReasoning: "true",
	}}, &config.Config{})
	svc := &OpenAIGatewayService{settingService: settingSvc}
	req := &apicompat.ResponsesRequest{Model: "gpt-5.6-sol"}

	svc.applyDefaultGPT56SolMaxReasoning(context.Background(), "gpt-5.6-sol", req)

	require.NotNil(t, req.Reasoning)
	require.Equal(t, "max", req.Reasoning.Effort)
	require.Equal(t, "auto", req.Reasoning.Summary)

	existing := &apicompat.ResponsesRequest{
		Model:     "gpt-5.6-sol",
		Reasoning: &apicompat.ResponsesReasoning{Effort: "high"},
	}
	svc.applyDefaultGPT56SolMaxReasoning(context.Background(), "gpt-5.6-sol", existing)
	require.Equal(t, "high", existing.Reasoning.Effort)
}

func newOpenAIChatBlockingReadCloser(data []byte) *openAIChatBlockingReadCloser {
	return &openAIChatBlockingReadCloser{
		data:   data,
		closed: make(chan struct{}),
	}
}

func (r *openAIChatBlockingReadCloser) Read(p []byte) (int, error) {
	if r.offset < len(r.data) {
		n := copy(p, r.data[r.offset:])
		r.offset += n
		return n, nil
	}
	<-r.closed
	return 0, io.EOF
}

func (r *openAIChatBlockingReadCloser) Close() error {
	r.closeOnce.Do(func() {
		close(r.closed)
	})
	return nil
}

func TestNormalizeResponsesRequestServiceTier(t *testing.T) {
	t.Parallel()

	req := &apicompat.ResponsesRequest{ServiceTier: " fast "}
	normalizeResponsesRequestServiceTier(req)
	require.Equal(t, "priority", req.ServiceTier)

	req.ServiceTier = "flex"
	normalizeResponsesRequestServiceTier(req)
	require.Equal(t, "flex", req.ServiceTier)

	// OpenAI 官方合法 tier 应被透传保留。
	req.ServiceTier = "auto"
	normalizeResponsesRequestServiceTier(req)
	require.Equal(t, "auto", req.ServiceTier)

	req.ServiceTier = "default"
	normalizeResponsesRequestServiceTier(req)
	require.Equal(t, "default", req.ServiceTier)

	req.ServiceTier = "scale"
	normalizeResponsesRequestServiceTier(req)
	require.Equal(t, "scale", req.ServiceTier)

	// 真未知值仍被剥离。
	req.ServiceTier = "turbo"
	normalizeResponsesRequestServiceTier(req)
	require.Empty(t, req.ServiceTier)
}

func TestNormalizeResponsesBodyServiceTier(t *testing.T) {
	t.Parallel()

	body, tier, err := normalizeResponsesBodyServiceTier([]byte(`{"model":"gpt-5.1","service_tier":"fast"}`))
	require.NoError(t, err)
	require.Equal(t, "priority", tier)
	require.Equal(t, "priority", gjson.GetBytes(body, "service_tier").String())

	body, tier, err = normalizeResponsesBodyServiceTier([]byte(`{"model":"gpt-5.1","service_tier":"flex"}`))
	require.NoError(t, err)
	require.Equal(t, "flex", tier)
	require.Equal(t, "flex", gjson.GetBytes(body, "service_tier").String())

	// OpenAI 官方 tier 直接保留在 body 中（透传上游）。
	body, tier, err = normalizeResponsesBodyServiceTier([]byte(`{"model":"gpt-5.1","service_tier":"auto"}`))
	require.NoError(t, err)
	require.Equal(t, "auto", tier)
	require.Equal(t, "auto", gjson.GetBytes(body, "service_tier").String())

	body, tier, err = normalizeResponsesBodyServiceTier([]byte(`{"model":"gpt-5.1","service_tier":"default"}`))
	require.NoError(t, err)
	require.Equal(t, "default", tier)
	require.Equal(t, "default", gjson.GetBytes(body, "service_tier").String())

	body, tier, err = normalizeResponsesBodyServiceTier([]byte(`{"model":"gpt-5.1","service_tier":"scale"}`))
	require.NoError(t, err)
	require.Equal(t, "scale", tier)
	require.Equal(t, "scale", gjson.GetBytes(body, "service_tier").String())

	// 真未知值才会被删除。
	body, tier, err = normalizeResponsesBodyServiceTier([]byte(`{"model":"gpt-5.1","service_tier":"turbo"}`))
	require.NoError(t, err)
	require.Empty(t, tier)
	require.False(t, gjson.GetBytes(body, "service_tier").Exists())
}

func TestForwardAsChatCompletions_GLMOpenAINormalizesRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"glm-5.2","messages":[{"role":"user","content":"hi","model_id":"grok-4.5"},{"role":"assistant","content":"hello","model_id":"gpt-5.6-sol"}],"reasoning":{"effort":"xhigh"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"stop after forwarding"}}`)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{
		ID:       51,
		Name:     "glm-openai",
		Platform: PlatformGLM,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":     "glm-key",
			"compat_mode": GLMCompatModeOpenAI,
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "glm-5.2")
	require.Error(t, err)
	require.Nil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://open.bigmodel.cn/api/paas/v4/chat/completions", upstream.lastReq.URL.String())
	require.Equal(t, "max", gjson.GetBytes(upstream.lastBody, "reasoning.effort").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "messages.0.model_id").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "messages.1.model_id").Exists())
}

func TestForwardAsChatCompletions_APIKeyPropagatesPromptCacheKeyInResponsesBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("api_key", &APIKey{ID: 99})

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_chat_prompt_cache"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"stop before response parsing"}}`)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          2,
		Name:        "openai-compatible",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "sk-compatible",
		},
		Extra: map[string]any{
			"openai_responses_supported": true,
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "cache-key-123", "gpt-5.4")
	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, "cache-key-123", gjson.GetBytes(upstream.lastBody, "prompt_cache_key").String())
	require.Equal(t, "gpt-5.4", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "https://api.openai.com/v1/responses", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-compatible", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, generateSessionUUID(isolateOpenAISessionID(99, "cache-key-123")), upstream.lastReq.Header.Get("session_id"))
}

func TestForwardAsChatCompletions_ClientDisconnectDrainsUpstreamUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Writer = &openAIChatFailingWriter{ResponseWriter: c.Writer, failAfter: 0}
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-5.4","status":"in_progress","output":[]}}`,
		"",
		`data: {"type":"response.output_text.delta","delta":"ok"}`,
		"",
		`data: {"type":"response.completed","response":{"id":"resp_1","object":"response","model":"gpt-5.4","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":11,"output_tokens":5,"total_tokens":16,"input_tokens_details":{"cached_tokens":4}}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_chat_disconnect"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}

	svc := &OpenAIGatewayService{httpUpstream: upstream, cfg: &config.Config{}}
	account := &Account{
		ID:          1,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.1")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 11, result.Usage.InputTokens)
	require.Equal(t, 5, result.Usage.OutputTokens)
	require.Equal(t, 4, result.Usage.CacheReadInputTokens)
}

func TestForwardAsChatCompletions_StreamsUsageWithoutClientStreamOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-5.4","status":"in_progress","output":[]}}`,
		"",
		`data: {"type":"response.output_text.delta","delta":"ok"}`,
		"",
		`data: {"type":"response.completed","response":{"id":"resp_1","object":"response","model":"gpt-5.4","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":13,"output_tokens":7,"total_tokens":20,"input_tokens_details":{"cached_tokens":5}}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_chat_usage_no_stream_options"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}

	svc := &OpenAIGatewayService{httpUpstream: upstream, cfg: &config.Config{}}
	account := &Account{
		ID:          1,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.1")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 13, result.Usage.InputTokens)
	require.Equal(t, 7, result.Usage.OutputTokens)
	require.Equal(t, 5, result.Usage.CacheReadInputTokens)

	responseBody := rec.Body.String()
	require.Contains(t, responseBody, `"usage"`)
	require.Contains(t, responseBody, `"prompt_tokens":13`)
	require.Contains(t, responseBody, `"completion_tokens":7`)
	require.Contains(t, responseBody, `"cached_tokens":5`)
}

func TestForwardAsChatCompletions_StreamsTopLevelTerminalUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_top","model":"gpt-5.4","status":"in_progress","output":[]}}`,
		"",
		`data: {"type":"response.output_text.delta","delta":"ok"}`,
		"",
		`data: {"type":"response.completed","response":{"id":"resp_top","object":"response","model":"gpt-5.4","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}]},"usage":{"input_tokens":21,"output_tokens":9,"total_tokens":30,"input_tokens_details":{"cached_tokens":4}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_chat_top_level_usage"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}

	svc := &OpenAIGatewayService{httpUpstream: upstream, cfg: &config.Config{}}
	account := &Account{
		ID:          1,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.1")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 21, result.Usage.InputTokens)
	require.Equal(t, 9, result.Usage.OutputTokens)
	require.Equal(t, 4, result.Usage.CacheReadInputTokens)

	responseBody := rec.Body.String()
	require.Contains(t, responseBody, `"usage"`)
	require.Contains(t, responseBody, `"prompt_tokens":21`)
	require.Contains(t, responseBody, `"completion_tokens":9`)
	require.Contains(t, responseBody, `"cached_tokens":4`)
}

func TestForwardAsChatCompletions_BufferedTopLevelTerminalUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"type":"response.completed","response":{"id":"resp_top_buffered","object":"response","model":"gpt-5.4","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}]},"usage":{"input_tokens":18,"output_tokens":6,"total_tokens":24,"input_tokens_details":{"cached_tokens":3}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_chat_buffered_top_level_usage"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}

	svc := &OpenAIGatewayService{httpUpstream: upstream, cfg: &config.Config{}}
	account := &Account{
		ID:          1,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.1")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 18, result.Usage.InputTokens)
	require.Equal(t, 6, result.Usage.OutputTokens)
	require.Equal(t, 3, result.Usage.CacheReadInputTokens)

	responseBody := rec.Body.String()
	require.Contains(t, responseBody, `"prompt_tokens":18`)
	require.Contains(t, responseBody, `"completion_tokens":6`)
	require.Contains(t, responseBody, `"cached_tokens":3`)
}

func TestForwardAsChatCompletions_EventNamedTerminalReturns(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`event: response.created`,
		`data: {"response":{"id":"resp_1","model":"gpt-5.4","status":"in_progress","output":[]}}`,
		"",
		`event: response.output_text.delta`,
		`data: {"delta":"ok"}`,
		"",
		`event: response.completed`,
		`data: {"response":{"id":"resp_1","object":"response","model":"gpt-5.4","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":17,"output_tokens":8,"total_tokens":25,"input_tokens_details":{"cached_tokens":6}}}}`,
		"",
		"",
	}, "\n")
	upstreamStream := newOpenAIChatBlockingReadCloser([]byte(upstreamBody))
	defer func() { _ = upstreamStream.Close() }()
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_chat_event_terminal"}},
		Body:       upstreamStream,
	}}

	svc := &OpenAIGatewayService{httpUpstream: upstream, cfg: &config.Config{}}
	account := &Account{
		ID:          1,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
	}

	type forwardResult struct {
		result *OpenAIForwardResult
		err    error
	}
	resultCh := make(chan forwardResult, 1)
	go func() {
		result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.1")
		resultCh <- forwardResult{result: result, err: err}
	}()

	select {
	case got := <-resultCh:
		require.NoError(t, got.err)
		require.NotNil(t, got.result)
		require.Equal(t, 17, got.result.Usage.InputTokens)
		require.Equal(t, 8, got.result.Usage.OutputTokens)
		require.Equal(t, 6, got.result.Usage.CacheReadInputTokens)
		require.Contains(t, rec.Body.String(), `"content":"ok"`)
		require.Contains(t, rec.Body.String(), `data: [DONE]`)
	case <-time.After(time.Second):
		require.Fail(t, "ForwardAsChatCompletions should use SSE event names when data payloads omit type")
	}
}

func TestForwardAsChatCompletions_DoneSentinelWithoutTerminalReturnsError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_chat_missing_terminal"}},
		Body:       io.NopCloser(strings.NewReader("data: [DONE]\n\n")),
	}}

	svc := &OpenAIGatewayService{httpUpstream: upstream, cfg: &config.Config{}}
	account := &Account{
		ID:          1,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing terminal event")
	require.NotNil(t, result)
	require.Zero(t, result.Usage.InputTokens)
	require.Zero(t, result.Usage.OutputTokens)
}

func TestForwardAsChatCompletions_BufferedTerminalWithoutUpstreamCloseReturns(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := []byte(`data: {"type":"response.completed","response":{"id":"resp_1","object":"response","model":"gpt-5.4","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":17,"output_tokens":8,"total_tokens":25,"input_tokens_details":{"cached_tokens":6}}}}` + "\n\n")
	upstreamStream := newOpenAIChatBlockingReadCloser(upstreamBody)
	defer func() { _ = upstreamStream.Close() }()
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_chat_buffered_terminal_no_close"}},
		Body:       upstreamStream,
	}}

	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          1,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
	}

	type forwardResult struct {
		result *OpenAIForwardResult
		err    error
	}
	resultCh := make(chan forwardResult, 1)
	go func() {
		result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.1")
		resultCh <- forwardResult{result: result, err: err}
	}()

	select {
	case got := <-resultCh:
		require.NoError(t, got.err)
		require.NotNil(t, got.result)
		require.Equal(t, 17, got.result.Usage.InputTokens)
		require.Equal(t, 8, got.result.Usage.OutputTokens)
		require.Equal(t, 6, got.result.Usage.CacheReadInputTokens)
		require.Contains(t, rec.Body.String(), `"finish_reason":"stop"`)
	case <-time.After(time.Second):
		require.Fail(t, "ForwardAsChatCompletions buffered response should return after terminal usage event even if upstream keeps the connection open")
	}
}

func TestForwardAsChatCompletions_UpstreamRequestIgnoresClientCancel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	reqCtx, cancel := context.WithCancel(context.Background())
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body)).WithContext(reqCtx)
	c.Request.Header.Set("Content-Type", "application/json")
	cancel()

	upstreamBody := strings.Join([]string{
		`data: {"type":"response.completed","response":{"id":"resp_1","object":"response","model":"gpt-5.4","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_chat_ctx"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}

	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          1,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
	}

	result, err := svc.ForwardAsChatCompletions(reqCtx, c, account, body, "", "gpt-5.1")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.NoError(t, upstream.lastReq.Context().Err())
}

func TestForwardAsChatCompletions_ThirdPartyStreamForcesUsageOption(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"openai/gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"ok"}}]}`,
		"",
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","choices":[],"usage":{"prompt_tokens":3,"completion_tokens":2,"prompt_tokens_details":{"cached_tokens":1}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_openrouter_usage"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}

	svc := &OpenAIGatewayService{httpUpstream: upstream, cfg: &config.Config{}}
	account := &Account{
		ID:          2,
		Name:        "openrouter",
		Platform:    PlatformOpenRouter,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "sk-or-test",
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream_options.include_usage").Bool())
	require.Equal(t, 3, result.Usage.InputTokens)
	require.Equal(t, 2, result.Usage.OutputTokens)
	require.Equal(t, 1, result.Usage.CacheReadInputTokens)
}

func TestForwardAsChatCompletions_ThirdPartyExtractsNestedReasoningEffort(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"hello"}],"reasoning":{"effort":"high"},"service_tier":"flex"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_deepseek_reasoning"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"chatcmpl_1","model":"deepseek-v4-flash","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop","index":0}],"usage":{"prompt_tokens":4,"completion_tokens":2}}`)),
	}}

	svc := &OpenAIGatewayService{httpUpstream: upstream, cfg: &config.Config{}}
	account := &Account{
		ID:          3,
		Name:        "deepseek",
		Platform:    PlatformDeepSeek,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "sk-ds-test",
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.ReasoningEffort)
	require.Equal(t, "high", *result.ReasoningEffort)
	require.NotNil(t, result.ServiceTier)
	require.Equal(t, "flex", *result.ServiceTier)
}
