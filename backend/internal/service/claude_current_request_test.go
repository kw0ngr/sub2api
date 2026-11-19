//go:build unit

package service

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestCurrentClaudePreservesSignedThinkingOnFirstAttempt(t *testing.T) {
	for _, model := range []string{"claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5", "claude-fable-5-2", "claude-mythos-5-2"} {
		body := []byte(`{"model":"` + model + `","thinking":{"type":"adaptive"},"output_config":{"effort":"xhigh"},"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"","signature":"upstream-signature"},{"type":"text","text":"progress"}]}]}`)
		require.Equal(t, body, FilterThinkingBlocksForModel(body, model), model)
		require.Equal(t, "xhigh", gjson.GetBytes(NormalizeClaudeOutputEffortInBody(body), "output_config.effort").String(), model)
		retry := gjson.ParseBytes(FilterThinkingBlocksForRetryModel(body, model))
		require.False(t, retry.Get("thinking").Exists(), model)
		require.JSONEq(t, `[{"type":"text","text":"progress"}]`, retry.Get("messages.0.content").Raw, model)
		require.Equal(t, "xhigh", retry.Get("output_config.effort").String(), model)
	}
}

func TestCurrentClaudeSignatureRetry(t *testing.T) {
	const model = "claude-sonnet-5-5"
	const user = `{"role":"user","content":"Reply with OK only."}`
	const thought = `{"type":"thinking","thinking":"Answer briefly.","signature":"ZmFrZS1zaWduYXR1cmU="}`
	const downgraded = `{"type":"text","text":"Answer briefly."}`
	const tool = `{"type":"tool_use","id":"toolu_signature_probe","name":"probe_result","input":{}}`
	const result = `{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_signature_probe","content":"OK"}]}`
	const tools = `"tools":[{"name":"probe_result","input_schema":{"type":"object","properties":{}}}],`
	const base = `{"model":"` + model + `","stream":true,"max_tokens":128,"output_config":{"effort":"low"},`
	const adaptive = base + `"thinking":{"type":"adaptive"},`
	const contextEdits = `"context_management":{"edits":[{"type":"clear_thinking_20251015"},{"type":"clear_tool_uses_20250919"}]},`
	const retainedEdits = `"context_management":{"edits":[{"type":"clear_tool_uses_20250919"}]},`
	const textHistory = `"messages":[` + user + `,{"role":"assistant","content":[%s,{"type":"text","text":"Earlier reply."}]},` + user + `]}`
	const toolHistory = `"messages":[` + user + `,{"role":"assistant","content":[%s,` + tool + `]},` + result + `]}`

	for _, tt := range []struct {
		name, body, want string
	}{
		{
			name: "ordinary multi-turn history",
			body: adaptive + contextEdits + strings.Replace(textHistory, "%s", thought, 1),
			want: base + retainedEdits + strings.Replace(textHistory, "%s", downgraded, 1),
		},
		{
			name: "tool continuation",
			body: adaptive + tools + strings.Replace(toolHistory, "%s", thought, 1),
			want: base + tools + strings.Replace(toolHistory, "%s", downgraded, 1),
		},
		{
			name: "redacted thinking leaves non-empty content",
			body: adaptive + `"messages":[` + user + `,{"role":"assistant","content":[{"type":"redacted_thinking","data":"opaque"}]},` + user + `]}`,
			want: base + `"messages":[` + user + `,{"role":"assistant","content":[{"type":"text","text":"(assistant content removed)"}]},` + user + `]}`,
		},
		{
			name: "no thinking stays byte-identical",
			body: base + `"messages": [ ` + user + ` ]}`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(tt.body)
			retry := FilterThinkingBlocksForRetryModel(body, model)
			stronger := FilterSignatureSensitiveBlocksForRetryModel(body, model)
			if tt.want == "" {
				require.Equal(t, body, retry)
				require.Equal(t, body, stronger)
				return
			}
			require.JSONEq(t, tt.want, string(retry))
			if tt.name == "tool continuation" {
				require.JSONEq(t, downgraded, gjson.GetBytes(stronger, "messages.1.content.0").Raw)
				require.Equal(t, "text", gjson.GetBytes(stronger, "messages.1.content.1.type").String())
				require.Equal(t, "text", gjson.GetBytes(stronger, "messages.2.content.0.type").String())
				require.False(t, gjson.GetBytes(stronger, "thinking").Exists())
			} else {
				require.JSONEq(t, tt.want, string(stronger))
			}
		})
	}
}

func TestCurrentClaudeAliasValidationReturns400WithoutUpstream(t *testing.T) {
	for _, route := range deepSeekCompatTestRoutes() {
		body := strings.ReplaceAll(route.body, "deepseek-v4-flash", "client-alias")
		body = strings.ReplaceAll(body, `"max"`, `"none"`)
		account := &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Credentials: map[string]any{
			"model_mapping": map[string]any{"client-alias": "claude-fable-5-2"},
		}}
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		result, err := route.forward(&GatewayService{}, context.Background(), c, account, []byte(body), &ParsedRequest{})
		require.Error(t, err)
		require.Nil(t, result)
		require.Equal(t, 400, rec.Code)
		require.Contains(t, rec.Body.String(), "reasoning effort")
	}
}
