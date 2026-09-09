package service

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAICodexFingerprintDefaultOffKeepsClientMetadata(t *testing.T) {
	// Given
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 101, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	c, _ := newTask12OpenAITestContext(t, 7, "client-session")
	metadata := `{"installation_id":"client-install","session_id":"client-session","thread_id":"client-thread","turn_id":"client-turn","window_id":"client-window"}`
	c.Request.Header.Set("x-codex-turn-metadata", metadata)

	// When
	req, err := svc.buildUpstreamRequest(context.Background(), c, account, []byte(`{"model":"gpt-5","input":[]}`), "token", false, "client-session", true)

	// Then
	require.NoError(t, err)
	require.Empty(t, req.Header.Get("x-codex-installation-id"))
	require.JSONEq(t, metadata, req.Header.Get("x-codex-turn-metadata"))
}

func TestOpenAICodexFingerprintExtraOptInRewritesHeaders(t *testing.T) {
	// Given
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 102, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{"codex_fingerprint_mode": "session"}}
	c, _ := newTask12OpenAITestContext(t, 7, "client-session")
	c.Request.Header.Set("x-codex-turn-metadata", `{"installation_id":"client-install","session_id":"client-session","thread_id":"client-thread","turn_id":"client-turn","window_id":"client-window","sandbox":"workspace-write"}`)

	// When
	req, err := svc.buildUpstreamRequest(context.Background(), c, account, []byte(`{"model":"gpt-5","input":[]}`), "token", false, "client-session", true)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, req.Header.Get("x-codex-installation-id"))
	require.NotEqual(t, "client-session", req.Header.Get("session_id"))
	require.NotEqual(t, "client-thread", req.Header.Get("x-client-request-id"))
	turnMetadata := req.Header.Get("x-codex-turn-metadata")
	require.Equal(t, req.Header.Get("x-codex-installation-id"), gjson.Get(turnMetadata, "installation_id").String())
	require.Equal(t, req.Header.Get("session_id"), gjson.Get(turnMetadata, "session_id").String())
	require.Equal(t, req.Header.Get("x-client-request-id"), gjson.Get(turnMetadata, "thread_id").String())
	require.Equal(t, "workspace-write", gjson.Get(turnMetadata, "sandbox").String())
}

func TestOpenAICodexFingerprintPassthroughExtraOptInRewritesHeaderAndBodyTogether(t *testing.T) {
	// Given
	upstream := &task12HTTPUpstreamRecorder{}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:       103,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "token",
		},
		Extra: map[string]any{"codex_fingerprint_mode": "session"},
	}
	c, _ := newTask12OpenAITestContext(t, 7, "client-session")
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.144.1 (Ubuntu 22.4.0; x86_64) xterm-256color")
	body := []byte(`{"model":"gpt-5","instructions":"ok","stream":false,"input":[],"client_metadata":{"traceparent":"00-abc-def-01","session_id":"client-session","thread_id":"client-thread","turn_id":"client-turn","x-codex-turn-metadata":"{\"installation_id\":\"client-install\",\"session_id\":\"client-session\",\"thread_id\":\"client-thread\",\"turn_id\":\"client-turn\",\"window_id\":\"client-window\"}"}}`)

	// When
	_, err := svc.forwardOpenAIPassthrough(context.Background(), c, account, body, "gpt-5", nil, false, time.Now())

	// Then
	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	req := upstream.requests[0]
	require.NotEmpty(t, req.Header.Get("x-codex-installation-id"))
	require.NotEmpty(t, req.Header.Get("x-client-request-id"))
	require.Len(t, upstream.bodies, 1)
	metadata := gjson.GetBytes(upstream.bodies[0], "client_metadata")
	require.Equal(t, "00-abc-def-01", metadata.Get("traceparent").String())
	require.Equal(t, req.Header.Get("x-codex-installation-id"), metadata.Get("x-codex-installation-id").String())
	require.Equal(t, req.Header.Get("session_id"), metadata.Get("session_id").String())
	require.Equal(t, req.Header.Get("x-client-request-id"), metadata.Get("thread_id").String())
	var embedded map[string]any
	require.NoError(t, json.Unmarshal([]byte(metadata.Get("x-codex-turn-metadata").String()), &embedded))
	require.Equal(t, metadata.Get("turn_id").String(), embedded["turn_id"])
}

func TestOpenAICodexFingerprintPassthroughDefaultOffKeepsClientMetadata(t *testing.T) {
	// Given
	upstream := &task12HTTPUpstreamRecorder{}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{ID: 104, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "token"}}
	c, _ := newTask12OpenAITestContext(t, 7, "client-session")
	body := []byte(`{"model":"gpt-5","instructions":"ok","stream":false,"input":[],"client_metadata":{"session_id":"client-session","thread_id":"client-thread"}}`)

	// When
	_, err := svc.forwardOpenAIPassthrough(context.Background(), c, account, body, "gpt-5", nil, false, time.Now())

	// Then
	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Empty(t, upstream.requests[0].Header.Get("x-codex-installation-id"))
	require.Equal(t, "client-session", gjson.GetBytes(upstream.bodies[0], "client_metadata.session_id").String())
	require.Equal(t, "client-thread", gjson.GetBytes(upstream.bodies[0], "client_metadata.thread_id").String())
}

func TestOpenAICodexFingerprintRejectsDifferentStagedOAuthAccount(t *testing.T) {
	// Given
	accountA := &Account{ID: 105, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{"codex_fingerprint_mode": "session"}}
	accountB := &Account{ID: 106, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{"codex_fingerprint_mode": "session"}}
	ids := resolveCodexFingerprintIDs(accountA, "client-session", codexFingerprintSession)
	require.NotNil(t, ids)
	c, _ := newTask12OpenAITestContext(t, 7, "account-b-session")
	stageCodexFingerprintIDs(c, ids)
	headers := http.Header{"Session-Id": []string{"account-b-session"}}

	// When
	applyStagedCodexFingerprintHeaders(c, accountB, headers)

	// Then
	require.Empty(t, headers.Get("x-codex-installation-id"))
	require.Equal(t, "account-b-session", headers.Get("session-id"))
}
