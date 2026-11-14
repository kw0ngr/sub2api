package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCanceledGatewayForwardDoesNotFabricateProvider502(t *testing.T) {
	for _, platform := range []string{service.PlatformGrok, service.PlatformGemini} {
		for _, path := range []string{"/v1/chat/completions", "/v1/responses"} {
			t.Run(platform+path, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				calls := 0
				router := geminiRegressionRouter(t, func(req *http.Request) (*http.Response, error) {
					calls++
					cancel()
					return nil, ctx.Err()
				}, platform)
				body := `{"model":"grok-4.7","messages":[{"role":"user","content":"hi"}],"input":"hi"}`
				if platform == service.PlatformGemini {
					body = strings.ReplaceAll(body, "grok-4.7", "gemini-4-hydrogen")
				}
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)).WithContext(ctx))
				require.Equal(t, 1, calls, "client cancellation must not fan out to other accounts")
				require.Equal(t, 499, rec.Code)
				require.Empty(t, rec.Body.String())
			})
		}
	}
}

func TestCanceledGatewayAfterStreamStartDoesNotAppendError(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	ctx, cancel := context.WithCancel(context.Background())
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx)
	c.String(200, "data: previous\n\n")
	cancel()
	require.True(t, abortCanceledGatewayRequest(c))
	require.Equal(t, 200, rec.Code)
	require.Equal(t, "data: previous\n\n", rec.Body.String())
	require.True(t, c.GetBool(service.OpsClientCanceledKey))
}

type canceledReadBody struct{ cancel context.CancelFunc }

func (r canceledReadBody) Read([]byte) (int, error) {
	r.cancel()
	return 0, context.Canceled
}
func (canceledReadBody) Close() error { return nil }

func TestCanceledGatewayBodyReadReturns499NotMalformedJSON(t *testing.T) {
	for _, path := range []string{"/v1/chat/completions", "/v1/responses"} {
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		router := geminiRegressionRouter(t, func(*http.Request) (*http.Response, error) {
			calls++
			return nil, io.ErrUnexpectedEOF
		})
		req := httptest.NewRequest(http.MethodPost, path, nil).WithContext(ctx)
		req.Body = canceledReadBody{cancel: cancel}
		req.ContentLength = -1
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		cancel()
		require.Equal(t, 499, rec.Code)
		require.Empty(t, rec.Body.String())
		require.Zero(t, calls)
	}
}

func TestCanceledGatewayUpstreamBodyReadDoesNotCommit502(t *testing.T) {
	for _, path := range []string{"/v1/chat/completions", "/v1/responses"} {
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		router := geminiRegressionRouter(t, func(*http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: 200,
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Body:   canceledReadBody{cancel: cancel},
			}, nil
		})
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"gemini-4-hydrogen","messages":[{"role":"user","content":"hi"}],"input":"hi","stream":false}`)).WithContext(ctx)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		cancel()
		require.Equal(t, 1, calls)
		require.Equal(t, 499, rec.Code)
		require.Empty(t, rec.Body.String())
	}
}

func TestActiveClientDoesNotHideInternalUpstreamCancellation(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	require.False(t, abortCanceledGatewayRequest(c))
	require.False(t, c.GetBool(service.OpsClientCanceledKey))
	require.False(t, c.IsAborted())
}

func TestActiveClientUpstreamBodyReadFailureRemains502(t *testing.T) {
	router := geminiRegressionRouter(t, func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200,
			Header: http.Header{"Content-Type": []string{"application/json"}},
			Body: canceledReadBody{cancel: func() {}},
		}, nil
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gemini-4-hydrogen","input":"hi","stream":false}`)))
	require.Equal(t, 502, rec.Code)
	require.Contains(t, rec.Body.String(), "Failed to read upstream response")
}
