package xai

import "strings"

// Model describes an xAI model in OpenAI-compatible /models shape.
type Model struct {
	ID                      string `json:"id"`
	Object                  string `json:"object,omitempty"`
	Type                    string `json:"type,omitempty"`
	Created                 int64  `json:"created,omitempty"`
	CreatedAt               string `json:"created_at,omitempty"`
	OwnedBy                 string `json:"owned_by,omitempty"`
	DisplayName             string `json:"display_name,omitempty"`
	ReasoningEffort         string `json:"reasoning_effort,omitempty"`
	SupportsReasoningEffort bool   `json:"supports_reasoning_effort"`
}

var defaultModels = []Model{
	{ID: "grok-4.5", Object: "model", OwnedBy: "xai", DisplayName: "Grok 4.5"},
	{ID: "grok-4.3", Object: "model", OwnedBy: "xai", DisplayName: "Grok 4.3"},
	{ID: "grok-build-0.1", Object: "model", OwnedBy: "xai", DisplayName: "Grok Build 0.1"},
	{ID: "grok-composer-2.5-fast", Object: "model", OwnedBy: "xai", DisplayName: "Grok Composer 2.5 Fast"},
	{ID: "grok-4.20-0309-reasoning", Object: "model", OwnedBy: "xai", DisplayName: "Grok 4.20 Reasoning"},
	{ID: "grok-4.20-0309-non-reasoning", Object: "model", OwnedBy: "xai", DisplayName: "Grok 4.20 Non Reasoning"},
	{ID: "grok-4.20-multi-agent-0309", Object: "model", OwnedBy: "xai", DisplayName: "Grok 4.20 Multi Agent"},
	{ID: "grok-imagine", Object: "model", OwnedBy: "xai", DisplayName: "Grok Imagine"},
	{ID: "grok-imagine-image", Object: "model", OwnedBy: "xai", DisplayName: "Grok Imagine Image"},
	{ID: "grok-imagine-image-quality", Object: "model", OwnedBy: "xai", DisplayName: "Grok Imagine Image Quality"},
	{ID: "grok-imagine-edit", Object: "model", OwnedBy: "xai", DisplayName: "Grok Imagine Edit"},
	{ID: "grok-imagine-video", Object: "model", OwnedBy: "xai", DisplayName: "Grok Imagine Video"},
	{ID: "grok-imagine-video-1.5", Object: "model", OwnedBy: "xai", DisplayName: "Grok Imagine Video 1.5"},
}

func DefaultModels() []Model {
	out := make([]Model, len(defaultModels))
	copy(out, defaultModels)
	for i := range out {
		setReasoningMetadata(&out[i])
	}
	return out
}

func Models(ids []string) []Model {
	models := make([]Model, len(ids))
	for i, id := range ids {
		models[i] = Model{
			ID:          id,
			Type:        "model",
			CreatedAt:   "2024-01-01T00:00:00Z",
			DisplayName: id,
		}
		setReasoningMetadata(&models[i])
	}
	return models
}

func setReasoningMetadata(model *Model) {
	id := strings.ToLower(strings.TrimSpace(model.ID))
	if strings.Contains(id, "non-reasoning") || strings.Contains(id, "imagine") {
		return
	}
	supportsReasoningEffort := id == "grok" || id == "grok-latest" ||
		strings.HasPrefix(id, "grok-4") ||
		strings.HasPrefix(id, "grok-build") ||
		strings.HasPrefix(id, "grok-composer")
	if supportsReasoningEffort {
		model.SupportsReasoningEffort = true
		model.ReasoningEffort = "high"
	}
}

func DefaultModelIDs() []string {
	models := DefaultModels()
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	return ids
}

func DefaultModelMapping() map[string]string {
	mapping := make(map[string]string, len(defaultModels)+5)
	for _, model := range defaultModels {
		mapping[model.ID] = model.ID
	}
	mapping["grok"] = "grok-4.5"
	mapping["grok-latest"] = "grok-4.5"
	mapping["grok-4.5-latest"] = "grok-4.5"
	mapping["grok-build"] = "grok-build-0.1"
	mapping["grok-build-latest"] = "grok-4.5"
	mapping["grok-composer"] = "grok-composer-2.5-fast"
	mapping["composer-2.5"] = "grok-composer-2.5-fast"
	mapping["grok-4.20-reasoning"] = "grok-4.20-0309-reasoning"
	mapping["grok-4.20-non-reasoning"] = "grok-4.20-0309-non-reasoning"
	return mapping
}
