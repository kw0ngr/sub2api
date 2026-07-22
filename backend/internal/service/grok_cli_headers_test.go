package service

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestApplyGrokCLIRequestHeadersPreservesContextAndForcesMappedModel(t *testing.T) {
	t.Setenv(xai.EnvCLIClientVersion, "")
	src := http.Header{}
	src.Set("x-grok-client-version", "0.2.777")
	src.Set("x-grok-client-mode", "headless")
	src.Set("x-grok-req-id", "xai-client-request")
	src.Set("x-grok-model-override", "caller-must-not-win")

	dst := http.Header{}
	copyGrokCLIRequestContextHeaders(dst, src)
	applyGrokCLIRequestHeaders(dst, nil, "grok-4.5")

	require.Equal(t, "0.2.777", dst.Get("x-grok-client-version"))
	require.Equal(t, "headless", dst.Get("x-grok-client-mode"))
	require.Equal(t, "xai-client-request", dst.Get("x-grok-req-id"))
	require.Equal(t, "grok-4.5", dst.Get("x-grok-model-override"))
	require.Equal(t, "xai-grok-cli", dst.Get("x-xai-token-auth"))
	require.Equal(t, "authenticate-response", dst.Get("x-authenticateresponse"))
	require.Equal(t, "grok-shell", dst.Get("x-grok-client-identifier"))
	require.Equal(t, "0", dst.Get("x-grok-turn-idx"))
	require.NotEmpty(t, dst.Get("User-Agent"))

	convID := dst.Get("x-grok-conv-id")
	require.NotEmpty(t, convID)
	require.Equal(t, convID, dst.Get("x-grok-session-id"))
	_, err := uuid.Parse(convID)
	require.NoError(t, err)
	_, err = uuid.Parse(dst.Get("x-grok-agent-id"))
	require.NoError(t, err)
}

func TestApplyGrokCLIRequestHeadersAccountOverridesClientIdentity(t *testing.T) {
	headers := http.Header{}
	headers.Set("x-grok-client-version", "from-client")
	headers.Set("x-grok-client-mode", "headless")
	account := &Account{Credentials: map[string]any{
		"headers": map[string]any{
			"x-grok-client-version":    "account-version",
			"x-grok-client-identifier": "account-client",
			"x-grok-client-mode":       "interactive",
		},
	}}

	applyGrokCLIRequestHeaders(headers, account, "grok-4.5")

	require.Equal(t, "account-version", headers.Get("x-grok-client-version"))
	require.Equal(t, "account-client", headers.Get("x-grok-client-identifier"))
	require.Equal(t, "interactive", headers.Get("x-grok-client-mode"))
}
