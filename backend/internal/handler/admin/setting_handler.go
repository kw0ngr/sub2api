package admin

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// semverPattern 预编译 semver 格式校验正则
var semverPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// menuItemIDPattern validates custom menu item IDs: alphanumeric, hyphens, underscores only.
var menuItemIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// generateMenuItemID generates a short random hex ID for a custom menu item.
func generateMenuItemID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate menu item ID: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func scopesContainOpenID(scopes string) bool {
	for _, scope := range strings.Fields(strings.ToLower(strings.TrimSpace(scopes))) {
		if scope == "openid" {
			return true
		}
	}
	return false
}

// SettingHandler 系统设置处理器
type SettingHandler struct {
	settingService       *service.SettingService
	emailService         *service.EmailService
	turnstileService     *service.TurnstileService
	opsService           *service.OpsService
	paymentConfigService *service.PaymentConfigService
	paymentService       *service.PaymentService
}

// NewSettingHandler 创建系统设置处理器
func NewSettingHandler(settingService *service.SettingService, emailService *service.EmailService, turnstileService *service.TurnstileService, opsService *service.OpsService, paymentConfigService *service.PaymentConfigService, paymentService *service.PaymentService) *SettingHandler {
	return &SettingHandler{
		settingService:       settingService,
		emailService:         emailService,
		turnstileService:     turnstileService,
		opsService:           opsService,
		paymentConfigService: paymentConfigService,
		paymentService:       paymentService,
	}
}

// GetSettings 获取所有系统设置
// GET /api/v1/admin/settings
func (h *SettingHandler) GetSettings(c *gin.Context) {
	settings, err := h.settingService.GetAllSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Check if ops monitoring is enabled (respects config.ops.enabled)
	opsEnabled := h.opsService != nil && h.opsService.IsMonitoringEnabled(c.Request.Context())
	defaultSubscriptions := make([]dto.DefaultSubscriptionSetting, 0, len(settings.DefaultSubscriptions))
	for _, sub := range settings.DefaultSubscriptions {
		defaultSubscriptions = append(defaultSubscriptions, dto.DefaultSubscriptionSetting{
			GroupID:      sub.GroupID,
			ValidityDays: sub.ValidityDays,
		})
	}

	// Load payment config
	var paymentCfg *service.PaymentConfig
	if h.paymentConfigService != nil {
		paymentCfg, _ = h.paymentConfigService.GetPaymentConfig(c.Request.Context())
	}
	if paymentCfg == nil {
		paymentCfg = &service.PaymentConfig{}
	}

	payload := dto.SystemSettings{
		RegistrationEnabled:                  settings.RegistrationEnabled,
		EmailVerifyEnabled:                   settings.EmailVerifyEnabled,
		RegistrationEmailSuffixWhitelist:     settings.RegistrationEmailSuffixWhitelist,
		PromoCodeEnabled:                     settings.PromoCodeEnabled,
		PasswordResetEnabled:                 settings.PasswordResetEnabled,
		FrontendURL:                          settings.FrontendURL,
		InvitationCodeEnabled:                settings.InvitationCodeEnabled,
		TotpEnabled:                          settings.TotpEnabled,
		TotpEncryptionKeyConfigured:          h.settingService.IsTotpEncryptionKeyConfigured(),
		SMTPHost:                             settings.SMTPHost,
		SMTPPort:                             settings.SMTPPort,
		SMTPUsername:                         settings.SMTPUsername,
		SMTPPasswordConfigured:               settings.SMTPPasswordConfigured,
		SMTPFrom:                             settings.SMTPFrom,
		SMTPFromName:                         settings.SMTPFromName,
		SMTPUseTLS:                           settings.SMTPUseTLS,
		TurnstileEnabled:                     settings.TurnstileEnabled,
		TurnstileSiteKey:                     settings.TurnstileSiteKey,
		TurnstileSecretKeyConfigured:         settings.TurnstileSecretKeyConfigured,
		LinuxDoConnectEnabled:                settings.LinuxDoConnectEnabled,
		LinuxDoConnectClientID:               settings.LinuxDoConnectClientID,
		LinuxDoConnectClientSecretConfigured: settings.LinuxDoConnectClientSecretConfigured,
		LinuxDoConnectRedirectURL:            settings.LinuxDoConnectRedirectURL,
		OIDCConnectEnabled:                   settings.OIDCConnectEnabled,
		OIDCConnectProviderName:              settings.OIDCConnectProviderName,
		OIDCConnectClientID:                  settings.OIDCConnectClientID,
		OIDCConnectClientSecretConfigured:    settings.OIDCConnectClientSecretConfigured,
		OIDCConnectIssuerURL:                 settings.OIDCConnectIssuerURL,
		OIDCConnectDiscoveryURL:              settings.OIDCConnectDiscoveryURL,
		OIDCConnectAuthorizeURL:              settings.OIDCConnectAuthorizeURL,
		OIDCConnectTokenURL:                  settings.OIDCConnectTokenURL,
		OIDCConnectUserInfoURL:               settings.OIDCConnectUserInfoURL,
		OIDCConnectJWKSURL:                   settings.OIDCConnectJWKSURL,
		OIDCConnectScopes:                    settings.OIDCConnectScopes,
		OIDCConnectRedirectURL:               settings.OIDCConnectRedirectURL,
		OIDCConnectFrontendRedirectURL:       settings.OIDCConnectFrontendRedirectURL,
		OIDCConnectTokenAuthMethod:           settings.OIDCConnectTokenAuthMethod,
		OIDCConnectUsePKCE:                   settings.OIDCConnectUsePKCE,
		OIDCConnectValidateIDToken:           settings.OIDCConnectValidateIDToken,
		OIDCConnectAllowedSigningAlgs:        settings.OIDCConnectAllowedSigningAlgs,
		OIDCConnectClockSkewSeconds:          settings.OIDCConnectClockSkewSeconds,
		OIDCConnectRequireEmailVerified:      settings.OIDCConnectRequireEmailVerified,
		OIDCConnectUserInfoEmailPath:         settings.OIDCConnectUserInfoEmailPath,
		OIDCConnectUserInfoIDPath:            settings.OIDCConnectUserInfoIDPath,
		OIDCConnectUserInfoUsernamePath:      settings.OIDCConnectUserInfoUsernamePath,
		SiteName:                             settings.SiteName,
		SiteLogo:                             settings.SiteLogo,
		SiteSubtitle:                         settings.SiteSubtitle,
		APIBaseURL:                           settings.APIBaseURL,
		ContactInfo:                          settings.ContactInfo,
		DocURL:                               settings.DocURL,
		HomeContent:                          settings.HomeContent,
		HideCcsImportButton:                  settings.HideCcsImportButton,
		PurchaseSubscriptionEnabled:          settings.PurchaseSubscriptionEnabled,
		PurchaseSubscriptionURL:              settings.PurchaseSubscriptionURL,
		TableDefaultPageSize:                 settings.TableDefaultPageSize,
		TablePageSizeOptions:                 settings.TablePageSizeOptions,
		CustomMenuItems:                      dto.ParseCustomMenuItems(settings.CustomMenuItems),
		CustomEndpoints:                      dto.ParseCustomEndpoints(settings.CustomEndpoints),
		DefaultConcurrency:                   settings.DefaultConcurrency,
		DefaultBalance:                       settings.DefaultBalance,
		DefaultSubscriptions:                 defaultSubscriptions,
		EnableModelFallback:                  settings.EnableModelFallback,
		FallbackModelAnthropic:               settings.FallbackModelAnthropic,
		FallbackModelOpenAI:                  settings.FallbackModelOpenAI,
		FallbackModelGemini:                  settings.FallbackModelGemini,
		FallbackModelAntigravity:             settings.FallbackModelAntigravity,
		EnableIdentityPatch:                  settings.EnableIdentityPatch,
		IdentityPatchPrompt:                  settings.IdentityPatchPrompt,
		OpsMonitoringEnabled:                 opsEnabled && settings.OpsMonitoringEnabled,
		OpsRealtimeMonitoringEnabled:         settings.OpsRealtimeMonitoringEnabled,
		OpsQueryModeDefault:                  settings.OpsQueryModeDefault,
		OpsMetricsIntervalSeconds:            settings.OpsMetricsIntervalSeconds,
		MinClaudeCodeVersion:                 settings.MinClaudeCodeVersion,
		MaxClaudeCodeVersion:                 settings.MaxClaudeCodeVersion,
		AllowUngroupedKeyScheduling:          settings.AllowUngroupedKeyScheduling,
		BackendModeEnabled:                   settings.BackendModeEnabled,
		EnableFingerprintUnification:         settings.EnableFingerprintUnification,
		EnableMetadataPassthrough:            settings.EnableMetadataPassthrough,
		EnableCCHSigning:                     settings.EnableCCHSigning,
		EnableAnthropicCacheTTL1hInjection:   settings.EnableAnthropicCacheTTL1hInjection,
		RewriteMessageCacheControl:           settings.RewriteMessageCacheControl,
		EnableGLMZCodeStrongMimic:            settings.EnableGLMZCodeStrongMimic,
		OpenAICyberSafetyRetryEnabled:        settings.OpenAICyberSafetyRetryEnabled,
		OpenAIGPT56SolDefaultMaxReasoning:    settings.OpenAIGPT56SolDefaultMaxReasoning,
		WebSearchEmulationEnabled:            settings.WebSearchEmulationEnabled,
		BalanceLowNotifyEnabled:              settings.BalanceLowNotifyEnabled,
		BalanceLowNotifyThreshold:            settings.BalanceLowNotifyThreshold,
		BalanceLowNotifyRechargeURL:          settings.BalanceLowNotifyRechargeURL,
		AccountQuotaNotifyEnabled:            settings.AccountQuotaNotifyEnabled,
		AccountQuotaNotifyEmails:             dto.NotifyEmailEntriesFromService(settings.AccountQuotaNotifyEmails),
		ChannelMonitorEnabled:                settings.ChannelMonitorEnabled,
		ChannelMonitorDefaultIntervalSeconds: settings.ChannelMonitorDefaultIntervalSeconds,
		AvailableChannelsEnabled:             settings.AvailableChannelsEnabled,
		PaymentEnabled:                       paymentCfg.Enabled,
		PaymentMinAmount:                     paymentCfg.MinAmount,
		PaymentMaxAmount:                     paymentCfg.MaxAmount,
		PaymentDailyLimit:                    paymentCfg.DailyLimit,
		PaymentOrderTimeoutMin:               paymentCfg.OrderTimeoutMin,
		PaymentMaxPendingOrders:              paymentCfg.MaxPendingOrders,
		PaymentEnabledTypes:                  paymentCfg.EnabledTypes,
		PaymentBalanceDisabled:               paymentCfg.BalanceDisabled,
		PaymentBalanceRechargeMultiplier:     paymentCfg.BalanceRechargeMultiplier,
		PaymentRechargeFeeRate:               paymentCfg.RechargeFeeRate,
		PaymentLoadBalanceStrat:              paymentCfg.LoadBalanceStrategy,
		PaymentProductNamePrefix:             paymentCfg.ProductNamePrefix,
		PaymentProductNameSuffix:             paymentCfg.ProductNameSuffix,
		PaymentHelpImageURL:                  paymentCfg.HelpImageURL,
		PaymentHelpText:                      paymentCfg.HelpText,
		PaymentCancelRateLimitEnabled:        paymentCfg.CancelRateLimitEnabled,
		PaymentCancelRateLimitMax:            paymentCfg.CancelRateLimitMax,
		PaymentCancelRateLimitWindow:         paymentCfg.CancelRateLimitWindow,
		PaymentCancelRateLimitUnit:           paymentCfg.CancelRateLimitUnit,
		PaymentCancelRateLimitMode:           paymentCfg.CancelRateLimitMode,
	}
	if fastPolicy, err := h.settingService.GetOpenAIFastPolicySettings(c.Request.Context()); err != nil {
		slog.Error("openai_fast_policy_settings_get_failed", "error", err)
	} else if fastPolicy != nil {
		payload.OpenAIFastPolicySettings = openaiFastPolicySettingsToDTO(fastPolicy)
	}
	response.Success(c, payload)
}

// GetClaudeCodeFingerprintLibrary 获取自动捕获的 Claude Code HTTP 指纹样本库
// GET /api/v1/admin/settings/claude-code-fingerprints
func (h *SettingHandler) GetClaudeCodeFingerprintLibrary(c *gin.Context) {
	library, err := h.settingService.GetClaudeCodeFingerprintLibrary(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, library)
}

