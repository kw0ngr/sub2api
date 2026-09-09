package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

func TestOpenAIResponsesNativeRemoteCompactionV2_keepsBareResponsesPathAndRawBody(t *testing.T) {
	// Given: a Codex native v2 compaction signal on the bare /responses route.
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body := []byte(`{"model":"gpt-5.6-sol","stream":true,"store":true,"prompt_cache_key":"seed-native","input":[{"type":"compaction_trigger"},{"type":"message","role":"user","content":"compact me"}],"reasoning":{"effort":"max"}}`)

	// When: the handler classifies and normalizes compact requests.
	got, ok := (&OpenAIGatewayHandler{}).normalizeOpenAIResponsesCompactRequest(c, zap.NewNop(), body)
	if ok && isOpenAIRemoteCompactionV2Request(got) {
		service.MarkOpenAINativeCompactionV2(c)
	}

	// Then: native v2 remains raw /responses traffic instead of legacy /compact.
	require.True(t, ok)
	require.Equal(t, "/v1/responses", c.Request.URL.Path)
	require.True(t, service.IsOpenAINativeCompactionV2(c))
	require.True(t, gjson.GetBytes(got, "stream").Bool())
	require.True(t, gjson.GetBytes(got, "store").Bool())
	require.Equal(t, "seed-native", gjson.GetBytes(got, "prompt_cache_key").String())
	require.Equal(t, "compaction_trigger", gjson.GetBytes(got, "input.1.type").String())
	require.False(t, service.IsOpenAIResponsesCompactPathForTest(c))
}

func TestOpenAIResponsesBodySignalNonStreaming_rewritesToLegacyCompact(t *testing.T) {
	// Given: an older non-streaming body signal on the bare /responses route.
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body := []byte(`{"model":"gpt-5.6-sol","stream":false,"store":true,"prompt_cache_key":"seed-legacy","input":[{"type":"message","role":"user","content":"compact me"},{"type":"compaction_trigger"}],"reasoning":{"effort":"max"}}`)

	// When: the handler normalizes compact requests.
	got, ok := (&OpenAIGatewayHandler{}).normalizeOpenAIResponsesCompactRequest(c, zap.NewNop(), body)

	// Then: legacy traffic is still promoted to /responses/compact and stripped.
	require.True(t, ok)
	require.Equal(t, "/v1/responses/compact", c.Request.URL.Path)
	require.True(t, service.IsOpenAIResponsesCompactPathForTest(c))
	require.False(t, service.IsOpenAINativeCompactionV2(c))
	require.False(t, gjson.GetBytes(got, "stream").Exists())
	require.False(t, gjson.GetBytes(got, "store").Exists())
	require.False(t, gjson.GetBytes(got, "prompt_cache_key").Exists())
	seed, exists := c.Get(service.OpenAICompactSessionSeedKeyForTest())
	require.True(t, exists)
	require.Equal(t, "seed-legacy", seed)
}

func TestOpenAIResponsesBodySignalRouteGuard_onlyBareResponsesCanRewrite(t *testing.T) {
	// Given: a Responses subresource whose suffix used to look like /responses.
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/projects/responses", nil)
	body := []byte(`{"model":"gpt-5.6-sol","stream":false,"input":[{"type":"compaction_trigger"}]}`)

	// When: compact normalization runs.
	got, ok := (&OpenAIGatewayHandler{}).normalizeOpenAIResponsesCompactRequest(c, zap.NewNop(), body)

	// Then: non-route lookalikes are not rewritten.
	require.True(t, ok)
	require.Equal(t, body, got)
	require.Equal(t, "/v1/projects/responses", c.Request.URL.Path)
	require.False(t, isBareOpenAIResponsesPath(c))
}
