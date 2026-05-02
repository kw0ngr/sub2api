package service

import (
	"net/http"
	"strings"
)

const (
	defaultAnthropicBaseURL       = "https://api.anthropic.com"
	deepSeekAnthropicBaseURL      = "https://api.deepseek.com/anthropic"
	openRouterAnthropicBaseURL    = "https://openrouter.ai/api"
	openRouterOpenAICompatBaseURL = "https://openrouter.ai/api/v1"
	deepSeekOpenAICompatBaseURL   = "https://api.deepseek.com"
)

func defaultAnthropicCompatibleBaseURL(platform string) string {
	switch strings.TrimSpace(platform) {
	case PlatformDeepSeek:
		return deepSeekAnthropicBaseURL
	case PlatformOpenRouter:
		return openRouterAnthropicBaseURL
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