// GetClaudeCodeFingerprintDrift 获取最近一次 outgoing Claude Code 指纹巡检结果
// GET /api/v1/admin/settings/claude-code-fingerprints/drift
func (h *SettingHandler) GetClaudeCodeFingerprintDrift(c *gin.Context) {
	status, err := h.settingService.GetClaudeCodeFingerprintDriftStatus(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

type ClaudeCodeFingerprintLabDiagnosis struct {
	Level              string                                   `json:"level"`
	Tone               string                                   `json:"tone"`
	Detail             string                                   `json:"detail"`
	Score              int                                      `json:"score"`
	HTTP               ClaudeCodeFingerprintLabHTTP             `json:"http"`
	TLS                ClaudeCodeFingerprintLabTLS              `json:"tls"`
	Drift              service.ClaudeCodeFingerprintDriftStatus `json:"drift"`
	Cards              []ClaudeCodeFingerprintLabCard           `json:"cards"`
	Checks             []ClaudeCodeFingerprintLabCheck          `json:"checks"`
	RecommendedActions []string                                 `json:"recommended_actions"`
	GeneratedAt        int64                                    `json:"generated_at"`
}

type ClaudeCodeFingerprintLabHTTP struct {
	Mode                 string                                `json:"mode"`
	ActiveProfileID      string                                `json:"active_profile_id,omitempty"`
	ActiveProfileName    string                                `json:"active_profile_name,omitempty"`
	RecommendedProfileID string                                `json:"recommended_profile_id,omitempty"`
	RecommendedName      string                                `json:"recommended_name,omitempty"`
	SampleApplied        bool                                  `json:"sample_applied"`
	SampleAvailable      bool                                  `json:"sample_available"`
	SampleSummary        string                                `json:"sample_summary"`
	PreviewSource        string                                `json:"preview_source"`
	Profile              *service.ClaudeCodeFingerprintProfile `json:"profile,omitempty"`
}

type ClaudeCodeFingerprintLabTLS struct {
	Level                  string   `json:"level"`
	Tone                   string   `json:"tone"`
	Score                  int      `json:"score"`
	RecommendedTemplate    string   `json:"recommended_template"`
	RecommendedDescription string   `json:"recommended_description"`
	ConsistencySummary     string   `json:"consistency_summary"`
	Reasons                []string `json:"reasons"`
}

type ClaudeCodeFingerprintLabCard struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Tone        string `json:"tone"`
}

type ClaudeCodeFingerprintLabCheck struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// GetClaudeCodeFingerprintLabDiagnosis 获取 Fingerprint Lab 聚合诊断
// GET /api/v1/admin/settings/claude-code-fingerprints/lab-diagnosis
func (h *SettingHandler) GetClaudeCodeFingerprintLabDiagnosis(c *gin.Context) {
	library, err := h.settingService.GetClaudeCodeFingerprintLibrary(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	drift, err := h.settingService.GetClaudeCodeFingerprintDriftStatus(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, buildClaudeCodeFingerprintLabDiagnosis(library, drift))
}

type UpdateActiveClaudeCodeFingerprintRequest struct {
	ID string `json:"id"`
}

// UpdateActiveClaudeCodeFingerprint 设置全局选中的 Claude Code HTTP 指纹样本
// PUT /api/v1/admin/settings/claude-code-fingerprints/active
func (h *SettingHandler) UpdateActiveClaudeCodeFingerprint(c *gin.Context) {
	var req UpdateActiveClaudeCodeFingerprintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.settingService.SetActiveClaudeCodeFingerprintProfile(c.Request.Context(), req.ID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	library, err := h.settingService.GetClaudeCodeFingerprintLibrary(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, library)
}

// DeleteClaudeCodeFingerprint 删除一个自动捕获的 Claude Code HTTP 指纹样本
// DELETE /api/v1/admin/settings/claude-code-fingerprints/:id
func (h *SettingHandler) DeleteClaudeCodeFingerprint(c *gin.Context) {
	if err := h.settingService.DeleteClaudeCodeFingerprintProfile(c.Request.Context(), c.Param("id")); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	library, err := h.settingService.GetClaudeCodeFingerprintLibrary(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, library)
}

func buildClaudeCodeFingerprintLabDiagnosis(library service.ClaudeCodeFingerprintLibrary, drift service.ClaudeCodeFingerprintDriftStatus) ClaudeCodeFingerprintLabDiagnosis {
	activeProfile := findClaudeCodeFingerprintProfileByID(library.Profiles, library.ActiveID)
	recommendedProfile := recommendedClaudeCodeFingerprintProfile(library.Profiles)
	driftProfile := profileFromDriftSummary(drift)

	http := ClaudeCodeFingerprintLabHTTP{
		Mode:                 "waiting",
		ActiveProfileID:      strings.TrimSpace(library.ActiveID),
		SampleApplied:        drift.SampleApplied,
		SampleAvailable:      len(library.Profiles) > 0 || driftProfile != nil,
		SampleSummary:        "等待真实 Claude Code 请求通过网关后自动学习。",
		PreviewSource:        "none",
		RecommendedProfileID: "",
		RecommendedName:      "",
	}
	if recommendedProfile != nil {
		http.RecommendedProfileID = recommendedProfile.ID
		http.RecommendedName = recommendedProfile.Name
	}
	if activeProfile != nil {
		http.Mode = "fixed"
		http.ActiveProfileName = activeProfile.Name
		http.SampleSummary = summarizeClaudeCodeFingerprintProfile(*activeProfile)
		http.PreviewSource = "active"
		profileCopy := *activeProfile
		http.Profile = &profileCopy
	} else if driftProfile != nil {
		http.Mode = "outgoing_preview"
		http.SampleSummary = summarizeClaudeCodeFingerprintProfile(*driftProfile)
		http.PreviewSource = "recent_outgoing"
		http.Profile = driftProfile
	} else if recommendedProfile != nil {
		http.Mode = "library_preview"
		http.SampleSummary = summarizeClaudeCodeFingerprintProfile(*recommendedProfile)
		http.PreviewSource = "library_latest"
		profileCopy := *recommendedProfile
		http.Profile = &profileCopy
	}

	tls := buildClaudeCodeFingerprintLabTLS(http.Profile, activeProfile != nil)
	score := summarizeClaudeCodeFingerprintLab(http, tls, drift, strings.TrimSpace(library.ActiveID) != "" && activeProfile == nil)
	level, tone, detail := claudeCodeLabDriftLabel(http, tls, drift, score)

	return ClaudeCodeFingerprintLabDiagnosis{
		Level:              level,
		Tone:               tone,
		Detail:             detail,
		Score:              score,
		HTTP:               http,
		TLS:                tls,
		Drift:              drift,
		Cards:              buildClaudeCodeHTTPCard(http, tls, drift),
		Checks:             buildClaudeCodeFingerprintLabChecks(http, tls, drift),
		RecommendedActions: buildClaudeCodeFingerprintLabActions(http, tls, drift, strings.TrimSpace(library.ActiveID) != "" && activeProfile == nil),
		GeneratedAt:        time.Now().Unix(),
	}
}

func findClaudeCodeFingerprintProfileByID(profiles []service.ClaudeCodeFingerprintProfile, id string) *service.ClaudeCodeFingerprintProfile {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	for i := range profiles {
		if profiles[i].ID == id {
			return &profiles[i]
		}
	}
	return nil
}

func recommendedClaudeCodeFingerprintProfile(profiles []service.ClaudeCodeFingerprintProfile) *service.ClaudeCodeFingerprintProfile {
	if len(profiles) == 0 {
		return nil
	}
	candidates := append([]service.ClaudeCodeFingerprintProfile(nil), profiles...)
	sort.SliceStable(candidates, func(i, j int) bool {
		leftSeen := candidates[i].LastSeenAt
		if leftSeen == 0 {
			leftSeen = candidates[i].UpdatedAt
		}
		rightSeen := candidates[j].LastSeenAt
		if rightSeen == 0 {
			rightSeen = candidates[j].UpdatedAt
		}
		if leftSeen != rightSeen {
			return leftSeen > rightSeen
		}
		if candidates[i].CompletenessScore != candidates[j].CompletenessScore {
			return candidates[i].CompletenessScore > candidates[j].CompletenessScore
		}
		return candidates[i].SeenCount > candidates[j].SeenCount
	})
	return &candidates[0]
}

func profileFromDriftSummary(drift service.ClaudeCodeFingerprintDriftStatus) *service.ClaudeCodeFingerprintProfile {
	summary := drift.OutgoingHeaderSummary
	if len(summary) == 0 {
		return nil
	}
	userAgent := getLabSummaryHeader(summary, "User-Agent")
	if strings.TrimSpace(userAgent) == "" {
		return nil
	}
	updatedAt := drift.UpdatedAt
	if updatedAt == 0 {
		updatedAt = time.Now().Unix()
	}
	return &service.ClaudeCodeFingerprintProfile{
		ID:                      "outgoing-drift-preview",
		Name:                    "最近 outgoing 捕获",
		Description:             "来自最近一次真实转发请求，仅用于 HTTP/TLS 一致性预览。",
		Source:                  "outgoing",
		AccountID:               drift.AccountID,
		AccountName:             strings.TrimSpace(drift.AccountName),
		UserAgent:               userAgent,
		Accept:                  getLabSummaryHeader(summary, "Accept"),
		ContentType:             getLabSummaryHeader(summary, "content-type"),
		AnthropicVersion:        getLabSummaryHeader(summary, "anthropic-version"),
		AnthropicBeta:           getLabSummaryHeader(summary, "anthropic-beta"),
		XApp:                    getLabSummaryHeader(summary, "x-app"),
		DirectBrowserAccess:     getLabSummaryHeader(summary, "anthropic-dangerous-direct-browser-access"),
		StainlessLang:           getLabSummaryHeader(summary, "X-Stainless-Lang"),
		StainlessPackageVersion: getLabSummaryHeader(summary, "X-Stainless-Package-Version"),
		StainlessOS:             getLabSummaryHeader(summary, "X-Stainless-OS"),
		StainlessArch:           getLabSummaryHeader(summary, "X-Stainless-Arch"),
		StainlessRuntime:        getLabSummaryHeader(summary, "X-Stainless-Runtime"),
		StainlessRuntimeVersion: getLabSummaryHeader(summary, "X-Stainless-Runtime-Version"),
		StainlessRetryCount:     getLabSummaryHeader(summary, "X-Stainless-Retry-Count"),
		StainlessTimeout:        getLabSummaryHeader(summary, "X-Stainless-Timeout"),
		HelperMethod:            getLabSummaryHeader(summary, "x-stainless-helper-method"),
		CompletenessScore:       len(summary),
		CreatedAt:               updatedAt,
		UpdatedAt:               updatedAt,
		LastSeenAt:              updatedAt,
		SeenCount:               1,
	}
}

func getLabSummaryHeader(summary map[string]string, header string) string {
	if len(summary) == 0 {
		return ""
	}
	if value := strings.TrimSpace(summary[header]); value != "" {
		return value
	}
	target := strings.ToLower(strings.TrimSpace(header))
	for key, value := range summary {
		if strings.ToLower(strings.TrimSpace(key)) == target {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func summarizeClaudeCodeFingerprintProfile(profile service.ClaudeCodeFingerprintProfile) string {
	version := service.ExtractCLIVersion(profile.UserAgent)
	if version == "" {
		version = "未知版本"
	}
	os := strings.TrimSpace(profile.StainlessOS)
	if os == "" {
		os = "未知 OS"
	}
	arch := strings.TrimSpace(profile.StainlessArch)
	if arch == "" {
		arch = "未知架构"
	}
	runtime := strings.TrimSpace(strings.Join([]string{profile.StainlessRuntime, profile.StainlessRuntimeVersion}, " "))
	if runtime == "" {
		runtime = "runtime?"
	}
	return fmt.Sprintf("Claude Code %s / %s / %s / %s", version, os, arch, runtime)
}

func buildClaudeCodeFingerprintLabTLS(profile *service.ClaudeCodeFingerprintProfile, fixedHTTPProfile bool) ClaudeCodeFingerprintLabTLS {
	tls := ClaudeCodeFingerprintLabTLS{
		Level:                  "中",
		Tone:                   "neutral",
		Score:                  58,
		RecommendedTemplate:    "Built-in Default (Node.js 24.x)",
		RecommendedDescription: "没有绑定真实模板时，先使用内置 Node.js 24.x 方向模板；有真实采集结果后在账号侧固定绑定。",
		ConsistencySummary:     "缺少 HTTP 样本时无法判断系统/架构，先保持保守默认。",
		Reasons:                []string{"内置模板比随机 TLS 更稳定，适合作为采集前的兜底。"},
	}
	if profile == nil {
		return tls
	}

	runtime := strings.ToLower(strings.TrimSpace(profile.StainlessRuntime + " " + profile.StainlessRuntimeVersion))
	osName := strings.ToLower(strings.TrimSpace(profile.StainlessOS))
	score := 62
	reasons := []string{"HTTP 样本可用，建议 TLS 模板和它的 OS / Arch / Runtime 保持同一族。"}
	if strings.Contains(runtime, "node") {
		score += 12
		reasons = append(reasons, "HTTP 样本 runtime 指向 Node.js。")
	}
	if strings.Contains(runtime, "v24") || strings.Contains(runtime, "24.") {
		score += 12
		reasons = append(reasons, "HTTP 样本 runtime 版本接近 Node.js 24。")
	}
	if strings.Contains(osName, "darwin") || strings.Contains(osName, "mac") {
		score += 4
		reasons = append(reasons, "HTTP 样本 OS 指向 macOS / Darwin。")
	}
	if !fixedHTTPProfile {
		score -= 8
		reasons = append([]string{"当前 HTTP 样本还未固定，TLS 推荐仅按最近捕获预览计算。"}, reasons...)
	}
	score = clampClaudeCodeLabScore(score)
	level := "中"
	tone := "neutral"
	if score >= 75 {
		level = "高"
		tone = "good"
	} else if score < 55 {
		level = "低"
		tone = "warn"
	}

	return ClaudeCodeFingerprintLabTLS{
		Level:                  level,
		Tone:                   tone,
		Score:                  score,
		RecommendedTemplate:    "Built-in Default (Node.js 24.x)",
		RecommendedDescription: "如果已导入真实 TLS 模板，优先选择名称/描述与该 HTTP 样本 OS、Arch、Runtime 一致的模板并固定到账号。",
		ConsistencySummary:     fmt.Sprintf("HTTP 样本：%s。TLS 推荐一致性：%s。", summarizeClaudeCodeFingerprintProfile(*profile), level),
		Reasons:                reasons,
	}
}

func buildClaudeCodeFingerprintLabChecks(http ClaudeCodeFingerprintLabHTTP, tls ClaudeCodeFingerprintLabTLS, drift service.ClaudeCodeFingerprintDriftStatus) []ClaudeCodeFingerprintLabCheck {
	checks := []ClaudeCodeFingerprintLabCheck{
		{
			Key:     "http_sample",
			Label:   "HTTP 样本",
			Status:  "blocked",
			Message: "还没有捕获到真实 Claude Code HTTP 样本。",
		},
		{
			Key:     "sample_applied",
			Label:   "样本生效",
			Status:  "warn",
			Message: "等待下一次真实转发刷新样本生效状态。",
		},
		{
			Key:     "default_overwrite",
			Label:   "默认值覆盖",
			Status:  "ok",
			Message: "未发现默认 header 覆盖样本字段。",
		},
		{
			Key:     "beta_tokens",
			Label:   "Beta Token",
			Status:  "ok",
			Message: "未发现 beta token 缺失或异常。",
		},
		{
			Key:     "cc_version",
			Label:   "cc_version",
			Status:  "warn",
			Message: "等待请求 body 中的 billing cc_version 和 UA 版本同时出现。",
		},
		{
			Key:     "tls_binding",
			Label:   "TLS 绑定",
			Status:  "warn",
			Message: tls.ConsistencySummary,
		},
	}

	if http.Mode == "fixed" {
		checks[0].Status = "ok"
		checks[0].Message = "已固定 HTTP 样本：" + http.SampleSummary
	} else if http.Profile != nil {
		checks[0].Status = "warn"
		checks[0].Message = "已有可预览样本，建议确认后固定。"
	}

	if drift.SampleApplied {
		checks[1].Status = "ok"
		checks[1].Message = "最近一次 outgoing 已确认应用选中样本。"
	} else if http.Mode == "fixed" && drift.Status == "warning" {
		checks[1].Status = "blocked"
		checks[1].Message = "已固定样本，但最近一次 outgoing 没有真正应用。"
	}

	if count := len(drift.DefaultOverwrites); count > 0 {
		checks[2].Status = "warn"
		checks[2].Message = fmt.Sprintf("发现 %d 个字段被默认值覆盖。", count)
	}

	if len(drift.BetaMissing)+len(drift.BetaUnexpected) > 0 {
		checks[3].Status = "warn"
		checks[3].Message = fmt.Sprintf("缺少 %d 个、额外 %d 个 beta token。", len(drift.BetaMissing), len(drift.BetaUnexpected))
	}

	if drift.CCVersionMatches {
		checks[4].Status = "ok"
		checks[4].Message = "UA 版本和 billing cc_version 一致。"
	} else if drift.CCVersionFromUA != "" || drift.CCVersionFromBilling != "" {
		checks[4].Status = "warn"
		checks[4].Message = fmt.Sprintf("UA=%s / billing=%s，需要保持一致。", emptyDash(drift.CCVersionFromUA), emptyDash(drift.CCVersionFromBilling))
	}

	if tls.Score >= 75 {
		checks[5].Status = "ok"
	} else if http.Profile == nil {
		checks[5].Status = "blocked"
		checks[5].Message = "需要先捕获 HTTP 样本，才能做 HTTP/TLS 绑定推荐。"
	}

	return checks
}

func summarizeClaudeCodeFingerprintLab(http ClaudeCodeFingerprintLabHTTP, tls ClaudeCodeFingerprintLabTLS, drift service.ClaudeCodeFingerprintDriftStatus, activeMissing bool) int {
	score := 25
	if http.Profile != nil {
		score = 58
	}
	if http.Mode == "fixed" {
		score = 70
	}
	if http.SampleApplied {
		score += 8
	}
	if drift.Status == "ok" {
		score = maxClaudeCodeLabScore(score, drift.Score)
		score += 6
	}
	if drift.Status == "warning" {
		score -= 12
	}
	if activeMissing {
		score -= 18
	}
	score += (tls.Score - 60) / 4
	return clampClaudeCodeLabScore(score)
}

func buildClaudeCodeHTTPCard(http ClaudeCodeFingerprintLabHTTP, tls ClaudeCodeFingerprintLabTLS, drift service.ClaudeCodeFingerprintDriftStatus) []ClaudeCodeFingerprintLabCard {
	httpTitle := "等待 HTTP 样本"
	httpTone := "warn"
	httpDesc := "让真实 Claude Code 通过网关请求一次后会自动学习。"
	if http.Mode == "fixed" {
		httpTitle = "HTTP 样本已固定"
		httpTone = "good"
		httpDesc = http.SampleSummary
	} else if http.Profile != nil {
		httpTitle = "有真实样本可应用"
		httpTone = "neutral"
		httpDesc = http.SampleSummary
	}

	return []ClaudeCodeFingerprintLabCard{
		{
			Title:       httpTitle,
			Description: httpDesc,
			Tone:        httpTone,
		},
		{
			Title:       "TLS 推荐：" + tls.Level,
			Description: fmt.Sprintf("%s · %d/100", tls.RecommendedTemplate, tls.Score),
			Tone:        tls.Tone,
		},
		{
			Title:       "漂移检测：" + claudeCodeLabStatusText(drift.Status),
			Description: strings.TrimSpace(drift.Message),
			Tone:        claudeCodeLabStatusTone(drift.Status),
		},
	}
}

func buildClaudeCodeFingerprintLabActions(http ClaudeCodeFingerprintLabHTTP, tls ClaudeCodeFingerprintLabTLS, drift service.ClaudeCodeFingerprintDriftStatus, activeMissing bool) []string {
	actions := make([]string, 0, 5)
	if http.Profile == nil {
		actions = append(actions, "让真实 Claude Code 使用当前网关发起一次请求，系统会自动学习 HTTP 指纹。")
	}
	if http.Mode != "fixed" && http.RecommendedProfileID != "" {
		actions = append(actions, "确认样本来源后点击“一键应用最近真实样本”，让后续 outgoing 固定使用它。")
	}
	if activeMissing {
		actions = append(actions, "当前固定样本已不存在，建议清空或重新选择一个真实样本。")
	}
	if len(drift.DefaultOverwrites) > 0 {
		actions = append(actions, "复查默认 header 注入逻辑，避免把已选样本字段覆盖回内置值。")
	}
	if len(drift.BetaMissing) > 0 {
		actions = append(actions, "补齐样本中的 anthropic-beta token，尤其是和 Claude Code 版本绑定的 beta。")
	}
	if drift.CCVersionFromUA != "" && drift.CCVersionFromBilling != "" && !drift.CCVersionMatches {
		actions = append(actions, "同步 User-Agent 版本和 billing metadata 中的 cc_version。")
	}
	if tls.Score < 75 && http.Profile != nil {
		actions = append(actions, "导入真实 TLS 采集模板，并在账号侧绑定与 HTTP 样本 OS/Runtime 一致的模板。")
	}
	if len(actions) == 0 {
		actions = append(actions, "保持当前配置，定期在真实请求后重新检测即可。")
	}
	return actions
}

func claudeCodeLabDriftLabel(http ClaudeCodeFingerprintLabHTTP, tls ClaudeCodeFingerprintLabTLS, drift service.ClaudeCodeFingerprintDriftStatus, score int) (string, string, string) {
	if http.Profile == nil {
		return "等待真实样本", "neutral", "还没有捕获到可复用 HTTP 指纹。"
	}
	if drift.Status == "warning" {
		return "建议修复", "warn", "最近 outgoing 摘要和当前样本存在偏差。"
	}
	if http.Mode == "fixed" && drift.SampleApplied && tls.Score >= 75 && score >= 75 {
		return "一致性高", "good", "HTTP 样本、outgoing 巡检和 TLS 推荐方向比较一致。"
	}
	return "可继续优化", "neutral", "已有样本或预览，但建议固定样本并绑定 TLS 模板。"
}

func claudeCodeLabStatusText(status string) string {
	switch status {
	case "ok":
		return "一致"
	case "warning":
		return "有偏差"
	case "idle":
		return "待采集"
	default:
		if strings.TrimSpace(status) == "" {
			return "待采集"
		}
		return status
	}
}

func claudeCodeLabStatusTone(status string) string {
	switch status {
	case "ok":
		return "good"
	case "warning":
		return "warn"
	default:
		return "neutral"
	}
}

func emptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return strings.TrimSpace(value)
}

func clampClaudeCodeLabScore(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func maxClaudeCodeLabScore(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// openaiFastPolicySettingsToDTO converts service -> dto for OpenAI fast policy.
func openaiFastPolicySettingsToDTO(s *service.OpenAIFastPolicySettings) *dto.OpenAIFastPolicySettings {
	if s == nil {
		return nil
	}
	rules := make([]dto.OpenAIFastPolicyRule, len(s.Rules))
	for i, r := range s.Rules {
		rules[i] = dto.OpenAIFastPolicyRule(r)
	}
	return &dto.OpenAIFastPolicySettings{Rules: rules}
}

// openaiFastPolicySettingsFromDTO converts dto -> service for OpenAI fast policy.
//
// 规范化 ServiceTier：在 DTO 进入 service 层之前统一把空字符串归一为
// service.OpenAIFastTierAny ("all")，避免管理员保存时空串与 "all" 同时
// 表达"匹配任意 tier"造成数据库取值的二义性。其它非空值原样透传，由
// service.SetOpenAIFastPolicySettings 负责合法值校验。
func openaiFastPolicySettingsFromDTO(s *dto.OpenAIFastPolicySettings) *service.OpenAIFastPolicySettings {
	if s == nil {
		return nil
	}
	rules := make([]service.OpenAIFastPolicyRule, len(s.Rules))
	for i, r := range s.Rules {
		rules[i] = service.OpenAIFastPolicyRule(r)
		tier := strings.ToLower(strings.TrimSpace(rules[i].ServiceTier))
		if tier == "" {
			tier = service.OpenAIFastTierAny
		}
		rules[i].ServiceTier = tier
	}
	return &service.OpenAIFastPolicySettings{Rules: rules}
}

// UpdateSettingsRequest 更新设置请求
type UpdateSettingsRequest struct {
	// 注册设置
	RegistrationEnabled              bool     `json:"registration_enabled"`
	EmailVerifyEnabled               bool     `json:"email_verify_enabled"`
	RegistrationEmailSuffixWhitelist []string `json:"registration_email_suffix_whitelist"`
	PromoCodeEnabled                 bool     `json:"promo_code_enabled"`
	PasswordResetEnabled             bool     `json:"password_reset_enabled"`
	FrontendURL                      string   `json:"frontend_url"`
	InvitationCodeEnabled            bool     `json:"invitation_code_enabled"`
	TotpEnabled                      bool     `json:"totp_enabled"` // TOTP 双因素认证

	// 邮件服务设置
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	SMTPFrom     string `json:"smtp_from_email"`
	SMTPFromName string `json:"smtp_from_name"`
	SMTPUseTLS   bool   `json:"smtp_use_tls"`

	// Cloudflare Turnstile 设置
	TurnstileEnabled   bool   `json:"turnstile_enabled"`
	TurnstileSiteKey   string `json:"turnstile_site_key"`
	TurnstileSecretKey string `json:"turnstile_secret_key"`

	// LinuxDo Connect OAuth 登录
	LinuxDoConnectEnabled      bool   `json:"linuxdo_connect_enabled"`
	LinuxDoConnectClientID     string `json:"linuxdo_connect_client_id"`
	LinuxDoConnectClientSecret string `json:"linuxdo_connect_client_secret"`
	LinuxDoConnectRedirectURL  string `json:"linuxdo_connect_redirect_url"`

	// Generic OIDC OAuth 登录
	OIDCConnectEnabled              bool   `json:"oidc_connect_enabled"`
	OIDCConnectProviderName         string `json:"oidc_connect_provider_name"`
	OIDCConnectClientID             string `json:"oidc_connect_client_id"`
	OIDCConnectClientSecret         string `json:"oidc_connect_client_secret"`
	OIDCConnectIssuerURL            string `json:"oidc_connect_issuer_url"`
	OIDCConnectDiscoveryURL         string `json:"oidc_connect_discovery_url"`
	OIDCConnectAuthorizeURL         string `json:"oidc_connect_authorize_url"`
	OIDCConnectTokenURL             string `json:"oidc_connect_token_url"`
	OIDCConnectUserInfoURL          string `json:"oidc_connect_userinfo_url"`
	OIDCConnectJWKSURL              string `json:"oidc_connect_jwks_url"`
	OIDCConnectScopes               string `json:"oidc_connect_scopes"`
	OIDCConnectRedirectURL          string `json:"oidc_connect_redirect_url"`
	OIDCConnectFrontendRedirectURL  string `json:"oidc_connect_frontend_redirect_url"`
	OIDCConnectTokenAuthMethod      string `json:"oidc_connect_token_auth_method"`
	OIDCConnectUsePKCE              bool   `json:"oidc_connect_use_pkce"`
	OIDCConnectValidateIDToken      bool   `json:"oidc_connect_validate_id_token"`
	OIDCConnectAllowedSigningAlgs   string `json:"oidc_connect_allowed_signing_algs"`
	OIDCConnectClockSkewSeconds     int    `json:"oidc_connect_clock_skew_seconds"`
	OIDCConnectRequireEmailVerified bool   `json:"oidc_connect_require_email_verified"`
	OIDCConnectUserInfoEmailPath    string `json:"oidc_connect_userinfo_email_path"`
	OIDCConnectUserInfoIDPath       string `json:"oidc_connect_userinfo_id_path"`
	OIDCConnectUserInfoUsernamePath string `json:"oidc_connect_userinfo_username_path"`

	// OEM设置
	SiteName                    string                `json:"site_name"`
	SiteLogo                    string                `json:"site_logo"`
	SiteSubtitle                string                `json:"site_subtitle"`
	APIBaseURL                  string                `json:"api_base_url"`
	ContactInfo                 string                `json:"contact_info"`
	DocURL                      string                `json:"doc_url"`
	HomeContent                 string                `json:"home_content"`
	HideCcsImportButton         bool                  `json:"hide_ccs_import_button"`
	PurchaseSubscriptionEnabled *bool                 `json:"purchase_subscription_enabled"`
	PurchaseSubscriptionURL     *string               `json:"purchase_subscription_url"`
	TableDefaultPageSize        int                   `json:"table_default_page_size"`
	TablePageSizeOptions        []int                 `json:"table_page_size_options"`
	CustomMenuItems             *[]dto.CustomMenuItem `json:"custom_menu_items"`
	CustomEndpoints             *[]dto.CustomEndpoint `json:"custom_endpoints"`

	// 默认配置
	DefaultConcurrency   int                              `json:"default_concurrency"`
	DefaultBalance       float64                          `json:"default_balance"`
	DefaultSubscriptions []dto.DefaultSubscriptionSetting `json:"default_subscriptions"`

	// Model fallback configuration
	EnableModelFallback      bool   `json:"enable_model_fallback"`
	FallbackModelAnthropic   string `json:"fallback_model_anthropic"`
	FallbackModelOpenAI      string `json:"fallback_model_openai"`
	FallbackModelGemini      string `json:"fallback_model_gemini"`
	FallbackModelAntigravity string `json:"fallback_model_antigravity"`

	// Identity patch configuration (Claude -> Gemini)
	EnableIdentityPatch bool   `json:"enable_identity_patch"`
	IdentityPatchPrompt string `json:"identity_patch_prompt"`

	// Ops monitoring (vNext)
	OpsMonitoringEnabled         *bool   `json:"ops_monitoring_enabled"`
	OpsRealtimeMonitoringEnabled *bool   `json:"ops_realtime_monitoring_enabled"`
	OpsQueryModeDefault          *string `json:"ops_query_mode_default"`
	OpsMetricsIntervalSeconds    *int    `json:"ops_metrics_interval_seconds"`

	MinClaudeCodeVersion string `json:"min_claude_code_version"`
	MaxClaudeCodeVersion string `json:"max_claude_code_version"`

	// 分组隔离
	AllowUngroupedKeyScheduling bool `json:"allow_ungrouped_key_scheduling"`

	// Backend Mode
	BackendModeEnabled bool `json:"backend_mode_enabled"`

	// Gateway forwarding behavior
	EnableFingerprintUnification       *bool `json:"enable_fingerprint_unification"`
	EnableMetadataPassthrough          *bool `json:"enable_metadata_passthrough"`
	EnableCCHSigning                   *bool `json:"enable_cch_signing"`
	EnableAnthropicCacheTTL1hInjection *bool `json:"enable_anthropic_cache_ttl_1h_injection"`
	RewriteMessageCacheControl         *bool `json:"rewrite_message_cache_control"`
	EnableGLMZCodeStrongMimic          *bool `json:"enable_glm_zcode_strong_mimic"`
	OpenAICyberSafetyRetryEnabled      *bool `json:"openai_cyber_safety_retry_enabled"`
	OpenAIGPT56SolDefaultMaxReasoning  *bool `json:"openai_gpt56_sol_default_max_reasoning_enabled"`

	// Balance low notification
	BalanceLowNotifyEnabled              *bool                   `json:"balance_low_notify_enabled"`
	BalanceLowNotifyThreshold            *float64                `json:"balance_low_notify_threshold"`
	BalanceLowNotifyRechargeURL          *string                 `json:"balance_low_notify_recharge_url"`
	AccountQuotaNotifyEnabled            *bool                   `json:"account_quota_notify_enabled"`
	AccountQuotaNotifyEmails             *[]dto.NotifyEmailEntry `json:"account_quota_notify_emails"`
	ChannelMonitorEnabled                *bool                   `json:"channel_monitor_enabled"`
	ChannelMonitorDefaultIntervalSeconds *int                    `json:"channel_monitor_default_interval_seconds"`
	AvailableChannelsEnabled             *bool                   `json:"available_channels_enabled"`

	// Payment configuration (integrated into settings, full replace)
	PaymentEnabled                   *bool    `json:"payment_enabled"`
	PaymentMinAmount                 *float64 `json:"payment_min_amount"`
	PaymentMaxAmount                 *float64 `json:"payment_max_amount"`
	PaymentDailyLimit                *float64 `json:"payment_daily_limit"`
	PaymentOrderTimeoutMin           *int     `json:"payment_order_timeout_minutes"`
	PaymentMaxPendingOrders          *int     `json:"payment_max_pending_orders"`
	PaymentEnabledTypes              []string `json:"payment_enabled_types"`
	PaymentBalanceDisabled           *bool    `json:"payment_balance_disabled"`
	PaymentBalanceRechargeMultiplier *float64 `json:"payment_balance_recharge_multiplier"`
	PaymentRechargeFeeRate           *float64 `json:"payment_recharge_fee_rate"`
	PaymentLoadBalanceStrat          *string  `json:"payment_load_balance_strategy"`
	PaymentProductNamePrefix         *string  `json:"payment_product_name_prefix"`
	PaymentProductNameSuffix         *string  `json:"payment_product_name_suffix"`
	PaymentHelpImageURL              *string  `json:"payment_help_image_url"`
	PaymentHelpText                  *string  `json:"payment_help_text"`

	// Cancel rate limit
	PaymentCancelRateLimitEnabled *bool   `json:"payment_cancel_rate_limit_enabled"`
	PaymentCancelRateLimitMax     *int    `json:"payment_cancel_rate_limit_max"`
	PaymentCancelRateLimitWindow  *int    `json:"payment_cancel_rate_limit_window"`
	PaymentCancelRateLimitUnit    *string `json:"payment_cancel_rate_limit_unit"`
	PaymentCancelRateLimitMode    *string `json:"payment_cancel_rate_limit_window_mode"`

	// OpenAI fast/flex policy (optional, only updated when provided)
	OpenAIFastPolicySettings *dto.OpenAIFastPolicySettings `json:"openai_fast_policy_settings,omitempty"`
}

// UpdateSettings 更新系统设置
// PUT /api/v1/admin/settings
func (h *SettingHandler) UpdateSettings(c *gin.Context) {
	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	previousSettings, err := h.settingService.GetAllSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 验证参数
	if req.DefaultConcurrency < 1 {
		req.DefaultConcurrency = 1
	}
	if req.DefaultBalance < 0 {
		req.DefaultBalance = 0
	}
	// 通用表格配置：兼容旧客户端未传字段时保留当前值。
	if req.TableDefaultPageSize <= 0 {
		req.TableDefaultPageSize = previousSettings.TableDefaultPageSize
	}
	if req.TablePageSizeOptions == nil {
		req.TablePageSizeOptions = previousSettings.TablePageSizeOptions
	}
	req.SMTPHost = strings.TrimSpace(req.SMTPHost)
	req.SMTPUsername = strings.TrimSpace(req.SMTPUsername)
	req.SMTPPassword = strings.TrimSpace(req.SMTPPassword)
	req.SMTPFrom = strings.TrimSpace(req.SMTPFrom)
	req.SMTPFromName = strings.TrimSpace(req.SMTPFromName)
	if req.SMTPPort <= 0 {
		req.SMTPPort = 587
	}
	req.DefaultSubscriptions = normalizeDefaultSubscriptions(req.DefaultSubscriptions)

	// SMTP 配置保护：如果请求中 smtp_host 为空但数据库中已有配置，则保留已有 SMTP 配置
	// 防止前端加载设置失败时空表单覆盖已保存的 SMTP 配置
	if req.SMTPHost == "" && previousSettings.SMTPHost != "" {
		req.SMTPHost = previousSettings.SMTPHost
		req.SMTPPort = previousSettings.SMTPPort
		req.SMTPUsername = previousSettings.SMTPUsername
		req.SMTPFrom = previousSettings.SMTPFrom
		req.SMTPFromName = previousSettings.SMTPFromName
		req.SMTPUseTLS = previousSettings.SMTPUseTLS
	}

	// Turnstile 参数验证
	if req.TurnstileEnabled {
		// 检查必填字段
		if req.TurnstileSiteKey == "" {
			response.BadRequest(c, "Turnstile Site Key is required when enabled")
			return
		}
		// 如果未提供 secret key，使用已保存的值（留空保留当前值）
		if req.TurnstileSecretKey == "" {
			if previousSettings.TurnstileSecretKey == "" {
				response.BadRequest(c, "Turnstile Secret Key is required when enabled")
				return
			}
			req.TurnstileSecretKey = previousSettings.TurnstileSecretKey
		}

		// 当 site_key 或 secret_key 任一变化时验证（避免配置错误导致无法登录）
		siteKeyChanged := previousSettings.TurnstileSiteKey != req.TurnstileSiteKey
		secretKeyChanged := previousSettings.TurnstileSecretKey != req.TurnstileSecretKey
		if siteKeyChanged || secretKeyChanged {
			if err := h.turnstileService.ValidateSecretKey(c.Request.Context(), req.TurnstileSecretKey); err != nil {
				response.ErrorFrom(c, err)
				return
			}
		}
	}

	// TOTP 双因素认证参数验证
	// 只有手动配置了加密密钥才允许启用 TOTP 功能
	if req.TotpEnabled && !previousSettings.TotpEnabled {
		// 尝试启用 TOTP，检查加密密钥是否已手动配置
		if !h.settingService.IsTotpEncryptionKeyConfigured() {
			response.BadRequest(c, "Cannot enable TOTP: TOTP_ENCRYPTION_KEY environment variable must be configured first. Generate a key with 'openssl rand -hex 32' and set it in your environment.")
			return
		}
	}

	// LinuxDo Connect 参数验证
	if req.LinuxDoConnectEnabled {
		req.LinuxDoConnectClientID = strings.TrimSpace(req.LinuxDoConnectClientID)
		req.LinuxDoConnectClientSecret = strings.TrimSpace(req.LinuxDoConnectClientSecret)
		req.LinuxDoConnectRedirectURL = strings.TrimSpace(req.LinuxDoConnectRedirectURL)

		if req.LinuxDoConnectClientID == "" {
			response.BadRequest(c, "LinuxDo Client ID is required when enabled")
			return
		}
		if req.LinuxDoConnectRedirectURL == "" {
			response.BadRequest(c, "LinuxDo Redirect URL is required when enabled")
			return
		}
		if err := config.ValidateAbsoluteHTTPURL(req.LinuxDoConnectRedirectURL); err != nil {
			response.BadRequest(c, "LinuxDo Redirect URL must be an absolute http(s) URL")
			return
		}

		// 如果未提供 client_secret，则保留现有值（如有）。
		if req.LinuxDoConnectClientSecret == "" {
			if previousSettings.LinuxDoConnectClientSecret == "" {
				response.BadRequest(c, "LinuxDo Client Secret is required when enabled")
				return
			}
			req.LinuxDoConnectClientSecret = previousSettings.LinuxDoConnectClientSecret
		}
	}

	// Generic OIDC 参数验证
	if req.OIDCConnectEnabled {
		req.OIDCConnectProviderName = strings.TrimSpace(req.OIDCConnectProviderName)
		req.OIDCConnectClientID = strings.TrimSpace(req.OIDCConnectClientID)
		req.OIDCConnectClientSecret = strings.TrimSpace(req.OIDCConnectClientSecret)
		req.OIDCConnectIssuerURL = strings.TrimSpace(req.OIDCConnectIssuerURL)
		req.OIDCConnectDiscoveryURL = strings.TrimSpace(req.OIDCConnectDiscoveryURL)
		req.OIDCConnectAuthorizeURL = strings.TrimSpace(req.OIDCConnectAuthorizeURL)
		req.OIDCConnectTokenURL = strings.TrimSpace(req.OIDCConnectTokenURL)
		req.OIDCConnectUserInfoURL = strings.TrimSpace(req.OIDCConnectUserInfoURL)
		req.OIDCConnectJWKSURL = strings.TrimSpace(req.OIDCConnectJWKSURL)
		req.OIDCConnectScopes = strings.TrimSpace(req.OIDCConnectScopes)
		req.OIDCConnectRedirectURL = strings.TrimSpace(req.OIDCConnectRedirectURL)
		req.OIDCConnectFrontendRedirectURL = strings.TrimSpace(req.OIDCConnectFrontendRedirectURL)
		req.OIDCConnectTokenAuthMethod = strings.ToLower(strings.TrimSpace(req.OIDCConnectTokenAuthMethod))
		req.OIDCConnectAllowedSigningAlgs = strings.TrimSpace(req.OIDCConnectAllowedSigningAlgs)
		req.OIDCConnectUserInfoEmailPath = strings.TrimSpace(req.OIDCConnectUserInfoEmailPath)
		req.OIDCConnectUserInfoIDPath = strings.TrimSpace(req.OIDCConnectUserInfoIDPath)
		req.OIDCConnectUserInfoUsernamePath = strings.TrimSpace(req.OIDCConnectUserInfoUsernamePath)

		if req.OIDCConnectProviderName == "" {
			req.OIDCConnectProviderName = "OIDC"
		}
		if req.OIDCConnectClientID == "" {
			response.BadRequest(c, "OIDC Client ID is required when enabled")
			return
		}
		if req.OIDCConnectIssuerURL == "" {
			response.BadRequest(c, "OIDC Issuer URL is required when enabled")
			return
		}
		if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectIssuerURL); err != nil {
			response.BadRequest(c, "OIDC Issuer URL must be an absolute http(s) URL")
			return
		}
		if req.OIDCConnectDiscoveryURL != "" {
			if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectDiscoveryURL); err != nil {
				response.BadRequest(c, "OIDC Discovery URL must be an absolute http(s) URL")
				return
			}
		}
		if req.OIDCConnectAuthorizeURL != "" {
			if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectAuthorizeURL); err != nil {
				response.BadRequest(c, "OIDC Authorize URL must be an absolute http(s) URL")
				return
			}
		}
		if req.OIDCConnectTokenURL != "" {
			if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectTokenURL); err != nil {
				response.BadRequest(c, "OIDC Token URL must be an absolute http(s) URL")
				return
			}
		}
		if req.OIDCConnectUserInfoURL != "" {
			if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectUserInfoURL); err != nil {
				response.BadRequest(c, "OIDC UserInfo URL must be an absolute http(s) URL")
				return
			}
		}
		if req.OIDCConnectRedirectURL == "" {
			response.BadRequest(c, "OIDC Redirect URL is required when enabled")
			return
		}
		if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectRedirectURL); err != nil {
			response.BadRequest(c, "OIDC Redirect URL must be an absolute http(s) URL")
			return
		}
		if req.OIDCConnectFrontendRedirectURL == "" {
			response.BadRequest(c, "OIDC Frontend Redirect URL is required when enabled")
			return
		}
		if err := config.ValidateFrontendRedirectURL(req.OIDCConnectFrontendRedirectURL); err != nil {
			response.BadRequest(c, "OIDC Frontend Redirect URL is invalid")
			return
		}
		if !scopesContainOpenID(req.OIDCConnectScopes) {
			response.BadRequest(c, "OIDC scopes must contain openid")
			return
		}
		switch req.OIDCConnectTokenAuthMethod {
		case "", "client_secret_post", "client_secret_basic", "none":
		default:
			response.BadRequest(c, "OIDC Token Auth Method must be one of client_secret_post/client_secret_basic/none")
			return
		}
		if req.OIDCConnectTokenAuthMethod == "none" && !req.OIDCConnectUsePKCE {
			response.BadRequest(c, "OIDC PKCE must be enabled when token_auth_method=none")
			return
		}
		if req.OIDCConnectClockSkewSeconds < 0 || req.OIDCConnectClockSkewSeconds > 600 {
			response.BadRequest(c, "OIDC clock skew seconds must be between 0 and 600")
			return
		}
		if req.OIDCConnectValidateIDToken {
			if req.OIDCConnectAllowedSigningAlgs == "" {
				response.BadRequest(c, "OIDC Allowed Signing Algs is required when validate_id_token=true")
				return
			}
		}
		if req.OIDCConnectJWKSURL != "" {
			if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectJWKSURL); err != nil {
				response.BadRequest(c, "OIDC JWKS URL must be an absolute http(s) URL")
				return
			}
		}
		if req.OIDCConnectTokenAuthMethod == "" || req.OIDCConnectTokenAuthMethod == "client_secret_post" || req.OIDCConnectTokenAuthMethod == "client_secret_basic" {
			if req.OIDCConnectClientSecret == "" {
				if previousSettings.OIDCConnectClientSecret == "" {
					response.BadRequest(c, "OIDC Client Secret is required when enabled")
					return
				}
				req.OIDCConnectClientSecret = previousSettings.OIDCConnectClientSecret
			}
		}
	}

	// “购买订阅”页面配置验证
	purchaseEnabled := previousSettings.PurchaseSubscriptionEnabled
	if req.PurchaseSubscriptionEnabled != nil {
		purchaseEnabled = *req.PurchaseSubscriptionEnabled
	}
	purchaseURL := previousSettings.PurchaseSubscriptionURL
	if req.PurchaseSubscriptionURL != nil {
		purchaseURL = strings.TrimSpace(*req.PurchaseSubscriptionURL)
	}

	// - 启用时要求 URL 合法且非空
	// - 禁用时允许为空；若提供了 URL 也做基本校验，避免误配置
	if purchaseEnabled {
		if purchaseURL == "" {
			response.BadRequest(c, "Purchase Subscription URL is required when enabled")
			return
		}
		if err := config.ValidateAbsoluteHTTPURL(purchaseURL); err != nil {
			response.BadRequest(c, "Purchase Subscription URL must be an absolute http(s) URL")
			return
		}
	} else if purchaseURL != "" {
		if err := config.ValidateAbsoluteHTTPURL(purchaseURL); err != nil {
			response.BadRequest(c, "Purchase Subscription URL must be an absolute http(s) URL")
			return
		}
	}

	// Frontend URL 验证
	req.FrontendURL = strings.TrimSpace(req.FrontendURL)
	if req.FrontendURL != "" {
		if err := config.ValidateAbsoluteHTTPURL(req.FrontendURL); err != nil {
			response.BadRequest(c, "Frontend URL must be an absolute http(s) URL")
			return
		}
	}

	// 自定义菜单项验证
	const (
		maxCustomMenuItems    = 20
		maxMenuItemLabelLen   = 50
		maxMenuItemURLLen     = 2048
		maxMenuItemIconSVGLen = 10 * 1024 // 10KB
		maxMenuItemIDLen      = 32
	)

	customMenuJSON := previousSettings.CustomMenuItems
	if req.CustomMenuItems != nil {
		items := *req.CustomMenuItems
		if len(items) > maxCustomMenuItems {
			response.BadRequest(c, "Too many custom menu items (max 20)")
			return
		}
		for i, item := range items {
			if strings.TrimSpace(item.Label) == "" {
				response.BadRequest(c, "Custom menu item label is required")
				return
			}
			if len(item.Label) > maxMenuItemLabelLen {
				response.BadRequest(c, "Custom menu item label is too long (max 50 characters)")
				return
			}
			if strings.TrimSpace(item.URL) == "" {
				response.BadRequest(c, "Custom menu item URL is required")
				return
			}
			if len(item.URL) > maxMenuItemURLLen {
				response.BadRequest(c, "Custom menu item URL is too long (max 2048 characters)")
				return
			}
			if err := config.ValidateAbsoluteHTTPURL(strings.TrimSpace(item.URL)); err != nil {
				response.BadRequest(c, "Custom menu item URL must be an absolute http(s) URL")
				return
			}
			if item.Visibility != "user" && item.Visibility != "admin" {
				response.BadRequest(c, "Custom menu item visibility must be 'user' or 'admin'")
				return
			}
			if len(item.IconSVG) > maxMenuItemIconSVGLen {
				response.BadRequest(c, "Custom menu item icon SVG is too large (max 10KB)")
				return
			}
			// Auto-generate ID if missing
			if strings.TrimSpace(item.ID) == "" {
				id, err := generateMenuItemID()
				if err != nil {
					response.Error(c, http.StatusInternalServerError, "Failed to generate menu item ID")
					return
				}
				items[i].ID = id
			} else if len(item.ID) > maxMenuItemIDLen {
				response.BadRequest(c, "Custom menu item ID is too long (max 32 characters)")
				return
			} else if !menuItemIDPattern.MatchString(item.ID) {
				response.BadRequest(c, "Custom menu item ID contains invalid characters (only a-z, A-Z, 0-9, - and _ are allowed)")
				return
			}
		}
		// ID uniqueness check
		seen := make(map[string]struct{}, len(items))
		for _, item := range items {
			if _, exists := seen[item.ID]; exists {
				response.BadRequest(c, "Duplicate custom menu item ID: "+item.ID)
				return
			}
			seen[item.ID] = struct{}{}
		}
		menuBytes, err := json.Marshal(items)
		if err != nil {
			response.BadRequest(c, "Failed to serialize custom menu items")
			return
		}
		customMenuJSON = string(menuBytes)
	}

	// 自定义端点验证
	const (
		maxCustomEndpoints        = 10
		maxEndpointNameLen        = 50
		maxEndpointURLLen         = 2048
		maxEndpointDescriptionLen = 200
	)

	customEndpointsJSON := previousSettings.CustomEndpoints
	if req.CustomEndpoints != nil {
		endpoints := *req.CustomEndpoints
		if len(endpoints) > maxCustomEndpoints {
			response.BadRequest(c, "Too many custom endpoints (max 10)")
			return
		}
		for _, ep := range endpoints {
			if strings.TrimSpace(ep.Name) == "" {
				response.BadRequest(c, "Custom endpoint name is required")
				return
			}
			if len(ep.Name) > maxEndpointNameLen {
				response.BadRequest(c, "Custom endpoint name is too long (max 50 characters)")
				return
			}
			if strings.TrimSpace(ep.Endpoint) == "" {
				response.BadRequest(c, "Custom endpoint URL is required")
				return
			}
			if len(ep.Endpoint) > maxEndpointURLLen {
				response.BadRequest(c, "Custom endpoint URL is too long (max 2048 characters)")
				return
			}
			if err := config.ValidateAbsoluteHTTPURL(strings.TrimSpace(ep.Endpoint)); err != nil {
				response.BadRequest(c, "Custom endpoint URL must be an absolute http(s) URL")
				return
			}
			if len(ep.Description) > maxEndpointDescriptionLen {
				response.BadRequest(c, "Custom endpoint description is too long (max 200 characters)")
				return
			}
		}
		endpointBytes, err := json.Marshal(endpoints)
		if err != nil {
			response.BadRequest(c, "Failed to serialize custom endpoints")
			return
		}
		customEndpointsJSON = string(endpointBytes)
	}

	// Ops metrics collector interval validation (seconds).
	if req.OpsMetricsIntervalSeconds != nil {
		v := *req.OpsMetricsIntervalSeconds
		if v < 60 {
			v = 60
		}
		if v > 3600 {
			v = 3600
		}
		req.OpsMetricsIntervalSeconds = &v
	}
	defaultSubscriptions := make([]service.DefaultSubscriptionSetting, 0, len(req.DefaultSubscriptions))
	for _, sub := range req.DefaultSubscriptions {
		defaultSubscriptions = append(defaultSubscriptions, service.DefaultSubscriptionSetting{
			GroupID:      sub.GroupID,
			ValidityDays: sub.ValidityDays,
		})
	}

	// 验证最低版本号格式（空字符串=禁用，或合法 semver）
	if req.MinClaudeCodeVersion != "" {
		if !semverPattern.MatchString(req.MinClaudeCodeVersion) {
			response.Error(c, http.StatusBadRequest, "min_claude_code_version must be empty or a valid semver (e.g. 2.1.63)")
			return
		}
	}

	// 验证最高版本号格式（空字符串=禁用，或合法 semver）
	if req.MaxClaudeCodeVersion != "" {
		if !semverPattern.MatchString(req.MaxClaudeCodeVersion) {
			response.Error(c, http.StatusBadRequest, "max_claude_code_version must be empty or a valid semver (e.g. 3.0.0)")
			return
		}
	}

	// 交叉验证：如果同时设置了最低和最高版本号，最高版本号必须 >= 最低版本号
	if req.MinClaudeCodeVersion != "" && req.MaxClaudeCodeVersion != "" {
		if service.CompareVersions(req.MaxClaudeCodeVersion, req.MinClaudeCodeVersion) < 0 {
			response.Error(c, http.StatusBadRequest, "max_claude_code_version must be greater than or equal to min_claude_code_version")
			return
		}
	}

	settings := &service.SystemSettings{
		RegistrationEnabled:              req.RegistrationEnabled,
		EmailVerifyEnabled:               req.EmailVerifyEnabled,
		RegistrationEmailSuffixWhitelist: req.RegistrationEmailSuffixWhitelist,
		PromoCodeEnabled:                 req.PromoCodeEnabled,
		PasswordResetEnabled:             req.PasswordResetEnabled,
		FrontendURL:                      req.FrontendURL,
		InvitationCodeEnabled:            req.InvitationCodeEnabled,
		TotpEnabled:                      req.TotpEnabled,
		SMTPHost:                         req.SMTPHost,
		SMTPPort:                         req.SMTPPort,
		SMTPUsername:                     req.SMTPUsername,
		SMTPPassword:                     req.SMTPPassword,
		SMTPFrom:                         req.SMTPFrom,
		SMTPFromName:                     req.SMTPFromName,
		SMTPUseTLS:                       req.SMTPUseTLS,
		TurnstileEnabled:                 req.TurnstileEnabled,
		TurnstileSiteKey:                 req.TurnstileSiteKey,
		TurnstileSecretKey:               req.TurnstileSecretKey,
		LinuxDoConnectEnabled:            req.LinuxDoConnectEnabled,
		LinuxDoConnectClientID:           req.LinuxDoConnectClientID,
		LinuxDoConnectClientSecret:       req.LinuxDoConnectClientSecret,
		LinuxDoConnectRedirectURL:        req.LinuxDoConnectRedirectURL,
		OIDCConnectEnabled:               req.OIDCConnectEnabled,
		OIDCConnectProviderName:          req.OIDCConnectProviderName,
		OIDCConnectClientID:              req.OIDCConnectClientID,
		OIDCConnectClientSecret:          req.OIDCConnectClientSecret,
		OIDCConnectIssuerURL:             req.OIDCConnectIssuerURL,
		OIDCConnectDiscoveryURL:          req.OIDCConnectDiscoveryURL,
		OIDCConnectAuthorizeURL:          req.OIDCConnectAuthorizeURL,
		OIDCConnectTokenURL:              req.OIDCConnectTokenURL,
		OIDCConnectUserInfoURL:           req.OIDCConnectUserInfoURL,
		OIDCConnectJWKSURL:               req.OIDCConnectJWKSURL,
		OIDCConnectScopes:                req.OIDCConnectScopes,
		OIDCConnectRedirectURL:           req.OIDCConnectRedirectURL,
		OIDCConnectFrontendRedirectURL:   req.OIDCConnectFrontendRedirectURL,
		OIDCConnectTokenAuthMethod:       req.OIDCConnectTokenAuthMethod,
		OIDCConnectUsePKCE:               req.OIDCConnectUsePKCE,
		OIDCConnectValidateIDToken:       req.OIDCConnectValidateIDToken,
		OIDCConnectAllowedSigningAlgs:    req.OIDCConnectAllowedSigningAlgs,
		OIDCConnectClockSkewSeconds:      req.OIDCConnectClockSkewSeconds,
		OIDCConnectRequireEmailVerified:  req.OIDCConnectRequireEmailVerified,
		OIDCConnectUserInfoEmailPath:     req.OIDCConnectUserInfoEmailPath,
		OIDCConnectUserInfoIDPath:        req.OIDCConnectUserInfoIDPath,
		OIDCConnectUserInfoUsernamePath:  req.OIDCConnectUserInfoUsernamePath,
		SiteName:                         req.SiteName,
		SiteLogo:                         req.SiteLogo,
		SiteSubtitle:                     req.SiteSubtitle,
		APIBaseURL:                       req.APIBaseURL,
		ContactInfo:                      req.ContactInfo,
		DocURL:                           req.DocURL,
		HomeContent:                      req.HomeContent,
		HideCcsImportButton:              req.HideCcsImportButton,
		PurchaseSubscriptionEnabled:      purchaseEnabled,
		PurchaseSubscriptionURL:          purchaseURL,
		TableDefaultPageSize:             req.TableDefaultPageSize,
		TablePageSizeOptions:             req.TablePageSizeOptions,
		CustomMenuItems:                  customMenuJSON,
		CustomEndpoints:                  customEndpointsJSON,
		DefaultConcurrency:               req.DefaultConcurrency,
		DefaultBalance:                   req.DefaultBalance,
		DefaultSubscriptions:             defaultSubscriptions,
		EnableModelFallback:              req.EnableModelFallback,
		FallbackModelAnthropic:           req.FallbackModelAnthropic,
		FallbackModelOpenAI:              req.FallbackModelOpenAI,
		FallbackModelGemini:              req.FallbackModelGemini,
		FallbackModelAntigravity:         req.FallbackModelAntigravity,
		EnableIdentityPatch:              req.EnableIdentityPatch,
		IdentityPatchPrompt:              req.IdentityPatchPrompt,
		MinClaudeCodeVersion:             req.MinClaudeCodeVersion,
		MaxClaudeCodeVersion:             req.MaxClaudeCodeVersion,
		AllowUngroupedKeyScheduling:      req.AllowUngroupedKeyScheduling,
		BackendModeEnabled:               req.BackendModeEnabled,
		OpsMonitoringEnabled: func() bool {
			if req.OpsMonitoringEnabled != nil {
				return *req.OpsMonitoringEnabled
			}
			return previousSettings.OpsMonitoringEnabled
		}(),
		OpsRealtimeMonitoringEnabled: func() bool {
			if req.OpsRealtimeMonitoringEnabled != nil {
				return *req.OpsRealtimeMonitoringEnabled
			}
			return previousSettings.OpsRealtimeMonitoringEnabled
		}(),
		OpsQueryModeDefault: func() string {
			if req.OpsQueryModeDefault != nil {
				return *req.OpsQueryModeDefault
			}
			return previousSettings.OpsQueryModeDefault
		}(),
		OpsMetricsIntervalSeconds: func() int {
			if req.OpsMetricsIntervalSeconds != nil {
				return *req.OpsMetricsIntervalSeconds
			}
			return previousSettings.OpsMetricsIntervalSeconds
		}(),
		EnableFingerprintUnification: func() bool {
			if req.EnableFingerprintUnification != nil {
				return *req.EnableFingerprintUnification
			}
			return previousSettings.EnableFingerprintUnification
		}(),
		EnableMetadataPassthrough: func() bool {
			if req.EnableMetadataPassthrough != nil {
				return *req.EnableMetadataPassthrough
			}
			return previousSettings.EnableMetadataPassthrough
		}(),
		EnableCCHSigning: func() bool {
			if req.EnableCCHSigning != nil {
				return *req.EnableCCHSigning
			}
			return previousSettings.EnableCCHSigning
		}(),
		EnableAnthropicCacheTTL1hInjection: func() bool {
			if req.EnableAnthropicCacheTTL1hInjection != nil {
				return *req.EnableAnthropicCacheTTL1hInjection
			}
			return previousSettings.EnableAnthropicCacheTTL1hInjection
		}(),
		RewriteMessageCacheControl: func() bool {
			if req.RewriteMessageCacheControl != nil {
				return *req.RewriteMessageCacheControl
			}
			return previousSettings.RewriteMessageCacheControl
		}(),
		EnableGLMZCodeStrongMimic: func() bool {
			if req.EnableGLMZCodeStrongMimic != nil {
				return *req.EnableGLMZCodeStrongMimic
			}
			return previousSettings.EnableGLMZCodeStrongMimic
		}(),
		OpenAICyberSafetyRetryEnabled: func() bool {
			if req.OpenAICyberSafetyRetryEnabled != nil {
				return *req.OpenAICyberSafetyRetryEnabled
			}
			return previousSettings.OpenAICyberSafetyRetryEnabled
		}(),
		OpenAIGPT56SolDefaultMaxReasoning: func() bool {
			if req.OpenAIGPT56SolDefaultMaxReasoning != nil {
				return *req.OpenAIGPT56SolDefaultMaxReasoning
			}
			return previousSettings.OpenAIGPT56SolDefaultMaxReasoning
		}(),
		BalanceLowNotifyEnabled: func() bool {
			if req.BalanceLowNotifyEnabled != nil {
				return *req.BalanceLowNotifyEnabled
			}
			return previousSettings.BalanceLowNotifyEnabled
		}(),
		BalanceLowNotifyThreshold: func() float64 {
			if req.BalanceLowNotifyThreshold != nil {
				return *req.BalanceLowNotifyThreshold
			}
			return previousSettings.BalanceLowNotifyThreshold
		}(),
		BalanceLowNotifyRechargeURL: func() string {
			if req.BalanceLowNotifyRechargeURL != nil {
				return *req.BalanceLowNotifyRechargeURL
			}
			return previousSettings.BalanceLowNotifyRechargeURL
		}(),
		AccountQuotaNotifyEnabled: func() bool {
			if req.AccountQuotaNotifyEnabled != nil {
				return *req.AccountQuotaNotifyEnabled
			}
			return previousSettings.AccountQuotaNotifyEnabled
		}(),
		AccountQuotaNotifyEmails: func() []service.NotifyEmailEntry {
			if req.AccountQuotaNotifyEmails != nil {
				return dto.NotifyEmailEntriesToService(*req.AccountQuotaNotifyEmails)
			}
			return previousSettings.AccountQuotaNotifyEmails
		}(),
		ChannelMonitorEnabled: func() bool {
			if req.ChannelMonitorEnabled != nil {
				return *req.ChannelMonitorEnabled
			}
			return previousSettings.ChannelMonitorEnabled
		}(),
		ChannelMonitorDefaultIntervalSeconds: func() int {
			if req.ChannelMonitorDefaultIntervalSeconds != nil {
				return *req.ChannelMonitorDefaultIntervalSeconds
			}
			return previousSettings.ChannelMonitorDefaultIntervalSeconds
		}(),
		AvailableChannelsEnabled: func() bool {
			if req.AvailableChannelsEnabled != nil {
				return *req.AvailableChannelsEnabled
			}
			return previousSettings.AvailableChannelsEnabled
		}(),
	}

	if err := h.settingService.UpdateSettings(c.Request.Context(), settings); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Update OpenAI fast policy (stored under dedicated key, only when provided).
	if req.OpenAIFastPolicySettings != nil {
		if err := h.settingService.SetOpenAIFastPolicySettings(c.Request.Context(), openaiFastPolicySettingsFromDTO(req.OpenAIFastPolicySettings)); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
	}

	// Update payment configuration (integrated into system settings).
	// Skip if no payment fields were provided (prevents accidental wipe).
	if h.paymentConfigService != nil && hasPaymentFields(req) {
		paymentReq := service.UpdatePaymentConfigRequest{
			Enabled:                   req.PaymentEnabled,
			MinAmount:                 req.PaymentMinAmount,
			MaxAmount:                 req.PaymentMaxAmount,
			DailyLimit:                req.PaymentDailyLimit,
			OrderTimeoutMin:           req.PaymentOrderTimeoutMin,
			MaxPendingOrders:          req.PaymentMaxPendingOrders,
			EnabledTypes:              req.PaymentEnabledTypes,
			BalanceDisabled:           req.PaymentBalanceDisabled,
			BalanceRechargeMultiplier: req.PaymentBalanceRechargeMultiplier,
			RechargeFeeRate:           req.PaymentRechargeFeeRate,
			LoadBalanceStrategy:       req.PaymentLoadBalanceStrat,
			ProductNamePrefix:         req.PaymentProductNamePrefix,
			ProductNameSuffix:         req.PaymentProductNameSuffix,
			HelpImageURL:              req.PaymentHelpImageURL,
			HelpText:                  req.PaymentHelpText,
			CancelRateLimitEnabled:    req.PaymentCancelRateLimitEnabled,
			CancelRateLimitMax:        req.PaymentCancelRateLimitMax,
			CancelRateLimitWindow:     req.PaymentCancelRateLimitWindow,
			CancelRateLimitUnit:       req.PaymentCancelRateLimitUnit,
			CancelRateLimitMode:       req.PaymentCancelRateLimitMode,
		}
		if err := h.paymentConfigService.UpdatePaymentConfig(c.Request.Context(), paymentReq); err != nil {
			response.ErrorFrom(c, err)
			return
		}
		// Refresh in-memory provider registry so config changes take effect immediately
		if h.paymentService != nil {
			h.paymentService.RefreshProviders(c.Request.Context())
		}
	}

	h.auditSettingsUpdate(c, previousSettings, settings, req)

	// 重新获取设置返回
	updatedSettings, err := h.settingService.GetAllSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	updatedDefaultSubscriptions := make([]dto.DefaultSubscriptionSetting, 0, len(updatedSettings.DefaultSubscriptions))
	for _, sub := range updatedSettings.DefaultSubscriptions {
		updatedDefaultSubscriptions = append(updatedDefaultSubscriptions, dto.DefaultSubscriptionSetting{
			GroupID:      sub.GroupID,
			ValidityDays: sub.ValidityDays,
		})
	}

	// Reload payment config for response
	var updatedPaymentCfg *service.PaymentConfig
	if h.paymentConfigService != nil {
		updatedPaymentCfg, _ = h.paymentConfigService.GetPaymentConfig(c.Request.Context())
	}
	if updatedPaymentCfg == nil {
		updatedPaymentCfg = &service.PaymentConfig{}
	}

	payload := dto.SystemSettings{
		RegistrationEnabled:                  updatedSettings.RegistrationEnabled,
		EmailVerifyEnabled:                   updatedSettings.EmailVerifyEnabled,
		RegistrationEmailSuffixWhitelist:     updatedSettings.RegistrationEmailSuffixWhitelist,
		PromoCodeEnabled:                     updatedSettings.PromoCodeEnabled,
		PasswordResetEnabled:                 updatedSettings.PasswordResetEnabled,
		FrontendURL:                          updatedSettings.FrontendURL,
		InvitationCodeEnabled:                updatedSettings.InvitationCodeEnabled,
		TotpEnabled:                          updatedSettings.TotpEnabled,
		TotpEncryptionKeyConfigured:          h.settingService.IsTotpEncryptionKeyConfigured(),
		SMTPHost:                             updatedSettings.SMTPHost,
		SMTPPort:                             updatedSettings.SMTPPort,
		SMTPUsername:                         updatedSettings.SMTPUsername,
		SMTPPasswordConfigured:               updatedSettings.SMTPPasswordConfigured,
		SMTPFrom:                             updatedSettings.SMTPFrom,
		SMTPFromName:                         updatedSettings.SMTPFromName,
		SMTPUseTLS:                           updatedSettings.SMTPUseTLS,
		TurnstileEnabled:                     updatedSettings.TurnstileEnabled,
		TurnstileSiteKey:                     updatedSettings.TurnstileSiteKey,
		TurnstileSecretKeyConfigured:         updatedSettings.TurnstileSecretKeyConfigured,
		LinuxDoConnectEnabled:                updatedSettings.LinuxDoConnectEnabled,
		LinuxDoConnectClientID:               updatedSettings.LinuxDoConnectClientID,
		LinuxDoConnectClientSecretConfigured: updatedSettings.LinuxDoConnectClientSecretConfigured,
		LinuxDoConnectRedirectURL:            updatedSettings.LinuxDoConnectRedirectURL,
		OIDCConnectEnabled:                   updatedSettings.OIDCConnectEnabled,
		OIDCConnectProviderName:              updatedSettings.OIDCConnectProviderName,
		OIDCConnectClientID:                  updatedSettings.OIDCConnectClientID,
		OIDCConnectClientSecretConfigured:    updatedSettings.OIDCConnectClientSecretConfigured,
		OIDCConnectIssuerURL:                 updatedSettings.OIDCConnectIssuerURL,
		OIDCConnectDiscoveryURL:              updatedSettings.OIDCConnectDiscoveryURL,
		OIDCConnectAuthorizeURL:              updatedSettings.OIDCConnectAuthorizeURL,
		OIDCConnectTokenURL:                  updatedSettings.OIDCConnectTokenURL,
		OIDCConnectUserInfoURL:               updatedSettings.OIDCConnectUserInfoURL,
		OIDCConnectJWKSURL:                   updatedSettings.OIDCConnectJWKSURL,
		OIDCConnectScopes:                    updatedSettings.OIDCConnectScopes,
		OIDCConnectRedirectURL:               updatedSettings.OIDCConnectRedirectURL,
		OIDCConnectFrontendRedirectURL:       updatedSettings.OIDCConnectFrontendRedirectURL,
		OIDCConnectTokenAuthMethod:           updatedSettings.OIDCConnectTokenAuthMethod,
		OIDCConnectUsePKCE:                   updatedSettings.OIDCConnectUsePKCE,
		OIDCConnectValidateIDToken:           updatedSettings.OIDCConnectValidateIDToken,
		OIDCConnectAllowedSigningAlgs:        updatedSettings.OIDCConnectAllowedSigningAlgs,
		OIDCConnectClockSkewSeconds:          updatedSettings.OIDCConnectClockSkewSeconds,
		OIDCConnectRequireEmailVerified:      updatedSettings.OIDCConnectRequireEmailVerified,
		OIDCConnectUserInfoEmailPath:         updatedSettings.OIDCConnectUserInfoEmailPath,
		OIDCConnectUserInfoIDPath:            updatedSettings.OIDCConnectUserInfoIDPath,
		OIDCConnectUserInfoUsernamePath:      updatedSettings.OIDCConnectUserInfoUsernamePath,
		SiteName:                             updatedSettings.SiteName,
		SiteLogo:                             updatedSettings.SiteLogo,
		SiteSubtitle:                         updatedSettings.SiteSubtitle,
		APIBaseURL:                           updatedSettings.APIBaseURL,
		ContactInfo:                          updatedSettings.ContactInfo,
		DocURL:                               updatedSettings.DocURL,
		HomeContent:                          updatedSettings.HomeContent,
		HideCcsImportButton:                  updatedSettings.HideCcsImportButton,
		PurchaseSubscriptionEnabled:          updatedSettings.PurchaseSubscriptionEnabled,
		PurchaseSubscriptionURL:              updatedSettings.PurchaseSubscriptionURL,
		TableDefaultPageSize:                 updatedSettings.TableDefaultPageSize,
		TablePageSizeOptions:                 updatedSettings.TablePageSizeOptions,
		CustomMenuItems:                      dto.ParseCustomMenuItems(updatedSettings.CustomMenuItems),
		CustomEndpoints:                      dto.ParseCustomEndpoints(updatedSettings.CustomEndpoints),
		DefaultConcurrency:                   updatedSettings.DefaultConcurrency,
		DefaultBalance:                       updatedSettings.DefaultBalance,
		DefaultSubscriptions:                 updatedDefaultSubscriptions,
		EnableModelFallback:                  updatedSettings.EnableModelFallback,
		FallbackModelAnthropic:               updatedSettings.FallbackModelAnthropic,
		FallbackModelOpenAI:                  updatedSettings.FallbackModelOpenAI,
		FallbackModelGemini:                  updatedSettings.FallbackModelGemini,
		FallbackModelAntigravity:             updatedSettings.FallbackModelAntigravity,
		EnableIdentityPatch:                  updatedSettings.EnableIdentityPatch,
		IdentityPatchPrompt:                  updatedSettings.IdentityPatchPrompt,
		OpsMonitoringEnabled:                 updatedSettings.OpsMonitoringEnabled,
		OpsRealtimeMonitoringEnabled:         updatedSettings.OpsRealtimeMonitoringEnabled,
		OpsQueryModeDefault:                  updatedSettings.OpsQueryModeDefault,
		OpsMetricsIntervalSeconds:            updatedSettings.OpsMetricsIntervalSeconds,
		MinClaudeCodeVersion:                 updatedSettings.MinClaudeCodeVersion,
		MaxClaudeCodeVersion:                 updatedSettings.MaxClaudeCodeVersion,
		AllowUngroupedKeyScheduling:          updatedSettings.AllowUngroupedKeyScheduling,
		BackendModeEnabled:                   updatedSettings.BackendModeEnabled,
		EnableFingerprintUnification:         updatedSettings.EnableFingerprintUnification,
		EnableMetadataPassthrough:            updatedSettings.EnableMetadataPassthrough,
		EnableCCHSigning:                     updatedSettings.EnableCCHSigning,
		EnableAnthropicCacheTTL1hInjection:   updatedSettings.EnableAnthropicCacheTTL1hInjection,
		RewriteMessageCacheControl:           updatedSettings.RewriteMessageCacheControl,
		EnableGLMZCodeStrongMimic:            updatedSettings.EnableGLMZCodeStrongMimic,
		OpenAICyberSafetyRetryEnabled:        updatedSettings.OpenAICyberSafetyRetryEnabled,
		OpenAIGPT56SolDefaultMaxReasoning:    updatedSettings.OpenAIGPT56SolDefaultMaxReasoning,
		BalanceLowNotifyEnabled:              updatedSettings.BalanceLowNotifyEnabled,
		BalanceLowNotifyThreshold:            updatedSettings.BalanceLowNotifyThreshold,
		BalanceLowNotifyRechargeURL:          updatedSettings.BalanceLowNotifyRechargeURL,
		AccountQuotaNotifyEnabled:            updatedSettings.AccountQuotaNotifyEnabled,
		AccountQuotaNotifyEmails:             dto.NotifyEmailEntriesFromService(updatedSettings.AccountQuotaNotifyEmails),
		ChannelMonitorEnabled:                updatedSettings.ChannelMonitorEnabled,
		ChannelMonitorDefaultIntervalSeconds: updatedSettings.ChannelMonitorDefaultIntervalSeconds,
		AvailableChannelsEnabled:             updatedSettings.AvailableChannelsEnabled,
		PaymentEnabled:                       updatedPaymentCfg.Enabled,
		PaymentMinAmount:                     updatedPaymentCfg.MinAmount,
		PaymentMaxAmount:                     updatedPaymentCfg.MaxAmount,
		PaymentDailyLimit:                    updatedPaymentCfg.DailyLimit,
		PaymentOrderTimeoutMin:               updatedPaymentCfg.OrderTimeoutMin,
		PaymentMaxPendingOrders:              updatedPaymentCfg.MaxPendingOrders,
		PaymentEnabledTypes:                  updatedPaymentCfg.EnabledTypes,
		PaymentBalanceDisabled:               updatedPaymentCfg.BalanceDisabled,
		PaymentBalanceRechargeMultiplier:     updatedPaymentCfg.BalanceRechargeMultiplier,
		PaymentRechargeFeeRate:               updatedPaymentCfg.RechargeFeeRate,
		PaymentLoadBalanceStrat:              updatedPaymentCfg.LoadBalanceStrategy,
		PaymentProductNamePrefix:             updatedPaymentCfg.ProductNamePrefix,
		PaymentProductNameSuffix:             updatedPaymentCfg.ProductNameSuffix,
		PaymentHelpImageURL:                  updatedPaymentCfg.HelpImageURL,
		PaymentHelpText:                      updatedPaymentCfg.HelpText,
		PaymentCancelRateLimitEnabled:        updatedPaymentCfg.CancelRateLimitEnabled,
		PaymentCancelRateLimitMax:            updatedPaymentCfg.CancelRateLimitMax,
		PaymentCancelRateLimitWindow:         updatedPaymentCfg.CancelRateLimitWindow,
		PaymentCancelRateLimitUnit:           updatedPaymentCfg.CancelRateLimitUnit,
		PaymentCancelRateLimitMode:           updatedPaymentCfg.CancelRateLimitMode,
	}
	if fastPolicy, err := h.settingService.GetOpenAIFastPolicySettings(c.Request.Context()); err != nil {
		slog.Error("openai_fast_policy_settings_get_failed", "error", err)
	} else if fastPolicy != nil {
		payload.OpenAIFastPolicySettings = openaiFastPolicySettingsToDTO(fastPolicy)
	}
	response.Success(c, payload)
}

