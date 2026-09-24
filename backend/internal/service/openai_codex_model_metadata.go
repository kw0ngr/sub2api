package service

import (
	"strings"

	openaiapi "github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

const (
	localCodexFallbackContext  = 272000
	localCodexOpenAIContext    = 1050000
	localCodexToolOutputTokens = 10000
)

type localCodexModelSpec struct {
	Slug             string
	MetadataID       string
	ForceAPIKey      bool
	UpstreamMetadata []UpstreamModelMetadata
}

type localCodexReasoningLevel struct {
	Effort      string `json:"effort"`
	Description string `json:"description"`
}

type localCodexTruncationPolicy struct {
	Mode  string `json:"mode"`
	Limit int    `json:"limit"`
}

type localCodexServiceTier struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type localCodexModelMessages struct {
	InstructionsTemplate  string `json:"instructions_template"`
	InstructionsVariables any    `json:"instructions_variables"`
	Approvals             any    `json:"approvals"`
	CollaborationModes    any    `json:"collaboration_modes"`
	AutoReview            any    `json:"auto_review"`
	Permissions           any    `json:"permissions"`
	MultiAgent            any    `json:"multi_agent"`
	TokenBudget           any    `json:"token_budget"`
	GuardianV2            any    `json:"guardian_v2"`
}

type localCodexModel struct {
	Slug                              string                     `json:"slug"`
	DisplayName                       string                     `json:"display_name"`
	Description                       string                     `json:"description"`
	DefaultReasoningLevel             string                     `json:"default_reasoning_level"`
	SupportedReasoningLevels          []localCodexReasoningLevel `json:"supported_reasoning_levels"`
	ShellType                         string                     `json:"shell_type"`
	Visibility                        string                     `json:"visibility"`
	SupportedInAPI                    bool                       `json:"supported_in_api"`
	Priority                          int                        `json:"priority"`
	AdditionalSpeedTiers              []string                   `json:"additional_speed_tiers"`
	ServiceTiers                      []localCodexServiceTier    `json:"service_tiers"`
	DefaultServiceTier                any                        `json:"default_service_tier"`
	AvailabilityNUX                   any                        `json:"availability_nux"`
	Upgrade                           any                        `json:"upgrade"`
	ModelMessages                     localCodexModelMessages    `json:"model_messages"`
	IncludeSkillsUsageInstructions    bool                       `json:"include_skills_usage_instructions"`
	IncludePluginUsageInstructions    bool                       `json:"include_plugin_usage_instructions"`
	IncludeAppsUsageInstructions      bool                       `json:"include_apps_usage_instructions"`
	SupportsReasoningSummaryParameter bool                       `json:"supports_reasoning_summary_parameter"`
	DefaultReasoningSummary           string                     `json:"default_reasoning_summary"`
	SupportVerbosity                  bool                       `json:"support_verbosity"`
	DefaultVerbosity                  any                        `json:"default_verbosity"`
	ApplyPatchToolType                any                        `json:"apply_patch_tool_type"`
	WebSearchToolType                 string                     `json:"web_search_tool_type"`
	TruncationPolicy                  localCodexTruncationPolicy `json:"truncation_policy"`
	SupportsImageDetailOriginal       bool                       `json:"supports_image_detail_original"`
	SupportsParallelToolCalls         bool                       `json:"supports_parallel_tool_calls"`
	ContextWindow                     int                        `json:"context_window"`
	MaxContextWindow                  int                        `json:"max_context_window"`
	AutoCompactTokenLimit             any                        `json:"auto_compact_token_limit"`
	CompHash                          any                        `json:"comp_hash"`
	EffectiveContextWindowPercent     int                        `json:"effective_context_window_percent"`
	ExperimentalSupportedTools        []string                   `json:"experimental_supported_tools"`
	InputModalities                   []string                   `json:"input_modalities"`
	SupportsSearchTool                bool                       `json:"supports_search_tool"`
	UseResponsesLite                  bool                       `json:"use_responses_lite"`
	NodeREPLAutoReviewRequired        bool                       `json:"node_repl_auto_review_required"`
	NodeREPLDisabled                  bool                       `json:"node_repl_disabled"`
	AutoReviewModelOverride           any                        `json:"auto_review_model_override"`
	ModelSpecialty                    any                        `json:"model_specialty"`
	ToolMode                          any                        `json:"tool_mode"`
	MultiAgentVersion                 any                        `json:"multi_agent_version"`
}

func buildLocalCodexModelFromSpec(spec localCodexModelSpec, priority int) localCodexModel {
	metadataID := strings.TrimSpace(spec.MetadataID)
	if metadataID == "" {
		metadataID = spec.Slug
	}
	model := localCodexModel{
		Slug:                              spec.Slug,
		DisplayName:                       localCodexDisplayName(metadataID),
		Description:                       "Custom model routed through Sub2API.",
		DefaultReasoningLevel:             "none",
		SupportedReasoningLevels:          localCodexReasoningLevels("none"),
		ShellType:                         "unified_exec",
		Visibility:                        "list",
		SupportedInAPI:                    true,
		Priority:                          priority,
		AdditionalSpeedTiers:              []string{},
		ServiceTiers:                      []localCodexServiceTier{},
		ModelMessages:                     localCodexModelMessages{InstructionsTemplate: "You are Codex, a coding agent."},
		SupportsReasoningSummaryParameter: true,
		DefaultReasoningSummary:           "auto",
		WebSearchToolType:                 "text",
		TruncationPolicy:                  localCodexTruncationPolicy{Mode: "tokens", Limit: localCodexToolOutputTokens},
		ContextWindow:                     localCodexFallbackContext,
		MaxContextWindow:                  localCodexFallbackContext,
		EffectiveContextWindowPercent:     95,
		ExperimentalSupportedTools:        []string{},
		InputModalities:                   []string{"text"},
	}
	baseModel := openAIBaseModelIDForEffortSupport(metadataID)
	if baseModel == "" {
		baseModel = strings.ToLower(strings.TrimSpace(metadataID))
	}
	if strings.HasPrefix(baseModel, "gpt-") {
		model.Description = "OpenAI GPT coding model routed through Sub2API."
		model.SupportsParallelToolCalls = true
		model.InputModalities = []string{"text", "image"}
		model.ContextWindow = localCodexOpenAIContext
		model.MaxContextWindow = localCodexOpenAIContext
		model.SupportVerbosity = true
		model.DefaultVerbosity = "low"
		model.DefaultReasoningSummary = "none"
	}
	switch {
	case openAIModelSupportsMaxReasoning(baseModel):
		model.DefaultReasoningLevel = "medium"
		if baseModel == "gpt-5.6-sol" {
			model.DefaultReasoningLevel = "low"
		}
		model.SupportedReasoningLevels = localCodexReasoningLevels("low", "medium", "high", "xhigh", "max")
		if isOpenAIGPT6SolOrLunaModel(baseModel) {
			model.SupportedReasoningLevels = localCodexReasoningLevels("none", "low", "medium", "high", "xhigh", "max")
		}
		model.ServiceTiers = localCodexFastServiceTiers()
	case strings.HasPrefix(baseModel, "gpt-5"):
		model.DefaultReasoningLevel = "medium"
		model.SupportedReasoningLevels = localCodexReasoningLevels("low", "medium", "high", "xhigh")
		model.ServiceTiers = localCodexFastServiceTiers()
	case strings.HasPrefix(baseModel, "o1"), strings.HasPrefix(baseModel, "o3"), strings.HasPrefix(baseModel, "o4"):
		model.DefaultReasoningLevel = "medium"
		model.SupportedReasoningLevels = localCodexReasoningLevels("low", "medium", "high")
	}
	if baseModel == "gpt-6-astra" {
		model.SupportsSearchTool = true
		model.ApplyPatchToolType = "freeform"
		model.CompHash = "3000"
	}
	if len(spec.UpstreamMetadata) > 0 {
		applyUpstreamMetadataToLocalCodexModel(&model, unionUpstreamModelMetadata(spec.UpstreamMetadata), spec.ForceAPIKey)
	}
	if spec.ForceAPIKey {
		model.UseResponsesLite = false
	}
	return model
}

func localCodexDisplayName(modelID string) string {
	canonical := normalizeKnownOpenAICodexModel(modelID)
	if canonical == "" {
		canonical = strings.TrimSpace(modelID)
	}
	for _, model := range openaiapi.DefaultModels {
		if strings.EqualFold(model.ID, canonical) && strings.TrimSpace(model.DisplayName) != "" {
			return model.DisplayName
		}
	}
	return strings.TrimSpace(modelID)
}

func localCodexFastServiceTiers() []localCodexServiceTier {
	return []localCodexServiceTier{{
		ID:          OpenAIFastTierPriority,
		Name:        "Fast",
		Description: "Priority processing for lower latency.",
	}}
}

func localCodexReasoningLevels(efforts ...string) []localCodexReasoningLevel {
	levels := make([]localCodexReasoningLevel, 0, len(efforts))
	for _, effort := range efforts {
		levels = append(levels, localCodexReasoningLevel{Effort: effort, Description: localCodexReasoningDescription(effort)})
	}
	return levels
}

func localCodexReasoningDescription(effort string) string {
	switch effort {
	case "none":
		return "Use the model's default behavior without configurable reasoning"
	case "low":
		return "Fast responses with lighter reasoning"
	case "medium":
		return "Balanced reasoning for most coding tasks"
	case "high":
		return "Greater reasoning depth for coding and agent tasks"
	case "xhigh":
		return "Extra-high reasoning depth for difficult tasks"
	case "max":
		return "Maximum reasoning depth for complex tasks"
	default:
		return "Reasoning effort supported by the upstream model"
	}
}
