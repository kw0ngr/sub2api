package geminicli

import "testing"

func TestDefaultTestModel_UsesAvailableStableModel(t *testing.T) {
	t.Parallel()

	if DefaultTestModel != "gemini-3.5-flash-lite" {
		t.Fatalf("expected default Gemini test model to be gemini-3.5-flash-lite, got %q", DefaultTestModel)
	}

	for _, model := range DefaultModels {
		if model.ID == "gemini-2.0-flash" || model.ID == "gemini-2.5-flash-image" {
			t.Fatalf("retired %s must not remain in the curated test model list", model.ID)
		}
	}
}

func TestDefaultModels_ContainsImageModels(t *testing.T) {
	t.Parallel()

	byID := make(map[string]Model, len(DefaultModels))
	for _, model := range DefaultModels {
		byID[model.ID] = model
	}

	required := []string{
		"gemini-3.6-flash",
		"gemini-3.5-flash-lite",
		"gemini-3.1-flash-lite",
		"gemini-3.7-flash",
		"gemini-3.8-flash",
		"gemini-nano-banana-2.1",
		"gemini-3.5-flash",
		"gemini-3.1-flash-image",
	}

	for _, id := range required {
		if _, ok := byID[id]; !ok {
			t.Fatalf("expected curated Gemini model %q to exist", id)
		}
	}
}
