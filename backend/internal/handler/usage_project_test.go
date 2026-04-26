//go:build unit

package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestExtractUsageProjectPrefersExplicitHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set("X-Sub2API-Project", " header-project ")
	ctx.Request = req

	got := ExtractUsageProject(ctx, []byte(`{"metadata":{"project":"body-project"}}`))

	require.Equal(t, "header-project", got)
}

func TestExtractUsageProjectReadsMetadataProject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/openai/v1/responses", nil)

	got := ExtractUsageProject(ctx, []byte(`{"metadata":{"project":"usage-dashboard"}}`))

	require.Equal(t, "usage-dashboard", got)
}
