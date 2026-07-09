package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/googleapi"
)

const (
	apiKeyProbeCooldown        = 72 * time.Hour
	apiKeyMinimumUsableBalance = 1.0
)

type APIKeyHealthCheckResult struct {
	Platform   string                    `json:"platform"`
	StatusCode int                       `json:"status_code"`
	Valid      bool                      `json:"valid"`
	Invalid    bool                      `json:"invalid"`
	Message    string                    `json:"message,omitempty"`
	ProbeQuota *APIKeyProbeQuotaSnapshot `json:"probe_quota,omitempty"`
}

type APIKeyStatusAction int

const (
	APIKeyStatusActionIgnore APIKeyStatusAction = iota
	APIKeyStatusActionValid
	APIKeyStatusActionPermanentDisable
	APIKeyStatusActionTemporaryCooldown
)

func DetectAPIKeyPlatform(rawKey string) (string, bool) {
	key := strings.TrimSpace(rawKey)
	switch {
	case strings.HasPrefix(key, "sk-ant-"):
		return PlatformAnthropic, true
	case strings.HasPrefix(key, "AIza"):
		return PlatformGemini, true
	case strings.HasPrefix(strings.ToLower(key), "sk-or-"):
		return PlatformOpenRouter, true
	case strings.HasPrefix(strings.ToLower(key), "sk-"):
		return PlatformOpenAI, true
	default:
		return "", false
	}
}

func NormalizeAPIKeyPlatform(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case PlatformAnthropic, "claude":
		return PlatformAnthropic, true
	case PlatformOpenAI:
		return PlatformOpenAI, true
	case PlatformGemini, "google":
		return PlatformGemini, true
	case PlatformOpenRouter:
		return PlatformOpenRouter, true
	case PlatformDeepSeek:
		return PlatformDeepSeek, true
	case PlatformGLM, "zhipu", "bigmodel":
		return PlatformGLM, true
	case PlatformGrok, "xai", "x.ai":
		return PlatformGrok, true
	default:
		return "", false
	}
}

func SupportedAPIKeyProbePlatform(platform string) bool {
	switch strings.TrimSpace(platform) {
	case PlatformAnthropic, PlatformOpenAI, PlatformGemini, PlatformOpenRouter, PlatformDeepSeek, PlatformGLM, PlatformGrok:
		return true
	default:
		return false
	}
}

func DefaultAPIKeyBaseURL(platform string) string {
	switch strings.TrimSpace(platform) {
	case PlatformAnthropic:
		return "https://api.anthropic.com"
	case PlatformOpenAI:
		return "https://api.openai.com"
	case PlatformGemini:
		return "https://generativelanguage.googleapis.com"
	case PlatformOpenRouter:
		return "https://openrouter.ai/api/v1"
	case PlatformDeepSeek:
		return "https://api.deepseek.com"
	case PlatformGLM:
		return "https://open.bigmodel.cn/api/paas/v4"
	case PlatformGrok:
		return "https://api.x.ai/v1"
	default:
		return ""
	}
}

func ShouldDisableAPIKeyStatus(account *Account, statusCode int, responseBody []byte) bool {
	return ClassifyAPIKeyStatusAction(account, statusCode, responseBody) == APIKeyStatusActionPermanentDisable
}

