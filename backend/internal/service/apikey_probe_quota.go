package service

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	APIKeyProbeQuotaExtraKey          = "apikey_probe_quota"
	APIKeyProbeQuotaUpdatedAtExtraKey = "apikey_probe_quota_updated_at"
)

type APIKeyProbeQuotaSnapshot struct {
	Provider                 string `json:"provider"`
	Supported                bool   `json:"supported"`
	Source                   string `json:"source"`
	UpdatedAt                string `json:"updated_at"`
	StatusCode               int    `json:"status_code,omitempty"`
	Model                    string `json:"model,omitempty"`
	RequestsLimit            string `json:"requests_limit,omitempty"`
	RequestsRemaining        string `json:"requests_remaining,omitempty"`
	RequestsReset            string `json:"requests_reset,omitempty"`
	TokensLimit              string `json:"tokens_limit,omitempty"`
	TokensRemaining          string `json:"tokens_remaining,omitempty"`
	TokensReset              string `json:"tokens_reset,omitempty"`
	InputTokensLimit         string `json:"input_tokens_limit,omitempty"`
	InputTokensRemaining     string `json:"input_tokens_remaining,omitempty"`
	InputTokensReset         string `json:"input_tokens_reset,omitempty"`
	OutputTokensLimit        string `json:"output_tokens_limit,omitempty"`
	OutputTokensRemaining    string `json:"output_tokens_remaining,omitempty"`
	OutputTokensReset        string `json:"output_tokens_reset,omitempty"`
	RetryAfter               string `json:"retry_after,omitempty"`
	RateLimitPolicy          string `json:"rate_limit_policy,omitempty"`
	QuotaProject             string `json:"quota_project,omitempty"`
	Note                     string `json:"note,omitempty"`
	HasRateLimitHeaderSignal bool   `json:"has_rate_limit_header_signal"`
}

func BuildAPIKeyProbeQuotaSnapshot(platform string, statusCode int, model string, headers http.Header, now time.Time) *APIKeyProbeQuotaSnapshot {
	provider := strings.TrimSpace(platform)
	if provider == "" {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}

	snapshot := &APIKeyProbeQuotaSnapshot{
		Provider:   provider,
		UpdatedAt:  now.Format(time.RFC3339),
		StatusCode: statusCode,
		Model:      strings.TrimSpace(model),
	}

	switch provider {
	case PlatformAnthropic:
		snapshot.Supported = true
		snapshot.RequestsLimit = headerValue(headers, "anthropic-ratelimit-requests-limit")
		snapshot.RequestsRemaining = headerValue(headers, "anthropic-ratelimit-requests-remaining")
		snapshot.RequestsReset = headerValue(headers, "anthropic-ratelimit-requests-reset")
		snapshot.TokensLimit = headerValue(headers, "anthropic-ratelimit-tokens-limit")
		snapshot.TokensRemaining = headerValue(headers, "anthropic-ratelimit-tokens-remaining")
		snapshot.TokensReset = headerValue(headers, "anthropic-ratelimit-tokens-reset")
		snapshot.InputTokensLimit = headerValue(headers, "anthropic-ratelimit-input-tokens-limit")
		snapshot.InputTokensRemaining = headerValue(headers, "anthropic-ratelimit-input-tokens-remaining")
		snapshot.InputTokensReset = headerValue(headers, "anthropic-ratelimit-input-tokens-reset")
		snapshot.OutputTokensLimit = headerValue(headers, "anthropic-ratelimit-output-tokens-limit")
		snapshot.OutputTokensRemaining = headerValue(headers, "anthropic-ratelimit-output-tokens-remaining")
		snapshot.OutputTokensReset = headerValue(headers, "anthropic-ratelimit-output-tokens-reset")
		snapshot.RetryAfter = headerValue(headers, "retry-after")
	case PlatformOpenAI:
		snapshot.Supported = true
		snapshot.RequestsLimit = headerValue(headers, "x-ratelimit-limit-requests")
		snapshot.RequestsRemaining = headerValue(headers, "x-ratelimit-remaining-requests")
		snapshot.RequestsReset = headerValue(headers, "x-ratelimit-reset-requests")
		snapshot.TokensLimit = headerValue(headers, "x-ratelimit-limit-tokens")
		snapshot.TokensRemaining = headerValue(headers, "x-ratelimit-remaining-tokens")
		snapshot.TokensReset = headerValue(headers, "x-ratelimit-reset-tokens")
		snapshot.RetryAfter = headerValue(headers, "retry-after")
	case PlatformGemini:
		snapshot.Supported = false
		snapshot.RequestsLimit = firstHeaderValue(headers, "x-ratelimit-limit-requests", "x-ratelimit-limit")
		snapshot.RequestsRemaining = firstHeaderValue(headers, "x-ratelimit-remaining-requests", "x-ratelimit-remaining")
		snapshot.RequestsReset = firstHeaderValue(headers, "x-ratelimit-reset-requests", "x-ratelimit-reset")
		snapshot.RetryAfter = headerValue(headers, "retry-after")
		snapshot.RateLimitPolicy = headerValue(headers, "x-ratelimit-policy")
		snapshot.QuotaProject = headerValue(headers, "x-goog-quota-project")
		snapshot.Note = "Gemini API key quotas are project-level; this gateway records response headers when Google returns them."
	default:
		return nil
	}

	snapshot.HasRateLimitHeaderSignal = snapshot.hasHeaderSignal()
	if snapshot.HasRateLimitHeaderSignal {
		snapshot.Source = "headers"
		if provider == PlatformGemini {
			snapshot.Supported = true
		}
	} else if provider == PlatformGemini {
		snapshot.Source = "project_quota"
	} else {
		snapshot.Source = "missing_headers"
		snapshot.Note = "The upstream response did not include rate-limit quota headers for this probe."
	}
	return snapshot
}

