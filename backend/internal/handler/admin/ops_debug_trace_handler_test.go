//go:build unit

package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newOpsDebugTraceTestRouter(handler *OpsHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
		c.Next()
	})
	r.GET("/debug-traces", handler.ListDebugTraces)
	r.GET("/debug-traces/:id", handler.GetDebugTrace)
	return r
}

func TestOpsHandler_ListAndGetDebugTraces(t *testing.T) {
	service.ResetDebugTraceStoreForTest()
	trace := &service.DebugTrace{
		RequestID:         "req-debug-1",
		Platform:          service.PlatformOpenAI,
		StatusCode:        http.StatusBadGateway,
		ReasonCode:        "fallback_exhausted",
		FallbackTriggered: true,
		CreatedAt:         time.Now(),
	}
	service.RecordDebugTrace(trace)
	items := service.ListDebugTraces(service.DebugTraceFilter{Limit: 1})
	require.Len(t, items, 1)

	h := NewOpsHandler(newRuntimeOpsService(t))
	r := newOpsDebugTraceTestRouter(h)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/debug-traces?reason=fallback_exhausted&only_fallback=true", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var listResp responseEnvelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listResp))
	require.Equal(t, 0, listResp.Code)
	require.Contains(t, string(listResp.Data), "req-debug-1")

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/debug-traces/"+items[0].ID, nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var getResp responseEnvelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &getResp))
	require.Equal(t, 0, getResp.Code)
	require.Contains(t, string(getResp.Data), "fallback_exhausted")
}
