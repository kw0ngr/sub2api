package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestNormalizeGrokThirdPartyChatToolMessageNames_adds_missing_names(t *testing.T) {
	t.Parallel()

	// Given
	body := []byte(`{
		"model":"grok-4.5",
		"tools":[
			{"type":"function","function":{"name":"run_terminal_command","parameters":{"type":"object"}}},
			{"type":"function","function":{"name":"todo_write","parameters":{"type":"object"}}}
		],
		"messages":[
			{"role":"tool","tool_call_id":"call-orphan","content":"orphan result"},
			{"role":"assistant","content":"","tool_calls":[
				{"id":"call-1","type":"function","function":{"name":"todo_write","arguments":"{}"}}
			]},
			{"role":"tool","tool_call_id":"call-1","content":"todo result"},
			{"role":"tool","tool_call_id":"call-kept","name":"already_named","content":"kept"}
		]
	}`)
	var req apicompat.ChatCompletionsRequest
	require.NoError(t, json.Unmarshal(body, &req))
	account := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": "https://ai.muapi.cn",
		},
	}

	// When
	got, modified, err := normalizeGrokThirdPartyChatToolMessageNames(account, body, &req)

	// Then
	require.NoError(t, err)
	require.True(t, modified)
	require.Equal(t, "run_terminal_command", gjson.GetBytes(got, "messages.0.name").String())
	require.Equal(t, "todo_write", gjson.GetBytes(got, "messages.2.name").String())
	require.Equal(t, "already_named", gjson.GetBytes(got, "messages.3.name").String())
}

func TestNormalizeGrokThirdPartyChatToolMessageNames_skips_official_xai(t *testing.T) {
	t.Parallel()

	// Given
	body := []byte(`{
		"model":"grok-4.5",
		"tools":[{"type":"function","function":{"name":"run_terminal_command","parameters":{"type":"object"}}}],
		"messages":[{"role":"tool","tool_call_id":"call-1","content":"result"}]
	}`)
	var req apicompat.ChatCompletionsRequest
	require.NoError(t, json.Unmarshal(body, &req))
	account := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": "https://api.x.ai/v1",
		},
	}

	// When
	got, modified, err := normalizeGrokThirdPartyChatToolMessageNames(account, body, &req)

	// Then
	require.NoError(t, err)
	require.False(t, modified)
	require.False(t, gjson.GetBytes(got, "messages.0.name").Exists())
}

func TestForwardAsChatCompletions_GrokThirdPartyAddsToolMessageNames(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Given
	body := []byte(`{
		"model":"grok-4.5",
		"messages":[
			{"role":"assistant","content":"","tool_calls":[
				{"id":"call-1","type":"function","function":{"name":"run_terminal_command","arguments":"{}"}}
			]},
			{"role":"tool","tool_call_id":"call-1","content":"ok"}
		],
		"tools":[{"type":"function","function":{"name":"run_terminal_command","parameters":{"type":"object"}}}]
	}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"stop after forwarding"}}`)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{
		ID:       1938,
		Name:     "third-party-grok",
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "grok-key",
			"base_url": "https://ai.muapi.cn",
		},
	}

	// When
	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "grok-4.5")

	// Then
	require.Error(t, err)
	require.Nil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://ai.muapi.cn/v1/chat/completions", upstream.lastReq.URL.String())
	require.Equal(t, "run_terminal_command", gjson.GetBytes(upstream.lastBody, "messages.1.name").String())
}
