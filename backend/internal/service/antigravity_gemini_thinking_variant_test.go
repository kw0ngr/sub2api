package service

import "testing"

func TestResolveGeminiThinkingVariant(t *testing.T) {
	account := &Account{Platform: PlatformAntigravity, Credentials: map[string]any{"model_mapping": map[string]any{
		"gemini-3.8-flash-low": "gemini-3.8-flash-low", "gemini-3.8-flash-high": "gemini-3.8-flash-high",
	}}}
	got, ok := resolveGeminiThinkingVariant(account, "gemini-3.8-flash", []byte(`{"generationConfig":{"thinkingConfig":{"thinkingBudget":1000}}}`))
	if !ok || got != "gemini-3.8-flash-low" {
		t.Fatalf("low variant got=%q matched=%v", got, ok)
	}
	got, ok = resolveGeminiThinkingVariant(account, "gemini-3.8-flash", []byte(`{"generationConfig":{"thinkingConfig":{"thinkingBudget":-1}}}`))
	if !ok || got != "gemini-3.8-flash-high" {
		t.Fatalf("high variant got=%q matched=%v", got, ok)
	}
}
