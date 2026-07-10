package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClassifyAPIKeyStatusAction_GrokThirdPartyServerErrorIgnored(t *testing.T) {
	// Given
	thirdParty := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": "https://ai.muapi.cn",
		},
	}
	official := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": "https://api.x.ai/v1",
		},
	}
	body := []byte(`{"error":{"message":"upstream error: do request failed"}}`)

	// When
	thirdPartyAction := ClassifyAPIKeyStatusAction(thirdParty, http.StatusServiceUnavailable, body)
	officialAction := ClassifyAPIKeyStatusAction(official, http.StatusServiceUnavailable, body)

	// Then
	require.Equal(t, APIKeyStatusActionIgnore, thirdPartyAction)
	require.Equal(t, APIKeyStatusActionTemporaryCooldown, officialAction)
}
