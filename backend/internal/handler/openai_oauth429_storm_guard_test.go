package handler

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestShouldStopOpenAIOAuth429FailoverAfterRecordingCurrentFailure(t *testing.T) {
	gateway := &service.OpenAIGatewayService{}
	account := &service.Account{ID: 42, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth}
	failoverErr := &service.UpstreamFailoverError{StatusCode: http.StatusTooManyRequests}

	for i := 0; i < 19; i++ {
		gateway.RecordOpenAIOAuth429(account, http.StatusTooManyRequests)
	}

	require.True(t, shouldStopOpenAIOAuth429Failover(gateway, account, failoverErr, 1))
}

func TestShouldStopOpenAIOAuth429FailoverDoesNotStopAPIKey(t *testing.T) {
	gateway := &service.OpenAIGatewayService{}
	account := &service.Account{ID: 43, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey}
	failoverErr := &service.UpstreamFailoverError{StatusCode: http.StatusTooManyRequests}

	for i := 0; i < 19; i++ {
		gateway.RecordOpenAIOAuth429(&service.Account{ID: 42, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth}, http.StatusTooManyRequests)
	}

	require.False(t, shouldStopOpenAIOAuth429Failover(gateway, account, failoverErr, 1))
}
