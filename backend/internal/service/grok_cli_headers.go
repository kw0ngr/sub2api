package service

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/google/uuid"
)

// grokCLIForwardedContextHeaders are official Grok CLI identity/tracing
// headers that are safe to preserve across this gateway. Authentication
// selectors are deliberately excluded and are supplied from trusted defaults
// or per-account credentials instead.
var grokCLIForwardedContextHeaders = []string{
	"x-grok-client-version",
	"x-grok-client-identifier",
	"x-grok-client-mode",
	"x-grok-conv-id",
	"x-grok-req-id",
	"x-grok-session-id",
	"x-grok-agent-id",
	"x-grok-turn-idx",
	"x-grok-deployment-id",
	"x-grok-user-id",
}

// copyGrokCLIRequestContextHeaders preserves the official CLI request context
// before the normal OpenAI-compatible header allowlist is applied. Account
// credentials.headers overrides are applied afterwards.
func copyGrokCLIRequestContextHeaders(dst, src http.Header) {
	if dst == nil || src == nil {
		return
	}
	for _, name := range grokCLIForwardedContextHeaders {
		if value := strings.TrimSpace(src.Get(name)); value != "" {
			dst.Set(name, value)
		}
	}
}

// applyGrokCLIClientHeaders injects the static cli-chat-proxy identity
// contract. Explicit per-account credentials.headers values take precedence;
// existing request headers otherwise take precedence over fallback defaults.
func applyGrokCLIClientHeaders(h http.Header, account *Account) {
	if h == nil {
		return
	}
	for _, name := range []string{
		"chatgpt-account-id",
		"conversation_id",
		"OpenAI-Beta",
		"originator",
		"session_id",
		"x-codex-turn-metadata",
		"x-codex-turn-state",
	} {
		h.Del(name)
	}
	if userAgent := strings.TrimSpace(h.Get("User-Agent")); userAgent != "" &&
		!strings.HasPrefix(strings.ToLower(userAgent), "grok-shell/") {
		h.Del("User-Agent")
	}
	defaults := xai.DefaultCLIClientHeaders()

	// Optional per-account overrides stored in credentials.headers or flat
	// credential keys. This retains compatibility with imported CPA accounts.
	overrides := map[string]string{}
	if account != nil {
		if raw, ok := account.Credentials["headers"]; ok {
			switch v := raw.(type) {
			case map[string]any:
				for k, val := range v {
					if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
						overrides[k] = strings.TrimSpace(s)
					}
				}
			case map[string]string:
				for k, s := range v {
					if strings.TrimSpace(s) != "" {
						overrides[k] = strings.TrimSpace(s)
					}
				}
			}
		}
		for _, key := range []string{
			"x-grok-client-version",
			"x-xai-token-auth",
			"x-authenticateresponse",
			"x-grok-client-identifier",
			"x-grok-client-mode",
			"user_agent",
			"User-Agent",
		} {
			if s := strings.TrimSpace(account.GetCredential(key)); s != "" {
				overrides[key] = s
			}
		}
	}

	setIfEmpty := func(name, value string) {
		if strings.TrimSpace(value) == "" || strings.TrimSpace(h.Get(name)) != "" {
			return
		}
		h.Set(name, value)
	}
	for k, v := range overrides {
		if strings.EqualFold(k, "user_agent") {
			k = "User-Agent"
		}
		h.Set(k, v)
	}
	for k, v := range defaults {
		setIfEmpty(k, v)
	}
}

// applyGrokCLIRequestHeaders completes the per-inference request fingerprint
// used by current grok-build clients. Client-provided IDs are preserved, and
// missing IDs are generated. The model override is always rewritten to the
// actual mapped upstream model so a caller cannot make the header disagree
// with the body selected for routing and billing.
func applyGrokCLIRequestHeaders(h http.Header, account *Account, upstreamModel string) {
	if h == nil {
		return
	}
	applyGrokCLIClientHeaders(h, account)

	sessionID := strings.TrimSpace(h.Get("x-grok-session-id"))
	if sessionID == "" {
		sessionID = strings.TrimSpace(h.Get("x-grok-conv-id"))
	}
	if sessionID == "" {
		sessionID = uuid.NewString()
	}

	setIfEmpty := func(name, value string) {
		if strings.TrimSpace(h.Get(name)) == "" {
			h.Set(name, value)
		}
	}
	setIfEmpty("x-grok-conv-id", sessionID)
	setIfEmpty("x-grok-session-id", sessionID)
	setIfEmpty("x-grok-req-id", "xai-"+uuid.NewString())
	setIfEmpty("x-grok-agent-id", uuid.NewString())
	setIfEmpty("x-grok-turn-idx", "0")

	if upstreamModel = strings.TrimSpace(upstreamModel); upstreamModel != "" {
		h.Set("x-grok-model-override", upstreamModel)
	}
}
