package service

import "strings"

func lastOpenAIModelSegment(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return ""
	}
	parts := strings.Split(model, "/")
	return strings.TrimSpace(parts[len(parts)-1])
}

func canonicalizeOpenAIModelAliasSpelling(model string) string {
	model = strings.ToLower(lastOpenAIModelSegment(model))
	if model == "" {
		return ""
	}

	normalized := strings.ReplaceAll(model, "_", "-")
	normalized = strings.Join(strings.Fields(normalized), "-")
	for strings.Contains(normalized, "--") {
		normalized = strings.ReplaceAll(normalized, "--", "-")
	}
	if strings.HasPrefix(normalized, "gpt5") {
		normalized = "gpt-5" + strings.TrimPrefix(normalized, "gpt5")
	}
	if !strings.HasPrefix(normalized, "gpt-") && !strings.Contains(normalized, "codex") {
		return ""
	}

	replacements := []struct{ from, to string }{
		{"gpt-5.4mini", "gpt-5.4-mini"},
		{"gpt-5.4nano", "gpt-5.4-nano"},
		{"gpt-5.3-codexspark", "gpt-5.3-codex-spark"},
		{"gpt-5.3codexspark", "gpt-5.3-codex-spark"},
		{"gpt-5.3codex", "gpt-5.3-codex"},
	}
	for _, replacement := range replacements {
		normalized = strings.ReplaceAll(normalized, replacement.from, replacement.to)
	}
	return normalized
}

func normalizeKnownOpenAICodexModel(model string) string {
	normalized := canonicalizeOpenAIModelAliasSpelling(model)
	if normalized == "" {
		return ""
	}
	if mapped := getNormalizedCodexModel(normalized); mapped != "" {
		return mapped
	}
	if strings.HasSuffix(normalized, "-openai-compact") {
		if mapped := getNormalizedCodexModel(strings.TrimSuffix(normalized, "-openai-compact")); mapped != "" {
			return mapped
		}
	}
	if normalized == "gpt-6" || normalized == "gpt-6-astra" {
		return "gpt-6-astra"
	}
	for _, item := range codexVersionModelPrefixes {
		if normalized == item.prefix {
			return item.target
		}
		if suffix, ok := strings.CutPrefix(normalized, item.prefix+"-"); ok && isKnownCodexModelSuffix(suffix) {
			return item.target
		}
	}
	if normalized == "gpt-5.6" {
		return "gpt-5.6-sol"
	}
	if suffix, ok := strings.CutPrefix(normalized, "gpt-5.6-"); ok {
		switch suffix {
		case "none", "minimal", "low", "medium", "high", "xhigh", "max", "2026-07-09":
			return "gpt-5.6-sol"
		default:
			return ""
		}
	}
	return ""
}
