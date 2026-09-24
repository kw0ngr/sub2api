package service

import (
	"encoding/json"
	"strings"
)

func applyUpstreamMetadataToLocalCodexModel(model *localCodexModel, metadata UpstreamModelMetadata, forceAPIKey bool) {
	if model == nil {
		return
	}
	if value := strings.TrimSpace(metadata.DisplayName); value != "" {
		model.DisplayName = value
	}
	if value := strings.TrimSpace(metadata.Description); value != "" {
		model.Description = value
	}
	if metadata.Reasoning != nil && *metadata.Reasoning {
		levels := normalizeSnapshotReasoningLevelsForModel(metadata.ID, metadata.SupportedReasoningLevels)
		uiLevels := make([]string, 0, len(levels))
		allowNone := isOpenAIGPT6SolOrLunaModel(metadata.ID)
		for _, level := range levels {
			if level != "none" || allowNone {
				uiLevels = append(uiLevels, level)
			}
		}
		if len(uiLevels) > 0 {
			model.SupportedReasoningLevels = localCodexReasoningLevels(uiLevels...)
			if defaultLevel := normalizeSnapshotReasoningLevel(metadata.DefaultReasoningLevel); containsString(uiLevels, defaultLevel) {
				model.DefaultReasoningLevel = defaultLevel
			} else {
				model.DefaultReasoningLevel = uiLevels[0]
			}
		}
	}
	if len(metadata.InputModalities) > 0 {
		model.InputModalities = normalizeCodexInputModalities(metadata.InputModalities)
	}
	if metadata.ContextWindow > 0 {
		model.ContextWindow = metadata.ContextWindow
		model.MaxContextWindow = metadata.ContextWindow
	}
	applySnapshotCodexToolCapabilities(model, metadata.CodexToolCapabilities)
	if forceAPIKey {
		model.UseResponsesLite = false
	}
}

func applySnapshotCodexToolCapabilities(model *localCodexModel, caps map[string]json.RawMessage) {
	if value, ok := jsonBool(caps["supports_search_tool"]); ok {
		model.SupportsSearchTool = value
	}
	if value, ok := jsonBool(caps["use_responses_lite"]); ok {
		model.UseResponsesLite = value
	}
	if value, ok := jsonNullableString(caps["apply_patch_tool_type"]); ok {
		model.ApplyPatchToolType = value
	}
	if value, ok := jsonNullableString(caps["comp_hash"]); ok {
		model.CompHash = value
	}
	if value, ok := jsonNullableString(caps["tool_mode"]); ok {
		model.ToolMode = value
	}
}
