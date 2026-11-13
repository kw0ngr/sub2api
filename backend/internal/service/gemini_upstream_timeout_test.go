package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type geminiTimeoutUpstream struct {
	HTTPUpstream
	do func(*http.Request) (*http.Response, error)
}

func (stub *geminiTimeoutUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return stub.do(req)
}

func TestGeminiHeaderTimeoutIsBoundedAndModelOnlyBackoff(t *testing.T) {
	account := signatureTestAccount()
	repo := &grokQuotaAccountRepoStub{account: account}
	upstream := &geminiTimeoutUpstream{do: func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	}}
	req := httptest.NewRequest(http.MethodPost, "https://example.test", nil)
	start := time.Now()
	_, err := doGeminiWithHeaderTimeout(upstream, req, "", account, 15*time.Millisecond)
	require.ErrorIs(t, err, errGeminiHeaderTimeout)
	require.Less(t, time.Since(start), time.Second)
	svc := &OpenAIGatewayService{accountRepo: repo}
	var failover *UpstreamFailoverError
	require.ErrorAs(t, svc.handleOpenAIUpstreamTransportError(req.Context(), nil, account, err, false, "gemini-3.8-flash"), &failover)
	require.Equal(t, http.StatusGatewayTimeout, failover.StatusCode)
	require.Equal(t, "gemini-3.8-flash", repo.modelRateLimitKey)
	require.WithinDuration(t, time.Now().Add(5*time.Minute), *repo.modelRateLimitAt, time.Second)
	require.Zero(t, repo.setErrorCalls)
	require.Zero(t, repo.setSchedulableN)
	require.Nil(t, repo.tempUnschedUntil)
}

func TestGeminiHeaderDeadlineDoesNotCancelHealthyStream(t *testing.T) {
	var upstreamContext context.Context
	upstream := &geminiTimeoutUpstream{do: func(req *http.Request) (*http.Response, error) {
		upstreamContext = req.Context()
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("stream-data"))}, nil
	}}
	req := httptest.NewRequest(http.MethodPost, "https://example.test", nil)
	resp, err := doGeminiWithHeaderTimeout(upstream, req, "", signatureTestAccount(), 10*time.Millisecond)
	require.NoError(t, err)
	time.Sleep(25 * time.Millisecond)
	require.NoError(t, upstreamContext.Err(), "headers are received: body generation can exceed the header deadline")
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "stream-data", string(body))
	require.NoError(t, resp.Body.Close())
	require.ErrorIs(t, upstreamContext.Err(), context.Canceled)
}

func TestGeminiClientCancellationDoesNotPenalizeAccount(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	upstream := &geminiTimeoutUpstream{do: func(req *http.Request) (*http.Response, error) {
		cancel()
		<-req.Context().Done()
		return nil, req.Context().Err()
	}}
	req := httptest.NewRequest(http.MethodPost, "https://example.test", nil).WithContext(ctx)
	account := signatureTestAccount()
	_, err := doGeminiWithHeaderTimeout(upstream, req, "", account, time.Second)
	require.ErrorIs(t, err, context.Canceled)
	repo := &grokQuotaAccountRepoStub{account: account}
	err = (&OpenAIGatewayService{accountRepo: repo}).handleOpenAIUpstreamTransportError(ctx, nil, account, err, false, "gemini-3.8-flash")
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, repo.modelRateLimitN)
	require.Zero(t, repo.setErrorCalls)
	require.Nil(t, repo.tempUnschedUntil)
}

func TestGeminiCumulativeWaitBudgetStopsWithoutNewNetworkAttempt(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(geminiHeaderBudgetKey, time.Now().Add(-geminiHeaderWaitBudget-time.Second))
	calls := 0
	upstream := &geminiTimeoutUpstream{do: func(req *http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("must not send")
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	req := httptest.NewRequest(http.MethodPost, "https://example.test", nil)
	_, err := svc.doOpenAICompatibleUpstream(c, req, "", signatureTestAccount())
	require.ErrorIs(t, err, errGeminiWaitBudget)
	require.Zero(t, calls)
	account := signatureTestAccount()
	account.Platform = PlatformOpenAI
	_, err = svc.doOpenAICompatibleUpstream(c, req, "", account)
	require.EqualError(t, err, "must not send")
	require.Equal(t, 1, calls, "other providers retain their existing timeout behavior")
}

func TestGeminiHighThinkingGetsLongerButBoundedHeaderWait(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	wait, budget := geminiHeaderLimits(c)
	require.Equal(t, 60*time.Second, wait)
	require.Equal(t, 90*time.Second, budget)
	for _, body := range []string{
		`{"reasoning_effort":"high"}`,
		`{"reasoning_effort":"xhigh"}`,
		`{"extra_body":{"google":{"thinking_config":{"thinking_level":"high"}}}}`,
	} {
		c.Set(OpsUpstreamRequestBodyKey, []byte(body))
		wait, budget = geminiHeaderLimits(c)
		require.Equal(t, 90*time.Second, wait)
		require.Equal(t, 120*time.Second, budget)
	}
}