func ClassifyAPIKeyStatusAction(account *Account, statusCode int, responseBody []byte) APIKeyStatusAction {
	if account == nil || account.Type != AccountTypeAPIKey {
		return APIKeyStatusActionIgnore
	}
	if statusCode == http.StatusOK {
		return APIKeyStatusActionValid
	}

	msg := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(responseBody)))
	code := strings.ToLower(strings.TrimSpace(extractUpstreamErrorCode(responseBody)))
	bodyUpper := strings.ToUpper(string(responseBody))

	if account.Platform == PlatformOpenAI && isOpenAIContentPolicyRejection(statusCode, responseBody) {
		return APIKeyStatusActionIgnore
	}
	if statusCode == http.StatusBadRequest && isClientRequestParameterValidationError(msg) {
		return APIKeyStatusActionIgnore
	}

	// 5xx and 529 are always temporary cooldowns regardless of platform
	switch statusCode {
	case 529, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return APIKeyStatusActionTemporaryCooldown
	}

	switch account.Platform {
	case PlatformOpenAI:
		switch statusCode {
		case http.StatusUnauthorized, http.StatusPaymentRequired:
			return APIKeyStatusActionPermanentDisable
		case http.StatusTooManyRequests:
			// insufficient_quota is permanent billing exhaustion, not a temporary rate limit
			if code == "insufficient_quota" || containsAny(msg, "exceeded your current quota", "insufficient_quota") {
				return APIKeyStatusActionPermanentDisable
			}
			return APIKeyStatusActionTemporaryCooldown
		case http.StatusBadRequest:
			// Prefer structured error code: high-precision, no false positives
			if containsAny(code,
				"account_deactivated",
				"deactivated_workspace",
				"billing_not_active",
				"account_inactive",
				"billing_hard_limit_reached",
				"invalid_api_key",
			) {
				return APIKeyStatusActionPermanentDisable
			}
			// Message text fallback: use precise phrases that cannot appear in normal API errors
			if containsAny(msg,
				"organization has been disabled",
				"project has been disabled",
				"workspace has been deactivated",
				"workspace has been disabled",
				"account has been deactivated",
				"account has been suspended",
				"account has been blocked",
				"key is disabled",
				"api key disabled",
				"account is not active",
				"billing_hard_limit_reached",
				"billing hard limit reached",
			) {
				return APIKeyStatusActionPermanentDisable
			}
			// Unrecognized 400: could be a parameter issue or an unknown account error.
			// Treat as temporary cooldown to avoid hammering a potentially disabled key.
			return APIKeyStatusActionTemporaryCooldown
		case http.StatusForbidden:
			// Prefer structured error code
			if containsAny(code,
				"invalid_api_key",
				"token_invalidated",
				"token_revoked",
				"account_deactivated",
				"deactivated_workspace",
				"billing_not_active",
				"account_inactive",
				"insufficient_quota",
			) {
				return APIKeyStatusActionPermanentDisable
			}
			// Message text fallback: precise phrases only
			if containsAny(msg,
				"exceeded your current quota",
				"insufficient_quota",
				"invalid api key",
				"incorrect api key",
				"no api key provided",
				"token invalidated",
				"token revoked",
				"account has been deactivated",
				"workspace has been deactivated",
				"organization has been disabled",
				"project has been disabled",
				"key is disabled",
				"api key disabled",
				"account is not active",
				"account has been suspended",
				"account has been blocked",
			) {
				return APIKeyStatusActionPermanentDisable
			}
			// Unrecognized 403: treat as temporary cooldown.
			return APIKeyStatusActionTemporaryCooldown
		}
	case PlatformAnthropic:
		switch statusCode {
		case http.StatusUnauthorized:
			// 401 is always a permanent key/auth failure for Anthropic
			return APIKeyStatusActionPermanentDisable
		case http.StatusForbidden:
			// Anthropic 403: check for known account-level error types first.
			// Some 403s are model-level permission issues (e.g. no access to claude-opus),
			// not key invalidation. Use structured type field when available.
			errType := strings.ToLower(strings.TrimSpace(extractUpstreamErrorType(responseBody)))
			if containsAny(errType,
				"authentication_error",
				"permission_error",
			) {
				return APIKeyStatusActionPermanentDisable
			}
			// Fallback: precise message phrases that only appear for account-level issues
			if containsAny(msg,
				"invalid api key",
				"api key is invalid",
				"account has been disabled",
				"organization has been disabled",
				"account has been deactivated",
			) {
				return APIKeyStatusActionPermanentDisable
			}
			// Unknown 403: treat as temporary, not permanent — model access restriction
			return APIKeyStatusActionTemporaryCooldown
		case http.StatusPaymentRequired:
			// 402 is temporary billing issue (payment needed), not permanent key invalidation
			return APIKeyStatusActionTemporaryCooldown
		case http.StatusTooManyRequests:
			return APIKeyStatusActionTemporaryCooldown
		case http.StatusBadRequest:
			// Anthropic returns 400 for credit balance exhaustion (not 402/429)
			if containsAny(msg,
				"credit balance is too low",
				"your credit balance is",
				"insufficient credits",
				"account has been disabled",
				"organization has been disabled",
				"account has been deactivated",
			) {
				return APIKeyStatusActionPermanentDisable
			}
			return APIKeyStatusActionTemporaryCooldown
		}
	case PlatformGemini:
		switch statusCode {
		case http.StatusTooManyRequests:
			return APIKeyStatusActionTemporaryCooldown
		case http.StatusUnauthorized:
			return APIKeyStatusActionPermanentDisable
		case http.StatusForbidden:
			// Use structured reason check first (covers BILLING_DISABLED, CONSUMER_SUSPENDED, PROJECT_DISABLED, SERVICE_DISABLED)
			if googleapi.IsPermanentlyDisabledError(string(responseBody)) {
				return APIKeyStatusActionPermanentDisable
			}
			// Match known permanent-disable message patterns.
			// Avoid catch-all: some 403s indicate model-level permission issues (not account problems).
			if containsAny(msg,
				"billing is disabled",
				"billing disabled",
				"consumer suspended",
				"project disabled",
				"project has been suspended",
			) {
				return APIKeyStatusActionPermanentDisable
			}
			// Unknown 403: treat as temporary cooldown rather than ignore.
			// A model-level permission 403 is transient for this key/model combo;
			// a temporary cooldown avoids hammering a key that may be account-level suspended.
			return APIKeyStatusActionTemporaryCooldown
		case http.StatusBadRequest:
			if strings.Contains(bodyUpper, "API_KEY_INVALID") || googleapi.IsServiceDisabledError(string(responseBody)) {
				return APIKeyStatusActionPermanentDisable
			}
			// FAILED_PRECONDITION with billing/free-tier messages: permanent disable.
			// Bare FAILED_PRECONDITION without billing context may be a request issue, not key failure.
			if strings.Contains(bodyUpper, "FAILED_PRECONDITION") && containsAny(msg,
				"free tier is not available",
				"enable billing",
				"billing account",
				"requires a billing",
			) {
				return APIKeyStatusActionPermanentDisable
			}
			if containsAny(msg,
				"api key not valid",
				"invalid api key",
				"api_key_invalid",
				"api key is invalid",
				"before or it is disabled",
				"service disabled",
				"api has not been used in project",
				"unregistered callers",
				"caller not registered",
				"free tier is not available",
				"enable billing",
			) {
				return APIKeyStatusActionPermanentDisable
			}
			return APIKeyStatusActionTemporaryCooldown
		}
	case PlatformGLM:
		switch statusCode {
		case http.StatusUnauthorized:
			return APIKeyStatusActionPermanentDisable
		case http.StatusForbidden:
			if isGLMResettableQuotaError(responseBody) {
				return APIKeyStatusActionTemporaryCooldown
			}
			if containsAny(code, "invalid_api_key", "invalid_token", "authentication_error") ||
				containsAny(msg,
					"invalid api key",
					"incorrect api key",
					"no api key provided",
					"authentication failed",
					"invalid token",
					"api key 无效",
					"密钥无效",
					"鉴权失败",
					"认证失败",
				) {
				return APIKeyStatusActionPermanentDisable
			}
			return APIKeyStatusActionTemporaryCooldown
		case http.StatusPaymentRequired:
			return APIKeyStatusActionPermanentDisable
		case http.StatusTooManyRequests:
			return APIKeyStatusActionTemporaryCooldown
		case http.StatusBadRequest:
			if containsAny(code, "invalid_api_key", "invalid_api_key_format", "authentication_error") ||
				containsAny(msg,
					"invalid api key",
					"incorrect api key",
					"no api key provided",
					"authentication failed",
					"invalid token",
					"insufficient balance",
					"insufficient credits",
					"insufficient quota",
					"credit balance",
					"balance is insufficient",
					"credits exhausted",
					"resource package",
					"no available resource",
					"资源包",
					"余额不足",
				) {
				return APIKeyStatusActionPermanentDisable
			}
			return APIKeyStatusActionTemporaryCooldown
		}
	case PlatformOpenRouter, PlatformDeepSeek:
		switch statusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return APIKeyStatusActionPermanentDisable
		case http.StatusPaymentRequired:
			return APIKeyStatusActionPermanentDisable
		case http.StatusTooManyRequests:
			return APIKeyStatusActionTemporaryCooldown
		case http.StatusBadRequest:
			if containsAny(code, "invalid_api_key", "invalid_api_key_format", "authentication_error") ||
				containsAny(msg,
					"invalid api key",
					"incorrect api key",
					"no api key provided",
					"authentication failed",
					"invalid token",
					"insufficient balance",
					"insufficient credits",
					"insufficient quota",
					"credit balance",
					"balance is insufficient",
					"credits exhausted",
					"resource package",
					"no available resource",
					"资源包",
					"余额不足",
				) {
				return APIKeyStatusActionPermanentDisable
			}
			return APIKeyStatusActionTemporaryCooldown
		}
	}

	// All other non-200 status codes (404, 405, 422, etc.) that are not explicitly handled above:
	// treat as temporary cooldown so the key is not scheduled again immediately.
	// This covers endpoint-not-found, method-not-allowed, and any future unknown error codes.
	return APIKeyStatusActionTemporaryCooldown
}

