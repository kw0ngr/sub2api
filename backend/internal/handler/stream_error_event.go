package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// writeResponsesFailedSSE terminates an already-started Responses stream with
// an event recognized by strict clients such as Codex CLI.
func writeResponsesFailedSSE(c *gin.Context, errType, message string) bool {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return false
	}

	responseID := synthesizeResponseID(c)
	model := requestModel(c)
	code := mapResponsesErrorCode(errType)

	var payload strings.Builder
	payload.Grow(256 + len(message) + len(model))
	_, _ = payload.WriteString(`{"type":"response.failed","response":{"id":`)
	_, _ = payload.WriteString(strconv.Quote(responseID))
	_, _ = payload.WriteString(`,"object":"response"`)
	if model != "" {
		_, _ = payload.WriteString(`,"model":`)
		_, _ = payload.WriteString(strconv.Quote(model))
	}
	_, _ = payload.WriteString(`,"status":"failed","output":[],"error":{"code":`)
	_, _ = payload.WriteString(strconv.Quote(code))
	_, _ = payload.WriteString(`,"message":`)
	_, _ = payload.WriteString(strconv.Quote(message))
	_, _ = payload.WriteString(`}}}`)

	if _, err := fmt.Fprintf(c.Writer, "event: response.failed\ndata: %s\n\n", payload.String()); err != nil {
		_ = c.Error(err)
		return true
	}
	flusher.Flush()
	return true
}

// inboundIsResponses covers canonical, bare and Codex direct Responses routes.
func inboundIsResponses(c *gin.Context) bool {
	if c == nil {
		return false
	}
	path := strings.TrimRight(c.FullPath(), "/")
	if path == "" && c.Request != nil && c.Request.URL != nil {
		path = strings.TrimRight(c.Request.URL.Path, "/")
	}
	if path == "" {
		return false
	}
	return strings.HasSuffix(path, "/responses") || strings.Contains(path, "/responses/")
}

func synthesizeResponseID(c *gin.Context) string {
	if c != nil && c.Request != nil {
		if requestID, ok := c.Request.Context().Value(ctxkey.RequestID).(string); ok {
			if requestID = strings.TrimSpace(requestID); requestID != "" {
				return "resp_" + strings.ReplaceAll(requestID, "-", "")
			}
		}
	}
	return "resp_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}

func requestModel(c *gin.Context) string {
	if c == nil {
		return ""
	}
	value, ok := c.Get(opsModelKey)
	if !ok {
		return ""
	}
	model, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(model)
}

func mapResponsesErrorCode(errType string) string {
	switch errType {
	case "rate_limit_error":
		return "rate_limit_exceeded"
	case "invalid_request_error":
		return "invalid_request"
	case "permission_error":
		return "permission_denied"
	case "authentication_error":
		return "authentication_failed"
	case "upstream_error":
		return "upstream_error"
	case "server_error", "api_error", "":
		return "server_error"
	default:
		return errType
	}
}
