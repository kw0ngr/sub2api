package service

import (
	"net/http"
	"net/url"
	"strings"
)

const (
	defaultAnthropicBaseURL       = "https://api.anthropic.com"
	deepSeekAnthropicBaseURL      = "https://api.deepseek.com/anthropic"
	openRouterAnthropicBaseURL    = "https://openrouter.ai/api"
	openRouterOpenAICompatBaseURL = "https://openrouter.ai/api/v1"
	deepSeekOpenAICompatBaseURL   = "https://api.deepseek.com"
	glmAnthropicBaseURL           = "https://open.bigmodel.cn/api/anthropic"
)

func DefaultGLMAnthropicBaseURL() string {
	return glmAnthropicBaseURL
}

func defaultAnthropicCompatibleBaseURL(platform string) string {
	switch strings.TrimSpace(platform) {
	case PlatformDeepSeek:
		return deepSeekAnthropicBaseURL
	case PlatformOpenRouter:
		return openRouterAnthropicBaseURL
	case PlatformGLM:
		return glmAnthropicBaseURL
	default:
		return defaultAnthropicBaseURL
	}
}

func anthropicCompatibleBaseURLForAccount(account *Account) string {
	if account == nil {
		return defaultAnthropicBaseURL
	}

	rawBaseURL := strings.TrimRight(strings.TrimSpace(account.GetCredential("base_url")), "/")
	normalized := strings.ToLower(rawBaseURL)
	switch strings.TrimSpace(account.Platform) {
	case PlatformDeepSeek:
		if normalized == "" ||
			normalized == deepSeekOpenAICompatBaseURL ||
			normalized == deepSeekOpenAICompatBaseURL+"/v1" {
			return deepSeekAnthropicBaseURL
		}
	case PlatformOpenRouter:
		if normalized == "" ||
			normalized == "https://openrouter.ai" ||
			normalized == openRouterAnthropicBaseURL ||
			normalized == openRouterOpenAICompatBaseURL {
			return openRouterAnthropicBaseURL
		}
	case PlatformGLM:
		if normalized == "" ||
			normalized == "https://open.bigmodel.cn" ||
			normalized == "https://open.bigmodel.cn/api" ||
			normalized == "https://open.bigmodel.cn/api/paas/v4" {
			return glmAnthropicBaseURL
		}
	}

	if rawBaseURL != "" {
		return rawBaseURL
	}
	if baseURL := strings.TrimRight(strings.TrimSpace(account.GetBaseURL()), "/"); baseURL != "" {
		return baseURL
	}
	return defaultAnthropicCompatibleBaseURL(account.Platform)
}

func setAnthropicCompatibleAPIKeyAuthHeader(header http.Header, platform string, token string) {
	header.Del("authorization")
	header.Del("x-api-key")
	header.Del("x-goog-api-key")

	if strings.TrimSpace(platform) == PlatformOpenRouter {
		setHeaderRaw(header, "authorization", "Bearer "+token)
		return
	}
	setHeaderRaw(header, "x-api-key", token)
}

func setAnthropicCompatibleAPIKeyAuthHeaderForAccount(header http.Header, account *Account, token string) {
	if usesBearerForAnthropicCompatibleAPIKey(account) {
		header.Del("authorization")
		header.Del("x-api-key")
		header.Del("x-goog-api-key")
		setHeaderRaw(header, "authorization", "Bearer "+token)
		return
	}
	setAnthropicCompatibleAPIKeyAuthHeader(header, accountPlatform(account), token)
}

func usesBearerForAnthropicCompatibleAPIKey(account *Account) bool {
	if account == nil {
		return false
	}
	if strings.TrimSpace(account.Platform) == PlatformOpenRouter {
		return true
	}
	if strings.TrimSpace(account.Platform) != PlatformGLM {
		return false
	}
	if !account.IsGLMAnthropicCompatible() {
		return false
	}
	return isOllamaHostedBaseURL(account.GetCredential("base_url"))
}

func isOllamaHostedBaseURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "ollama.com" || strings.HasSuffix(host, ".ollama.com")
}

func mixedSchedulingPlatformsForGateway(platform string) []string {
	switch strings.TrimSpace(platform) {
	case PlatformAnthropic:
		return []string{PlatformAnthropic, PlatformAntigravity, PlatformOpenRouter, PlatformDeepSeek}
	case PlatformGemini:
		return []string{PlatformGemini, PlatformAntigravity}
	default:
		return []string{platform}
	}
}

func isAnthropicCompatibleMixedAPIKeyAccount(account *Account) bool {
	if account == nil || account.Type != AccountTypeAPIKey {
		return false
	}
	switch account.Platform {
	case PlatformOpenRouter, PlatformDeepSeek:
		return len(account.GetModelMapping()) > 0
	default:
		return false
	}
}

func shouldIncludeMixedSchedulingAccount(basePlatform string, account *Account) bool {
	if account == nil {
		return false
	}
	if account.Platform == basePlatform {
		return true
	}
	if account.Platform == PlatformAntigravity {
		return account.IsMixedSchedulingEnabled()
	}
	return strings.TrimSpace(basePlatform) == PlatformAnthropic && isAnthropicCompatibleMixedAPIKeyAccount(account)
}
