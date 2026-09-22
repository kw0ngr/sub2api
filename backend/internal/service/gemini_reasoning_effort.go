package service

import (
	"strings"

	"github.com/tidwall/gjson"
)

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
