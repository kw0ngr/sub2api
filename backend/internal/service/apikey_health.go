package service

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/googleapi"
)

const apiKeyProbeCooldown = 72 * time.Hour

type APIKeyHealthCheckResult struct {
	Platform   string `json:"platform"`
	StatusCode int    `json:"status_code"`
	Valid      bool   `json:"valid"`
	Invalid    bool   `json:"invalid"`
	Message    string `json:"message,omitempty"`
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
	case strings.HasPrefix(strings.ToLower(key), "sk-"):
		return PlatformOpenAI, true
	default:
		return "", false
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
			) {
				return APIKeyStatusActionPermanentDisable
			}
			// Message text fallback: precise phrases only
			if containsAny(msg,
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
	}

	// All other non-200 status codes (404, 405, 422, etc.) that are not explicitly handled above:
	// treat as temporary cooldown so the key is not scheduled again immediately.
	// This covers endpoint-not-found, method-not-allowed, and any future unknown error codes.
	return APIKeyStatusActionTemporaryCooldown
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
	case PlatformAnthropic, PlatformOpenAI, PlatformGemini:
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

	// Run the same real chat completions test used by single-account "test connection".
	// Account state (SetError, SetSchedulable, SetTempUnschedulable, etc.) is written
	// inside the platform-specific test functions, so no additional state writes are needed here.
	result, err := s.RunTestBackground(ctx, account.ID, "")
	if err != nil {
		return nil, err
	}

	return buildAPIKeyHealthCheckResultFromScheduledResult(account, result), nil
}

func buildAPIKeyHealthCheckResultFromScheduledResult(account *Account, result *ScheduledTestResult) *APIKeyHealthCheckResult {
	health := &APIKeyHealthCheckResult{}
	if account != nil {
		health.Platform = account.Platform
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
