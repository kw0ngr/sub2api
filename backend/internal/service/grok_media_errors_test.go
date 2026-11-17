//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGrokMediaUpstreamErrorFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		name    string
		status  int
		body    string
		message string
		code    string
	}{
		{"flat error", 400, `{"code":"invalid_image","error":"Image could not be decoded."}`, "Image could not be decoded.", "invalid_image"},
		{"nested code takes priority", 400, `{"code":"outer","error":{"code":"invalid_size","message":"Invalid size."}}`, "Invalid size.", "invalid_size"},
		{"numeric code", 400, `{"code":400,"error":"Bad input."}`, "Bad input.", "400"},
		{"no code", 400, `{"error":"Bad input."}`, "Bad input.", ""},
		{"object code", 400, `{"code":{"debug":"private diagnostic"},"error":"Bad input."}`, "Bad input.", ""},
		{"array code", 400, `{"code":["private diagnostic"],"error":"Bad input."}`, "Bad input.", ""},
		{"boolean code", 400, `{"code":true,"error":"Bad input."}`, "Bad input.", ""},
		{"null code", 400, `{"code":null,"error":"Bad input."}`, "Bad input.", ""},
		{"redact message and code", 400, `{"code":"https://example.test?key=secret&n=1","error":"Bad image: https://example.test?access_token=secret&n=1"}`, "Bad image: https://example.test?access_token=***&n=1", "https://example.test?key=***&n=1"},
		{"content refusal", 403, grokContentRefusalBody, "I'm sorry, I can't help with that request.", "permission-denied"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
			account := &Account{ID: 1, Platform: PlatformGrok, Type: AccountTypeOAuth, Schedulable: true}
			svc := &OpenAIGatewayService{}

			_, err := svc.handleGrokMediaErrorResponse(context.Background(), &http.Response{
				StatusCode: tt.status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(tt.body)),
			}, c, account, "request-id", "grok-imagine")

			require.Error(t, err)
			var failover *UpstreamFailoverError
			require.NotErrorAs(t, err, &failover)
			require.True(t, IsResponseCommitted(c))
			require.Equal(t, tt.status, recorder.Code)
			require.Equal(t, tt.message, gjson.GetBytes(recorder.Body.Bytes(), "error.message").String())
			require.Equal(t, tt.code, gjson.GetBytes(recorder.Body.Bytes(), "error.code").String())
			if tt.code == "" {
				require.False(t, gjson.GetBytes(recorder.Body.Bytes(), "error.code").Exists())
			}
			require.NotContains(t, recorder.Body.String(), "secret")
			require.NotContains(t, recorder.Body.String(), "private diagnostic")
			require.True(t, account.Schedulable)
			require.Nil(t, account.TempUnschedulableUntil)
		})
	}
}

func TestGrokMediaErrorCodeDoesNotBypassRewrites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, configuredRule := range []bool{false, true} {
		name := "account status filter"
		if configuredRule {
			name = "configured response rule"
		}
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
			account := &Account{ID: 1, Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{
				"custom_error_codes_enabled": true,
				"custom_error_codes":         []any{float64(http.StatusTooManyRequests)},
			}}
			wantStatus, wantMessage := http.StatusInternalServerError, "Upstream gateway error"
			if configuredRule {
				rules := &ErrorPassthroughService{}
				rules.setLocalCache([]*model.ErrorPassthroughRule{
					newNonFailoverPassthroughRule(http.StatusBadRequest, "invalid_image", http.StatusUnprocessableEntity, "Configured error"),
				})
				BindErrorPassthroughService(c, rules)
				wantStatus, wantMessage = http.StatusUnprocessableEntity, "Configured error"
			}
			svc := &OpenAIGatewayService{}
			_, err := svc.handleGrokMediaErrorResponse(context.Background(), &http.Response{
				StatusCode: http.StatusBadRequest, Header: http.Header{},
				Body: io.NopCloser(strings.NewReader(`{"code":"invalid_image","error":"Original error"}`)),
			}, c, account, "request-id", "grok-imagine")

			require.Error(t, err)
			require.Equal(t, wantStatus, recorder.Code)
			require.Equal(t, wantMessage, gjson.GetBytes(recorder.Body.Bytes(), "error.message").String())
			require.False(t, gjson.GetBytes(recorder.Body.Bytes(), "error.code").Exists())
			require.NotContains(t, recorder.Body.String(), "Original error")
		})
	}
}

func TestGrokMediaErrorCodeKeepsFailoverBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
	body := `{"code":"temporarily_unavailable","error":"Try again later"}`
	svc := &OpenAIGatewayService{}
	_, err := svc.handleGrokMediaErrorResponse(context.Background(), &http.Response{
		StatusCode: http.StatusServiceUnavailable, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)),
	}, c, &Account{ID: 1, Platform: PlatformGrok, Type: AccountTypeOAuth}, "request-id", "grok-imagine")

	var failover *UpstreamFailoverError
	require.ErrorAs(t, err, &failover)
	require.Equal(t, http.StatusServiceUnavailable, failover.StatusCode)
	require.Equal(t, body, string(failover.ResponseBody))
	require.False(t, IsResponseCommitted(c))
	require.False(t, c.Writer.Written())
	require.Empty(t, recorder.Body.String())
}

func TestGrokMediaErrorCodeDoesNotChangeSharedExtractor(t *testing.T) {
	for _, body := range []string{
		`{"code":"invalid_image","error":"Bad image"}`,
		`{"object":"error","message":"model not found","code":400}`,
		`{"error":{"message":"m"},"code":"token_revoked"}`,
	} {
		require.Empty(t, extractUpstreamErrorCode([]byte(body)))
	}
}
