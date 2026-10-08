package service

import (
	"encoding/json"
	"strings"
)

func extractUpstreamModelMetadata(body []byte) map[string]UpstreamModelMetadata {
	entries := extractUpstreamModelRawEntries(body)
	if len(entries) == 0 {
		return nil
	}
	out := map[string]UpstreamModelMetadata{}
	for _, entry := range entries {
		id := canonicalUpstreamModelID(rawModelID(entry))
		if id == "" {
			continue
		}
		metadata := UpstreamModelMetadata{ID: id, Sources: []string{"upstream"}}
		if value := jsonString(entry["display_name"]); value != "" {
			metadata.DisplayName = value
		}
		if value := jsonString(entry["description"]); value != "" {
			metadata.Description = value
		}
		if value, ok := jsonBool(entry["reasoning"]); ok {
			metadata.Reasoning = task6BoolPtr(value)
		}
		metadata.SupportedReasoningLevels = normalizeSnapshotReasoningLevelsForModel(id, jsonSnapshotReasoningLevels(entry["supported_reasoning_levels"]))
		if value := normalizeSnapshotReasoningLevel(jsonString(entry["default_reasoning_level"])); containsString(metadata.SupportedReasoningLevels, value) {
			metadata.DefaultReasoningLevel = value
		}
		metadata.InputModalities = jsonStringArray(entry["input_modalities"])
		metadata.ContextWindow = jsonPositiveInt(entry["context_window"])
		metadata.MaxOutputTokens = jsonPositiveInt(entry["max_output_tokens"])
		metadata.CodexToolCapabilities = parseCodexToolCapabilities(entry)
		out[id] = metadata
	}
	return out
}

func extractUpstreamModelRawEntries(body []byte) []map[string]json.RawMessage {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err == nil && envelope != nil {
		for _, key := range []string{"data", "models"} {
			var entries []map[string]json.RawMessage
			if err := json.Unmarshal(envelope[key], &entries); err == nil && len(entries) > 0 {
				return entries
			}
		}
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(body, &entries); err == nil {
		return entries
	}
	return nil
}

func rawModelID(entry map[string]json.RawMessage) string {
	for _, key := range []string{"id", "slug", "name"} {
		if value := strings.TrimPrefix(jsonString(entry[key]), "models/"); value != "" {
			return value
		}
	}
	return ""
}

func localOfficialUpstreamModelMetadata(modelID string) (UpstreamModelMetadata, bool) {
	canonical := canonicalUpstreamModelID(modelID)
	if canonical == "chat-latest" {
		reasoning := true
		return UpstreamModelMetadata{
			ID: canonical, Sources: []string{"local_official_default"}, DisplayName: "Chat Latest",
			Description: "Latest ChatGPT Instant model routed through Sub2API.", Reasoning: &reasoning,
			DefaultReasoningLevel: "medium", SupportedReasoningLevels: []string{"medium"},
			InputModalities: []string{"text", "image"}, ContextWindow: 400000, MaxOutputTokens: 128000,
			CodexToolCapabilities: map[string]json.RawMessage{"supports_search_tool": json.RawMessage("true"), "use_responses_lite": json.RawMessage("false")},
		}, true
	}
	if !openAIModelSupportsMaxReasoning(canonical) {
		return UpstreamModelMetadata{}, false
	}
	reasoning := true
	levels := []string{"none", "low", "medium", "high", "xhigh", "max"}
	defaultLevel := "medium"
	switch canonical {
	case "gpt-6-astra", "gpt-6.1-sol":
		levels = []string{"low", "medium", "high", "xhigh", "max"}
		if canonical == "gpt-6.1-sol" {
			defaultLevel = "low"
		}
	case "gpt-5.6-sol":
		defaultLevel = "low"
	}
	return UpstreamModelMetadata{
		ID: canonical, Sources: []string{"local_official_default"}, DisplayName: localCodexDisplayName(canonical),
		Description: "OpenAI GPT coding model routed through Sub2API.", Reasoning: &reasoning,
		DefaultReasoningLevel: defaultLevel, SupportedReasoningLevels: levels, InputModalities: []string{"text", "image"},
		ContextWindow: localCodexOpenAIContext, MaxOutputTokens: 128000, CodexToolCapabilities: localOfficialCodexToolCapabilities(canonical),
	}, true
}

func localOfficialCodexToolCapabilities(modelID string) map[string]json.RawMessage {
	caps := map[string]json.RawMessage{"supports_search_tool": json.RawMessage("false"), "use_responses_lite": json.RawMessage("false")}
	if modelID == "gpt-6-astra" {
		caps["supports_search_tool"] = json.RawMessage("true")
		caps["apply_patch_tool_type"] = json.RawMessage(`"freeform"`)
		caps["comp_hash"] = json.RawMessage(`"3000"`)
		caps["tool_mode"] = json.RawMessage("null")
	}
	return caps
}
