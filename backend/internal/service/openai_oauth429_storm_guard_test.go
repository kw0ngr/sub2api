package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShouldStopOpenAIOAuth429FailoverOnlyDuringStorm(t *testing.T) {
	svc := &OpenAIGatewayService{}
	oauthAccount := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	apiKeyAccount := &Account{ID: 43, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	require.False(t, svc.ShouldStopOpenAIOAuth429Failover(oauthAccount, http.StatusTooManyRequests, 1))

	for i := 0; i < openAIOAuth429StormThreshold; i++ {
		svc.RecordOpenAIOAuth429(oauthAccount, http.StatusTooManyRequests)
	}

	require.True(t, svc.ShouldStopOpenAIOAuth429Failover(oauthAccount, http.StatusTooManyRequests, 1))
	require.False(t, svc.ShouldStopOpenAIOAuth429Failover(apiKeyAccount, http.StatusTooManyRequests, 1))
	require.False(t, svc.ShouldStopOpenAIOAuth429Failover(oauthAccount, http.StatusInternalServerError, 1))
	require.False(t, svc.ShouldStopOpenAIOAuth429Failover(oauthAccount, http.StatusTooManyRequests, 0))
}
