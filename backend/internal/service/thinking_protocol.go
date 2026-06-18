package service

import "strings"

type ThinkingProtocol int

const (
	ThinkingProtocolUnknown ThinkingProtocol = iota
	ThinkingProtocolAnthropicStrict
	ThinkingProtocolPassbackRequired
)

func ResolveThinkingProtocol(modelID string) ThinkingProtocol {
	id := strings.ToLower(strings.TrimSpace(modelID))
	if id == "" {
		return ThinkingProtocolUnknown
	}
	if slash := strings.LastIndex(id, "/"); slash >= 0 && slash+1 < len(id) {
		id = id[slash+1:]
	}

	switch {
	case strings.HasPrefix(id, "deepseek-"),
		strings.HasPrefix(id, "kimi-"),
		strings.HasPrefix(id, "moonshot-"),
		strings.HasPrefix(id, "glm-"),
		strings.HasPrefix(id, "minimax-m"):
		return ThinkingProtocolPassbackRequired
	case (strings.HasPrefix(id, "qwen-") ||
		strings.HasPrefix(id, "qwen2-") ||
		strings.HasPrefix(id, "qwen3-") ||
		strings.HasPrefix(id, "qwen4-")) && strings.Contains(id, "-thinking"):
		return ThinkingProtocolPassbackRequired
	case strings.HasPrefix(id, "claude-"),
		strings.HasPrefix(id, "opus-"),
		strings.HasPrefix(id, "sonnet-"),
		strings.HasPrefix(id, "haiku-"):
		return ThinkingProtocolAnthropicStrict
	default:
		return ThinkingProtocolUnknown
	}
}

func ShouldPreFilterThinkingBlocks(modelID string) bool {
	return ResolveThinkingProtocol(modelID) == ThinkingProtocolAnthropicStrict
}

func ShouldRectifyThinkingSignatureError(modelID string) bool {
	return ResolveThinkingProtocol(modelID) == ThinkingProtocolAnthropicStrict
}

func ShouldApplyRetryFilters(modelID string) bool {
	return ResolveThinkingProtocol(modelID) == ThinkingProtocolAnthropicStrict
}