func BuildAPIKeyProbeQuotaExtraUpdates(snapshot *APIKeyProbeQuotaSnapshot) map[string]any {
	if snapshot == nil {
		return nil
	}
	payload := snapshot.toMap()
	return map[string]any{
		APIKeyProbeQuotaExtraKey:          payload,
		APIKeyProbeQuotaUpdatedAtExtraKey: snapshot.UpdatedAt,
	}
}

func APIKeyProbeQuotaSnapshotFromExtra(extra map[string]any) *APIKeyProbeQuotaSnapshot {
	if len(extra) == 0 {
		return nil
	}
	raw, ok := extra[APIKeyProbeQuotaExtraKey]
	if !ok || raw == nil {
		return nil
	}
	switch value := raw.(type) {
	case *APIKeyProbeQuotaSnapshot:
		return value
	case APIKeyProbeQuotaSnapshot:
		cp := value
		return &cp
	case map[string]any:
		return apiKeyProbeQuotaSnapshotFromMap(value)
	case map[string]string:
		mapped := make(map[string]any, len(value))
		for k, v := range value {
			mapped[k] = v
		}
		return apiKeyProbeQuotaSnapshotFromMap(mapped)
	default:
		payload, err := json.Marshal(raw)
		if err != nil {
			return nil
		}
		var snapshot APIKeyProbeQuotaSnapshot
		if err := json.Unmarshal(payload, &snapshot); err != nil {
			return nil
		}
		if strings.TrimSpace(snapshot.Provider) == "" {
			return nil
		}
		return &snapshot
	}
}

func (s *APIKeyProbeQuotaSnapshot) hasHeaderSignal() bool {
	if s == nil {
		return false
	}
	return strings.TrimSpace(s.RequestsLimit) != "" ||
		strings.TrimSpace(s.RequestsRemaining) != "" ||
		strings.TrimSpace(s.RequestsReset) != "" ||
		strings.TrimSpace(s.TokensLimit) != "" ||
		strings.TrimSpace(s.TokensRemaining) != "" ||
		strings.TrimSpace(s.TokensReset) != "" ||
		strings.TrimSpace(s.InputTokensLimit) != "" ||
		strings.TrimSpace(s.InputTokensRemaining) != "" ||
		strings.TrimSpace(s.InputTokensReset) != "" ||
		strings.TrimSpace(s.OutputTokensLimit) != "" ||
		strings.TrimSpace(s.OutputTokensRemaining) != "" ||
		strings.TrimSpace(s.OutputTokensReset) != "" ||
		strings.TrimSpace(s.RetryAfter) != "" ||
		strings.TrimSpace(s.RateLimitPolicy) != ""
}