// hasPaymentFields returns true if any payment-related field was explicitly provided.
func hasPaymentFields(req UpdateSettingsRequest) bool {
	return req.PaymentEnabled != nil || req.PaymentMinAmount != nil ||
		req.PaymentMaxAmount != nil || req.PaymentDailyLimit != nil ||
		req.PaymentOrderTimeoutMin != nil || req.PaymentMaxPendingOrders != nil ||
		req.PaymentEnabledTypes != nil || req.PaymentBalanceDisabled != nil ||
		req.PaymentBalanceRechargeMultiplier != nil || req.PaymentRechargeFeeRate != nil ||
		req.PaymentLoadBalanceStrat != nil || req.PaymentProductNamePrefix != nil ||
		req.PaymentProductNameSuffix != nil || req.PaymentHelpImageURL != nil ||
		req.PaymentHelpText != nil || req.PaymentCancelRateLimitEnabled != nil ||
		req.PaymentCancelRateLimitMax != nil || req.PaymentCancelRateLimitWindow != nil ||
		req.PaymentCancelRateLimitUnit != nil || req.PaymentCancelRateLimitMode != nil
}

func (h *SettingHandler) auditSettingsUpdate(c *gin.Context, before *service.SystemSettings, after *service.SystemSettings, req UpdateSettingsRequest) {
	if before == nil || after == nil {
		return
	}

	changed := diffSettings(before, after, req)
	if len(changed) == 0 {
		return
	}

	subject, _ := middleware.GetAuthSubjectFromContext(c)
	role, _ := middleware.GetUserRoleFromContext(c)
	slog.Info("settings updated",
		"audit", true,
		"user_id", subject.UserID,
		"role", role,
		"changed", changed,
	)
}

