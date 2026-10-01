// Package openai provides helpers and types for OpenAI API integration.
package openai

import (
	_ "embed"
	"fmt"
	"strings"
)

// Model represents an OpenAI model
type Model struct {
	ID          string `json:"id"`
	Object      string `json:"object"`
	Created     int64  `json:"created"`
	OwnedBy     string `json:"owned_by"`
	Type        string `json:"type"`
	DisplayName string `json:"display_name"`
}

// DefaultModels OpenAI models list
var DefaultModels = []Model{
	{ID: "gpt-6-astra", Object: "model", Created: 1783555200, OwnedBy: "openai", Type: "model", DisplayName: "GPT-6 Astra"},
	{ID: "gpt-6.1-sol", Object: "model", Created: 1790640000, OwnedBy: "openai", Type: "model", DisplayName: "GPT-6.1 Sol"},
	{ID: "gpt-6-sol", Object: "model", Created: 1790035200, OwnedBy: "openai", Type: "model", DisplayName: "GPT-6 Sol"},
	{ID: "gpt-6-luna", Object: "model", Created: 1790035200, OwnedBy: "openai", Type: "model", DisplayName: "GPT-6 Luna"},
	{ID: "gpt-5.6-sol", Object: "model", Created: 1780876800, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.6 Sol"},
	{ID: "gpt-5.6-terra", Object: "model", Created: 1780876800, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.6 Terra"},
	{ID: "gpt-5.6-luna", Object: "model", Created: 1780876800, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.6 Luna"},
	{ID: "gpt-5.5", Object: "model", Created: 1776873600, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.5"},
	{ID: "gpt-5.4", Object: "model", Created: 1738368000, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.4"},
	{ID: "gpt-5.4-mini", Object: "model", Created: 1738368000, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.4 Mini"},
	{ID: "gpt-5.4-nano", Object: "model", Created: 1738368000, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.4 Nano"},
	{ID: "gpt-5.3-codex", Object: "model", Created: 1735689600, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.3 Codex"},
	{ID: "gpt-5.3-codex-spark", Object: "model", Created: 1735689600, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.3 Codex Spark"},
	{ID: "codex-auto-review", Object: "model", Created: 1776902400, OwnedBy: "openai", Type: "model", DisplayName: "Codex Auto Review"},
	{ID: "gpt-5.2", Object: "model", Created: 1733875200, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.2"},
	{ID: "gpt-5.2-codex", Object: "model", Created: 1733011200, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.2 Codex"},
	{ID: "gpt-5.1-codex-max", Object: "model", Created: 1730419200, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.1 Codex Max"},
	{ID: "gpt-5.1-codex", Object: "model", Created: 1730419200, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.1 Codex"},
	{ID: "gpt-5.1", Object: "model", Created: 1731456000, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.1"},
	{ID: "gpt-5.1-codex-mini", Object: "model", Created: 1730419200, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.1 Codex Mini"},
	{ID: "gpt-5", Object: "model", Created: 1722988800, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5"},
}

// DefaultModelIDs returns the default model ID list
func DefaultModelIDs() []string {
	ids := make([]string, len(DefaultModels))
	for i, m := range DefaultModels {
		ids[i] = m.ID
	}
	return ids
}

// DefaultTestModel is retained for ChatGPT/Codex OAuth probes.
const DefaultTestModel = "gpt-5.1-codex"

// API keys use a published API model, not a retired ChatGPT/Codex identifier.
const DefaultAPIKeyTestModel = "gpt-4o-mini"

// DefaultInstructions default instructions for non-Codex CLI requests
// Content loaded from instructions.txt at compile time
//
//go:embed instructions.txt
var DefaultInstructions string

// IsGPT61SolModelSpelling recognizes the published ID and supported local aliases.
func IsGPT61SolModelSpelling(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	if idx := strings.LastIndexByte(model, '/'); idx >= 0 {
		model = model[idx+1:]
	}
	model = strings.ReplaceAll(model, "_", "-")
	if model == "gpt-6.1-sol" {
		return true
	}
	suffix, ok := strings.CutPrefix(model, "gpt-6.1-sol-")
	if !ok {
		return false
	}
	switch suffix {
	case "none", "minimal", "low", "medium", "high", "xhigh", "max", "openai-compact":
		return true
	default:
		return false
	}
}

// ValidateGPT61SolReasoningEffort keeps unsupported no-reasoning requests visible to callers.
func ValidateGPT61SolReasoningEffort(model, effort string) error {
	if IsGPT61SolModelSpelling(model) {
		switch strings.ToLower(strings.TrimSpace(effort)) {
		case "none", "minimal":
			return fmt.Errorf("gpt-6.1-sol does not support reasoning effort %q; use low, medium, high, xhigh or max", effort)
		}
	}
	return nil
}
