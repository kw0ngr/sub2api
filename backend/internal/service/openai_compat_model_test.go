package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestNormalizeOpenAICompatRequestedModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "gpt reasoning alias strips xhigh", input: "gpt-5.4-xhigh", want: "gpt-5.4"},
		{name: "gpt reasoning alias strips none", input: "gpt-5.4-none", want: "gpt-5.4"},
		{name: "codex max model stays intact", input: "gpt-5.1-codex-max", want: "gpt-5.1-codex-max"},
		{name: "non openai model unchanged", input: "claude-opus-4-6", want: "claude-opus-4-6"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, NormalizeOpenAICompatRequestedModel(tt.input))
		})
	}
}

func TestApplyOpenAICompatModelNormalization(t *testing.T) {
	t.Parallel()

	t.Run("derives xhigh from model suffix when output config missing", func(t *testing.T) {
		req := &apicompat.AnthropicRequest{Model: "gpt-5.4-xhigh"}

		applyOpenAICompatModelNormalization(req)

		require.Equal(t, "gpt-5.4", req.Model)
		require.NotNil(t, req.OutputConfig)
		require.Equal(t, "max", req.OutputConfig.Effort)
	})

	t.Run("explicit output config wins over model suffix", func(t *testing.T) {
		req := &apicompat.AnthropicRequest{
			Model:        "gpt-5.4-xhigh",
			OutputConfig: &apicompat.AnthropicOutputConfig{Effort: "low"},
		}

		applyOpenAICompatModelNormalization(req)

		require.Equal(t, "gpt-5.4", req.Model)
		require.NotNil(t, req.OutputConfig)
		require.Equal(t, "low", req.OutputConfig.Effort)
	})

	t.Run("non openai model is untouched", func(t *testing.T) {
		req := &apicompat.AnthropicRequest{Model: "claude-opus-4-6"}

		applyOpenAICompatModelNormalization(req)

		require.Equal(t, "claude-opus-4-6", req.Model)
		require.Nil(t, req.OutputConfig)
	})
}

func TestForwardAsAnthropic_NormalizesRoutingAndEffortForGpt54XHigh(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4-xhigh","max_tokens":16,"messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"type":"response.completed","response":{"id":"resp_1","object":"response","model":"gpt-5.4","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_compat"}},
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
			"model_mapping": map[string]any{
				"gpt-5.4": "gpt-5.4",
			},
		},
	}

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "gpt-5.1")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "gpt-5.4-xhigh", result.Model)
	require.Equal(t, "gpt-5.4", result.UpstreamModel)
	require.Equal(t, "gpt-5.4", result.BillingModel)
	require.NotNil(t, result.ReasoningEffort)
	require.Equal(t, "xhigh", *result.ReasoningEffort)

	require.Equal(t, "gpt-5.4", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "xhigh", gjson.GetBytes(upstream.lastBody, "reasoning.effort").String())
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "gpt-5.4-xhigh", gjson.GetBytes(rec.Body.Bytes(), "model").String())
	require.Equal(t, "ok", gjson.GetBytes(rec.Body.Bytes(), "content.0.text").String())
	t.Logf("upstream body: %s", string(upstream.lastBody))
	t.Logf("response body: %s", rec.Body.String())
}

func TestForwardAsAnthropic_ClientDisconnectDrainsUpstreamUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Writer = &openAIChatFailingWriter{ResponseWriter: c.Writer, failAfter: 0}
	body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":100,"stream":true,"messages":[{"role":"user","content":"hello"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
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
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_messages_disconnect"}},
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

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "gpt-5.1")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 11, result.Usage.InputTokens)
	require.Equal(t, 5, result.Usage.OutputTokens)
	require.Equal(t, 4, result.Usage.CacheReadInputTokens)
}

