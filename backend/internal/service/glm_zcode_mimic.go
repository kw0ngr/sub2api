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

	applyGLMZCodeLightMimicHeaders(req)
	if !strong {
		return true
	}
	applyGLMZCodeStrongMimicHeaders(req)
	return true
}

func applyGLMZCodeLightMimicHeaders(req *http.Request) {
	setHeaderRaw(req.Header, "User-Agent", glmZCodeLightUserAgent)
	setHeaderRaw(req.Header, "HTTP-Referer", "https://zcode.z.ai")
	setHeaderRaw(req.Header, "X-Title", "Z Code@electron")
	setHeaderRaw(req.Header, "X-ZCode-Agent", "glm")
}

func applyGLMZCodeStrongMimicHeaders(req *http.Request) {
	setHeaderRaw(req.Header, "User-Agent", glmZCodeStrongUserAgent)
	setHeaderRaw(req.Header, "X-ZCode-App-Version", glmZCodeAppVersion)
	setHeaderRaw(req.Header, "X-Platform", "darwin-arm64")
	setHeaderRaw(req.Header, "X-Release-Channel", "stable")
	setHeaderRaw(req.Header, "X-Client-Language", "zh-CN")
	setHeaderRaw(req.Header, "X-Client-Timezone", "Asia/Shanghai")
	setHeaderRaw(req.Header, "X-Os-Category", "macos")
	setHeaderRaw(req.Header, "X-Os-Version", "Darwin Kernel Version 25.5.0")
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
	if !strings.EqualFold(u.Scheme, "https") {
		return false
	}
	if u.Port() != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	path := strings.TrimRight(u.EscapedPath(), "/")
	switch host {
	case "open.bigmodel.cn":
		return path == "/api/anthropic" || path == "/api/anthropic/v1/messages"
	case "api.z.ai":
		return path == "/api/anthropic" || path == "/api/anthropic/v1/messages"
	default:
		return false
	}
}
