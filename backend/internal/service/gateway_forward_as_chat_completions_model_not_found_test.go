//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGatewayService_ForwardAsChatCompletions_DeepSeek404FailsOverMappedModelOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"deepseek-alias","messages":[{"role":"user","content":"hi"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))

	upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusNotFound,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":{"message":"model: deepseek-v4-pro","type":"server_error"}}`))),
	}}
	repo := &rateLimitAccountRepoStub{}
	svc := &GatewayService{
		cfg:                 &config.Config{},
		httpUpstream:        upstream,
		rateLimitService:    NewRateLimitService(repo, nil, &config.Config{}, nil, nil),
		tlsFPProfileService: NewTLSFingerprintProfileService(&tlsProfileRepoStub{}, nil),
	}
	account := &Account{
		ID:          809,
		Name:        "deepseek-apikey",
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "sk-test",
			"model_mapping": map[string]any{
				"deepseek-alias": "deepseek-v4-pro",
			},
		},
	}

	_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, &ParsedRequest{})

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusNotFound, failoverErr.StatusCode)
	require.Equal(t, []string{"deepseek-v4-pro"}, repo.modelRateLimitScopes)
	require.Zero(t, repo.tempCalls)
	require.Zero(t, repo.rateLimitedCalls)
}
