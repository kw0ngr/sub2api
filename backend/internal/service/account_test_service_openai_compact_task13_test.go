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

type task13CompactAccountTestRepo struct {
	snapshotUpdateAccountRepo
}

func (r *task13CompactAccountTestRepo) SetError(context.Context, int64, string) error {
	return nil
}

func (r *task13CompactAccountTestRepo) SetSchedulable(context.Context, int64, bool) error {
	return nil
}

type task13CompactAccountTestFixture struct {
	service  *AccountTestService
	account  *Account
	context  *gin.Context
	recorder *httptest.ResponseRecorder
	upstream *httpUpstreamRecorder
	updates  <-chan map[string]any
}

func newTask13CompactAccountTestFixture(status int, body string) *task13CompactAccountTestFixture {
	account := Account{
		ID:          13013,
		Name:        "task13-compact-probe",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "t",
			"chatgpt_account_id": "task13-account",
		},
	}
	updateCalls := make(chan map[string]any, 1)
	repo := &task13CompactAccountTestRepo{snapshotUpdateAccountRepo: snapshotUpdateAccountRepo{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{account}},
		updateExtraCalls:      updateCalls,
	}}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}}
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/13013/test", nil)

	return &task13CompactAccountTestFixture{
		service:  &AccountTestService{accountRepo: repo, httpUpstream: upstream, cfg: &config.Config{}},
		account:  &account,
		context:  context,
		recorder: recorder,
		upstream: upstream,
		updates:  updateCalls,
	}
}

func (f *task13CompactAccountTestFixture) run() error {
	return f.service.TestAccountConnection(f.context, f.account.ID, "gpt-5.6-sol", "", AccountTestModeCompact)
}

func (f *task13CompactAccountTestFixture) requireExtraUpdate(t *testing.T) map[string]any {
	t.Helper()
	select {
	case updates := <-f.updates:
		return updates
	default:
		require.FailNow(t, "expected compact probe UpdateExtra call")
		return nil
	}
}

func TestAccountTestService_CompactProbeNativeV2SendsPayloadHeaderAndPersistsSupported(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	body := "data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"compaction\",\"id\":\"cmp_probe\"}}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"output\":[]}}\n\n"
	fixture := newTask13CompactAccountTestFixture(http.StatusOK, body)

	// When
	err := fixture.run()

	// Then
	require.NotNil(t, fixture.upstream.lastReq)
	require.Equal(t, chatgptCodexAPIURL, fixture.upstream.lastReq.URL.String())
	require.Equal(t, "text/event-stream", fixture.upstream.lastReq.Header.Get("Accept"))
	require.Contains(t, fixture.upstream.lastReq.Header.Get("x-codex-beta-features"), "remote_compaction_v2")
	require.NotEmpty(t, fixture.upstream.lastReq.Header.Get("session_id"))
	require.Equal(t, "gpt-5.6-sol", gjson.GetBytes(fixture.upstream.lastBody, "model").String())
	require.True(t, gjson.GetBytes(fixture.upstream.lastBody, "stream").Bool())
	require.False(t, gjson.GetBytes(fixture.upstream.lastBody, "store").Bool())
	require.Equal(t, "compaction_trigger", gjson.GetBytes(fixture.upstream.lastBody, "input.#(type==\"compaction_trigger\").type").String())
	require.NoError(t, err)
	updates := fixture.requireExtraUpdate(t)
	require.Equal(t, true, updates["openai_compact_supported"])
	require.Equal(t, http.StatusOK, updates["openai_compact_last_status"])
	require.Contains(t, fixture.recorder.Body.String(), "test_complete")
	t.Log("ACCOUNT_TEST_NATIVE_V2_PAYLOAD_HEADER_OK")
}

func TestAccountTestService_CompactProbe2xxWithoutItemPersistsUnsupported(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	body := "data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"message\",\"id\":\"msg_probe\"}}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"output\":[]}}\n\n"
	fixture := newTask13CompactAccountTestFixture(http.StatusOK, body)

	// When
	err := fixture.run()

	// Then
	require.Error(t, err)
	updates := fixture.requireExtraUpdate(t)
	require.Equal(t, false, updates["openai_compact_supported"])
	require.Equal(t, http.StatusOK, updates["openai_compact_last_status"])
	require.Contains(t, updates["openai_compact_last_error"], "without a compaction output item")
	t.Log("ACCOUNT_TEST_COMPACT_PROBE_FALSE_UPDATE_OK")
}

func TestAccountTestService_CompactProbe502LeavesSupportInconclusive(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	fixture := newTask13CompactAccountTestFixture(http.StatusBadGateway, `{"error":{"message":"temporary upstream failure"}}`)

	// When
	err := fixture.run()

	// Then
	require.Error(t, err)
	updates := fixture.requireExtraUpdate(t)
	require.Equal(t, http.StatusBadGateway, updates["openai_compact_last_status"])
	require.Contains(t, updates["openai_compact_last_error"], "temporary upstream failure")
	_, wroteUnsupported := updates["openai_compact_supported"]
	require.False(t, wroteUnsupported)
	t.Log("ACCOUNT_TEST_COMPACT_PROBE_INCONCLUSIVE_NO_UNSUPPORTED_WRITE_OK")
}