func isOpenAIContentPolicyRejection(statusCode int, responseBody []byte) bool {
	if statusCode < http.StatusBadRequest {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(responseBody)))
	code := strings.ToLower(strings.TrimSpace(extractUpstreamErrorCode(responseBody)))
	errType := strings.ToLower(strings.TrimSpace(extractUpstreamErrorType(responseBody)))
	body := strings.ToLower(string(responseBody))
	combined := msg + " " + code + " " + errType + " " + body
	return containsAny(combined,
		"this content was flagged",
		"possible cybersecurity risk",
		"trusted access for cyber",
		"high-risk cyber",
		"cyber_policy",
		"content policy",
		"content_policy",
	)
}

func isGLMResettableQuotaError(responseBody []byte) bool {
	msg := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(responseBody)))
	code := strings.ToLower(strings.TrimSpace(extractUpstreamErrorCode(responseBody)))
	if code == "1310" {
		return true
	}
	return containsAny(msg, "usage limit", "使用上限", "限额将在") &&
		containsAny(msg, "reset", "重置")
}

func isGLMModelOverloadedError(responseBody []byte) bool {
	msg := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(responseBody)))
	code := strings.ToLower(strings.TrimSpace(extractUpstreamErrorCode(responseBody)))
	errType := strings.ToLower(strings.TrimSpace(extractUpstreamErrorType(responseBody)))
	return code == "1305" ||
		errType == "overloaded_error" ||
		containsAny(msg, "模型当前访问量过大", "请您稍后再试")
}

