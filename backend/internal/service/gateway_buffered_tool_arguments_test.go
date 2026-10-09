package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBufferedChatToolArgumentsDiscardAnthropicStartPlaceholder(t *testing.T) {
	stream := `event: message_start
data: {"type":"message_start","message":{"id":"msg_args","model":"deepseek-flash","content":[]}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"call_args","name":"release_probe","input":{}}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"value\":"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"OK\"}"}}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":1}}

event: message_stop
data: {"type":"message_stop"}

`
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	_, err := (&GatewayService{}).handleCCBufferedFromAnthropic(
		&http.Response{Body: io.NopCloser(strings.NewReader(stream))}, ctx,
		"deepseek-flash", "deepseek-flash", nil, time.Now(),
	)
	require.NoError(t, err)
	var out struct {
		Choices []struct {
			Message struct {
				ToolCalls []struct {
					Function struct {
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Choices, 1)
	require.Len(t, out.Choices[0].Message.ToolCalls, 1)
	require.JSONEq(t, `{"value":"OK"}`, out.Choices[0].Message.ToolCalls[0].Function.Arguments)
	require.Equal(t, json.RawMessage(`{}`), appendRawJSON(json.RawMessage(`{}`), ""))
	require.Equal(t, json.RawMessage(`{}`), appendRawJSON(json.RawMessage(`null`), "{}"))
}
