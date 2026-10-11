package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestLocalQueueFailureDoesNotCreateProviderErrorContext(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, EndpointChatCompletions, nil)
	cache := &concurrencyCacheMock{
		acquireAccountSlotFn:   func(context.Context, int64, int, string) (bool, error) { return false, nil },
		incrementAccountWaitFn: func(context.Context, int64, int) (bool, error) { return false, nil },
	}
	h := &OpenAIGatewayHandler{concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second)}
	started := false
	_, _, failover := h.acquireResponsesAccountSlot(c, nil, "", &service.AccountSelectionResult{
		Account:  &service.Account{ID: 2159},
		WaitPlan: &service.AccountWaitPlan{MaxConcurrency: 2, MaxWaiting: 3},
	}, false, &started, zap.NewNop())
	require.NotNil(t, failover)
	h.handleFailoverExhausted(c, failover, false)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	_, hasUpstream := c.Get(service.OpsUpstreamStatusCodeKey)
	require.False(t, hasUpstream, "no upstream request was made")
	require.Equal(t, "2", rec.Header().Get("Retry-After"))
	require.Contains(t, rec.Body.String(), "Gateway account")
	phase := classifyOpsPhase("rate_limit_error", "Gateway account concurrency queue is full; retry later", "")
	require.Equal(t, "routing", phase)
	require.Equal(t, "gateway", classifyOpsErrorSource(phase, ""))
	require.Equal(t, "platform", classifyOpsErrorOwner(phase, ""))
}

func TestGLMResourceFailureDoesNotBecomeTransientRetryable429(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, EndpointChatCompletions, nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{Group: &service.Group{Platform: service.PlatformGLM}})
	(&OpenAIGatewayHandler{}).handleFailoverExhausted(c, &service.UpstreamFailoverError{
		StatusCode: 429, ResponseBody: []byte(`{"error":{"code":"1113","message":"余额不足或无可用资源包"}}`),
	}, false)
	require.Equal(t, http.StatusPaymentRequired, rec.Code)
	require.Contains(t, rec.Body.String(), "billing_error")
	require.Contains(t, rec.Body.String(), "resources")
	upstream, exists := c.Get(service.OpsUpstreamStatusCodeKey)
	require.True(t, exists)
	require.Equal(t, 429, upstream)
}