func isClientRequestParameterValidationError(msg string) bool {
	msg = strings.ToLower(strings.TrimSpace(msg))
	if msg == "" {
		return false
	}
	if strings.Contains(msg, "max_tokens") &&
		containsAny(msg, "above maximum value", "expected a value <=", "less than or equal", "must be <=", "maximum value") {
		return true
	}
	if strings.Contains(msg, "messages.content.type") &&
		strings.Contains(msg, "redacted_thinking") &&
		containsAny(msg, "invalid value", "not valid", "supported values") {
		return true
	}
	if strings.Contains(msg, "tool_use.input") &&
		containsAny(msg, "input should be an object", "expected an object") {
		return true
	}
	if strings.Contains(msg, "effort level") &&
		strings.Contains(msg, "supported levels") {
		return true
	}
	if strings.Contains(msg, "thinking mode") &&
		strings.Contains(msg, "does not support") &&
		strings.Contains(msg, "tool_choice") {
		return true
	}
	if strings.Contains(msg, "image generation items") &&
		strings.Contains(msg, "without") &&
		strings.Contains(msg, "id") &&
		strings.Contains(msg, "not supported") {
		return true
	}
	return false
}

func ShouldDisableAPIKeyAuthFailure(account *Account, statusCode int, responseBody []byte) bool {
	return ShouldDisableAPIKeyStatus(account, statusCode, responseBody)
}

// ClassifyAPIKeyProbeResponse classifies a probe response into (valid, invalid, cooldown, message).
// valid=true: key works. invalid=true: key is permanently disabled. cooldown=true: key needs temp cooldown.
func ClassifyAPIKeyProbeResponse(account *Account, statusCode int, responseBody []byte) (valid bool, invalid bool, cooldown bool, message string) {
	if account == nil || account.Type != AccountTypeAPIKey {
		return false, false, false, "unsupported account type"
	}

	message = strings.TrimSpace(extractUpstreamErrorMessage(responseBody))
	if message == "" {
		message = http.StatusText(statusCode)
	}
	message = sanitizeUpstreamErrorMessage(message)

	switch account.Platform {
	case PlatformAnthropic, PlatformOpenAI, PlatformGemini, PlatformOpenRouter, PlatformDeepSeek, PlatformGLM, PlatformGrok:
		switch ClassifyAPIKeyStatusAction(account, statusCode, responseBody) {
		case APIKeyStatusActionValid:
			return true, false, false, message
		case APIKeyStatusActionPermanentDisable:
			return false, true, false, message
		case APIKeyStatusActionTemporaryCooldown:
			return false, false, true, message
		default:
			return false, false, false, message
		}
	default:
		return false, false, false, message
	}
}

