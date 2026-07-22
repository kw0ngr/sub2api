package xai

import "testing"

func TestDefaultCLIClientHeadersMatchesCurrentGrokBuildWireIdentity(t *testing.T) {
	t.Setenv(EnvCLIClientVersion, "")

	headers := DefaultCLIClientHeaders()
	if got := headers["x-grok-client-version"]; got != DefaultCLIClientVersion {
		t.Fatalf("x-grok-client-version = %q, want %q", got, DefaultCLIClientVersion)
	}
	if DefaultCLIClientVersion != "0.2.109" {
		t.Fatalf("DefaultCLIClientVersion = %q, want current official version 0.2.109", DefaultCLIClientVersion)
	}
	if got := headers["x-grok-client-mode"]; got != DefaultCLIClientMode {
		t.Fatalf("x-grok-client-mode = %q, want %q", got, DefaultCLIClientMode)
	}
	if got := headers["User-Agent"]; got != DefaultCLIUserAgent {
		t.Fatalf("User-Agent = %q, want %q", got, DefaultCLIUserAgent)
	}
}

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
