package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

var task12GinModeOnce sync.Once

func newTask12OpenAITestContext(t *testing.T, apiKeyID int64, sessionID string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	task12GinModeOnce.Do(func() { gin.SetMode(gin.TestMode) })
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader([]byte(`{"model":"gpt-5","input":[]}`)))
	if sessionID != "" {
		c.Request.Header.Set("session_id", sessionID)
		c.Request.Header.Set("session-id", sessionID)
	}
	if apiKeyID > 0 {
		c.Set("api_key", &APIKey{ID: apiKeyID})
	}
	return c, rec
}

func task12OpenAISuccessResponse(state string) *http.Response {
	header := http.Header{"Content-Type": []string{"application/json"}}
	if state != "" {
		header.Set("x-codex-turn-state", state)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp_task12","usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}`)),
	}
}

func task12OpenAIStreamSuccessResponse() *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"response.output_text.delta","delta":"ok"}`,
			`data: {"type":"response.completed","response":{"id":"resp_task12_stream","usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}}`,
			`data: [DONE]`,
		}, "\n\n"))),
	}
}

func task12RecordTurnState(t *testing.T, svc *OpenAIGatewayService, apiKeyID int64, sessionID string, account *Account, state string) {
	t.Helper()
	c, _ := newTask12OpenAITestContext(t, apiKeyID, sessionID)
	usage, err := svc.handleNonStreamingResponse(context.Background(), task12OpenAISuccessResponse(state), c, account, "gpt-5", "gpt-5")
	require.NoError(t, err)
	require.NotNil(t, usage)
}

func task12BuildOpenAIRequest(t *testing.T, svc *OpenAIGatewayService, apiKeyID int64, sessionID string, account *Account, state string) *http.Request {
	t.Helper()
	c, _ := newTask12OpenAITestContext(t, apiKeyID, sessionID)
	if state != "" {
		c.Request.Header.Set("x-codex-turn-state", state)
	}
	req, err := svc.buildUpstreamRequest(context.Background(), c, account, []byte(`{"model":"gpt-5","input":[]}`), "token", false, sessionID, true)
	require.NoError(t, err)
	return req
}

func task12OriginLen(t *testing.T, svc *OpenAIGatewayService) int {
	t.Helper()
	field := reflect.ValueOf(svc).Elem().FieldByName("openaiCodexTurnStateOrigins")
	require.True(t, field.IsValid(), "OpenAIGatewayService must own a bounded turn-state provenance map")
	if field.Kind() == reflect.Map && field.IsNil() {
		return 0
	}
	require.Equal(t, reflect.Map, field.Kind(), "turn-state provenance must be a bounded map")
	return field.Len()
}

func TestOpenAICodexTurnStateRelaysResponseAndPreservesSameAccountEcho(t *testing.T) {
	// Given
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"chatgpt_account_id": "acct-A"}}
	c, rec := newTask12OpenAITestContext(t, 7, "sess-relay")

	// When
	usage, err := svc.handleNonStreamingResponse(context.Background(), task12OpenAISuccessResponse("blob-A"), c, account, "gpt-5", "gpt-5")

	// Then
	require.NoError(t, err)
	require.NotNil(t, usage)
	require.Equal(t, "blob-A", rec.Header().Get("X-Codex-Turn-State"))

	req := task12BuildOpenAIRequest(t, svc, 7, "sess-relay", account, "blob-A")
	require.Equal(t, "blob-A", req.Header.Get("x-codex-turn-state"))
}

func TestOpenAICodexTurnStateStripsCrossAccountEcho(t *testing.T) {
	// Given
	svc := &OpenAIGatewayService{}
	task12RecordTurnState(t, svc, 7, "sess-cross", &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth}, "blob-A")

	// When
	req := task12BuildOpenAIRequest(t, svc, 7, "sess-cross", &Account{ID: 43, Platform: PlatformOpenAI, Type: AccountTypeOAuth}, "blob-A")

	// Then
	require.Empty(t, req.Header.Get("x-codex-turn-state"))
}

func TestOpenAICodexTurnStateWSHeadersStripKnownCrossAccountEcho(t *testing.T) {
	// Given
	svc := &OpenAIGatewayService{}
	accountA := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"chatgpt_account_id": "acct-A"}}
	accountB := &Account{ID: 43, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"chatgpt_account_id": "acct-B"}}
	task12RecordTurnState(t, svc, 7, "sess-ws-cross", accountA, "blob-A")
	c, _ := newTask12OpenAITestContext(t, 7, "sess-ws-cross")
	c.Request.Header.Set(openAIWSTurnStateHeader, "blob-A")

	// When
	headers, _ := svc.buildOpenAIWSHeaders(c, accountB, "token", OpenAIWSProtocolDecision{}, true, c.GetHeader(openAIWSTurnStateHeader), "", "")

	// Then
	require.Empty(t, headers.Get(openAIWSTurnStateHeader))
}

