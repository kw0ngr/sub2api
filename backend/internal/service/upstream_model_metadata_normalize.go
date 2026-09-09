package service

import (
	"encoding/json"
	"sort"
	"strings"
)

func configuredUpstreamModelTargets(account *Account) []string {
	if account == nil {
		return nil
	}
	models := make([]string, 0)
	for _, mappedModel := range account.GetModelMapping() {
		if mappedModel = strings.TrimSpace(mappedModel); mappedModel != "" && !strings.Contains(mappedModel, "*") {
			models = append(models, canonicalUpstreamModelID(mappedModel))
		}
	}
	return dedupeAndSortModelIDs(models)
}

func upstreamModelMetadataIsComplete(modelID string, metadata UpstreamModelMetadata) bool {
	if metadata.Reasoning == nil || len(normalizeCodexInputModalities(metadata.InputModalities)) == 0 || metadata.ContextWindow <= 0 {
		return false
	}
	if *metadata.Reasoning && len(normalizeSnapshotReasoningLevelsForModel(modelID, metadata.SupportedReasoningLevels)) == 0 {
		return false
	}
	if openAIModelSupportsMaxReasoning(canonicalUpstreamModelID(modelID)) && metadata.MaxOutputTokens != 128000 {
		return false
	}
	return true
}

func metadataSnapshotSource(models map[string]UpstreamModelMetadata) string {
	var source string
	for _, model := range models {
		if len(model.Sources) != 1 || (source != "" && source != model.Sources[0]) {
			return "mixed"
		}
		source = model.Sources[0]
	}
	if source == "" {
		return "mixed"
	}
	return source
}

func canonicalUpstreamModelID(modelID string) string {
	modelID = strings.TrimSpace(modelID)
	if canonical := normalizeKnownOpenAICodexModel(modelID); canonical != "" {
		return canonical
	}
	return modelID
}

func isCodexDedicatedMediaModel(modelID string) bool {
	modelID = strings.ToLower(strings.TrimSpace(modelID))
	return strings.HasPrefix(modelID, "gpt-image-") || strings.Contains(modelID, "video")
}

func normalizeMetadataSources(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "upstream" && value != "local_official_default" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func normalizeCodexInputModalities(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "text" && value != "image" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func jsonSnapshotReasoningLevels(raw json.RawMessage) []string {
	var stringsOnly []string
	if err := json.Unmarshal(raw, &stringsOnly); err == nil {
		return stringsOnly
	}
	var objects []struct {
		Effort string `json:"effort"`
	}
	if err := json.Unmarshal(raw, &objects); err != nil {
		return nil
	}
	values := make([]string, 0, len(objects))
	for _, object := range objects {
		values = append(values, object.Effort)
	}
	return values
}

func normalizeSnapshotReasoningLevelsForModel(modelID string, values []string) []string {
	levels := normalizeSnapshotReasoningLevels(values)
	if canonicalUpstreamModelID(modelID) != "gpt-6-astra" {
		return levels
	}
	out := levels[:0]
	for _, level := range levels {
		if level != "none" {
			out = append(out, level)
		}
	}
	return out
}

func normalizeSnapshotReasoningLevels(values []string) []string {
	order := map[string]int{"none": 0, "low": 1, "medium": 2, "high": 3, "xhigh": 4, "max": 5}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		level := normalizeSnapshotReasoningLevel(value)
		if level == "" {
			continue
		}
		if _, ok := seen[level]; ok {
			continue
		}
		seen[level] = struct{}{}
		out = append(out, level)
	}
	sort.Slice(out, func(i, j int) bool { return order[out[i]] < order[out[j]] })
	return out
}

func normalizeSnapshotReasoningLevel(value string) string {
	switch strings.NewReplacer("-", "", "_", "", " ", "").Replace(strings.ToLower(strings.TrimSpace(value))) {
	case "none", "minimal":
		return "none"
	case "low":
		return "low"
	case "medium":
		return "medium"
	case "high":
		return "high"
	case "xhigh", "extrahigh":
		return "xhigh"
	case "max":
		return "max"
	default:
		return ""
	}
}

func unionReasoningLevels(left, right []string) []string {
	return normalizeSnapshotReasoningLevels(append(append([]string{}, left...), right...))
}

func unionStrings(left, right []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(left)+len(right))
	for _, value := range append(append([]string{}, left...), right...) {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func task6BoolPtr(value bool) *bool { return &value }
