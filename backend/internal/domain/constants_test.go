package domain

import "testing"

func TestDefaultAntigravityModelMapping_ImageCompatibilityAliases(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"gemini-2.5-flash-image":         "gemini-2.5-flash-image",
		"gemini-2.5-flash-image-preview": "gemini-2.5-flash-image",
		"gemini-3.1-flash-image":         "gemini-3.1-flash-image",
		"gemini-3.1-flash-image-preview": "gemini-3.1-flash-image",
		"gemini-3-pro-image":             "gemini-3.1-flash-image",
		"gemini-3-pro-image-preview":     "gemini-3.1-flash-image",
	}

	for from, want := range cases {
		got, ok := DefaultAntigravityModelMapping[from]
		if !ok {
			t.Fatalf("expected mapping for %q to exist", from)
		}
		if got != want {
			t.Fatalf("unexpected mapping for %q: got %q want %q", from, got, want)
		}
	}
}

func TestDefaultModelMappings_ClaudeOpus48(t *testing.T) {
	t.Parallel()

	if got := DefaultAntigravityModelMapping["claude-opus-4-8"]; got != "claude-opus-4-8" {
		t.Fatalf("DefaultAntigravityModelMapping[claude-opus-4-8] = %q, want claude-opus-4-8", got)
	}

	if got := DefaultBedrockModelMapping["claude-opus-4-8"]; got != "us.anthropic.claude-opus-4-8-v1" {
		t.Fatalf("DefaultBedrockModelMapping[claude-opus-4-8] = %q, want us.anthropic.claude-opus-4-8-v1", got)
	}
}
