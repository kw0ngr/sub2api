package service

import (
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func normalizeGeminiOpenAIReasoningEffort(body []byte, model string) ([]byte, bool) {
	switch lastSegment(strings.ToLower(strings.TrimSpace(model))) {
	case "gemini-3.6-flash", "gemini-3.7-flash", "gemini-3.8-flash":
	default:
		return body, false
	}
	effort := strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "reasoning_effort").String()))
	switch effort {
	case "max", "xhigh", "x-high", "ultra":
		normalized, err := sjson.SetBytes(body, "reasoning_effort", "high")
		if err == nil {
			return normalized, true
		}
	}
	return body, false
}

// extractGeminiReasoningEffortFromBody returns only the explicit level that is
// actually forwarded. A budget alone does not define a billing tier.
func extractGeminiReasoningEffortFromBody(body []byte) *string {
	config := gjson.GetBytes(body, "generationConfig")
	if !config.Exists() {
		config = gjson.GetBytes(body, "generation_config")
	}
	thinking := config.Get("thinkingConfig")
	if !thinking.Exists() {
		thinking = config.Get("thinking_config")
	}
	level := thinking.Get("thinkingLevel")
	if !level.Exists() {
		level = thinking.Get("thinking_level")
	}
	effort := strings.ToLower(strings.TrimSpace(level.String()))
	switch effort {
	case "minimal", "low", "medium", "high":
		return &effort
	default:
		return nil
	}
}
