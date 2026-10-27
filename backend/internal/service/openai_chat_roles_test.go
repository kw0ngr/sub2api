package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestStrictChatDeveloperRolesSelectedDestination(t *testing.T) {
	for _, tc := range []struct {
		platform, target, model, role string
	}{
		{PlatformDeepSeek, "https://relay.example.test/v1", "alias", "system"},
		{PlatformGLM, "https://relay.example.test/v1", "glm-5.3", "system"},
		{PlatformOpenAI, "https://api.deepseek.com/v1", "deepseek-flash", "system"},
		{PlatformOpenAI, "https://open.bigmodel.cn/api/coding/paas/v4", "glm-5.3", "system"},
		{PlatformOpenAI, "https://opencode.ai/zen/go/v1", "deepseek-v4.1-flash", "system"},
		{PlatformOpenAI, "https://opencode.ai/zen/go/v1", "opencode-go/DeepSeek-V4-Pro", "system"},
		{PlatformOpenAI, "https://opencode.ai/zen/v1", "gpt-6.1-sol", "developer"},
		{PlatformOpenAI, "https://opencode.ai.example.test/v1", "deepseek-flash", "developer"},
		{PlatformOpenAI, "https://relay.example.test/api.deepseek.com", "deepseek-flash", "developer"},
		{PlatformOpenAI, "https://api.openai.com/v1", "gpt-6.1-sol", "developer"},
		{PlatformGrok, "https://api.x.ai/v1", "grok-4.7", "developer"},
	} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%s/%s/stream=%t", tc.platform, tc.target, tc.model, stream), func(t *testing.T) {
				account := &Account{Platform: tc.platform, Type: AccountTypeAPIKey,
					Credentials: map[string]any{"base_url": "https://api.deepseek.com"}}
				body := []byte(fmt.Sprintf(`{"model":%q,"stream":%t,"messages":[{"role":"developer","content":"rules","custom":1234567890123456789},{"role":"user","content":"hi"}]}`, tc.model, stream))
				original := bytes.Clone(body)
				got, err := normalizeStrictChatDeveloperRoles(account, tc.target, body)
				require.NoError(t, err)
				require.Equal(t, tc.role, gjson.GetBytes(got, "messages.0.role").String())
				require.Equal(t, "1234567890123456789", gjson.GetBytes(got, "messages.0.custom").Raw)
				require.Equal(t, original, body)
				if tc.role == "developer" {
					require.Equal(t, original, got)
				}
			})
		}
	}
}

func TestStrictChatDeveloperRolesAppliedToOutboundRequest(t *testing.T) {
	account := &Account{Platform: PlatformDeepSeek, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"base_url": "https://api.deepseek.com"}}
	body := []byte(`{"model":"deepseek-flash","messages":[{"role":"developer","content":"rules"}]}`)
	req, err := (&OpenAIGatewayService{cfg: &config.Config{}}).buildUpstreamRequestOpenAICompatibleChatCompletions(context.Background(), nil, account, body, "test")
	require.NoError(t, err)
	actual, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, "system", gjson.GetBytes(actual, "messages.0.role").String())
	require.Equal(t, "developer", gjson.GetBytes(body, "messages.0.role").String())
}

func TestStrictChatDeveloperRolesRejectsMalformedMessages(t *testing.T) {
	account := &Account{Platform: PlatformDeepSeek, Type: AccountTypeAPIKey}
	for _, body := range []string{`{`, `{"messages":{}}`, `{"messages":[null]}`, `{"messages":[{"content":"hi"}]}`} {
		_, err := normalizeStrictChatDeveloperRoles(account, "https://api.deepseek.com", []byte(body))
		require.Error(t, err, body)
	}
}
