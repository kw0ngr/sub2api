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

func TestOpenAINativeRemoteCompactionV2BuildRequest_preservesResponsesEndpointAndAddsBetaFeature(t *testing.T) {
	// Given: an OAuth passthrough native v2 request already classified by the handler.
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.6-sol","stream":true,"store":true,"prompt_cache_key":"seed-native","input":[{"type":"message","role":"user","content":"compact me"},{"type":"compaction_trigger"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(string(body)))
	MarkOpenAINativeCompactionV2(c)
	account := &Account{ID: 61, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"chatgpt_account_id": "chatgpt-acc"}}

	// When: the passthrough upstream request is built.
	req, err := (&OpenAIGatewayService{}).buildUpstreamRequestOpenAIPassthrough(c.Request.Context(), c, account, body, "token")

	// Then: native v2 stays on raw /responses and advertises remote_compaction_v2.
	require.NoError(t, err)
	require.Equal(t, chatgptCodexURL, req.URL.String())
	require.Equal(t, "text/event-stream", req.Header.Get("Accept"))
	require.Empty(t, req.Header.Get("Version"))
	require.Contains(t, req.Header.Get("x-codex-beta-features"), "remote_compaction_v2")
}

func TestOpenAINativeRemoteCompactionV2Forward_preservesRawStreamingResponse(t *testing.T) {
	// Given: native v2 passthrough returns the Responses SSE wire format.
	gin.SetMode(gin.TestMode)
	upstreamBody := "data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"compaction\",\"content\":\"raw\"}}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":1,\"output_tokens\":2}}}\n\n"
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid-native"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 62, Name: "oauth-pass", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1, Credentials: map[string]any{"access_token": "t", "chatgpt_account_id": "chatgpt-acc"}, Extra: map[string]any{"openai_passthrough": true}, Status: StatusActive, Schedulable: true}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.6-sol","stream":true,"store":true,"prompt_cache_key":"seed-native","input":[{"type":"message","role":"user","content":"compact me"},{"type":"compaction_trigger"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(string(body)))
	MarkOpenAINativeCompactionV2(c)

	// When: the request is forwarded through passthrough mode.
	result, err := svc.Forward(context.Background(), c, account, body)

	// Then: the upstream request and downstream response both stay on native raw SSE.
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Stream)
	require.Equal(t, chatgptCodexURL, upstream.lastReq.URL.String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Equal(t, "seed-native", gjson.GetBytes(upstream.lastBody, "prompt_cache_key").String())
	require.Equal(t, "compaction_trigger", gjson.GetBytes(upstream.lastBody, "input.1.type").String())
	require.Contains(t, upstream.lastReq.Header.Get("x-codex-beta-features"), "remote_compaction_v2")
	require.Contains(t, rec.Body.String(), "response.output_item.done")
	require.Contains(t, rec.Body.String(), "\"type\":\"compaction\"")
	require.NotContains(t, rec.Body.String(), "cmp_")
}

func TestOpenAILegacyCompactSchedulerEligibility_skipsExplicitlyUnsupportedButNativeResponsesDoesNot(t *testing.T) {
	// Given: a legacy-unsupported account has confirmed Responses capability, while a backup is compact-capable.
	ctx := context.Background()
	groupID := int64(13013)
	legacyUnsupported := Account{ID: 13001, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10, Extra: map[string]any{"openai_responses_supported": true, "openai_compact_supported": false}}
	legacySupported := Account{ID: 13002, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0, Extra: map[string]any{"openai_responses_supported": true, "openai_compact_supported": true}}
	legacySvc := &OpenAIGatewayService{accountRepo: stubOpenAIAccountRepo{accounts: []Account{legacyUnsupported, legacySupported}}, cfg: &config.Config{}, concurrencyService: NewConcurrencyService(stubConcurrencyCache{})}
	nativeSvc := &OpenAIGatewayService{accountRepo: stubOpenAIAccountRepo{accounts: []Account{legacyUnsupported}}, cfg: &config.Config{}, concurrencyService: NewConcurrencyService(stubConcurrencyCache{})}

	// When: legacy compact requires compact eligibility, then native v2 only requires Responses.
	legacySelection, _, legacyErr := legacySvc.SelectAccountWithSchedulerForPlatformAndCapability(ctx, PlatformOpenAI, &groupID, "", "legacy-compact-session", "gpt-5.6-sol", nil, OpenAIUpstreamTransportAny, OpenAIEndpointCapabilityResponses, true)
	nativeSelection, _, nativeErr := nativeSvc.SelectAccountWithSchedulerForPlatformAndCapability(ctx, PlatformOpenAI, &groupID, "", "native-v2-session", "gpt-5.6-sol", nil, OpenAIUpstreamTransportAny, OpenAIEndpointCapabilityResponses, false)

	// Then: only legacy /responses/compact excludes the account with unsupported compact probe state.
	require.NoError(t, legacyErr)
	require.NotNil(t, legacySelection)
	require.Equal(t, int64(13002), legacySelection.Account.ID)
	require.NoError(t, nativeErr)
	require.NotNil(t, nativeSelection)
	require.Equal(t, int64(13001), nativeSelection.Account.ID)
}

func TestOpenAILegacyCompactReasoningForSol_downgradesMaxToXHigh(t *testing.T) {
	// Given: Sol compact still arrives at the legacy compact endpoint.
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses/compact", nil)
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	body := []byte(`{"model":"gpt-5.6-sol","input":"compact me","reasoning":{"effort":"max"}}`)

	// When: compact reasoning normalization runs.
	normalized, changed, err := normalizeOpenAICodexCompactReasoningEffortForAccount(c, account, body)

	// Then: Sol compact max remains downgraded to xhigh.
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "xhigh", gjson.GetBytes(normalized, "reasoning.effort").String())
}