func TestOpenAICodexTurnStateStripsDifferentOAuthIdentityForSameAccount(t *testing.T) {
	// Given
	svc := &OpenAIGatewayService{}
	task12RecordTurnState(t, svc, 7, "sess-identity", &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"chatgpt_account_id": "acct-A"}}, "blob-A")

	// When
	req := task12BuildOpenAIRequest(t, svc, 7, "sess-identity", &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"chatgpt_account_id": "acct-B"}}, "blob-A")

	// Then
	require.Empty(t, req.Header.Get("x-codex-turn-state"))
}

func TestOpenAICodexTurnStatePreservesUnknownSessionEcho(t *testing.T) {
	// Given
	svc := &OpenAIGatewayService{}
	task12RecordTurnState(t, svc, 7, "sess-known", &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth}, "blob-A")

	// When
	req := task12BuildOpenAIRequest(t, svc, 7, "sess-other", &Account{ID: 43, Platform: PlatformOpenAI, Type: AccountTypeOAuth}, "blob-A")

	// Then
	require.Equal(t, "blob-A", req.Header.Get("x-codex-turn-state"))
}

func TestOpenAICodexTurnStatePassthroughStripsCrossAccountEcho(t *testing.T) {
	// Given
	svc := &OpenAIGatewayService{}
	task12RecordTurnState(t, svc, 7, "sess-pass", &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth}, "blob-A")
	c, _ := newTask12OpenAITestContext(t, 7, "sess-pass")
	c.Request.Header.Set("x-codex-turn-state", "blob-A")

	// When
	req, err := svc.buildUpstreamRequestOpenAIPassthrough(context.Background(), c, &Account{ID: 43, Platform: PlatformOpenAI, Type: AccountTypeOAuth}, []byte(`{"model":"gpt-5","instructions":"ok","input":[]}`), "token")

	// Then
	require.NoError(t, err)
	require.Empty(t, req.Header.Get("x-codex-turn-state"))
}

func TestOpenAICodexTurnStatePassthroughResponseHeadersRelayAndClear(t *testing.T) {
	// Given
	dst := http.Header{}
	src := http.Header{}
	src.Set("x-codex-turn-state", "blob-P")

	// When
	writeOpenAIPassthroughResponseHeaders(dst, src, nil)

	// Then
	require.Equal(t, "blob-P", dst.Get("X-Codex-Turn-State"))

	// When
	writeOpenAIPassthroughResponseHeaders(dst, http.Header{"Content-Type": []string{"application/json"}}, nil)

	// Then
	require.Empty(t, dst.Get("X-Codex-Turn-State"))
}

func TestOpenAICodexTurnStateOriginMapIsBounded(t *testing.T) {
	// Given
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	// When
	for i := 0; i < 1100; i++ {
		task12RecordTurnState(t, svc, 7, fmt.Sprintf("sess-bound-%d", i), account, fmt.Sprintf("blob-%d", i))
	}

	// Then
	require.LessOrEqual(t, task12OriginLen(t, svc), 1024)
}

func TestOpenAICodexTurnStateSweepUsesSessionStickyTTL(t *testing.T) {
	// Given
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{OpenAIWS: config.GatewayOpenAIWSConfig{StickySessionTTLSeconds: 1}}}}
	account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	for i := 0; i < 4; i++ {
		task12RecordTurnState(t, svc, 7, fmt.Sprintf("sess-expired-%d", i), account, "blob-old")
	}
	time.Sleep(1100 * time.Millisecond)

	// When
	for i := 0; i < 252; i++ {
		task12RecordTurnState(t, svc, 7, fmt.Sprintf("sess-fresh-%d", i), account, "blob-new")
	}

	// Then
	require.LessOrEqual(t, task12OriginLen(t, svc), 252)
}

func TestOpenAICodexTurnStateConcurrentRecordAndGuardRace(t *testing.T) {
	// Given
	svc := &OpenAIGatewayService{}
	origin := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	foreign := &Account{ID: 43, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	var wg sync.WaitGroup

	// When
	for i := 0; i < 64; i++ {
		sessionID := fmt.Sprintf("sess-race-%d", i)
		wg.Add(2)
		go func() {
			defer wg.Done()
			task12RecordTurnState(t, svc, 7, sessionID, origin, "blob-race")
		}()
		go func() {
			defer wg.Done()
			req := task12BuildOpenAIRequest(t, svc, 7, sessionID, foreign, "blob-race")
			got := req.Header.Get("x-codex-turn-state")
			require.True(t, got == "" || got == "blob-race")
		}()
	}
	wg.Wait()

	// Then
	require.LessOrEqual(t, task12OriginLen(t, svc), 1024)
}

type task12HTTPUpstreamRecorder struct {
	requests []*http.Request
	bodies   [][]byte
}

func (u *task12HTTPUpstreamRecorder) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	u.requests = append(u.requests, req)
	u.bodies = append(u.bodies, body)
	return task12OpenAIStreamSuccessResponse(), nil
}

func (u *task12HTTPUpstreamRecorder) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}
