package service

import (
	"context"
	"encoding/json"
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

func TestOpenAIGatewayService_Forward_HTTPThinkingSignatureRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const signatureMessage = "The encrypted content abc could not be verified. Reason: Encrypted content could not be decrypted or parsed."
	for _, tc := range []struct {
		name        string
		code        string
		message     string
		status      int
		noEncrypted bool
		rejectRetry bool
		wantCalls   int
	}{
		{name: "recover", code: "thinking_signature_invalid", message: signatureMessage, status: 400, wantCalls: 2},
		{name: "retry only once", code: "thinking_signature_invalid", message: signatureMessage, status: 400, rejectRetry: true, wantCalls: 2},
		{name: "no encrypted reasoning", code: "thinking_signature_invalid", message: signatureMessage, status: 400, noEncrypted: true, wantCalls: 1},
		{name: "ordinary bad request", code: "invalid_request_error", message: signatureMessage, status: 400, wantCalls: 1},
		{name: "unrelated signature", code: "thinking_signature_invalid", message: "Invalid signature", status: 400, wantCalls: 1},
		{name: "missing encrypted content", code: "thinking_signature_invalid", message: "Signature could not be verified and could not be decrypted or parsed", status: 400, wantCalls: 1},
		{name: "missing verification failure", code: "thinking_signature_invalid", message: "The encrypted content could not be decrypted or parsed", status: 400, wantCalls: 1},
		{name: "missing decryption failure", code: "thinking_signature_invalid", message: "The encrypted content could not be verified", status: 400, wantCalls: 1},
		{name: "non bad request", code: "thinking_signature_invalid", message: signatureMessage, status: 422, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			errorBody, err := json.Marshal(map[string]any{"error": map[string]any{"code": tc.code, "message": tc.message}})
			require.NoError(t, err)
			response := func(status int, body string) *http.Response {
				return &http.Response{
					StatusCode: status,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(body)),
				}
			}
			next := response(http.StatusOK, `{"id":"resp_recovered","usage":{"input_tokens":1,"output_tokens":2}}`)
			if tc.rejectRetry {
				next = response(tc.status, string(errorBody))
			}
			upstream := &httpUpstreamSequenceRecorder{responses: []*http.Response{
				response(tc.status, string(errorBody)),
				next,
			}}
			cfg := &config.Config{}
			cfg.Security.URLAllowlist.Enabled = false
			svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}
			account := &Account{
				ID: 10, Name: "openai-apikey", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1,
				Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://upstream.example"},
				Extra:       map[string]any{"use_responses_api": true},
			}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
			SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)
			body := `{"model":"gpt-6.1-sol","stream":false,"reasoning":{"effort":"max"},"tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}],"input":[{"type":"reasoning","encrypted_content":"stale","summary":[{"type":"summary_text","text":"keep"}]},{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]},{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{}"},{"type":"function_call_output","call_id":"call_1","output":"result"}]}`
			if tc.noEncrypted {
				body = strings.Replace(body, `"encrypted_content":"stale",`, "", 1)
			}
			result, err := svc.Forward(context.Background(), c, account, []byte(body))
			require.Equal(t, tc.wantCalls, upstream.callCount)
			require.Len(t, upstream.bodies, tc.wantCalls)
			if tc.wantCalls == 2 && !tc.rejectRetry {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, http.StatusOK, rec.Code)
				require.Equal(t, "resp_recovered", gjson.Get(rec.Body.String(), "id").String())
			} else {
				require.Error(t, err)
			}
			if tc.wantCalls != 2 {
				return
			}
			require.Equal(t, "stale", gjson.GetBytes(upstream.bodies[0], "input.0.encrypted_content").String())
			require.False(t, gjson.GetBytes(upstream.bodies[1], "input.0.encrypted_content").Exists())
			require.Equal(t, "keep", gjson.GetBytes(upstream.bodies[1], "input.0.summary.0.text").String())
			for _, field := range []string{"model", "reasoning", "tools", "input.1", "input.2", "input.3"} {
				require.JSONEq(t, gjson.GetBytes(upstream.bodies[0], field).Raw, gjson.GetBytes(upstream.bodies[1], field).Raw, field)
			}
			require.Equal(t, "max", gjson.GetBytes(upstream.bodies[1], "reasoning.effort").String())
		})
	}
}
