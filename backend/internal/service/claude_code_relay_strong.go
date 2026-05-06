package service

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/tidwall/gjson"
)

func setAnthropicAPIKeyMimicAuthHeader(header http.Header, account *Account, token string) {
	if account != nil && account.IsClaudeCodeRelayStrongModeEnabled() {
		header.Del("authorization")
		header.Del("x-api-key")
		header.Del("x-goog-api-key")
		setHeaderRaw(header, "authorization", "Bearer "+token)
		return
	}
	setAnthropicCompatibleAPIKeyAuthHeader(header, accountPlatform(account), token)
}

func accountPlatform(account *Account) string {
	if account == nil {
		return ""
	}
	return account.Platform
}

func claudeCodeAPIKeyMimicryBetasForAccount(account *Account, modelID string, body []byte) []string {
	if account != nil && account.IsClaudeCodeRelayStrongModeEnabled() {
		return claudeCodeMimicryBetas(modelID, body)
	}
	return claudeCodeAPIKeyMimicryBetas(modelID, body)
}

func claudeCodeAPIKeyMimicDropSetForAccount(account *Account, policySet map[string]struct{}) map[string]struct{} {
	if account != nil && account.IsClaudeCodeRelayStrongModeEnabled() {
		return mergeDropSets(policySet)
	}
	return mergeDropSets(policySet, claude.BetaOAuth)
}

func claudeCodeMimicTokenLabel(base string, account *Account) string {
	if account != nil && account.IsClaudeCodeRelayStrongModeEnabled() {
		if strings.TrimSpace(base) == "" {
			return "apikey-relay-strong"
		}
		return strings.TrimSpace(base) + "-relay-strong"
	}
	return base
}

func buildClaudeMimicDiagnosticData(
	req *http.Request,
	body []byte,
	account *Account,
	tokenType string,
	mimicClaudeCode bool,
	activeFingerprintApplied bool,
	tlsFingerprintEnabled bool,
	tlsProfileName string,
) map[string]any {
	beta := ""
	userAgent := ""
	authScheme := ""
	if req != nil {
		beta = getHeaderRaw(req.Header, "anthropic-beta")
		userAgent = getHeaderRaw(req.Header, "user-agent")
		switch {
		case getHeaderRaw(req.Header, "authorization") != "":
			authScheme = "bearer"
		case getHeaderRaw(req.Header, "x-api-key") != "":
			authScheme = "x-api-key"
		default:
			authScheme = "none"
		}
	}

	systemPreview := extractSystemPreviewFromBody(body)
	return map[string]any{
		"mimic_enabled":        mimicClaudeCode,
		"relay_strong":         account != nil && account.IsClaudeCodeRelayStrongModeEnabled(),
		"token_type":           tokenType,
		"auth_scheme":          authScheme,
		"user_agent":           userAgent,
		"active_fingerprint":   activeFingerprintApplied,
		"tls_fingerprint":      tlsFingerprintEnabled,
		"tls_profile":          strings.TrimSpace(tlsProfileName),
		"has_claude_code_beta": strings.Contains(beta, claude.BetaClaudeCode),
		"has_oauth_beta":       strings.Contains(beta, claude.BetaOAuth),
		"has_extended_ttl":     strings.Contains(beta, claude.BetaExtendedCacheTTL),
		"metadata_user_id":     strings.TrimSpace(extractMetadataUserID(body)) != "",
		"billing_header":       strings.Contains(systemPreview, "x-anthropic-billing-header"),
		"cch_signed":           strings.Contains(systemPreview, "cch=") && !strings.Contains(systemPreview, "cch=00000"),
	}
}

func extractMetadataUserID(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	return strings.TrimSpace(gjson.GetBytes(body, "metadata.user_id").String())
}