func diffSettings(before *service.SystemSettings, after *service.SystemSettings, req UpdateSettingsRequest) []string {
	changed := make([]string, 0, 20)
	if before.RegistrationEnabled != after.RegistrationEnabled {
		changed = append(changed, "registration_enabled")
	}
	if before.EmailVerifyEnabled != after.EmailVerifyEnabled {
		changed = append(changed, "email_verify_enabled")
	}
	if !equalStringSlice(before.RegistrationEmailSuffixWhitelist, after.RegistrationEmailSuffixWhitelist) {
		changed = append(changed, "registration_email_suffix_whitelist")
	}
	if before.PromoCodeEnabled != after.PromoCodeEnabled {
		changed = append(changed, "promo_code_enabled")
	}
	if before.InvitationCodeEnabled != after.InvitationCodeEnabled {
		changed = append(changed, "invitation_code_enabled")
	}
	if before.PasswordResetEnabled != after.PasswordResetEnabled {
		changed = append(changed, "password_reset_enabled")
	}
	if before.FrontendURL != after.FrontendURL {
		changed = append(changed, "frontend_url")
	}
	if before.TotpEnabled != after.TotpEnabled {
		changed = append(changed, "totp_enabled")
	}
	if before.SMTPHost != after.SMTPHost {
		changed = append(changed, "smtp_host")
	}
	if before.SMTPPort != after.SMTPPort {
		changed = append(changed, "smtp_port")
	}
	if before.SMTPUsername != after.SMTPUsername {
		changed = append(changed, "smtp_username")
	}
	if req.SMTPPassword != "" {
		changed = append(changed, "smtp_password")
	}
	if before.SMTPFrom != after.SMTPFrom {
		changed = append(changed, "smtp_from_email")
	}
	if before.SMTPFromName != after.SMTPFromName {
		changed = append(changed, "smtp_from_name")
	}
	if before.SMTPUseTLS != after.SMTPUseTLS {
		changed = append(changed, "smtp_use_tls")
	}
	if before.TurnstileEnabled != after.TurnstileEnabled {
		changed = append(changed, "turnstile_enabled")
	}
	if before.TurnstileSiteKey != after.TurnstileSiteKey {
		changed = append(changed, "turnstile_site_key")
	}
	if req.TurnstileSecretKey != "" {
		changed = append(changed, "turnstile_secret_key")
	}
	if before.LinuxDoConnectEnabled != after.LinuxDoConnectEnabled {
		changed = append(changed, "linuxdo_connect_enabled")
	}
	if before.LinuxDoConnectClientID != after.LinuxDoConnectClientID {
		changed = append(changed, "linuxdo_connect_client_id")
	}
	if req.LinuxDoConnectClientSecret != "" {
		changed = append(changed, "linuxdo_connect_client_secret")
	}
	if before.LinuxDoConnectRedirectURL != after.LinuxDoConnectRedirectURL {
		changed = append(changed, "linuxdo_connect_redirect_url")
	}
	if before.OIDCConnectEnabled != after.OIDCConnectEnabled {
		changed = append(changed, "oidc_connect_enabled")
	}
	if before.OIDCConnectProviderName != after.OIDCConnectProviderName {
		changed = append(changed, "oidc_connect_provider_name")
	}
	if before.OIDCConnectClientID != after.OIDCConnectClientID {
		changed = append(changed, "oidc_connect_client_id")
	}
	if req.OIDCConnectClientSecret != "" {
		changed = append(changed, "oidc_connect_client_secret")
	}
	if before.OIDCConnectIssuerURL != after.OIDCConnectIssuerURL {
		changed = append(changed, "oidc_connect_issuer_url")
	}
	if before.OIDCConnectDiscoveryURL != after.OIDCConnectDiscoveryURL {
		changed = append(changed, "oidc_connect_discovery_url")
	}
	if before.OIDCConnectAuthorizeURL != after.OIDCConnectAuthorizeURL {
		changed = append(changed, "oidc_connect_authorize_url")
	}
	if before.OIDCConnectTokenURL != after.OIDCConnectTokenURL {
		changed = append(changed, "oidc_connect_token_url")
	}
	if before.OIDCConnectUserInfoURL != after.OIDCConnectUserInfoURL {
		changed = append(changed, "oidc_connect_userinfo_url")
	}
	if before.OIDCConnectJWKSURL != after.OIDCConnectJWKSURL {
		changed = append(changed, "oidc_connect_jwks_url")
	}
	if before.OIDCConnectScopes != after.OIDCConnectScopes {
		changed = append(changed, "oidc_connect_scopes")
	}
	if before.OIDCConnectRedirectURL != after.OIDCConnectRedirectURL {
		changed = append(changed, "oidc_connect_redirect_url")
	}
	if before.OIDCConnectFrontendRedirectURL != after.OIDCConnectFrontendRedirectURL {
		changed = append(changed, "oidc_connect_frontend_redirect_url")
	}
	if before.OIDCConnectTokenAuthMethod != after.OIDCConnectTokenAuthMethod {
		changed = append(changed, "oidc_connect_token_auth_method")
	}
	if before.OIDCConnectUsePKCE != after.OIDCConnectUsePKCE {
		changed = append(changed, "oidc_connect_use_pkce")
	}
	if before.OIDCConnectValidateIDToken != after.OIDCConnectValidateIDToken {
		changed = append(changed, "oidc_connect_validate_id_token")
	}
	if before.OIDCConnectAllowedSigningAlgs != after.OIDCConnectAllowedSigningAlgs {
		changed = append(changed, "oidc_connect_allowed_signing_algs")
	}
	if before.OIDCConnectClockSkewSeconds != after.OIDCConnectClockSkewSeconds {
		changed = append(changed, "oidc_connect_clock_skew_seconds")
	}
	if before.OIDCConnectRequireEmailVerified != after.OIDCConnectRequireEmailVerified {
		changed = append(changed, "oidc_connect_require_email_verified")
	}
	if before.OIDCConnectUserInfoEmailPath != after.OIDCConnectUserInfoEmailPath {
		changed = append(changed, "oidc_connect_userinfo_email_path")
	}
	if before.OIDCConnectUserInfoIDPath != after.OIDCConnectUserInfoIDPath {
		changed = append(changed, "oidc_connect_userinfo_id_path")
	}
	if before.OIDCConnectUserInfoUsernamePath != after.OIDCConnectUserInfoUsernamePath {
		changed = append(changed, "oidc_connect_userinfo_username_path")
	}
	if before.SiteName != after.SiteName {
		changed = append(changed, "site_name")
	}
	if before.SiteLogo != after.SiteLogo {
		changed = append(changed, "site_logo")
	}
	if before.SiteSubtitle != after.SiteSubtitle {
		changed = append(changed, "site_subtitle")
	}
	if before.APIBaseURL != after.APIBaseURL {
		changed = append(changed, "api_base_url")
	}
	if before.ContactInfo != after.ContactInfo {
		changed = append(changed, "contact_info")
	}
	if before.DocURL != after.DocURL {
		changed = append(changed, "doc_url")
	}
	if before.HomeContent != after.HomeContent {
		changed = append(changed, "home_content")
	}
	if before.HideCcsImportButton != after.HideCcsImportButton {
		changed = append(changed, "hide_ccs_import_button")
	}
	if before.DefaultConcurrency != after.DefaultConcurrency {
		changed = append(changed, "default_concurrency")
	}
	if before.DefaultBalance != after.DefaultBalance {
		changed = append(changed, "default_balance")
	}
	if !equalDefaultSubscriptions(before.DefaultSubscriptions, after.DefaultSubscriptions) {
		changed = append(changed, "default_subscriptions")
	}
	if before.EnableModelFallback != after.EnableModelFallback {
		changed = append(changed, "enable_model_fallback")
	}
	if before.FallbackModelAnthropic != after.FallbackModelAnthropic {
		changed = append(changed, "fallback_model_anthropic")
	}
	if before.FallbackModelOpenAI != after.FallbackModelOpenAI {
		changed = append(changed, "fallback_model_openai")
	}
	if before.FallbackModelGemini != after.FallbackModelGemini {
		changed = append(changed, "fallback_model_gemini")
	}
	if before.FallbackModelAntigravity != after.FallbackModelAntigravity {
		changed = append(changed, "fallback_model_antigravity")
	}
	if before.EnableIdentityPatch != after.EnableIdentityPatch {
		changed = append(changed, "enable_identity_patch")
	}
	if before.IdentityPatchPrompt != after.IdentityPatchPrompt {
		changed = append(changed, "identity_patch_prompt")
	}
	if before.OpsMonitoringEnabled != after.OpsMonitoringEnabled {
		changed = append(changed, "ops_monitoring_enabled")
	}
	if before.OpsRealtimeMonitoringEnabled != after.OpsRealtimeMonitoringEnabled {
		changed = append(changed, "ops_realtime_monitoring_enabled")
	}
	if before.OpsQueryModeDefault != after.OpsQueryModeDefault {
		changed = append(changed, "ops_query_mode_default")
	}
	if before.OpsMetricsIntervalSeconds != after.OpsMetricsIntervalSeconds {
		changed = append(changed, "ops_metrics_interval_seconds")
	}
	if before.MinClaudeCodeVersion != after.MinClaudeCodeVersion {
		changed = append(changed, "min_claude_code_version")
	}
	if before.MaxClaudeCodeVersion != after.MaxClaudeCodeVersion {
		changed = append(changed, "max_claude_code_version")
	}
	if before.AllowUngroupedKeyScheduling != after.AllowUngroupedKeyScheduling {
		changed = append(changed, "allow_ungrouped_key_scheduling")
	}
	if before.BackendModeEnabled != after.BackendModeEnabled {
		changed = append(changed, "backend_mode_enabled")
	}
	if before.PurchaseSubscriptionEnabled != after.PurchaseSubscriptionEnabled {
		changed = append(changed, "purchase_subscription_enabled")
	}
	if before.PurchaseSubscriptionURL != after.PurchaseSubscriptionURL {
		changed = append(changed, "purchase_subscription_url")
	}
	if before.TableDefaultPageSize != after.TableDefaultPageSize {
		changed = append(changed, "table_default_page_size")
	}
	if !equalIntSlice(before.TablePageSizeOptions, after.TablePageSizeOptions) {
		changed = append(changed, "table_page_size_options")
	}
	if before.CustomMenuItems != after.CustomMenuItems {
		changed = append(changed, "custom_menu_items")
	}
	if before.CustomEndpoints != after.CustomEndpoints {
		changed = append(changed, "custom_endpoints")
	}
	if before.EnableFingerprintUnification != after.EnableFingerprintUnification {
		changed = append(changed, "enable_fingerprint_unification")
	}
	if before.EnableMetadataPassthrough != after.EnableMetadataPassthrough {
		changed = append(changed, "enable_metadata_passthrough")
	}
	if before.EnableCCHSigning != after.EnableCCHSigning {
		changed = append(changed, "enable_cch_signing")
	}
	if before.EnableAnthropicCacheTTL1hInjection != after.EnableAnthropicCacheTTL1hInjection {
		changed = append(changed, "enable_anthropic_cache_ttl_1h_injection")
	}
	if before.RewriteMessageCacheControl != after.RewriteMessageCacheControl {
		changed = append(changed, "rewrite_message_cache_control")
	}
	if before.EnableGLMZCodeStrongMimic != after.EnableGLMZCodeStrongMimic {
		changed = append(changed, "enable_glm_zcode_strong_mimic")
	}
	if before.OpenAICyberSafetyRetryEnabled != after.OpenAICyberSafetyRetryEnabled {
		changed = append(changed, "openai_cyber_safety_retry_enabled")
	}
	if before.OpenAIGPT56SolDefaultMaxReasoning != after.OpenAIGPT56SolDefaultMaxReasoning {
		changed = append(changed, "openai_gpt56_sol_default_max_reasoning_enabled")
	}
	// Balance & quota notification
	if before.BalanceLowNotifyEnabled != after.BalanceLowNotifyEnabled {
		changed = append(changed, "balance_low_notify_enabled")
	}
	if before.BalanceLowNotifyThreshold != after.BalanceLowNotifyThreshold {
		changed = append(changed, "balance_low_notify_threshold")
	}
	if before.BalanceLowNotifyRechargeURL != after.BalanceLowNotifyRechargeURL {
		changed = append(changed, "balance_low_notify_recharge_url")
	}
	if before.AccountQuotaNotifyEnabled != after.AccountQuotaNotifyEnabled {
		changed = append(changed, "account_quota_notify_enabled")
	}
	if !equalNotifyEmailEntries(before.AccountQuotaNotifyEmails, after.AccountQuotaNotifyEmails) {
		changed = append(changed, "account_quota_notify_emails")
	}
	if before.ChannelMonitorEnabled != after.ChannelMonitorEnabled {
		changed = append(changed, "channel_monitor_enabled")
	}
	if before.ChannelMonitorDefaultIntervalSeconds != after.ChannelMonitorDefaultIntervalSeconds {
		changed = append(changed, "channel_monitor_default_interval_seconds")
	}
	if before.AvailableChannelsEnabled != after.AvailableChannelsEnabled {
		changed = append(changed, "available_channels_enabled")
	}
	return changed
}

