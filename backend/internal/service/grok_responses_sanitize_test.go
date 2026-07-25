package service

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestSanitizeGrokResponsesBodyDropsUnsupportedFields(t *testing.T) {
	body := []byte(`{
		"model":"grok",
		"max_completion_tokens":128,
		"metadata":{"audit":"drop"},
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
	for _, path := range []string{"max_completion_tokens", "metadata", "prompt_cache_retention", "safety_identifier", "presence_penalty", "stop", "input.0.content.0.external_web_access"} {
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

func TestIsGrokInvalidEncryptedContentResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		want       bool
	}{
		{
			name:       "exact xai decryption error",
			statusCode: 400,
			body:       `{"code":"invalid-argument","error":"Could not decrypt the provided encrypted_content. Ensure the value is unmodified."}`,
			want:       true,
		},
		{
			name:       "wrong status",
			statusCode: 422,
			body:       `{"code":"invalid-argument","error":"Could not decrypt the provided encrypted_content."}`,
			want:       false,
		},
		{
			name:       "wrong code",
			statusCode: 400,
			body:       `{"code":"bad-request","error":"Could not decrypt the provided encrypted_content."}`,
			want:       false,
		},
		{
			name:       "not a decryption error",
			statusCode: 400,
			body:       `{"code":"invalid-argument","error":"The provided encrypted_content is invalid."}`,
			want:       false,
		},
		{
			name:       "nested error is not the observed xai shape",
			statusCode: 400,
			body:       `{"code":"invalid-argument","error":{"message":"Could not decrypt the provided encrypted_content."}}`,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isGrokInvalidEncryptedContentResponse(tt.statusCode, []byte(tt.body)); got != tt.want {
				t.Fatalf("isGrokInvalidEncryptedContentResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}