func TestForwardAsAnthropic_EventNamedTerminalReturns(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","max_tokens":16,"messages":[{"role":"user","content":"hello"}],"stream":true}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`: upstream ping`,
		"",
		`event: response.completed`,
		`data: {"response":{"id":"resp_1","object":"response","model":"gpt-5.4","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":15,"output_tokens":6,"total_tokens":21,"input_tokens_details":{"cached_tokens":5}}}}`,
		"",
		"",
	}, "\n")
	upstreamStream := newOpenAIChatBlockingReadCloser([]byte(upstreamBody))
	defer func() { _ = upstreamStream.Close() }()
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_messages_event_terminal"}},
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
		result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "gpt-5.1")
		resultCh <- forwardResult{result: result, err: err}
	}()

	select {
	case got := <-resultCh:
		require.NoError(t, got.err)
		require.NotNil(t, got.result)
		require.Equal(t, 15, got.result.Usage.InputTokens)
		require.Equal(t, 6, got.result.Usage.OutputTokens)
		require.Equal(t, 5, got.result.Usage.CacheReadInputTokens)
		require.Contains(t, rec.Body.String(), `"stop_reason":"end_turn"`)
	case <-time.After(time.Second):
		require.Fail(t, "ForwardAsAnthropic should use SSE event names when data payloads omit type")
	}
}

func TestForwardAsAnthropic_BufferedEventNamedTerminalReturns(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","max_tokens":16,"messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`event: response.completed`,
		`data: {"response":{"id":"resp_1","object":"response","model":"gpt-5.4","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":15,"output_tokens":6,"total_tokens":21,"input_tokens_details":{"cached_tokens":5}}}}`,
		"",
		"",
	}, "\n")
	upstreamStream := newOpenAIChatBlockingReadCloser([]byte(upstreamBody))
	defer func() { _ = upstreamStream.Close() }()
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_messages_buffered_event_terminal"}},
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
		result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "gpt-5.1")
		resultCh <- forwardResult{result: result, err: err}
	}()

	select {
	case got := <-resultCh:
		require.NoError(t, got.err)
		require.NotNil(t, got.result)
		require.Equal(t, 15, got.result.Usage.InputTokens)
		require.Equal(t, 6, got.result.Usage.OutputTokens)
		require.Equal(t, 5, got.result.Usage.CacheReadInputTokens)
		require.Equal(t, "ok", gjson.GetBytes(rec.Body.Bytes(), "content.0.text").String())
	case <-time.After(time.Second):
		require.Fail(t, "ForwardAsAnthropic buffered response should use SSE event names when data payloads omit type")
	}
}

func TestForwardAsAnthropic_ForcedCodexInstructionsTemplatePrependsRenderedInstructions(t *testing.T) {
	t.Parallel()

	templateDir := t.TempDir()
	templatePath := filepath.Join(templateDir, "codex-instructions.md.tmpl")
	require.NoError(t, os.WriteFile(templatePath, []byte("server-prefix\n\n{{ .ExistingInstructions }}"), 0o644))

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","max_tokens":16,"system":"client-system","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"type":"response.completed","response":{"id":"resp_1","object":"response","model":"gpt-5.4","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_forced"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}

	svc := &OpenAIGatewayService{
		cfg: &config.Config{Gateway: config.GatewayConfig{
			ForcedCodexInstructionsTemplateFile: templatePath,
			ForcedCodexInstructionsTemplate:     "server-prefix\n\n{{ .ExistingInstructions }}",
		}},
		httpUpstream: upstream,
	}
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

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "gpt-5.1")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "server-prefix\n\nclient-system", gjson.GetBytes(upstream.lastBody, "instructions").String())
}

func TestForwardAsAnthropic_ForcedCodexInstructionsTemplateUsesCachedTemplateContent(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","max_tokens":16,"system":"client-system","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"type":"response.completed","response":{"id":"resp_1","object":"response","model":"gpt-5.4","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_forced_cached"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}

	svc := &OpenAIGatewayService{
		cfg: &config.Config{Gateway: config.GatewayConfig{
			ForcedCodexInstructionsTemplateFile: "/path/that/should/not/be/read.tmpl",
			ForcedCodexInstructionsTemplate:     "cached-prefix\n\n{{ .ExistingInstructions }}",
		}},
		httpUpstream: upstream,
	}
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

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "gpt-5.1")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "cached-prefix\n\nclient-system", gjson.GetBytes(upstream.lastBody, "instructions").String())
}

