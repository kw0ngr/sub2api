//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestForwardAsChatCompletions_FakeRateLimitMarksAccountRateLimited(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &rateLimitAccountRepoStub{}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"deepseek-v4.1-flash","messages":[{"role":"user","content":"hello"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(openAICompatibleFakeRateLimitJSON)),
	}}
	cfg := &config.Config{}
	svc := &OpenAIGatewayService{
		httpUpstream:     upstream,
		cfg:              cfg,
		rateLimitService: NewRateLimitService(repo, nil, cfg, nil, nil),
	}
	account := &Account{
		ID:          4578,
		Platform:    PlatformDeepSeek,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-ds-test", "base_url": "https://codexfree.xiaotuxp.com/v1"},
	}

	before := time.Now()
	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
	after := time.Now()

	require.Nil(t, result)
	var failover *UpstreamFailoverError
	require.ErrorAs(t, err, &failover)
	require.Equal(t, http.StatusTooManyRequests, failover.StatusCode)
	require.Equal(t, 0, repo.setErrorCalls)
	require.Equal(t, 1, repo.rateLimitedCalls)
	require.Equal(t, 0, repo.tempCalls)
	require.NotNil(t, repo.lastRateLimitResetAt)
	require.WithinDuration(t, before.Add(apiKey429Cooldown), *repo.lastRateLimitResetAt, after.Sub(before)+time.Second)
	require.False(t, c.Writer.Written())
}