// CheckAPIKeyValidity tests an API key account using a real chat completions request,
// identical to the single-account "test connection" flow. This ensures health check
// results are authoritative and consistent with manual test results.
func (s *AccountTestService) CheckAPIKeyValidity(ctx context.Context, account *Account) (*APIKeyHealthCheckResult, error) {
	if account == nil {
		return nil, fmt.Errorf("account is required")
	}
	if account.Type != AccountTypeAPIKey {
		return nil, fmt.Errorf("account %d is not an apikey account", account.ID)
	}
	if s == nil || s.httpUpstream == nil {
		return nil, fmt.Errorf("account test service is not configured")
	}
	switch account.Platform {
	case PlatformGrok:
		return s.checkGrokAPIKey(ctx, account)
	case PlatformOpenRouter:
		return s.checkOpenRouterAPIKey(ctx, account)
	case PlatformDeepSeek:
		return s.checkDeepSeekAPIKey(ctx, account)
	case PlatformGLM:
		return s.checkGLMAPIKey(ctx, account)
	}

	// Run the same real chat completions test used by single-account "test connection".
	// Account state (SetError, SetSchedulable, SetTempUnschedulable, etc.) is written
	// inside the platform-specific test functions, so no additional state writes are needed here.
	result, err := s.RunTestBackground(ctx, account.ID, "")
	if err != nil {
		return nil, err
	}

	resultAccount := account
	if s.accountRepo != nil {
		if refreshed, refreshErr := s.accountRepo.GetByID(ctx, account.ID); refreshErr == nil && refreshed != nil {
			resultAccount = refreshed
		}
	}

	return buildAPIKeyHealthCheckResultFromScheduledResult(resultAccount, result), nil
}

func (s *AccountTestService) checkOpenRouterAPIKey(ctx context.Context, account *Account) (*APIKeyHealthCheckResult, error) {
	baseURL := openRouterProbeBaseURL(account.GetCredential("base_url"))

	statusCode, body, err := s.doAPIKeyProbe(ctx, account, http.MethodGet, baseURL+"/credits", map[string]string{
		"Authorization": "Bearer " + strings.TrimSpace(account.GetCredential("api_key")),
		"Accept":        "application/json",
	}, nil)
	if err != nil {
		return nil, err
	}
	if statusCode == http.StatusOK {
		balance, total, used, remaining := parseOpenRouterCredits(body)
		snapshot := BuildAPIKeyProbeBalanceSnapshot(PlatformOpenRouter, statusCode, balance, total, used, remaining, "", time.Now())
		s.persistAPIKeyProbeQuotaSnapshot(ctx, account, snapshot)
		if low, ok := apiKeyProbeBalanceBelowMinimum(firstNonEmpty(remaining, total), apiKeyMinimumUsableBalance); ok && low {
			snapshot.Note = lowBalanceProbeNote()
			return &APIKeyHealthCheckResult{
				Platform:   PlatformOpenRouter,
				StatusCode: statusCode,
				Invalid:    true,
				Message:    balanceBelowMinimumMessage(balance),
				ProbeQuota: snapshot,
			}, nil
		}
		return &APIKeyHealthCheckResult{
			Platform:   PlatformOpenRouter,
			StatusCode: statusCode,
			Valid:      true,
			Message:    balanceMessage(balance),
			ProbeQuota: snapshot,
		}, nil
	}
	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		return buildAPIKeyHealthCheckResultFromProbe(account, statusCode, body), nil
	}

	// Some OpenRouter keys cannot access /credits but can still be used for inference.
	return s.checkAPIKeyModelsEndpoint(ctx, account, PlatformOpenRouter, baseURL+"/models")
}

func (s *AccountTestService) checkDeepSeekAPIKey(ctx context.Context, account *Account) (*APIKeyHealthCheckResult, error) {
	baseURL := deepSeekProbeBaseURL(account.GetCredential("base_url"))

	statusCode, body, err := s.doAPIKeyProbe(ctx, account, http.MethodGet, baseURL+"/user/balance", map[string]string{
		"Authorization": "Bearer " + strings.TrimSpace(account.GetCredential("api_key")),
		"Accept":        "application/json",
	}, nil)
	if err != nil {
		return nil, err
	}
	if statusCode == http.StatusOK {
		balance, currency, amounts := parseDeepSeekBalanceDetails(body)
		snapshot := BuildAPIKeyProbeBalanceSnapshot(PlatformDeepSeek, statusCode, balance, "", "", "", currency, time.Now())
		s.persistAPIKeyProbeQuotaSnapshot(ctx, account, snapshot)
		if apiKeyProbeBalancesBelowMinimum(amounts, apiKeyMinimumUsableBalance) {
			snapshot.Note = lowBalanceProbeNote()
			return &APIKeyHealthCheckResult{
				Platform:   PlatformDeepSeek,
				StatusCode: statusCode,
				Invalid:    true,
				Message:    balanceBelowMinimumMessage(balance),
				ProbeQuota: snapshot,
			}, nil
		}
		return &APIKeyHealthCheckResult{
			Platform:   PlatformDeepSeek,
			StatusCode: statusCode,
			Valid:      true,
			Message:    balanceMessage(balance),
			ProbeQuota: snapshot,
		}, nil
	}
	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		return buildAPIKeyHealthCheckResultFromProbe(account, statusCode, body), nil
	}

	return s.checkAPIKeyModelsEndpoint(ctx, account, PlatformDeepSeek, baseURL+"/models")
}