func TestForwardAsAnthropic_GPT6AstraPromptCacheIdentityStableAcrossAppendedTurns(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{}
	svc := &OpenAIGatewayService{httpUpstream: upstream, cfg: &config.Config{}}
	account := &Account{
		ID:          606,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
	}
	firstBody := []byte(`{"model":"gpt-6","max_tokens":16,"stream":false,"messages":[{"role":"user","content":"open repo"}]}`)
	secondBody := []byte(`{"model":"gpt-6","max_tokens":16,"stream":false,"messages":[{"role":"user","content":"open repo"},{"role":"assistant","content":"opened"},{"role":"user","content":"run tests"}]}`)

	// When
	upstream.resp = task7AnthropicCompatResponse("resp_first")
	firstCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	firstCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(firstBody))
	firstResult, firstErr := svc.ForwardAsAnthropic(context.Background(), firstCtx, account, firstBody, "", "gpt-5.4")
	firstSession := upstream.lastReq.Header.Get("session_id")

	upstream.resp = task7AnthropicCompatResponse("resp_second")
	secondCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	secondCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(secondBody))
	secondResult, secondErr := svc.ForwardAsAnthropic(context.Background(), secondCtx, account, secondBody, "", "gpt-5.4")
	secondSession := upstream.lastReq.Header.Get("session_id")

	// Then
	require.NoError(t, firstErr)
	require.NotNil(t, firstResult)
	require.NoError(t, secondErr)
	require.NotNil(t, secondResult)
	require.NotEmpty(t, firstSession)
	require.Equal(t, firstSession, secondSession)
	require.Equal(t, "gpt-6", firstResult.Model)
	require.Equal(t, "gpt-6-astra", firstResult.UpstreamModel)
	require.Equal(t, "gpt-6-astra", secondResult.UpstreamModel)
}

func task7AnthropicCompatResponse(responseID string) *http.Response {
	body := strings.Join([]string{
		`data: {"type":"response.completed","response":{"id":"` + responseID + `","object":"response","model":"gpt-6-astra","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func TestForwardAsAnthropic_ReplaysFullToolHistoryWhenPreviousResponseUnavailable(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamSequenceRecorder{responses: []*http.Response{
		openAICompatContinuationErrorResponse(http.StatusBadRequest, "previous_response_id is not available for this user"),
		openAICompatCompletedResponse("resp_replayed", "gpt-6-astra"),
	}}
	repo := &grokProbeStreamAccountRepo{}
	svc := &OpenAIGatewayService{
		httpUpstream:     upstream,
		rateLimitService: NewRateLimitService(repo, nil, nil, nil, nil),
		cfg:              &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}
	account := task10OpenAIAstraAPIKeyAccount(10)
	svc.bindOpenAICompatSessionResponseID(context.Background(), nil, account, "stable-cache-key", "resp_stale")
	body := []byte(`{"model":"gpt-6-astra","max_tokens":16,"messages":[{"role":"user","content":"first"},{"role":"assistant","content":[{"type":"tool_use","id":"call_1","name":"lookup","input":{"q":"first"}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"call_1","content":"found"},{"type":"text","text":"second"}]}],"tools":[{"name":"lookup","input_schema":{"type":"object"}}],"stream":false}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))

	// When
	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "stable-cache-key", "gpt-6-astra")

	// Then
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.reqs, 2)
	require.Equal(t, "resp_stale", gjson.GetBytes(upstream.bodies[0], "previous_response_id").String())
	require.Equal(t, "stable-cache-key", gjson.GetBytes(upstream.bodies[0], "prompt_cache_key").String())
	second := upstream.bodies[1]
	require.False(t, gjson.GetBytes(second, "previous_response_id").Exists())
	require.Equal(t, "stable-cache-key", gjson.GetBytes(second, "prompt_cache_key").String())
	require.Equal(t, int64(5), gjson.GetBytes(second, "input.#").Int())
	require.Contains(t, gjson.GetBytes(second, "input.0.content.0.text").String(), "<sub2api-claude-code-todo-guard>")
	require.Equal(t, "first", gjson.GetBytes(second, "input.1.content.0.text").String())
	require.Equal(t, "function_call", gjson.GetBytes(second, "input.2.type").String())
	require.Equal(t, "call_1", gjson.GetBytes(second, "input.2.call_id").String())
	require.Equal(t, "function_call_output", gjson.GetBytes(second, "input.3.type").String())
	require.Equal(t, "call_1", gjson.GetBytes(second, "input.3.call_id").String())
	require.Equal(t, "found", gjson.GetBytes(second, "input.3.output").String())
	require.Equal(t, "second", gjson.GetBytes(second, "input.4.content.0.text").String())
	require.Zero(t, repo.setErrorCalls)
	require.Zero(t, repo.setSchedulableCalls)
	require.Zero(t, repo.rateLimitedCalls)
	require.Zero(t, repo.tempCalls)
	require.Zero(t, repo.modelCooldownCalls)
}

