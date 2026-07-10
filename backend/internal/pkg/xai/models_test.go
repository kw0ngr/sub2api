package xai

import (
	"encoding/json"
	"testing"
)

func TestDefaultModelsExposeReasoningEffortForGrokCLI(t *testing.T) {
	payload, err := json.Marshal(DefaultModels())
	if err != nil {
		t.Fatal(err)
	}

	var models []struct {
		ID                      string `json:"id"`
		ReasoningEffort         string `json:"reasoning_effort"`
		SupportsReasoningEffort bool   `json:"supports_reasoning_effort"`
	}
	if err := json.Unmarshal(payload, &models); err != nil {
		t.Fatal(err)
	}

	byID := make(map[string]struct {
		effort   string
		supports bool
	}, len(models))
	for _, model := range models {
		byID[model.ID] = struct {
			effort   string
			supports bool
		}{model.ReasoningEffort, model.SupportsReasoningEffort}
	}

	if got := byID["grok-4.5"]; !got.supports || got.effort != "high" {
		t.Fatalf("grok-4.5 reasoning metadata = %+v, want supports=true effort=high", got)
	}
	if got := byID["grok-4.20-0309-non-reasoning"]; got.supports || got.effort != "" {
		t.Fatalf("non-reasoning model metadata = %+v, want supports=false with no effort", got)
	}

	mapped := Models([]string{"grok-4-0709", "grok-imagine-image", "grok", "grok-latest"})
	if !mapped[0].SupportsReasoningEffort || mapped[0].ReasoningEffort != "high" {
		t.Fatalf("mapped reasoning model metadata = %+v, want supports=true effort=high", mapped[0])
	}
	if mapped[1].SupportsReasoningEffort || mapped[1].ReasoningEffort != "" {
		t.Fatalf("mapped media model metadata = %+v, want supports=false with no effort", mapped[1])
	}
	if !mapped[2].SupportsReasoningEffort || mapped[2].ReasoningEffort != "high" {
		t.Fatalf("mapped grok alias metadata = %+v, want supports=true effort=high", mapped[2])
	}
	if !mapped[3].SupportsReasoningEffort || mapped[3].ReasoningEffort != "high" {
		t.Fatalf("mapped grok-latest alias metadata = %+v, want supports=true effort=high", mapped[3])
	}
}