func (s *APIKeyProbeQuotaSnapshot) toMap() map[string]any {
	payload := map[string]any{
		"provider":                     s.Provider,
		"supported":                    s.Supported,
		"source":                       s.Source,
		"updated_at":                   s.UpdatedAt,
		"has_rate_limit_header_signal": s.HasRateLimitHeaderSignal,
	}
	setMapValue(payload, "status_code", s.StatusCode)
	setMapValue(payload, "model", s.Model)
	setMapValue(payload, "requests_limit", s.RequestsLimit)
	setMapValue(payload, "requests_remaining", s.RequestsRemaining)
	setMapValue(payload, "requests_reset", s.RequestsReset)
	setMapValue(payload, "tokens_limit", s.TokensLimit)
	setMapValue(payload, "tokens_remaining", s.TokensRemaining)
	setMapValue(payload, "tokens_reset", s.TokensReset)
	setMapValue(payload, "input_tokens_limit", s.InputTokensLimit)
	setMapValue(payload, "input_tokens_remaining", s.InputTokensRemaining)
	setMapValue(payload, "input_tokens_reset", s.InputTokensReset)
	setMapValue(payload, "output_tokens_limit", s.OutputTokensLimit)
	setMapValue(payload, "output_tokens_remaining", s.OutputTokensRemaining)
	setMapValue(payload, "output_tokens_reset", s.OutputTokensReset)
	setMapValue(payload, "retry_after", s.RetryAfter)
	setMapValue(payload, "rate_limit_policy", s.RateLimitPolicy)
	setMapValue(payload, "quota_project", s.QuotaProject)
	setMapValue(payload, "note", s.Note)
	return payload
}

func apiKeyProbeQuotaSnapshotFromMap(raw map[string]any) *APIKeyProbeQuotaSnapshot {
	snapshot := &APIKeyProbeQuotaSnapshot{
		Provider:                 mapString(raw, "provider"),
		Supported:                mapBool(raw, "supported"),
		Source:                   mapString(raw, "source"),
		UpdatedAt:                mapString(raw, "updated_at"),
		StatusCode:               mapInt(raw, "status_code"),
		Model:                    mapString(raw, "model"),
		RequestsLimit:            mapString(raw, "requests_limit"),
		RequestsRemaining:        mapString(raw, "requests_remaining"),
		RequestsReset:            mapString(raw, "requests_reset"),
		TokensLimit:              mapString(raw, "tokens_limit"),
		TokensRemaining:          mapString(raw, "tokens_remaining"),
		TokensReset:              mapString(raw, "tokens_reset"),
		InputTokensLimit:         mapString(raw, "input_tokens_limit"),
		InputTokensRemaining:     mapString(raw, "input_tokens_remaining"),
		InputTokensReset:         mapString(raw, "input_tokens_reset"),
		OutputTokensLimit:        mapString(raw, "output_tokens_limit"),
		OutputTokensRemaining:    mapString(raw, "output_tokens_remaining"),
		OutputTokensReset:        mapString(raw, "output_tokens_reset"),
		RetryAfter:               mapString(raw, "retry_after"),
		RateLimitPolicy:          mapString(raw, "rate_limit_policy"),
		QuotaProject:             mapString(raw, "quota_project"),
		Note:                     mapString(raw, "note"),
		HasRateLimitHeaderSignal: mapBool(raw, "has_rate_limit_header_signal"),
	}
	if strings.TrimSpace(snapshot.Provider) == "" {
		return nil
	}
	if snapshot.Source == "" {
		if snapshot.hasHeaderSignal() {
			snapshot.Source = "headers"
		} else {
			snapshot.Source = "unknown"
		}
	}
	return snapshot
}

func headerValue(headers http.Header, key string) string {
	if headers == nil {
		return ""
	}
	return strings.TrimSpace(headers.Get(key))
}

func firstHeaderValue(headers http.Header, keys ...string) string {
	for _, key := range keys {
		if value := headerValue(headers, key); value != "" {
			return value
		}
	}
	return ""
}

func setMapValue(payload map[string]any, key string, value any) {
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) != "" {
			payload[key] = typed
		}
	case int:
		if typed != 0 {
			payload[key] = typed
		}
	default:
		if typed != nil {
			payload[key] = typed
		}
	}
}

func mapString(raw map[string]any, key string) string {
	value, ok := raw[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case bool:
		return strconv.FormatBool(typed)
	default:
		return strings.TrimSpace(strings.Trim(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(toJSONString(typed)), "\n", ""), "\t", ""), `"`))
	}
}

func mapBool(raw map[string]any, key string) bool {
	value, ok := raw[key]
	if !ok || value == nil {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, _ := strconv.ParseBool(strings.TrimSpace(typed))
		return parsed
	default:
		return false
	}
}

func mapInt(raw map[string]any, key string) int {
	value, ok := raw[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(typed))
		return parsed
	default:
		return 0
	}
}

func toJSONString(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(payload)
}
