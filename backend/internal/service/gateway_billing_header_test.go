package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestSyncBillingHeaderVersion(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		userAgent string
		wantSub   string // substring expected in result
		unchanged bool   // expect body to remain the same
	}{
		{
			name:      "replaces cc_version preserving message-derived suffix",
			body:      `{"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.81.df2; cc_entrypoint=cli; cch=00000;"},{"type":"text","text":"You are Claude Code.","cache_control":{"type":"ephemeral"}}],"messages":[]}`,
			userAgent: "claude-cli/2.1.22 (external, cli)",
			wantSub:   "cc_version=2.1.22.df2",
		},
		{
			name:      "no billing header in system",
			body:      `{"system":[{"type":"text","text":"You are Claude Code."}],"messages":[]}`,
			userAgent: "claude-cli/2.1.22",
			unchanged: true,
		},
		{
			name:      "no system field",
			body:      `{"messages":[]}`,
			userAgent: "claude-cli/2.1.22",
			unchanged: true,
		},
		{
			name:      "user-agent without version",
			body:      `{"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.81; cc_entrypoint=cli; cch=00000;"}],"messages":[]}`,
			userAgent: "Mozilla/5.0",
			unchanged: true,
		},
		{
			name:      "empty user-agent",
			body:      `{"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.81; cc_entrypoint=cli; cch=00000;"}],"messages":[]}`,
			userAgent: "",
			unchanged: true,
		},
		{
			name:      "version already matches",
			body:      `{"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.22; cc_entrypoint=cli; cch=00000;"}],"messages":[]}`,
			userAgent: "claude-cli/2.1.22",
			unchanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := syncBillingHeaderVersion([]byte(tt.body), tt.userAgent)
			if tt.unchanged {
				assert.Equal(t, tt.body, string(result), "body should remain unchanged")
			} else {
				assert.Contains(t, string(result), tt.wantSub)
				// Ensure old semver is gone
				assert.NotContains(t, string(result), "cc_version=2.1.81")
			}
		})
	}
}

func TestSignBillingHeaderCCH(t *testing.T) {
	t.Run("strips stale cch from billing header", func(t *testing.T) {
		body := []byte(`{"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.63.a43; cc_entrypoint=cli; cch=00000;"}],"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)
		result := signBillingHeaderCCH(body)

		billingText := gjson.GetBytes(result, "system.0.text").String()
		require.Equal(t, "x-anthropic-billing-header: cc_version=2.1.63.a43; cc_entrypoint=cli;", billingText)
	})

	t.Run("existing signed cch is stripped", func(t *testing.T) {
		body := []byte(`{"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.63; cc_entrypoint=cli; cch=abcde;"}],"messages":[]}`)
		result := signBillingHeaderCCH(body)
		billingText := gjson.GetBytes(result, "system.0.text").String()
		require.Equal(t, "x-anthropic-billing-header: cc_version=2.1.63; cc_entrypoint=cli;", billingText)
	})

	t.Run("no billing header - body unchanged", func(t *testing.T) {
		body := []byte(`{"system":[{"type":"text","text":"You are Claude Code."}],"messages":[]}`)
		result := signBillingHeaderCCH(body)
		assert.Equal(t, string(body), string(result))
	})

	t.Run("cch=00000 in user content is not touched", func(t *testing.T) {
		body := []byte(`{"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.63; cc_entrypoint=cli; cch=00000;"}],"messages":[{"role":"user","content":[{"type":"text","text":"keep literal cch=00000 in this message"}]}]}`)
		result := signBillingHeaderCCH(body)

		billingText := gjson.GetBytes(result, "system.0.text").String()
		assert.NotContains(t, billingText, "cch=")

		userText := gjson.GetBytes(result, "messages.0.content.0.text").String()
		assert.Contains(t, userText, "cch=00000")
	})
}
