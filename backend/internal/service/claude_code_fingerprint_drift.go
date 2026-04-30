package service

import (
	"context"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

const claudeCodeFingerprintDriftMaxValueLen = 240

var (
	claudeCodeFingerprintDriftLatest atomic.Value // *ClaudeCodeFingerprintDriftStatus
	ccVersionSummaryRe               = regexp.MustCompile(`\bcc_version=([0-9]+\.[0-9]+\.[0-9]+(?:\.[0-9A-Fa-f]+)?)`)
)

type ClaudeCodeFingerprintHeaderDiff struct {
	Header   string `json:"header"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Default  string `json:"default,omitempty"`
}

type ClaudeCodeFingerprintDriftStatus struct {
	Status                string                            `json:"status"`
	Message               string                            `json:"message,omitempty"`
	Endpoint              string                            `json:"endpoint,omitempty"`
	AccountID             int64                             `json:"account_id,omitempty"`
	AccountName           string                            `json:"account_name,omitempty"`
	MimicClaudeCode       bool                              `json:"mimic_claude_code"`
	ActiveProfileID       string                            `json:"active_profile_id,omitempty"`
	ActiveProfileName     string                            `json:"active_profile_name,omitempty"`
	SampleApplied         bool                              `json:"sample_applied"`
	Score                 int                               `json:"score"`
	HeaderMatches         []string                          `json:"header_matches,omitempty"`
	HeaderMismatches      []ClaudeCodeFingerprintHeaderDiff `json:"header_mismatches,omitempty"`
	MissingHeaders        []string                          `json:"missing_headers,omitempty"`
	DefaultOverwrites     []ClaudeCodeFingerprintHeaderDiff `json:"default_overwrites,omitempty"`
	BetaExpected          []string                          `json:"beta_expected,omitempty"`
	BetaActual            []string                          `json:"beta_actual,omitempty"`
	BetaMissing           []string                          `json:"beta_missing,omitempty"`
	BetaUnexpected        []string                          `json:"beta_unexpected,omitempty"`
	CCVersionFromUA       string                            `json:"cc_version_from_ua,omitempty"`
	CCVersionFromBilling  string                            `json:"cc_version_from_billing,omitempty"`
	CCVersionMatches      bool                              `json:"cc_version_matches"`
	Warnings              []string                          `json:"warnings,omitempty"`
	OutgoingHeaderSummary map[string]string                 `json:"outgoing_header_summary,omitempty"`
	UpdatedAt             int64                             `json:"updated_at"`
}

func (s *SettingService) RecordClaudeCodeFingerprintDrift(
	ctx context.Context,
	req *http.Request,
	body []byte,
	account *Account,
	tokenType string,
	mimicClaudeCode bool,
	sampleApplied bool,
	endpoint string,
) {
	if s == nil || req == nil || tokenType != "oauth" {
		return
	}
	status := s.buildClaudeCodeFingerprintDriftStatus(ctx, req, body, account, mimicClaudeCode, sampleApplied, endpoint)
	claudeCodeFingerprintDriftLatest.Store(&status)
}

func (s *SettingService) GetClaudeCodeFingerprintDriftStatus(context.Context) (ClaudeCodeFingerprintDriftStatus, error) {
	if raw, ok := claudeCodeFingerprintDriftLatest.Load().(*ClaudeCodeFingerprintDriftStatus); ok && raw != nil {
		cp := *raw
		cp.HeaderMatches = append([]string(nil), raw.HeaderMatches...)
		cp.HeaderMismatches = append([]ClaudeCodeFingerprintHeaderDiff(nil), raw.HeaderMismatches...)
		cp.MissingHeaders = append([]string(nil), raw.MissingHeaders...)
		cp.DefaultOverwrites = append([]ClaudeCodeFingerprintHeaderDiff(nil), raw.DefaultOverwrites...)
		cp.BetaExpected = append([]string(nil), raw.BetaExpected...)
		cp.BetaActual = append([]string(nil), raw.BetaActual...)
		cp.BetaMissing = append([]string(nil), raw.BetaMissing...)
		cp.BetaUnexpected = append([]string(nil), raw.BetaUnexpected...)
		cp.Warnings = append([]string(nil), raw.Warnings...)
		if raw.OutgoingHeaderSummary != nil {
			cp.OutgoingHeaderSummary = make(map[string]string, len(raw.OutgoingHeaderSummary))
			for key, value := range raw.OutgoingHeaderSummary {
				cp.OutgoingHeaderSummary[key] = value
			}
		}
		return cp, nil
	}
	return ClaudeCodeFingerprintDriftStatus{
		Status:    "idle",
		Message:   "还没有捕获到 OAuth 上游转发指纹。让 Claude Code / 兼容客户端请求一次后再查看。",
		UpdatedAt: time.Now().Unix(),
	}, nil
}

func (s *SettingService) buildClaudeCodeFingerprintDriftStatus(
	ctx context.Context,
	req *http.Request,
	body []byte,
	account *Account,
	mimicClaudeCode bool,
	sampleApplied bool,
	endpoint string,
) ClaudeCodeFingerprintDriftStatus {
	status := ClaudeCodeFingerprintDriftStatus{
		Status:                "idle",
		Endpoint:              endpoint,
		MimicClaudeCode:       mimicClaudeCode,
		SampleApplied:         false,
		OutgoingHeaderSummary: claudeCodeOutgoingHeaderSummary(req),
		UpdatedAt:             time.Now().Unix(),
	}
	if account != nil {
		status.AccountID = account.ID
		status.AccountName = account.Name
	}

	library, err := s.getClaudeCodeFingerprintLibraryCached(ctx)
	if err != nil {
		status.Status = "warning"
		status.Message = "读取 Claude Code 指纹样本库失败，已仅记录 outgoing 摘要。"
		status.Warnings = append(status.Warnings, err.Error())
		return status
	}

	activeProfile, activeID := findActiveClaudeCodeFingerprintProfile(library)
	status.ActiveProfileID = activeID
	if activeProfile == nil {
		if activeID == "" {
			status.Message = "未选择固定 HTTP 样本，当前处于按账号自动学习模式。"
		} else {
			status.Status = "warning"
			status.Message = "已配置的 HTTP 样本不存在。"
			status.Warnings = append(status.Warnings, "active HTTP sample is missing from library")
		}
		return status
	}

	status.ActiveProfileName = activeProfile.Name
	status.SampleApplied = sampleApplied
	if !sampleApplied {
		status.Warnings = append(status.Warnings, "当前选中的 HTTP 样本没有真正应用到本次 outgoing 请求")
	}

	expectedHeaders := claudeCodeExpectedHeadersFromProfile(*activeProfile)
	totalChecks := 1 // sample_applied
	passedChecks := 0
	if sampleApplied {
		passedChecks++
	}

	for _, header := range sortedHeaderKeys(expectedHeaders) {
		expected := strings.TrimSpace(expectedHeaders[header])
		if expected == "" {
			continue
		}
		if strings.EqualFold(header, "anthropic-beta") {
			continue
		}
		totalChecks++
		actual := strings.TrimSpace(getHeaderRaw(req.Header, header))
		if actual == "" {
			status.MissingHeaders = append(status.MissingHeaders, header)
			continue
		}
		if actual == expected {
			status.HeaderMatches = append(status.HeaderMatches, header)
			passedChecks++
			continue
		}
		diff := ClaudeCodeFingerprintHeaderDiff{
			Header:   header,
			Expected: truncateDriftValue(expected),
			Actual:   truncateDriftValue(actual),
		}
		status.HeaderMismatches = append(status.HeaderMismatches, diff)
		if defaultValue := claudeCodeDefaultHeaderValue(header); defaultValue != "" && actual == defaultValue {
			diff.Default = truncateDriftValue(defaultValue)
			status.DefaultOverwrites = append(status.DefaultOverwrites, diff)
		}
	}

	status.BetaExpected = splitBetaTokens(activeProfile.AnthropicBeta)
	status.BetaActual = splitBetaTokens(getHeaderRaw(req.Header, "anthropic-beta"))
	status.BetaMissing, status.BetaUnexpected = diffTokenSets(status.BetaExpected, status.BetaActual)
	totalChecks++
	if len(status.BetaMissing) == 0 {
		passedChecks++
	}

	status.CCVersionFromUA = ExtractCLIVersion(getHeaderRaw(req.Header, "User-Agent"))
	status.CCVersionFromBilling = extractBillingCCVersion(body)
	totalChecks++
	status.CCVersionMatches = status.CCVersionFromUA != "" &&
		claudeCodeSemverCore(status.CCVersionFromBilling) == status.CCVersionFromUA
	if status.CCVersionMatches {
		passedChecks++
	}

	status.Score = scorePercent(passedChecks, totalChecks)
	status.Status = claudeCodeDriftStatus(status)
	status.Message = claudeCodeDriftMessage(status)
	status.Warnings = append(status.Warnings, claudeCodeDriftWarnings(status)...)
	return status
}

func findActiveClaudeCodeFingerprintProfile(library ClaudeCodeFingerprintLibrary) (*ClaudeCodeFingerprintProfile, string) {
	activeID := strings.TrimSpace(library.ActiveID)
	if activeID == "" {
		return nil, ""
	}
	for i := range library.Profiles {
		if library.Profiles[i].ID == activeID {
			return &library.Profiles[i], activeID
		}
	}
	return nil, activeID
}

func claudeCodeExpectedHeadersFromProfile(profile ClaudeCodeFingerprintProfile) map[string]string {
	return map[string]string{
		"User-Agent":        profile.UserAgent,
		"Accept":            profile.Accept,
		"content-type":      profile.ContentType,
		"anthropic-version": profile.AnthropicVersion,
		"anthropic-beta":    profile.AnthropicBeta,
		"x-app":             profile.XApp,
		"anthropic-dangerous-direct-browser-access": profile.DirectBrowserAccess,
		"X-Stainless-Lang":                          profile.StainlessLang,
		"X-Stainless-Package-Version":               profile.StainlessPackageVersion,
		"X-Stainless-OS":                            profile.StainlessOS,
		"X-Stainless-Arch":                          profile.StainlessArch,
		"X-Stainless-Runtime":                       profile.StainlessRuntime,
		"X-Stainless-Runtime-Version":               profile.StainlessRuntimeVersion,
		"X-Stainless-Retry-Count":                   profile.StainlessRetryCount,
		"X-Stainless-Timeout":                       profile.StainlessTimeout,
		"x-stainless-helper-method":                 profile.HelperMethod,
	}
}

func claudeCodeDefaultHeaderValue(header string) string {
	switch strings.ToLower(header) {
	case "user-agent":
		return defaultFingerprint.UserAgent
	case "accept":
		return defaultFingerprint.Accept
	case "content-type":
		return defaultFingerprint.ContentType
	case "anthropic-version":
		return defaultFingerprint.AnthropicVersion
	case "anthropic-beta":
		return defaultFingerprint.AnthropicBeta
	case "x-app":
		return defaultFingerprint.XApp
	case "anthropic-dangerous-direct-browser-access":
		return defaultFingerprint.DirectBrowserAccess
	case "x-stainless-lang":
		return defaultFingerprint.StainlessLang
	case "x-stainless-package-version":
		return defaultFingerprint.StainlessPackageVersion
	case "x-stainless-os":
		return defaultFingerprint.StainlessOS
	case "x-stainless-arch":
		return defaultFingerprint.StainlessArch
	case "x-stainless-runtime":
		return defaultFingerprint.StainlessRuntime
	case "x-stainless-runtime-version":
		return defaultFingerprint.StainlessRuntimeVersion
	case "x-stainless-retry-count":
		return defaultFingerprint.StainlessRetryCount
	case "x-stainless-timeout":
		return defaultFingerprint.StainlessTimeout
	default:
		return ""
	}
}

func claudeCodeOutgoingHeaderSummary(req *http.Request) map[string]string {
	if req == nil {
		return nil
	}
	headers := []string{
		"User-Agent",
		"Accept",
		"content-type",
		"anthropic-version",
		"anthropic-beta",
		"x-app",
		"anthropic-dangerous-direct-browser-access",
		"X-Stainless-Lang",
		"X-Stainless-Package-Version",
		"X-Stainless-OS",
		"X-Stainless-Arch",
		"X-Stainless-Runtime",
		"X-Stainless-Runtime-Version",
		"X-Stainless-Retry-Count",
		"X-Stainless-Timeout",
		"x-stainless-helper-method",
		"X-Claude-Code-Session-Id",
	}
	out := make(map[string]string, len(headers))
	for _, header := range headers {
		if value := strings.TrimSpace(getHeaderRaw(req.Header, header)); value != "" {
			out[header] = truncateDriftValue(value)
		}
	}
	return out
}

func sortedHeaderKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func diffTokenSets(expected, actual []string) ([]string, []string) {
	expectedSet := make(map[string]struct{}, len(expected))
	actualSet := make(map[string]struct{}, len(actual))
	for _, token := range expected {
		expectedSet[token] = struct{}{}
	}
	for _, token := range actual {
		actualSet[token] = struct{}{}
	}
	missing := make([]string, 0)
	for _, token := range expected {
		if _, ok := actualSet[token]; !ok {
			missing = append(missing, token)
		}
	}
	unexpected := make([]string, 0)
	for _, token := range actual {
		if _, ok := expectedSet[token]; !ok {
			unexpected = append(unexpected, token)
		}
	}
	return missing, unexpected
}

func splitBetaTokens(header string) []string {
	parts := strings.Split(header, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	return out
}

func extractBillingCCVersion(body []byte) string {
	match := ccVersionSummaryRe.FindSubmatch(body)
	if len(match) < 2 {
		return ""
	}
	return string(match[1])
}

func claudeCodeSemverCore(version string) string {
	parts := strings.Split(strings.TrimSpace(version), ".")
	if len(parts) < 3 {
		return ""
	}
	return strings.Join(parts[:3], ".")
}

func scorePercent(passed, total int) int {
	if total <= 0 {
		return 0
	}
	score := passed * 100 / total
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func claudeCodeDriftStatus(status ClaudeCodeFingerprintDriftStatus) string {
	if status.ActiveProfileID == "" {
		return "idle"
	}
	if !status.SampleApplied || len(status.MissingHeaders) > 0 || len(status.BetaMissing) > 0 || !status.CCVersionMatches {
		return "warning"
	}
	if len(status.HeaderMismatches) > 0 || len(status.DefaultOverwrites) > 0 {
		return "warning"
	}
	return "ok"
}

func claudeCodeDriftMessage(status ClaudeCodeFingerprintDriftStatus) string {
	switch status.Status {
	case "ok":
		return "选中的 Claude Code HTTP 样本已应用，关键 header 与 cc_version 一致。"
	case "warning":
		return "检测到 outgoing 指纹与选中样本存在偏差，请查看缺失、覆盖或 beta 差异。"
	default:
		if status.Message != "" {
			return status.Message
		}
		return "等待下一次 OAuth 上游转发生成巡检结果。"
	}
}

func claudeCodeDriftWarnings(status ClaudeCodeFingerprintDriftStatus) []string {
	warnings := make([]string, 0, 4)
	if len(status.MissingHeaders) > 0 {
		warnings = append(warnings, "存在样本要求但 outgoing 缺失的 header")
	}
	if len(status.DefaultOverwrites) > 0 {
		warnings = append(warnings, "部分 header 被内置默认值覆盖")
	}
	if len(status.BetaMissing) > 0 || len(status.BetaUnexpected) > 0 {
		warnings = append(warnings, "anthropic-beta token 与样本存在集合差异")
	}
	if status.ActiveProfileID != "" && !status.CCVersionMatches {
		warnings = append(warnings, "billing header 的 cc_version 与 User-Agent 版本不一致")
	}
	return warnings
}

func truncateDriftValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= claudeCodeFingerprintDriftMaxValueLen {
		return value
	}
	return value[:claudeCodeFingerprintDriftMaxValueLen] + "..."
}
