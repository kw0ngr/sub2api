package service

import (
	"bytes"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func writeAnthropicCountTokensError(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"type": "error",
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}

func isOpenAIInputTokensUnsupported(statusCode int, body []byte) bool {
	if statusCode != http.StatusNotFound {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(body)))
	return strings.Contains(msg, "input_tokens") && strings.Contains(msg, "not found")
}

func writeOpenAIOAuthInputTokensFallback(c *gin.Context, prepared *openAIInputTokensCountPrepared) {
	estimated := estimateOpenAIInputTokens(prepared.Request)
	if estimated < openAIInputTokensFallbackMinimum {
		estimated = openAIInputTokensFallbackMinimum
	}
	c.JSON(http.StatusOK, gin.H{"input_tokens": estimated})
}

func isOpenAIOAuthInputTokensUnsupported(statusCode int, body []byte) bool {
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
	default:
		return false
	}

	bodyLower := strings.ToLower(string(body))
	msg := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(body)))
	code := strings.ToLower(strings.TrimSpace(extractUpstreamErrorCode(body)))
	if code == "missing_scope" ||
		strings.Contains(bodyLower, "api.responses.write") ||
		strings.Contains(bodyLower, "missing scopes") ||
		strings.Contains(bodyLower, "insufficient_scope") {
		return true
	}
	if statusCode == http.StatusNotFound && isOpenAIInputTokensUnsupported(statusCode, body) {
		return true
	}
	return strings.Contains(msg, "input_tokens") &&
		(strings.Contains(msg, "not found") ||
			strings.Contains(msg, "not supported") ||
			strings.Contains(msg, "unsupported"))
}

func estimateOpenAIInputTokens(req openAIInputTokensCountRequest) int {
	totalChars := len(strings.TrimSpace(req.Instructions))
	totalChars += len(bytes.TrimSpace(req.Input))
	for _, tool := range req.Tools {
		if raw, err := marshalOpenAIUpstreamJSON(tool); err == nil {
			totalChars += len(raw)
		}
	}
	totalChars += len(bytes.TrimSpace(req.ToolChoice))
	if totalChars == 0 {
		return 0
	}
	return (totalChars + 3) / 4
}
