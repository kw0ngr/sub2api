package service

import (
	"net/http"
	"net/url"
	"strings"
)

const (
	glmZCodeAppVersion      = "3.1.2"
	glmZCodeLightUserAgent  = "ZCode/unknown"
	glmZCodeStrongUserAgent = "ZCode/" + glmZCodeAppVersion
)

func applyGLMZCodeMimicHeaders(req *http.Request, account *Account, strong bool) bool {
	if req == nil || !shouldApplyGLMZCodeMimic(account) {
		return false
	}

	setHeaderRaw(req.Header, "User-Agent", glmZCodeLightUserAgent)
	setHeaderRaw(req.Header, "HTTP-Referer", "https://zcode.z.ai")
	setHeaderRaw(req.Header, "X-Title", "Z Code@electron")
	setHeaderRaw(req.Header, "X-ZCode-Agent", "glm")

	if !strong {
		return true
	}

	setHeaderRaw(req.Header, "User-Agent", glmZCodeStrongUserAgent)
	setHeaderRaw(req.Header, "X-ZCode-App-Version", glmZCodeAppVersion)
	setHeaderRaw(req.Header, "X-Platform", "darwin-arm64")
	setHeaderRaw(req.Header, "X-Release-Channel", "stable")
	setHeaderRaw(req.Header, "X-Client-Language", "zh-CN")
	setHeaderRaw(req.Header, "X-Client-Timezone", "Asia/Shanghai")
	setHeaderRaw(req.Header, "X-Os-Category", "macos")
	setHeaderRaw(req.Header, "X-Os-Version", "Darwin Kernel Version 25.5.0")
	return true
}

func shouldApplyGLMZCodeMimic(account *Account) bool {
	if account == nil || !account.IsGLMAnthropicCompatible() {
		return false
	}
	return isOfficialGLMAnthropicBaseURL(anthropicCompatibleBaseURLForAccount(account))
}

func isOfficialGLMAnthropicBaseURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return true
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
	path := strings.TrimRight(strings.ToLower(u.EscapedPath()), "/")
	switch host {
	case "open.bigmodel.cn":
		return path == "" || path == "/api" || path == "/api/anthropic" || strings.HasPrefix(path, "/api/anthropic/v1/messages")
	case "api.z.ai":
		return path == "" || path == "/api" || path == "/api/anthropic" || strings.HasPrefix(path, "/api/anthropic/v1/messages")
	default:
		return false
	}
}
