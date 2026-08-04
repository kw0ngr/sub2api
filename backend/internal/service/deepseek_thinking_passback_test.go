package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestForwardAsChatCompletions_RestoresDeepSeekThinkingBeforeToolHistory(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{
		"model":"deepseek-v4-flash",
		"max_tokens":4096,
		"reasoning_effort":"max",
		"messages":[
			{"role":"user","content":"run it"},
			{"role":"assistant","reasoning_content":"I should use the tool.","content":"Running it.","tool_calls":[{"id":"call_1","type":"function","function":{"name":"exec","arguments":"{\"cmd\":\"true\"}"}}]},
			{"role":"tool","tool_call_id":"call_1","content":"ok"},
			{"role":"user","content":"continue"}
		],
		"tools":[{"type":"function","function":{"name":"exec","parameters":{"type":"object"}}}],
		"stream":false
	}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))

	upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":{"message":"probe"}}`))),
	}}
	svc := &GatewayService{
		cfg:                 &config.Config{},
		httpUpstream:        upstream,
		tlsFPProfileService: NewTLSFingerprintProfileService(&tlsProfileRepoStub{}, nil),
	}
	account := &Account{
		ID:          1917,
		Name:        "deepseek-apikey",
		Platform:    PlatformDeepSeek,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-test"},
	}

	_, _ = svc.ForwardAsChatCompletions(context.Background(), c, account, body, &ParsedRequest{})
	require.NotEmpty(t, upstream.lastBody)

	var sent apicompat.AnthropicRequest
	require.NoError(t, json.Unmarshal(upstream.lastBody, &sent))
	require.NotNil(t, sent.Thinking)
	require.Equal(t, "enabled", sent.Thinking.Type)

	var toolTurn []apicompat.AnthropicContentBlock
	for _, message := range sent.Messages {
		if message.Role != "assistant" {
			continue
		}
		var blocks []apicompat.AnthropicContentBlock
		require.NoError(t, json.Unmarshal(message.Content, &blocks))
		for _, block := range blocks {
			if block.Type == "tool_use" && block.Name == "exec" {
				toolTurn = blocks
			}
		}
	}
	require.NotEmpty(t, toolTurn)
	require.Equal(t, "thinking", toolTurn[0].Type)
	require.Equal(t, "I should use the tool.", toolTurn[0].Thinking)
	require.Equal(t, "text", toolTurn[1].Type)
	require.Equal(t, "Running it.", toolTurn[1].Text)
}

func TestExtractCCReasoningEffort_PreservesDeepSeekMax(t *testing.T) {
	effort := extractCCReasoningEffortFromBody([]byte(`{"model":"deepseek-v4-flash","reasoning_effort":"max"}`))
	require.NotNil(t, effort)
	require.Equal(t, "max", *effort)
}
