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
	"github.com/tidwall/gjson"
)

func deepSeekCompatTestRoutes() []struct {
	name, path, body string
	forward          func(*GatewayService, context.Context, *gin.Context, *Account, []byte, *ParsedRequest) (*ForwardResult, error)
} {
	return []struct {
		name, path, body string
		forward          func(*GatewayService, context.Context, *gin.Context, *Account, []byte, *ParsedRequest) (*ForwardResult, error)
	}{
		{"chat", "/v1/chat/completions",
			`{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"hi"}],"reasoning_effort":"max","tools":[{"type":"function","function":{"name":"health_check","parameters":{"type":"object","properties":{}}}}]}`,
			(*GatewayService).ForwardAsChatCompletions},
		{"responses", "/v1/responses",
			`{"model":"deepseek-v4-flash","input":"hi","reasoning":{"effort":"max"},"tools":[{"type":"function","name":"health_check","parameters":{"type":"object","properties":{}}}]}`,
			(*GatewayService).ForwardAsResponses},
	}
}

func TestDeepSeekCompat402DisablesAccountAndReturnsUncommittedFailover(t *testing.T) {
	for _, route := range deepSeekCompatTestRoutes() {
		t.Run(route.name, func(t *testing.T) {
			repo := &rateLimitAccountRepoStubWithSchedulable{}
			errorBody := []byte(`{"error":{"type":"invalid_request_error","message":"Insufficient Balance"}}`)
			upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusPaymentRequired,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewReader(errorBody)),
			}}
			cfg := &config.Config{}
			svc := &GatewayService{
				cfg: cfg, httpUpstream: upstream,
				tlsFPProfileService: NewTLSFingerprintProfileService(&tlsProfileRepoStub{}, nil),
				rateLimitService:    NewRateLimitService(repo, nil, cfg, nil, nil),
			}
			account := &Account{ID: 2389, Platform: PlatformDeepSeek, Type: AccountTypeAPIKey,
				Status: StatusActive, Schedulable: true, Concurrency: 3,
				Credentials: map[string]any{"api_key": "sk-test", "model_mapping": map[string]any{"deepseek-v4-flash": "deepseek-flash"}},
			}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, route.path, bytes.NewBufferString(route.body))
			result, err := route.forward(svc, context.Background(), c, account, []byte(route.body), &ParsedRequest{})
			require.Nil(t, result)
			var failover *UpstreamFailoverError
			require.ErrorAs(t, err, &failover)
			require.Equal(t, http.StatusPaymentRequired, failover.StatusCode)
			require.Equal(t, errorBody, failover.ResponseBody)
			require.Equal(t, 1, repo.setErrorCalls)
			require.Equal(t, 1, repo.setSchedulableCalls)
			require.False(t, repo.lastSchedulable)
			require.Zero(t, repo.tempCalls)
			require.False(t, c.Writer.Written(), "handler must be able to retry another account")
			require.Empty(t, rec.Body.String())
		})
	}
}

func TestDeepSeekCompatForcedToolsDisableThinkingButAutoPreservesMax(t *testing.T) {
	for _, route := range deepSeekCompatTestRoutes() {
		for _, mode := range []string{"auto", "named", "required"} {
			name := route.name + "/" + mode
			forced := mode != "auto"
			choice := `"auto"`
			if mode == "required" {
				choice = `"required"`
			} else if mode == "named" {
				if route.name == "chat" {
					choice = `{"type":"function","function":{"name":"health_check"}}`
				} else {
					choice = `{"type":"function","name":"health_check"}`
				}
			}
			t.Run(name, func(t *testing.T) {
				body := route.body[:len(route.body)-1] + `,"tool_choice":` + choice + `}`
				upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{
					StatusCode: http.StatusBadRequest, Header: http.Header{},
					Body: io.NopCloser(bytes.NewBufferString(`{"error":{"message":"test response"}}`)),
				}}
				svc := &GatewayService{
					cfg: &config.Config{}, httpUpstream: upstream,
					tlsFPProfileService: NewTLSFingerprintProfileService(&tlsProfileRepoStub{}, nil),
				}
				account := &Account{ID: 2393, Platform: PlatformDeepSeek, Type: AccountTypeAPIKey, Concurrency: 3,
					Credentials: map[string]any{"api_key": "sk-test", "model_mapping": map[string]any{"deepseek-v4-flash": "deepseek-flash"}},
				}
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, route.path, bytes.NewBufferString(body))
				_, _ = route.forward(svc, context.Background(), c, account, []byte(body), &ParsedRequest{})
				require.NotEmpty(t, upstream.lastBody)
				require.Equal(t, "deepseek-flash", gjson.GetBytes(upstream.lastBody, "model").String())
				if forced {
					require.Equal(t, "disabled", gjson.GetBytes(upstream.lastBody, "thinking.type").String())
					if mode == "named" {
						require.Equal(t, "health_check", gjson.GetBytes(upstream.lastBody, "tool_choice.name").String())
					} else {
						require.Equal(t, "any", gjson.GetBytes(upstream.lastBody, "tool_choice.type").String())
					}
				} else {
					require.Equal(t, "enabled", gjson.GetBytes(upstream.lastBody, "thinking.type").String())
					require.Equal(t, "max", gjson.GetBytes(upstream.lastBody, "output_config.effort").String())
				}
			})
		}
	}
}

func TestGatewayFailoverDoesNotTreatClientParameterErrorsAsAccountFailures(t *testing.T) {
	svc := &GatewayService{}
	for _, status := range []int{http.StatusBadRequest, http.StatusNotFound, http.StatusUnprocessableEntity} {
		require.False(t, svc.shouldFailoverUpstreamError(status), "status=%d", status)
	}
	require.True(t, svc.shouldFailoverUpstreamError(http.StatusPaymentRequired))
}

func TestDeepSeekForcedToolNormalizationDoesNotChangeOtherPlatforms(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-5","thinking":{"type":"enabled","budget_tokens":1024},"tool_choice":{"type":"tool","name":"health_check"},"messages":[{"role":"user","content":"hi"}]}`)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	svc := &GatewayService{cfg: &config.Config{}}
	account := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"base_url": "https://api.anthropic.com"}}
	req, err := svc.buildUpstreamRequest(context.Background(), c, account, body, "sk-test", "apikey", "claude-sonnet-4-5", true, false)
	require.NoError(t, err)
	sent, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, "enabled", gjson.GetBytes(sent, "thinking.type").String())
}
