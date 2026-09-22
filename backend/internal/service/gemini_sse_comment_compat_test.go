package service

import "testing"

func TestGeminiClientRejectsSSEComments(t *testing.T) {
	for _, test := range []struct {
		hint string
		want bool
	}{
		{"google-genai-sdk/1.71.0 gl-go/go1.28", true},
		{"google-genai-sdk/1.20.0 gl-python/3.12", true},
		{"google-genai-sdk/1.9.0 gl-node/22.3.0", false},
		{"curl/8.7.1", false},
	} {
		if got := geminiClientRejectsSSEComments(test.hint); got != test.want {
			t.Fatalf("geminiClientRejectsSSEComments(%q)=%v want %v", test.hint, got, test.want)
		}
	}
}
