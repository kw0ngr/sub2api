package service

import (
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func normalizeGeminiOpenAIReasoningEffort(body []byte, model string) ([]byte, bool) {
	modelID := lastSegment(strings.ToLower(strings.TrimSpace(model)))
	switch modelID {
	case "gemini-4-carbon", "gemini-4-hydrogen", "gemini-4-argon", "gemini-3.6-flash", "gemini-3.7-flash", "gemini-3.8-flash":
	default:
		return body, false
	}

	effort := strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "reasoning_effort").String()))
	if effort == "" {
		effort = strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "reasoning.effort").String()))
	}
	if effort == "" {
		switch strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "thinking.type").String())) {
		case "disabled":
			effort = "none"
		case "enabled", "adaptive":
			effort = "high"
		}
	}
	if effort == "" && gjson.GetBytes(body, "enable_thinking").Exists() {
		if gjson.GetBytes(body, "enable_thinking").Bool() {
			effort = "high"
		} else {
			effort = "none"
		}
	}

	switch effort {
	case "max", "xhigh", "x-high", "ultra":
		effort = "high"
		if modelID == "gemini-4-hydrogen" {
			effort = "xhigh"
		}
	}

	normalized := body
	changed := false
	if effort != "" && gjson.GetBytes(normalized, "reasoning_effort").String() != effort {
		next, err := sjson.SetBytes(normalized, "reasoning_effort", effort)
		if err == nil {
			normalized = next
			changed = true
		}
	}
	// Google OpenAI accepts reasoning_effort, but rejects client-specific aliases.
	for _, field := range []string{"thinking", "enable_thinking", "reasoning"} {
		if !gjson.GetBytes(normalized, field).Exists() {
			continue
		}
		next, err := sjson.DeleteBytes(normalized, field)
		if err == nil {
			normalized = next
			changed = true
		}
	}
	return normalized, changed
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
	case "minimal", "low", "medium", "high", "xhigh":
		return &effort
	default:
		return nil
	}
}