func (s *AccountTestService) checkGrokAPIKey(ctx context.Context, account *Account) (*APIKeyHealthCheckResult, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(account.GetOpenAIBaseURL()), "/")
	if baseURL == "" {
		baseURL = DefaultAPIKeyBaseURL(PlatformGrok)
	}
	return s.checkAPIKeyModelsEndpoint(ctx, account, PlatformGrok, baseURL+"/models")
}

func (s *AccountTestService) checkGLMAPIKey(ctx context.Context, account *Account) (*APIKeyHealthCheckResult, error) {
	if account.IsGLMOpenAICompatible() {
		baseURL := strings.TrimRight(strings.TrimSpace(account.GetOpenAIBaseURL()), "/")
		if baseURL == "" {
			baseURL = DefaultAPIKeyBaseURL(PlatformGLM)
		}
		quota := s.fetchGLMQuotaLimitSnapshot(ctx, account, baseURL)
		health, err := s.checkAPIKeyModelsEndpoint(ctx, account, PlatformGLM, buildGLMOpenAIModelsURL(baseURL))
		if err != nil {
			return nil, err
		}
		attachProbeQuota(health, quota)
		if health.Valid || health.Invalid || health.StatusCode == http.StatusTooManyRequests {
			return health, nil
		}
		health, err = s.checkGLMOpenAIChatProbe(ctx, account, baseURL)
		attachProbeQuota(health, quota)
		return health, err
	}
	baseURL := anthropicCompatibleBaseURLForAccount(account)
	quota := s.fetchGLMQuotaLimitSnapshot(ctx, account, baseURL)
	health, err := s.checkGLMAnthropicMessagesProbe(ctx, account, baseURL)
	attachProbeQuota(health, quota)
	return health, err
}

func (s *AccountTestService) fetchGLMQuotaLimitSnapshot(ctx context.Context, account *Account, baseURL string) *APIKeyProbeQuotaSnapshot {
	endpoint, ok := buildGLMMonitorQuotaLimitURL(baseURL)
	if !ok {
		return nil
	}
	statusCode, body, err := s.doAPIKeyProbe(ctx, account, http.MethodGet, endpoint, map[string]string{
		"Authorization":   strings.TrimSpace(account.GetCredential("api_key")),
		"Accept":          "application/json",
		"Accept-Language": "en-US,en",
		"Content-Type":    "application/json",
	}, nil)
	if err != nil {
		return nil
	}
	snapshot := BuildGLMAPIKeyProbeQuotaSnapshot(statusCode, body, time.Now())
	s.persistAPIKeyProbeQuotaSnapshot(ctx, account, snapshot)
	return snapshot
}

func attachProbeQuota(health *APIKeyHealthCheckResult, snapshot *APIKeyProbeQuotaSnapshot) {
	if health != nil && snapshot != nil {
		health.ProbeQuota = snapshot
	}
}

func buildGLMMonitorQuotaLimitURL(baseURL string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", false
	}
	host := strings.ToLower(parsed.Hostname())
	switch host {
	case "api.z.ai", "open.bigmodel.cn", "dev.bigmodel.cn":
		return parsed.Scheme + "://" + parsed.Host + "/api/monitor/usage/quota/limit", true
	default:
		return "", false
	}
}

func (s *AccountTestService) checkGLMAnthropicMessagesProbe(ctx context.Context, account *Account, baseURL string) (*APIKeyHealthCheckResult, error) {
	endpoint := buildGLMAnthropicMessagesURL(baseURL)
	body, _ := json.Marshal(map[string]any{
		"model":      "glm-5.2",
		"max_tokens": 1,
		"stream":     false,
		"messages": []map[string]any{
			{"role": "user", "content": "hi"},
		},
	})
	statusCode, respBody, err := s.doAPIKeyProbe(ctx, account, http.MethodPost, endpoint, map[string]string{
		"x-api-key":         strings.TrimSpace(account.GetCredential("api_key")),
		"anthropic-version": "2023-06-01",
		"Content-Type":      "application/json",
		"Accept":            "application/json",
	}, body)
	if err != nil {
		return nil, err
	}
	return buildAPIKeyHealthCheckResultFromProbe(account, statusCode, respBody), nil
}

