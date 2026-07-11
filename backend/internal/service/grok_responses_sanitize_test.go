package service

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestSanitizeGrokResponsesBodyDropsUnsupportedFields(t *testing.T) {
	body := []byte(`{
		"model":"grok",
		"prompt_cache_retention":"24h",
		"safety_identifier":"user",
		"presence_penalty":0.5,
		"stop":["x"],
		"input":[{"content":[{"external_web_access":true,"text":"hi"}]}]
	}`)

	out, changed, err := sanitizeGrokResponsesBody(body, "grok-4.5")

	if err != nil {
		t.Fatalf("sanitizeGrokResponsesBody returned error: %v", err)
	}
	if !changed {
		t.Fatal("sanitizeGrokResponsesBody changed=false, want true")
	}
	for _, path := range []string{"prompt_cache_retention", "safety_identifier", "presence_penalty", "stop", "input.0.content.0.external_web_access"} {
		if gjson.GetBytes(out, path).Exists() {
			t.Fatalf("sanitized body still contains %s: %s", path, string(out))
		}
	}
	if got := gjson.GetBytes(out, "model").String(); got != "grok-4.5" {
		t.Fatalf("model = %q, want grok-4.5", got)
	}
}

func TestSanitizeGrokResponsesBodyFiltersUnsupportedTools(t *testing.T) {
	body := []byte(`{
		"model":"grok-4.3",
		"tools":[
			{"type":"function","name":"keep"},
			{"type":"image_generation"},
			{"type":"web_search"}
		],
		"tool_choice":{"type":"function","name":"drop"}
	}`)

	out, changed, err := sanitizeGrokResponsesBody(body, "grok-4.3")

	if err != nil {
		t.Fatalf("sanitizeGrokResponsesBody returned error: %v", err)
	}
	if !changed {
		t.Fatal("sanitizeGrokResponsesBody changed=false, want true")
	}
	tools := gjson.GetBytes(out, "tools").Array()
	if len(tools) != 2 {
		t.Fatalf("tools length = %d, want 2: %s", len(tools), string(out))
	}
	if gjson.GetBytes(out, "tool_choice").Exists() {
		t.Fatalf("tool_choice should be removed when target function was filtered: %s", string(out))
	}
}
