package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/tidwall/gjson"
)

func NormalizeOpenAICompatRequestedModel(model string) string {
	if openai.IsGPT61SolModelSpelling(model) {
		return "gpt-6.1-sol"
	}
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		return ""
	}

	normalized, _, ok := splitOpenAICompatReasoningModel(trimmed)
	if !ok || normalized == "" {
		return trimmed
	}
	return normalized
}

func applyOpenAICompatModelNormalization(req *apicompat.AnthropicRequest) {
	if req == nil {
		return
	}
	if openai.IsGPT61SolModelSpelling(req.Model) {
		canonical := strings.ToLower(lastOpenAIModelSegment(req.Model))
		canonical = strings.ReplaceAll(canonical, "_", "-")
		if effort, ok := strings.CutPrefix(canonical, "gpt-6.1-sol-"); ok && effort != "openai-compact" {
			req.Model = "gpt-6.1-sol"
			if req.OutputConfig == nil {
				req.OutputConfig = &apicompat.AnthropicOutputConfig{}
			}
			if req.OutputConfig.Effort == "" {
				req.OutputConfig.Effort = effort
			}
			return
		}
	}

	originalModel := strings.TrimSpace(req.Model)
	if originalModel == "" {
		return
	}

	normalizedModel, derivedEffort, hasReasoningSuffix := splitOpenAICompatReasoningModel(originalModel)
	if hasReasoningSuffix && normalizedModel != "" {
		req.Model = normalizedModel
	}

	if req.OutputConfig != nil && strings.TrimSpace(req.OutputConfig.Effort) != "" {
		return
	}

	claudeEffort := openAIReasoningEffortToClaudeOutputEffort(derivedEffort)
	if claudeEffort == "" {
		return
	}

	if req.OutputConfig == nil {
		req.OutputConfig = &apicompat.AnthropicOutputConfig{}
	}
	req.OutputConfig.Effort = claudeEffort
}

// validateOpenAIModelReasoningCompatRequest runs before any compatibility conversion can
// silently replace or discard an explicit no-reasoning request.
func validateOpenAIModelReasoningCompatRequest(body []byte, upstreamModel string) error {
	if !openai.IsGPT61SolModelSpelling(upstreamModel) && !openai.IsChatLatestModelSpelling(upstreamModel) {
		return nil
	}
	for _, path := range []string{"reasoning.effort", "reasoning_effort", "output_config.effort"} {
		if err := openai.ValidateModelReasoningEffort(upstreamModel, gjson.GetBytes(body, path).String()); err != nil {
			return err
		}
	}
	if strings.EqualFold(gjson.GetBytes(body, "thinking.type").String(), "disabled") {
		return openai.ValidateModelReasoningEffort(upstreamModel, "none")
	}
	requestedModel := strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "model").String()))
	for _, effort := range []string{"none", "minimal"} {
		if strings.HasSuffix(requestedModel, "-"+effort) {
			return openai.ValidateModelReasoningEffort(upstreamModel, effort)
		}
	}
	return nil
}

func splitOpenAICompatReasoningModel(model string) (normalizedModel string, reasoningEffort string, ok bool) {
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		return "", "", false
	}

	modelID := trimmed
	if strings.Contains(modelID, "/") {
		parts := strings.Split(modelID, "/")
		modelID = parts[len(parts)-1]
	}
	modelID = strings.TrimSpace(modelID)
	if !strings.HasPrefix(strings.ToLower(modelID), "gpt-") {
		return trimmed, "", false
	}

	parts := strings.FieldsFunc(strings.ToLower(modelID), func(r rune) bool {
		switch r {
		case '-', '_', ' ':
			return true
		default:
			return false
		}
	})
	if len(parts) == 0 {
		return trimmed, "", false
	}

	last := strings.NewReplacer("-", "", "_", "", " ", "").Replace(parts[len(parts)-1])
	switch last {
	case "none", "minimal":
	case "low", "medium", "high":
		reasoningEffort = last
	case "xhigh", "extrahigh":
		reasoningEffort = "xhigh"
	default:
		return trimmed, "", false
	}

	return normalizeCodexModel(modelID), reasoningEffort, true
}

func openAIReasoningEffortToClaudeOutputEffort(effort string) string {
	switch strings.TrimSpace(effort) {
	case "low", "medium", "high":
		return effort
	case "xhigh":
		return "max"
	default:
		return ""
	}
}