func (s *AccountTestService) checkGLMOpenAIChatProbe(ctx context.Context, account *Account, baseURL string) (*APIKeyHealthCheckResult, error) {
	body, _ := json.Marshal(map[string]any{
		"model":      "glm-5.2",
		"max_tokens": 1,
		"stream":     false,
		"messages": []map[string]any{
			{"role": "user", "content": "hi"},
		},
	})
	statusCode, respBody, err := s.doAPIKeyProbe(ctx, account, http.MethodPost, buildOpenAICompatibleChatCompletionsURL(PlatformGLM, baseURL), map[string]string{
		"Authorization": "Bearer " + strings.TrimSpace(account.GetCredential("api_key")),
		"Content-Type":  "application/json",
		"Accept":        "application/json",
	}, body)
	if err != nil {
		return nil, err
	}
	return buildAPIKeyHealthCheckResultFromProbe(account, statusCode, respBody), nil
}

func (s *AccountTestService) checkAPIKeyModelsEndpoint(ctx context.Context, account *Account, platform, endpoint string) (*APIKeyHealthCheckResult, error) {
	statusCode, body, err := s.doAPIKeyProbe(ctx, account, http.MethodGet, endpoint, map[string]string{
		"Authorization": "Bearer " + strings.TrimSpace(account.GetCredential("api_key")),
		"Accept":        "application/json",
	}, nil)
	if err != nil {
		return nil, err
	}
	health := buildAPIKeyHealthCheckResultFromProbe(account, statusCode, body)
	health.Platform = platform
	if health.Valid && strings.TrimSpace(health.Message) == "" {
		health.Message = "models endpoint available"
	}
	return health, nil
}

func (s *AccountTestService) doAPIKeyProbe(ctx context.Context, account *Account, method, endpoint string, headers map[string]string, body []byte) (int, []byte, error) {
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return 0, nil, err
	}
	for key, value := range headers {
		if strings.TrimSpace(value) != "" {
			req.Header.Set(key, value)
		}
	}

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	var resp *http.Response
	if s.tlsFPProfileService != nil {
		resp, err = s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, s.tlsFPProfileService.ResolveTLSProfile(account))
	} else {
		resp, err = s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, nil)
	}
	if err != nil {
		return 0, nil, err
	}
	if resp == nil {
		return 0, nil, fmt.Errorf("empty probe response")
	}
	if resp.Body == nil {
		return resp.StatusCode, nil, nil
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, respBody, nil
}

func (s *AccountTestService) persistAPIKeyProbeQuotaSnapshot(ctx context.Context, account *Account, snapshot *APIKeyProbeQuotaSnapshot) {
	if s == nil || s.accountRepo == nil || account == nil || snapshot == nil {
		return
	}
	updates := BuildAPIKeyProbeQuotaExtraUpdates(snapshot)
	if len(updates) == 0 {
		return
	}
	if err := s.accountRepo.UpdateExtra(ctx, account.ID, updates); err == nil {
		mergeAccountExtra(account, updates)
	}
}

func buildAPIKeyHealthCheckResultFromProbe(account *Account, statusCode int, body []byte) *APIKeyHealthCheckResult {
	valid, invalid, _, message := ClassifyAPIKeyProbeResponse(account, statusCode, body)
	return &APIKeyHealthCheckResult{
		Platform:   account.Platform,
		StatusCode: statusCode,
		Valid:      valid,
		Invalid:    invalid,
		Message:    message,
	}
}

func openRouterProbeBaseURL(raw string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(raw), "/")
	if baseURL == "" || baseURL == "https://openrouter.ai" {
		return "https://openrouter.ai/api/v1"
	}
	if strings.HasSuffix(baseURL, "/api") {
		return baseURL + "/v1"
	}
	return baseURL
}

func deepSeekProbeBaseURL(raw string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(raw), "/")
	if baseURL == "" {
		return "https://api.deepseek.com"
	}
	if strings.HasSuffix(baseURL, "/v1") && strings.Contains(baseURL, "api.deepseek.com") {
		return strings.TrimSuffix(baseURL, "/v1")
	}
	return baseURL
}

