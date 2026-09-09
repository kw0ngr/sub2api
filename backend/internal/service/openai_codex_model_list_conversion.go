package service

import (
	"encoding/json"
	"strings"
)

func convertOpenAIModelListToCodexManifestForAccount(body []byte, account *Account) ([]byte, bool) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil || envelope == nil {
		return body, false
	}
	if _, ok := envelope["models"]; ok {
		return body, false
	}
	var object string
	if err := json.Unmarshal(envelope["object"], &object); err != nil || object != "list" {
		return body, false
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(envelope["data"], &entries); err != nil {
		return body, false
	}
	specBySlug := map[string]localCodexModelSpec{}
	overlays := map[string]map[string]json.RawMessage{}
	for _, entry := range entries {
		var id string
		if err := json.Unmarshal(entry["id"], &id); err != nil {
			continue
		}
		target := id
		if account != nil {
			target = account.GetMappedModel(id)
		}
		spec, ok := localCodexModelSpecForMapping(id, target)
		if !ok {
			continue
		}
		specBySlug[spec.Slug] = spec
		if strings.EqualFold(strings.TrimSpace(id), spec.Slug) {
			overlays[spec.Slug] = entry
		}
	}
	if len(specBySlug) == 0 {
		return body, false
	}
	specs := make([]localCodexModelSpec, 0, len(specBySlug))
	for _, spec := range specBySlug {
		specs = append(specs, spec)
	}
	sortLocalCodexModelSpecs(specs)
	converted, err := buildCodexModelsManifestBody(specs, overlays)
	if err != nil {
		return body, false
	}
	return converted, true
}

func applyOpenAIModelListOverlay(model *localCodexModel, entry map[string]json.RawMessage) {
	if model == nil {
		return
	}
	if value := jsonString(entry["display_name"]); value != "" {
		model.DisplayName = value
	}
	if value := jsonString(entry["description"]); value != "" {
		model.Description = value
	}
	if levels := jsonReasoningLevels(entry["supported_reasoning_levels"]); len(levels) > 0 {
		model.SupportedReasoningLevels = localCodexReasoningLevels(levels...)
		if defaultLevel := normalizeCodexReasoningLevel(jsonString(entry["default_reasoning_level"])); defaultLevel != "" && containsString(levels, defaultLevel) {
			model.DefaultReasoningLevel = defaultLevel
		} else {
			model.DefaultReasoningLevel = levels[0]
		}
	}
	if modalities := jsonStringArray(entry["input_modalities"]); len(modalities) > 0 {
		model.InputModalities = modalities
	}
	if value := jsonPositiveInt(entry["context_window"]); value > 0 {
		model.ContextWindow = value
		model.MaxContextWindow = value
	}
	if value := jsonPositiveInt(entry["max_context_window"]); value > 0 {
		model.MaxContextWindow = value
	}
	if value, ok := jsonBool(entry["supports_search_tool"]); ok {
		model.SupportsSearchTool = value
	}
	if value, ok := jsonBool(entry["use_responses_lite"]); ok {
		model.UseResponsesLite = value
	}
	if value, ok := jsonNullableString(entry["apply_patch_tool_type"]); ok {
		model.ApplyPatchToolType = value
	}
	if value, ok := jsonNullableString(entry["comp_hash"]); ok {
		model.CompHash = value
	}
	if value, ok := jsonNullableString(entry["tool_mode"]); ok {
		model.ToolMode = value
	}
}

func jsonString(raw json.RawMessage) string {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func jsonNullableString(raw json.RawMessage) (any, bool) {
	if string(raw) == "null" {
		return nil, true
	}
	value := jsonString(raw)
	if value == "" {
		return nil, false
	}
	return value, true
}

func jsonBool(raw json.RawMessage) (bool, bool) {
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, false
	}
	return value, true
}

func jsonPositiveInt(raw json.RawMessage) int {
	var value int
	if err := json.Unmarshal(raw, &value); err != nil || value <= 0 {
		return 0
	}
	return value
}

func jsonStringArray(raw json.RawMessage) []string {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil
	}
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

func jsonReasoningLevels(raw json.RawMessage) []string {
	var stringsOnly []string
	if err := json.Unmarshal(raw, &stringsOnly); err == nil {
		return normalizeCodexReasoningLevels(stringsOnly)
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
	return normalizeCodexReasoningLevels(values)
}

func normalizeCodexReasoningLevels(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		normalized := normalizeCodexReasoningLevel(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func normalizeCodexReasoningLevel(value string) string {
	switch strings.NewReplacer("-", "", "_", "", " ", "").Replace(strings.ToLower(strings.TrimSpace(value))) {
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

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