func TestForwardAsAnthropic_PreviousResponseUnavailableRetryFailureStops(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamSequenceRecorder{responses: []*http.Response{
		openAICompatContinuationErrorResponse(http.StatusBadRequest, "previous_response_id is not available for this user"),
		openAICompatContinuationErrorResponse(http.StatusBadRequest, "previous_response_id is not available for this user"),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}
	account := task10OpenAIAstraAPIKeyAccount(11)
	svc.bindOpenAICompatSessionResponseID(context.Background(), nil, account, "stable-cache-key", "resp_stale")
	body := []byte(`{"model":"gpt-6-astra","max_tokens":16,"messages":[{"role":"user","content":"hello"}],"stream":false}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))

	// When
	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "stable-cache-key", "gpt-6-astra")

	// Then
	require.Error(t, err)
	require.Nil(t, result)
	require.Len(t, upstream.reqs, 2)
	require.Equal(t, "resp_stale", gjson.GetBytes(upstream.bodies[0], "previous_response_id").String())
	require.False(t, gjson.GetBytes(upstream.bodies[1], "previous_response_id").Exists())
}

func TestForwardAsAnthropic_PreviousResponseNearFormsDoNotRetry(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		message string
	}{
		{name: "forbidden exact unavailable", status: http.StatusForbidden, message: "previous_response_id is not available for this user"},
		{name: "project near match", status: http.StatusBadRequest, message: "previous_response_id is not available for this project"},
		{name: "model near match", status: http.StatusBadRequest, message: "The model is not available for this user"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			gin.SetMode(gin.TestMode)
			upstream := &httpUpstreamSequenceRecorder{responses: []*http.Response{
				openAICompatContinuationErrorResponse(tt.status, tt.message),
				openAICompatCompletedResponse("resp_should_not_be_used", "gpt-6-astra"),
			}}
			svc := &OpenAIGatewayService{
				httpUpstream: upstream,
				cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
			}
			account := task10OpenAIAstraAPIKeyAccount(12)
			svc.bindOpenAICompatSessionResponseID(context.Background(), nil, account, "stable-cache-key", "resp_stale")
			body := []byte(`{"model":"gpt-6-astra","max_tokens":16,"messages":[{"role":"user","content":"hello"}],"stream":false}`)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))

			// When
			_, _ = svc.ForwardAsAnthropic(context.Background(), c, account, body, "stable-cache-key", "gpt-6-astra")

			// Then
			require.Len(t, upstream.reqs, 1)
			require.Equal(t, "resp_stale", gjson.GetBytes(upstream.bodies[0], "previous_response_id").String())
		})
	}
}

func TestOpenAICompatPreviousResponseContinuationRecognitionForAstraHTTPForms(t *testing.T) {
	// Given
	body := []byte(`{"error":{"message":"previous_response_id is not available for this user","type":"invalid_request_error"}}`)

	// When / Then
	require.True(t, isOpenAICompatPreviousResponseNotFound(http.StatusBadRequest, "", body))
	require.True(t, isOpenAICompatPreviousResponseUnsupported(http.StatusBadRequest, "previous_response_id requires an OpenAI API-key account for HTTP requests", nil))
	require.True(t, isOpenAICompatPreviousResponseUnsupported(http.StatusBadRequest, "previous_response_id is only supported on Responses WebSocket connections", nil))
	require.False(t, isOpenAICompatPreviousResponseNotFound(http.StatusForbidden, "previous_response_id is not available for this user", nil))
	require.False(t, isOpenAICompatPreviousResponseUnsupported(http.StatusUnauthorized, "previous_response_id requires an OpenAI API-key account for HTTP requests", nil))
	require.False(t, isOpenAICompatPreviousResponseUnsupported(http.StatusBadRequest, "The model is not available for this user", nil))
}

func TestForwardAsAnthropic_AstraContinuationUnsupportedFormsDisableSession(t *testing.T) {
	for _, message := range []string{
		"previous_response_id requires an OpenAI API-key account for HTTP requests",
		"previous_response_id is only supported on Responses WebSocket connections",
	} {
		t.Run(message, func(t *testing.T) {
			// Given
			gin.SetMode(gin.TestMode)
			upstream := &httpUpstreamSequenceRecorder{responses: []*http.Response{
				openAICompatContinuationErrorResponse(http.StatusBadRequest, message),
				openAICompatCompletedResponse("resp_replayed", "gpt-6-astra"),
				openAICompatCompletedResponse("resp_later", "gpt-6-astra"),
			}}
			svc := &OpenAIGatewayService{
				httpUpstream: upstream,
				cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
			}
			account := task10OpenAIAstraAPIKeyAccount(13)
			svc.bindOpenAICompatSessionResponseID(context.Background(), nil, account, "stable-cache-key", "resp_stale")
			body := []byte(`{"model":"gpt-6-astra","max_tokens":16,"messages":[{"role":"user","content":"first"},{"role":"assistant","content":"ok"},{"role":"user","content":"second"}],"stream":false}`)
			for i := 0; i < 2; i++ {
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))

				// When
				result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "stable-cache-key", "gpt-6-astra")

				// Then
				require.NoError(t, err)
				require.NotNil(t, result)
			}
			require.Len(t, upstream.reqs, 3)
			require.Equal(t, "resp_stale", gjson.GetBytes(upstream.bodies[0], "previous_response_id").String())
			for _, sent := range upstream.bodies[1:] {
				require.False(t, gjson.GetBytes(sent, "previous_response_id").Exists())
				require.Equal(t, "stable-cache-key", gjson.GetBytes(sent, "prompt_cache_key").String())
				require.Equal(t, int64(4), gjson.GetBytes(sent, "input.#").Int())
				require.Equal(t, "first", gjson.GetBytes(sent, "input.1.content.0.text").String())
				require.Equal(t, "second", gjson.GetBytes(sent, "input.3.content.0.text").String())
			}
		})
	}
}

func task10OpenAIAstraAPIKeyAccount(id int64) *Account {
	return &Account{
		ID:          id,
		Name:        "openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://api.openai.com/v1",
		},
		Status:      StatusActive,
		Schedulable: true,
	}
}

func openAICompatContinuationErrorResponse(status int, message string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"` + message + `","type":"invalid_request_error"}}`)),
	}
}

func openAICompatCompletedResponse(id, model string) *http.Response {
	body := strings.Join([]string{
		`data: {"type":"response.completed","response":{"id":"` + id + `","object":"response","model":"` + model + `","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
