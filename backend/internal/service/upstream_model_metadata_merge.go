package service

import (
	"encoding/json"
	"strings"
)

func fillUpstreamModelMetadata(primary, fallback UpstreamModelMetadata) UpstreamModelMetadata {
	merged := primary
	if strings.TrimSpace(merged.ID) == "" {
		merged.ID = strings.TrimSpace(fallback.ID)
	}
	if strings.TrimSpace(merged.DisplayName) == "" {
		merged.DisplayName = strings.TrimSpace(fallback.DisplayName)
	}
	if strings.TrimSpace(merged.Description) == "" {
		merged.Description = strings.TrimSpace(fallback.Description)
	}
	if merged.Reasoning == nil && fallback.Reasoning != nil {
		merged.Reasoning = task6BoolPtr(*fallback.Reasoning)
	}
	if len(merged.SupportedReasoningLevels) == 0 {
		merged.SupportedReasoningLevels = append([]string(nil), fallback.SupportedReasoningLevels...)
	}
	if strings.TrimSpace(merged.DefaultReasoningLevel) == "" {
		merged.DefaultReasoningLevel = strings.TrimSpace(fallback.DefaultReasoningLevel)
	}
	if len(merged.InputModalities) == 0 {
		merged.InputModalities = append([]string(nil), fallback.InputModalities...)
	}
	if merged.ContextWindow <= 0 {
		merged.ContextWindow = fallback.ContextWindow
	}
	if merged.MaxOutputTokens <= 0 {
		merged.MaxOutputTokens = fallback.MaxOutputTokens
	}
	merged.Sources = normalizeMetadataSources(append(merged.Sources, fallback.Sources...))
	merged.CodexToolCapabilities = fillCodexToolCapabilities(merged.CodexToolCapabilities, fallback.CodexToolCapabilities)
	return merged
}

func unionUpstreamModelMetadata(values []UpstreamModelMetadata) UpstreamModelMetadata {
	var merged UpstreamModelMetadata
	for _, value := range values {
		if strings.TrimSpace(merged.ID) == "" {
			merged.ID = strings.TrimSpace(value.ID)
		}
		if strings.TrimSpace(merged.DisplayName) == "" {
			merged.DisplayName = strings.TrimSpace(value.DisplayName)
		}
		if strings.TrimSpace(merged.Description) == "" {
			merged.Description = strings.TrimSpace(value.Description)
		}
		if merged.Reasoning == nil && value.Reasoning != nil {
			merged.Reasoning = task6BoolPtr(*value.Reasoning)
		}
		if strings.TrimSpace(merged.DefaultReasoningLevel) == "" {
			merged.DefaultReasoningLevel = strings.TrimSpace(value.DefaultReasoningLevel)
		}
		merged.SupportedReasoningLevels = unionReasoningLevels(merged.SupportedReasoningLevels, value.SupportedReasoningLevels)
		merged.InputModalities = unionStrings(merged.InputModalities, value.InputModalities)
		if merged.ContextWindow <= 0 {
			merged.ContextWindow = value.ContextWindow
		}
		if merged.MaxOutputTokens <= 0 {
			merged.MaxOutputTokens = value.MaxOutputTokens
		}
		merged.Sources = normalizeMetadataSources(append(merged.Sources, value.Sources...))
		merged.CodexToolCapabilities = unionCodexToolCapabilities(merged.CodexToolCapabilities, value.CodexToolCapabilities)
	}
	return merged
}

func parseCodexToolCapabilities(entry map[string]json.RawMessage) map[string]json.RawMessage {
	caps := map[string]json.RawMessage{}
	var nested map[string]json.RawMessage
	if err := json.Unmarshal(entry["codex_tool_capabilities"], &nested); err == nil {
		copyValidCodexToolCapabilities(caps, nested)
	}
	copyValidCodexToolCapabilities(caps, entry)
	if len(caps) == 0 {
		return nil
	}
	return caps
}

func copyValidCodexToolCapabilities(dst, src map[string]json.RawMessage) {
	for _, key := range []string{"supports_search_tool", "use_responses_lite"} {
		if value, ok := jsonBool(src[key]); ok {
			encoded, _ := json.Marshal(value)
			dst[key] = encoded
		}
	}
	for _, key := range []string{"apply_patch_tool_type", "comp_hash", "tool_mode"} {
		if value, ok := jsonNullableString(src[key]); ok {
			encoded, _ := json.Marshal(value)
			dst[key] = encoded
		}
	}
}

func fillCodexToolCapabilities(primary, fallback map[string]json.RawMessage) map[string]json.RawMessage {
	if len(primary) == 0 && len(fallback) == 0 {
		return nil
	}
	out := cloneRawMessageMap(primary)
	for key, value := range fallback {
		if _, ok := out[key]; !ok {
			out[key] = append(json.RawMessage(nil), value...)
		}
	}
	return out
}

func unionCodexToolCapabilities(primary, fallback map[string]json.RawMessage) map[string]json.RawMessage {
	out := fillCodexToolCapabilities(primary, fallback)
	for _, key := range []string{"supports_search_tool", "use_responses_lite"} {
		left, leftOK := jsonBool(primary[key])
		right, rightOK := jsonBool(fallback[key])
		if leftOK || rightOK {
			encoded, _ := json.Marshal(left || right)
			out[key] = encoded
		}
	}
	return out
}

func cloneRawMessageMap(in map[string]json.RawMessage) map[string]json.RawMessage {
	if len(in) == 0 {
		return map[string]json.RawMessage{}
	}
	out := make(map[string]json.RawMessage, len(in))
	for key, value := range in {
		out[key] = append(json.RawMessage(nil), value...)
	}
	return out
}
