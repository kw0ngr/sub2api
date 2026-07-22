package xai

import "testing"

func TestIsCLIChatProxyBaseURLMatchesExactHost(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    bool
	}{
		{name: "official CLI base", baseURL: "https://cli-chat-proxy.grok.com/v1", want: true},
		{name: "official CLI base uppercase", baseURL: "HTTPS://CLI-CHAT-PROXY.GROK.COM/v1/", want: true},
		{name: "hostname suffix spoof", baseURL: "https://cli-chat-proxy.grok.com.example.test/v1", want: false},
		{name: "hostname in path", baseURL: "https://relay.example.test/cli-chat-proxy.grok.com/v1", want: false},
		{name: "empty", baseURL: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCLIChatProxyBaseURL(tt.baseURL); got != tt.want {
				t.Fatalf("IsCLIChatProxyBaseURL(%q) = %v, want %v", tt.baseURL, got, tt.want)
			}
		})
	}
}
