package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestHasCompactionTriggerInInput_detects_body_signal(t *testing.T) {
	require.True(t, HasCompactionTriggerInInput([]byte(`{"input":[{"type":"message"},{"type":"compaction_trigger"}]}`)))
	require.False(t, HasCompactionTriggerInInput([]byte(`{"input":"compaction_trigger"}`)))
}

func TestBuildOpenAICompactSSEPayload_emits_items_and_completed(t *testing.T) {
	body := []byte(`{
		"id":"resp_compact",
		"output":[{"id":"item_1","type":"compaction","content":"summary"}],
		"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}
	}`)

	got, ok := buildOpenAICompactSSEPayload(body)

	require.True(t, ok)
	text := string(got)
	require.Contains(t, text, "event: response.output_item.done\n")
	require.Contains(t, text, "event: response.completed\n")
	require.Equal(t, "compaction", gjson.Get(compactSSEEventData(t, text, "response.completed"), "response.output.0.type").String())
}

func TestBuildOpenAICompactSSEPayload_injects_id_and_drops_malformed_usage(t *testing.T) {
	body := []byte(`{"output":[{"type":"compaction"}],"usage":{"input_tokens":"bad"}}`)

	got, ok := buildOpenAICompactSSEPayload(body)

	require.True(t, ok)
	completed := gjson.Get(compactSSEEventData(t, string(got), "response.completed"), "response")
	require.True(t, completed.Get("id").Exists())
	require.False(t, completed.Get("usage").Exists())
}

func compactSSEEventData(t *testing.T, body string, eventType string) string {
	t.Helper()
	blocks := strings.Split(body, "\n\n")
	for _, block := range blocks {
		if !strings.Contains(block, "event: "+eventType+"\n") {
			continue
		}
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(line, "data: ") {
				return strings.TrimPrefix(line, "data: ")
			}
		}
	}
	t.Fatalf("event %s not found", eventType)
	return ""
}

func TestWriteOpenAICompactSSEBridge_requires_mark_and_success_status(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"id":"resp_compact","output":[{"type":"compaction"}]}`)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	require.False(t, writeOpenAICompactSSEBridge(c, http.StatusOK, body))

	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	MarkOpenAICompactClientStream(c)
	require.False(t, writeOpenAICompactSSEBridge(c, http.StatusBadGateway, body))

	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	MarkOpenAICompactClientStream(c)
	require.True(t, writeOpenAICompactSSEBridge(c, http.StatusOK, body))
	require.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	require.Contains(t, rec.Body.String(), "event: response.completed")
}

func TestWriteOpenAICompactSSEBridge_committed_keepalive_writes_failed_event(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)
	MarkOpenAICompactClientStream(c)
	k := &openAICompactSSEKeepalive{writer: c.Writer, stop: make(chan struct{})}
	c.Set(openAICompactSSEKeepaliveKey, k)
	c.Writer = &openAICompactKeepaliveWriter{ResponseWriter: c.Writer, k: k}
	require.True(t, k.beat())

	ok := writeOpenAICompactSSEBridge(c, http.StatusBadGateway, []byte(`{"error":{"message":"upstream busy"}}`))

	require.True(t, ok)
	require.Contains(t, rec.Body.String(), "event: response.failed")
	require.Contains(t, rec.Body.String(), "upstream busy")
}

func TestOpenAICompactKeepaliveAdjustedWrittenSize_ignores_heartbeat_bytes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)
	MarkOpenAICompactClientStream(c)
	k := &openAICompactSSEKeepalive{writer: c.Writer, stop: make(chan struct{})}
	c.Set(openAICompactSSEKeepaliveKey, k)
	c.Writer = &openAICompactKeepaliveWriter{ResponseWriter: c.Writer, k: k}
	require.True(t, k.beat())

	require.Equal(t, -1, OpenAICompactKeepaliveAdjustedWrittenSize(c))
}

func TestReconstructResponseOutputFromSSE_prefers_raw_done_items(t *testing.T) {
	bodyText := strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"hi"}`,
		`data: {"type":"response.output_item.done","output_index":0,"item":{"id":"msg_1","type":"message","content":[{"type":"output_text","text":"hi"}],"opaque":{"kept":true}}}`,
		`data: {"type":"response.completed","response":{"id":"resp_1","output":[]}}`,
	}, "\n")

	outputJSON, ok := reconstructResponseOutputFromSSE(bodyText)

	require.True(t, ok)
	items := gjson.ParseBytes(outputJSON).Array()
	require.Len(t, items, 1)
	require.Equal(t, "msg_1", items[0].Get("id").String())
	require.True(t, items[0].Get("opaque.kept").Bool())
}

func TestSupplementCompactionItemFromSSE_adds_missing_compaction_for_compact(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)
	bodyText := `data: {"type":"response.output_item.done","item":{"id":"cmp_1","type":"compaction","encrypted_content":"x"}}`
	finalResponse := []byte(`{"id":"resp_1","output":[{"id":"msg_1","type":"message"}]}`)

	patched := supplementCompactionItemFromSSE(c, finalResponse, bodyText)

	require.Len(t, gjson.GetBytes(patched, "output").Array(), 2)
	require.Equal(t, "compaction", gjson.GetBytes(patched, "output.1.type").String())
	require.Equal(t, "x", gjson.GetBytes(patched, "output.1.encrypted_content").String())
}
