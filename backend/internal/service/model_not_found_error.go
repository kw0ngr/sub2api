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

// isUpstreamModelNotFoundError recognizes deterministic account-model access
// failures without mistaking an endpoint/base-URL 404 or an arbitrary 5xx for
// account health. Explicit structured model_not_found codes are authoritative
// even when a Responses stream wraps the terminal failure as HTTP 502; message
// heuristics remain limited to the conventional 400/404 statuses.
func isUpstreamModelNotFoundError(statusCode int, body []byte) bool {
	if statusCode < http.StatusBadRequest {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(body)))
	code := strings.ToLower(strings.TrimSpace(extractUpstreamErrorCode(body)))
	errType := strings.ToLower(strings.TrimSpace(extractUpstreamErrorType(body)))
	for _, keyword := range upstreamModelNotFoundKeywords {
		if containsNormalizedModelNotFound(code, keyword) ||
			containsNormalizedModelNotFound(errType, keyword) {
			return true
		}
	}
	if statusCode != http.StatusNotFound && statusCode != http.StatusBadRequest {
		return false
	}
	for _, keyword := range upstreamModelNotFoundKeywords {
		if containsNormalizedModelNotFound(message, keyword) {
			return true
		}
	}
	// Some compatible providers qualify the phrase with their own name, for
	// example `Unknown Umans model`, so the words are not contiguous.
	fields := strings.Fields(strings.NewReplacer("-", " ", "_", " ").Replace(message))
	if len(fields) >= 3 && fields[0] == "unknown" && fields[2] == "model" {
		return true
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
