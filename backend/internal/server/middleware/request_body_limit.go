package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func isResponsesRequestPath(path string) bool {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return false
	}
	return trimmed == "/v1/responses" ||
		trimmed == "/responses" ||
		trimmed == "/openai/v1/responses" ||
		strings.HasPrefix(trimmed, "/v1/responses/") ||
		strings.HasPrefix(trimmed, "/responses/") ||
		strings.HasPrefix(trimmed, "/openai/v1/responses/")
}

// ResolveRequestBodyLimit returns the effective request body limit for the incoming request.
// Responses paths may use a larger dedicated limit to support Codex-sized payloads.
func ResolveRequestBodyLimit(req *http.Request, defaultMaxBytes, responsesMaxBytes int64) int64 {
	limit := defaultMaxBytes
	if req == nil {
		return limit
	}
	if responsesMaxBytes > limit && isResponsesRequestPath(req.URL.Path) {
		return responsesMaxBytes
	}
	return limit
}

// RequestBodyLimit 使用 MaxBytesReader 限制请求体大小。
func RequestBodyLimit(maxBytes int64, responsesMaxBytes ...int64) gin.HandlerFunc {
	responsesLimit := maxBytes
	if len(responsesMaxBytes) > 0 {
		responsesLimit = responsesMaxBytes[0]
	}
	return func(c *gin.Context) {
		limit := ResolveRequestBodyLimit(c.Request, maxBytes, responsesLimit)
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}

// RequestBodyLimitHandler applies the same dynamic body limit logic at the net/http layer.
// This protects the server before Gin routing while still allowing larger Responses payloads.
func RequestBodyLimitHandler(next http.Handler, maxBytes, responsesMaxBytes int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := ResolveRequestBodyLimit(r, maxBytes, responsesMaxBytes)
		http.MaxBytesHandler(next, limit).ServeHTTP(w, r)
	})
}
