package service

import (
	"context"
	"io"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGeminiFlatChatToolsOnlyNormalizedForGoogle(t *testing.T) {
	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	body := []byte(`{"model":"gemini-4-neon","tools":[{"type":"function","name":"lookup","description":"Lookup","parameters":{"type":"object","properties":{"value":{"const":1234567890123456789}}},"strict":false},{"type":"function","function":{"name":"existing"}}]}`)
	for _, platform := range []string{PlatformGemini, PlatformOpenAI, "relay"} {
		t.Run(platform, func(t *testing.T) {
			account := signatureTestAccount()
			switch platform {
			case PlatformOpenAI:
				account.Platform = PlatformOpenAI
			case "relay":
				account.Credentials["base_url"] = "https://relay.example/v1"
			}
			req, err := svc.buildUpstreamRequestOpenAICompatibleChatCompletions(context.Background(), signatureTestContext(33, 17), account, body, "test")
			require.NoError(t, err)
			out, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			if platform != PlatformGemini {
				require.Equal(t, body, out)
				return
			}
			require.False(t, gjson.GetBytes(out, "tools.0.name").Exists())
			require.JSONEq(t, `{"type":"function","function":{"name":"lookup","description":"Lookup","parameters":{"type":"object","properties":{"value":{"const":1234567890123456789}}},"strict":false}}`, gjson.GetBytes(out, "tools.0").Raw)
			require.Contains(t, string(out), "1234567890123456789")
			require.Equal(t, gjson.GetBytes(body, "tools.1").Raw, gjson.GetBytes(out, "tools.1").Raw)
		})
	}
}
