//go:build unit

package middleware

import (
	"context"
	"errors"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIKeyAuthLookupFailureWhenClientGoneReturns499(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{RunMode: config.RunModeSimple}

	for _, tc := range []struct {
		name   string
		cancel bool
		want   int
	}{
		{name: "client gone", cancel: true, want: 499},
		{name: "server failure", cancel: false, want: http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var cancel context.CancelFunc
			repo := &stubApiKeyRepo{getByKey: func(lookupCtx context.Context, _ string) (*service.APIKey, error) {
				if tc.cancel {
					// 查库期间客户端断开：net/http 取消请求 ctx，驱动返回 context 错误。
					cancel()
					return nil, lookupCtx.Err()
				}
				return nil, errors.New("db unavailable")
			}}
			svc := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)

			for _, mw := range []gin.HandlerFunc{
				gin.HandlerFunc(NewAPIKeyAuthMiddleware(svc, nil, cfg)),
				APIKeyAuthWithSubscriptionGoogle(svc, nil, cfg),
			} {
				var ctx context.Context
				ctx, cancel = context.WithCancel(context.Background())
				r := gin.New()
				r.Use(mw)
				r.GET("/t", func(c *gin.Context) { c.Status(http.StatusOK) })
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/t", nil).WithContext(ctx)
				req.Header.Set("x-api-key", "sk-test-key")
				req.Header.Set("x-goog-api-key", "sk-test-key")
				r.ServeHTTP(w, req)
				require.Equal(t, tc.want, w.Code)
				cancel()
			}
		})
	}
}

func TestAPIKeyAuthRejectsNonUTF8KeyAsInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &stubApiKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) {
		// Postgres 拒绝非 UTF-8 参数（SQLSTATE 22021），不应走到这里。
		return nil, errors.New("invalid byte sequence for encoding UTF8")
	}}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	svc := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)

	r := gin.New()
	r.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(svc, nil, cfg)))
	r.GET("/t", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/t", nil)
	req.Header.Set("x-api-key", "sk-\xff\xfe")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}