func normalizeDefaultSubscriptions(input []dto.DefaultSubscriptionSetting) []dto.DefaultSubscriptionSetting {
	if len(input) == 0 {
		return nil
	}
	normalized := make([]dto.DefaultSubscriptionSetting, 0, len(input))
	for _, item := range input {
		if item.GroupID <= 0 || item.ValidityDays <= 0 {
			continue
		}
		if item.ValidityDays > service.MaxValidityDays {
			item.ValidityDays = service.MaxValidityDays
		}
		normalized = append(normalized, item)
	}
	return normalized
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalDefaultSubscriptions(a, b []service.DefaultSubscriptionSetting) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].GroupID != b[i].GroupID || a[i].ValidityDays != b[i].ValidityDays {
			return false
		}
	}
	return true
}

func equalIntSlice(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalNotifyEmailEntries(a, b []service.NotifyEmailEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Email != b[i].Email || a[i].Verified != b[i].Verified || a[i].Disabled != b[i].Disabled {
			return false
		}
	}
	return true
}

// TestSMTPRequest 测试SMTP连接请求
type TestSMTPRequest struct {
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	SMTPUseTLS   bool   `json:"smtp_use_tls"`
}

// TestSMTPConnection 测试SMTP连接
// POST /api/v1/admin/settings/test-smtp
func (h *SettingHandler) TestSMTPConnection(c *gin.Context) {
	var req TestSMTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	req.SMTPHost = strings.TrimSpace(req.SMTPHost)
	req.SMTPUsername = strings.TrimSpace(req.SMTPUsername)

	var savedConfig *service.SMTPConfig
	if cfg, err := h.emailService.GetSMTPConfig(c.Request.Context()); err == nil && cfg != nil {
		savedConfig = cfg
	}

	if req.SMTPHost == "" && savedConfig != nil {
		req.SMTPHost = savedConfig.Host
	}
	if req.SMTPPort <= 0 {
		if savedConfig != nil && savedConfig.Port > 0 {
			req.SMTPPort = savedConfig.Port
		} else {
			req.SMTPPort = 587
		}
	}
	if req.SMTPUsername == "" && savedConfig != nil {
		req.SMTPUsername = savedConfig.Username
	}
	password := strings.TrimSpace(req.SMTPPassword)
	if password == "" && savedConfig != nil {
		password = savedConfig.Password
	}
	if req.SMTPHost == "" {
		response.BadRequest(c, "SMTP host is required")
		return
	}

	config := &service.SMTPConfig{
		Host:     req.SMTPHost,
		Port:     req.SMTPPort,
		Username: req.SMTPUsername,
		Password: password,
		UseTLS:   req.SMTPUseTLS,
	}

	err := h.emailService.TestSMTPConnectionWithConfig(config)
	if err != nil {
		response.BadRequest(c, "SMTP connection test failed: "+err.Error())
		return
	}

	response.Success(c, gin.H{"message": "SMTP connection successful"})
}

