package service

import (
	"net/http"
	"strings"
)

var upstreamModelNotFoundKeywords = []string{
	"model not found",
	"unknown model",
	"model_not_found",
	"model does not exist",
	"does not exist",
	"no longer available",
}

// isUpstreamModelNotFoundError only matches 404 errors that identify a model,
// so endpoint/base-URL 404s continue through the existing account policy.
func isUpstreamModelNotFoundError(statusCode int, body []byte) bool {
	if statusCode != http.StatusNotFound && statusCode != http.StatusBadRequest {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(body)))
	code := strings.ToLower(strings.TrimSpace(extractUpstreamErrorCode(body)))
	errType := strings.ToLower(strings.TrimSpace(extractUpstreamErrorType(body)))
	for _, keyword := range upstreamModelNotFoundKeywords {
		if containsNormalizedModelNotFound(message, keyword) ||
			containsNormalizedModelNotFound(code, keyword) ||
			containsNormalizedModelNotFound(errType, keyword) {
			return true
		}
	}
	// DeepSeek-compatible upstreams return 404 with the compact form `model: <id>`.
	return statusCode == http.StatusNotFound &&
		strings.HasPrefix(message, "model:") &&
		strings.TrimSpace(strings.TrimPrefix(message, "model:")) != ""
}

func containsNormalizedModelNotFound(text string, keyword string) bool {
	normalized := strings.NewReplacer("-", " ", "_", " ", "\n", " ", "\r", " ", "\t", " ").Replace(text)
	normalized = strings.Join(strings.Fields(normalized), " ")
	if !strings.Contains(normalized, "model") {
		return false
	}
	keyword = strings.NewReplacer("-", " ", "_", " ").Replace(strings.ToLower(keyword))
	keyword = strings.Join(strings.Fields(keyword), " ")
	return strings.Contains(normalized, keyword)
}