func parseOpenRouterCredits(body []byte) (balance, total, used, remaining string) {
	var payload struct {
		Data struct {
			TotalCredits any `json:"total_credits"`
			TotalUsage   any `json:"total_usage"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "available", "", "", ""
	}
	total = apiKeyProbeNumberString(payload.Data.TotalCredits)
	used = apiKeyProbeNumberString(payload.Data.TotalUsage)
	totalFloat, totalOK := apiKeyProbeFloat(payload.Data.TotalCredits)
	usedFloat, usedOK := apiKeyProbeFloat(payload.Data.TotalUsage)
	if totalOK && usedOK {
		remaining = strconv.FormatFloat(totalFloat-usedFloat, 'f', 4, 64)
		balance = "$" + remaining + " remaining"
		return balance, total, used, remaining
	}
	if total != "" {
		balance = "$" + total + " total"
		return balance, total, used, remaining
	}
	return "available", total, used, remaining
}

func parseDeepSeekBalanceDetails(body []byte) (balance, currency string, amounts []float64) {
	var payload struct {
		BalanceInfos []struct {
			TotalBalance any    `json:"total_balance"`
			Currency     string `json:"currency"`
		} `json:"balance_infos"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "available", "", nil
	}
	parts := make([]string, 0, len(payload.BalanceInfos))
	currencies := make([]string, 0, len(payload.BalanceInfos))
	for _, info := range payload.BalanceInfos {
		amount := apiKeyProbeNumberString(info.TotalBalance)
		if parsed, ok := apiKeyProbeFloat(info.TotalBalance); ok {
			amounts = append(amounts, parsed)
		}
		if amount == "" {
			amount = "?"
		}
		cur := strings.TrimSpace(info.Currency)
		if cur != "" {
			currencies = append(currencies, cur)
		}
		parts = append(parts, strings.TrimSpace(amount+" "+cur))
	}
	if len(parts) == 0 {
		return "available", "", nil
	}
	return strings.Join(parts, "; "), strings.Join(currencies, ";"), amounts
}

func apiKeyProbeNumberString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func apiKeyProbeFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func apiKeyProbeBalanceBelowMinimum(value string, minimum float64) (bool, bool) {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return false, false
	}
	return parsed < minimum, true
}

func apiKeyProbeBalancesBelowMinimum(amounts []float64, minimum float64) bool {
	if len(amounts) == 0 {
		return false
	}
	for _, amount := range amounts {
		if amount >= minimum {
			return false
		}
	}
	return true
}

func balanceBelowMinimumMessage(balance string) string {
	balance = strings.TrimSpace(balance)
	if balance == "" {
		return "balance below 1"
	}
	return "balance below 1: " + balance
}

func lowBalanceProbeNote() string {
	return "Balance is below 1; health check marks this API key invalid for scheduling."
}

func balanceMessage(balance string) string {
	if strings.TrimSpace(balance) == "" {
		return "balance available"
	}
	return "balance: " + strings.TrimSpace(balance)
}

func buildAPIKeyHealthCheckResultFromScheduledResult(account *Account, result *ScheduledTestResult) *APIKeyHealthCheckResult {
	health := &APIKeyHealthCheckResult{}
	if account != nil {
		health.Platform = account.Platform
		health.ProbeQuota = APIKeyProbeQuotaSnapshotFromExtra(account.Extra)
	}
	if result == nil {
		health.Message = "empty test result"
		return health
	}

	if result.Status == "success" {
		health.StatusCode = http.StatusOK
		health.Valid = true
		health.Message = strings.TrimSpace(result.ResponseText)
		return health
	}

	statusCode, responseBody, ok := parseAPIReturnedError(result.ErrorMessage)
	health.StatusCode = statusCode
	if !ok {
		health.Message = strings.TrimSpace(result.ErrorMessage)
		return health
	}

	valid, invalid, _, message := ClassifyAPIKeyProbeResponse(account, statusCode, responseBody)
	health.Valid = valid
	health.Invalid = invalid
	if message == "" {
		message = strings.TrimSpace(result.ErrorMessage)
	}
	health.Message = message
	return health
}

func parseAPIReturnedError(message string) (int, []byte, bool) {
	trimmed := strings.TrimSpace(message)
	const prefix = "API returned "
	if !strings.HasPrefix(trimmed, prefix) {
		return 0, nil, false
	}
	rest := strings.TrimPrefix(trimmed, prefix)
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return 0, nil, false
	}
	statusCode, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, nil, false
	}
	return statusCode, []byte(strings.TrimSpace(parts[1])), true
}

func containsAny(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if needle != "" && strings.Contains(haystack, needle) {
			return true
		}
	}
	return false
}