// SendTestEmailRequest 发送测试邮件请求
type SendTestEmailRequest struct {
	Email        string `json:"email" binding:"required,email"`
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	SMTPFrom     string `json:"smtp_from_email"`
	SMTPFromName string `json:"smtp_from_name"`
	SMTPUseTLS   bool   `json:"smtp_use_tls"`
}

// SendTestEmail 发送测试邮件
// POST /api/v1/admin/settings/send-test-email
func (h *SettingHandler) SendTestEmail(c *gin.Context) {
	var req SendTestEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	req.SMTPHost = strings.TrimSpace(req.SMTPHost)
	req.SMTPUsername = strings.TrimSpace(req.SMTPUsername)
	req.SMTPFrom = strings.TrimSpace(req.SMTPFrom)
	req.SMTPFromName = strings.TrimSpace(req.SMTPFromName)

	var savedConfig *service.SMTPConfig
	if cfg, err := h.emailService.GetSMTPConfig(c.Request.Context()); err == nil && cfg != nil {
		savedConfig = cfg
	}

	if req.SMTPHost == "" && savedConfig != nil {
		req.SMTPHost = savedConfig.Host
	}
	if req.SMTPPort <= 0 {
		if savedConfig != nil && savedConfig.Port > 0 {
			req.SMTPPort = savedConfig.Port
		} else {
			req.SMTPPort = 587
		}
	}
	if req.SMTPUsername == "" && savedConfig != nil {
		req.SMTPUsername = savedConfig.Username
	}
	password := strings.TrimSpace(req.SMTPPassword)
	if password == "" && savedConfig != nil {
		password = savedConfig.Password
	}
	if req.SMTPFrom == "" && savedConfig != nil {
		req.SMTPFrom = savedConfig.From
	}
	if req.SMTPFromName == "" && savedConfig != nil {
		req.SMTPFromName = savedConfig.FromName
	}
	if req.SMTPHost == "" {
		response.BadRequest(c, "SMTP host is required")
		return
	}

	config := &service.SMTPConfig{
		Host:     req.SMTPHost,
		Port:     req.SMTPPort,
		Username: req.SMTPUsername,
		Password: password,
		From:     req.SMTPFrom,
		FromName: req.SMTPFromName,
		UseTLS:   req.SMTPUseTLS,
	}

	siteName := h.settingService.GetSiteName(c.Request.Context())
	subject := "[" + siteName + "] Test Email"
	body := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background-color: #f5f5f5; margin: 0; padding: 20px; }
        .container { max-width: 600px; margin: 0 auto; background-color: #ffffff; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 8px rgba(0,0,0,0.1); }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 30px; text-align: center; }
        .content { padding: 40px 30px; text-align: center; }
        .success { color: #10b981; font-size: 48px; margin-bottom: 20px; }
        .footer { background-color: #f8f9fa; padding: 20px; text-align: center; color: #999; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>` + siteName + `</h1>
        </div>
        <div class="content">
            <div class="success">✓</div>
            <h2>Email Configuration Successful!</h2>
            <p>This is a test email to verify your SMTP settings are working correctly.</p>
        </div>
        <div class="footer">
            <p>This is an automated test message.</p>
        </div>
    </div>
</body>
</html>
`

	if err := h.emailService.SendEmailWithConfig(config, req.Email, subject, body); err != nil {
		response.BadRequest(c, "Failed to send test email: "+err.Error())
		return
	}

	response.Success(c, gin.H{"message": "Test email sent successfully"})
}

// GetAdminAPIKey 获取管理员 API Key 状态
// GET /api/v1/admin/settings/admin-api-key
func (h *SettingHandler) GetAdminAPIKey(c *gin.Context) {
	maskedKey, exists, err := h.settingService.GetAdminAPIKeyStatus(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"exists":     exists,
		"masked_key": maskedKey,
	})
}

// RegenerateAdminAPIKey 生成/重新生成管理员 API Key
// POST /api/v1/admin/settings/admin-api-key/regenerate
func (h *SettingHandler) RegenerateAdminAPIKey(c *gin.Context) {
	key, err := h.settingService.GenerateAdminAPIKey(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"key": key, // 完整 key 只在生成时返回一次
	})
}

// DeleteAdminAPIKey 删除管理员 API Key
// DELETE /api/v1/admin/settings/admin-api-key
func (h *SettingHandler) DeleteAdminAPIKey(c *gin.Context) {
	if err := h.settingService.DeleteAdminAPIKey(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Admin API key deleted"})
}

// GetOverloadCooldownSettings 获取529过载冷却配置
// GET /api/v1/admin/settings/overload-cooldown
func (h *SettingHandler) GetOverloadCooldownSettings(c *gin.Context) {
	settings, err := h.settingService.GetOverloadCooldownSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.OverloadCooldownSettings{
		Enabled:         settings.Enabled,
		CooldownMinutes: settings.CooldownMinutes,
	})
}

// UpdateOverloadCooldownSettingsRequest 更新529过载冷却配置请求
type UpdateOverloadCooldownSettingsRequest struct {
	Enabled         bool `json:"enabled"`
	CooldownMinutes int  `json:"cooldown_minutes"`
}

// UpdateOverloadCooldownSettings 更新529过载冷却配置
// PUT /api/v1/admin/settings/overload-cooldown
func (h *SettingHandler) UpdateOverloadCooldownSettings(c *gin.Context) {
	var req UpdateOverloadCooldownSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	settings := &service.OverloadCooldownSettings{
		Enabled:         req.Enabled,
		CooldownMinutes: req.CooldownMinutes,
	}

	if err := h.settingService.SetOverloadCooldownSettings(c.Request.Context(), settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	updatedSettings, err := h.settingService.GetOverloadCooldownSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.OverloadCooldownSettings{
		Enabled:         updatedSettings.Enabled,
		CooldownMinutes: updatedSettings.CooldownMinutes,
	})
}

// GetStreamTimeoutSettings 获取流超时处理配置
// GET /api/v1/admin/settings/stream-timeout
func (h *SettingHandler) GetStreamTimeoutSettings(c *gin.Context) {
	settings, err := h.settingService.GetStreamTimeoutSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.StreamTimeoutSettings{
		Enabled:                settings.Enabled,
		Action:                 settings.Action,
		TempUnschedMinutes:     settings.TempUnschedMinutes,
		ThresholdCount:         settings.ThresholdCount,
		ThresholdWindowMinutes: settings.ThresholdWindowMinutes,
	})
}

// GetRectifierSettings 获取请求整流器配置
// GET /api/v1/admin/settings/rectifier
func (h *SettingHandler) GetRectifierSettings(c *gin.Context) {
	settings, err := h.settingService.GetRectifierSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	patterns := settings.APIKeySignaturePatterns
	if patterns == nil {
		patterns = []string{}
	}
	response.Success(c, dto.RectifierSettings{
		Enabled:                  settings.Enabled,
		ThinkingSignatureEnabled: settings.ThinkingSignatureEnabled,
		ThinkingBudgetEnabled:    settings.ThinkingBudgetEnabled,
		APIKeySignatureEnabled:   settings.APIKeySignatureEnabled,
		APIKeySignaturePatterns:  patterns,
	})
}

// UpdateRectifierSettingsRequest 更新整流器配置请求
type UpdateRectifierSettingsRequest struct {
	Enabled                  bool     `json:"enabled"`
	ThinkingSignatureEnabled bool     `json:"thinking_signature_enabled"`
	ThinkingBudgetEnabled    bool     `json:"thinking_budget_enabled"`
	APIKeySignatureEnabled   bool     `json:"apikey_signature_enabled"`
	APIKeySignaturePatterns  []string `json:"apikey_signature_patterns"`
}

// UpdateRectifierSettings 更新请求整流器配置
// PUT /api/v1/admin/settings/rectifier
func (h *SettingHandler) UpdateRectifierSettings(c *gin.Context) {
	var req UpdateRectifierSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// 校验并清理自定义匹配关键词
	const maxPatterns = 50
	const maxPatternLen = 500
	if len(req.APIKeySignaturePatterns) > maxPatterns {
		response.BadRequest(c, "Too many signature patterns (max 50)")
		return
	}
	var cleanedPatterns []string
	for _, p := range req.APIKeySignaturePatterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if len(p) > maxPatternLen {
			response.BadRequest(c, "Signature pattern too long (max 500 characters)")
			return
		}
		cleanedPatterns = append(cleanedPatterns, p)
	}

	settings := &service.RectifierSettings{
		Enabled:                  req.Enabled,
		ThinkingSignatureEnabled: req.ThinkingSignatureEnabled,
		ThinkingBudgetEnabled:    req.ThinkingBudgetEnabled,
		APIKeySignatureEnabled:   req.APIKeySignatureEnabled,
		APIKeySignaturePatterns:  cleanedPatterns,
	}

	if err := h.settingService.SetRectifierSettings(c.Request.Context(), settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 重新获取设置返回
	updatedSettings, err := h.settingService.GetRectifierSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	updatedPatterns := updatedSettings.APIKeySignaturePatterns
	if updatedPatterns == nil {
		updatedPatterns = []string{}
	}
	response.Success(c, dto.RectifierSettings{
		Enabled:                  updatedSettings.Enabled,
		ThinkingSignatureEnabled: updatedSettings.ThinkingSignatureEnabled,
		ThinkingBudgetEnabled:    updatedSettings.ThinkingBudgetEnabled,
		APIKeySignatureEnabled:   updatedSettings.APIKeySignatureEnabled,
		APIKeySignaturePatterns:  updatedPatterns,
	})
}

// GetBetaPolicySettings 获取 Beta 策略配置
// GET /api/v1/admin/settings/beta-policy
func (h *SettingHandler) GetBetaPolicySettings(c *gin.Context) {
	settings, err := h.settingService.GetBetaPolicySettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	rules := make([]dto.BetaPolicyRule, len(settings.Rules))
	for i, r := range settings.Rules {
		rules[i] = dto.BetaPolicyRule(r)
	}
	response.Success(c, dto.BetaPolicySettings{Rules: rules})
}

// UpdateBetaPolicySettingsRequest 更新 Beta 策略配置请求
type UpdateBetaPolicySettingsRequest struct {
	Rules []dto.BetaPolicyRule `json:"rules"`
}

// UpdateBetaPolicySettings 更新 Beta 策略配置
// PUT /api/v1/admin/settings/beta-policy
func (h *SettingHandler) UpdateBetaPolicySettings(c *gin.Context) {
	var req UpdateBetaPolicySettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	rules := make([]service.BetaPolicyRule, len(req.Rules))
	for i, r := range req.Rules {
		rules[i] = service.BetaPolicyRule(r)
	}

	settings := &service.BetaPolicySettings{Rules: rules}
	if err := h.settingService.SetBetaPolicySettings(c.Request.Context(), settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Re-fetch to return updated settings
	updated, err := h.settingService.GetBetaPolicySettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	outRules := make([]dto.BetaPolicyRule, len(updated.Rules))
	for i, r := range updated.Rules {
		outRules[i] = dto.BetaPolicyRule(r)
	}
	response.Success(c, dto.BetaPolicySettings{Rules: outRules})
}

// UpdateStreamTimeoutSettingsRequest 更新流超时配置请求
type UpdateStreamTimeoutSettingsRequest struct {
	Enabled                bool   `json:"enabled"`
	Action                 string `json:"action"`
	TempUnschedMinutes     int    `json:"temp_unsched_minutes"`
	ThresholdCount         int    `json:"threshold_count"`
	ThresholdWindowMinutes int    `json:"threshold_window_minutes"`
}

// UpdateStreamTimeoutSettings 更新流超时处理配置
// PUT /api/v1/admin/settings/stream-timeout
func (h *SettingHandler) UpdateStreamTimeoutSettings(c *gin.Context) {
	var req UpdateStreamTimeoutSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	settings := &service.StreamTimeoutSettings{
		Enabled:                req.Enabled,
		Action:                 req.Action,
		TempUnschedMinutes:     req.TempUnschedMinutes,
		ThresholdCount:         req.ThresholdCount,
		ThresholdWindowMinutes: req.ThresholdWindowMinutes,
	}

	if err := h.settingService.SetStreamTimeoutSettings(c.Request.Context(), settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 重新获取设置返回
	updatedSettings, err := h.settingService.GetStreamTimeoutSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.StreamTimeoutSettings{
		Enabled:                updatedSettings.Enabled,
		Action:                 updatedSettings.Action,
		TempUnschedMinutes:     updatedSettings.TempUnschedMinutes,
		ThresholdCount:         updatedSettings.ThresholdCount,
		ThresholdWindowMinutes: updatedSettings.ThresholdWindowMinutes,
	})
}

// GetWebSearchEmulationConfig 获取 Web Search 模拟配置
// GET /api/v1/admin/settings/web-search-emulation
func (h *SettingHandler) GetWebSearchEmulationConfig(c *gin.Context) {
	cfg, err := h.settingService.GetWebSearchEmulationConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, service.PopulateWebSearchUsage(c.Request.Context(), cfg))
}

// UpdateWebSearchEmulationConfig 更新 Web Search 模拟配置
// PUT /api/v1/admin/settings/web-search-emulation
func (h *SettingHandler) UpdateWebSearchEmulationConfig(c *gin.Context) {
	var cfg service.WebSearchEmulationConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if err := h.settingService.SaveWebSearchEmulationConfig(c.Request.Context(), &cfg); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Re-read (with sanitized api keys) to return current state
	updated, err := h.settingService.GetWebSearchEmulationConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, service.PopulateWebSearchUsage(c.Request.Context(), updated))
}

// ResetWebSearchUsage 重置指定 provider 的配额用量
// POST /api/v1/admin/settings/web-search-emulation/reset-usage
func (h *SettingHandler) ResetWebSearchUsage(c *gin.Context) {
	var req struct {
		ProviderType string `json:"provider_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.ProviderType == "" {
		response.BadRequest(c, "provider_type is required")
		return
	}
	if err := service.ResetWebSearchUsage(c.Request.Context(), req.ProviderType); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nil)
}

// TestWebSearchEmulation 测试 Web Search 搜索
// POST /api/v1/admin/settings/web-search-emulation/test
func (h *SettingHandler) TestWebSearchEmulation(c *gin.Context) {
	var req struct {
		Query string `json:"query"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Query) == "" {
		req.Query = "搜索今年世界大事件"
	}

	result, err := service.TestWebSearch(c.Request.Context(), req.Query)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
