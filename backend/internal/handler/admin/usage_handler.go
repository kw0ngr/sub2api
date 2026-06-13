package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// UsageHandler handles admin usage-related requests
type UsageHandler struct {
	usageService   *service.UsageService
	apiKeyService  *service.APIKeyService
	adminService   service.AdminService
	cleanupService *service.UsageCleanupService
}

// NewUsageHandler creates a new admin usage handler
func NewUsageHandler(
	usageService *service.UsageService,
	apiKeyService *service.APIKeyService,
	adminService service.AdminService,
	cleanupService *service.UsageCleanupService,
) *UsageHandler {
	return &UsageHandler{
		usageService:   usageService,
		apiKeyService:  apiKeyService,
		adminService:   adminService,
		cleanupService: cleanupService,
	}
}

// CreateUsageCleanupTaskRequest represents cleanup task creation request
type CreateUsageCleanupTaskRequest struct {
	StartDate   string  `json:"start_date"`
	EndDate     string  `json:"end_date"`
	UserID      *int64  `json:"user_id"`
	APIKeyID    *int64  `json:"api_key_id"`
	AccountID   *int64  `json:"account_id"`
	GroupID     *int64  `json:"group_id"`
	Model       *string `json:"model"`
	RequestType *string `json:"request_type"`
	Stream      *bool   `json:"stream"`
	BillingType *int8   `json:"billing_type"`
	Timezone    string  `json:"timezone"`
}

type ReplayLabPackageResponse struct {
	Usage        *dto.AdminUsageLog  `json:"usage"`
	Summary      map[string]any      `json:"summary"`
	Route        ReplayLabRouteInfo  `json:"route"`
	Safety       ReplayLabSafetyInfo `json:"safety"`
	CurlTemplate string              `json:"curl_template"`
	Checks       []ReplayLabCheck    `json:"checks"`
	GeneratedAt  time.Time           `json:"generated_at"`
}

type ReplayLabRouteInfo struct {
	InboundEndpoint    string  `json:"inbound_endpoint"`
	UpstreamEndpoint   string  `json:"upstream_endpoint"`
	RequestType        string  `json:"request_type"`
	RequestedModel     string  `json:"requested_model"`
	UpstreamModel      *string `json:"upstream_model,omitempty"`
	ModelMappingChain  *string `json:"model_mapping_chain,omitempty"`
	ChannelID          *int64  `json:"channel_id,omitempty"`
	AccountID          int64   `json:"account_id"`
	AccountName        string  `json:"account_name,omitempty"`
	AccountPlatform    string  `json:"account_platform,omitempty"`
	AccountType        string  `json:"account_type,omitempty"`
	AccountStatus      string  `json:"account_status,omitempty"`
	AccountSchedulable *bool   `json:"account_schedulable,omitempty"`
	GroupID            *int64  `json:"group_id,omitempty"`
	ProjectKey         *string `json:"project_key,omitempty"`
	ProjectLabel       *string `json:"project_label,omitempty"`
}

type ReplayLabSafetyInfo struct {
	CanReplay            bool     `json:"can_replay"`
	RawSnapshotAvailable bool     `json:"raw_snapshot_available"`
	RequiresManualBody   bool     `json:"requires_manual_body"`
	RiskLevel            string   `json:"risk_level"`
	Reasons              []string `json:"reasons"`
	RecommendedMode      string   `json:"recommended_mode"`
}

type ReplayLabCheck struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// List handles listing all usage records with filters
// GET /api/v1/admin/usage
func (h *UsageHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	exactTotal := false
	if exactTotalRaw := strings.TrimSpace(c.Query("exact_total")); exactTotalRaw != "" {
		parsed, err := strconv.ParseBool(exactTotalRaw)
		if err != nil {
			response.BadRequest(c, "Invalid exact_total value, use true or false")
			return
		}
		exactTotal = parsed
	}

	// Parse filters
	var userID, apiKeyID, accountID, groupID int64
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		id, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid user_id")
			return
		}
		userID = id
	}

	if apiKeyIDStr := c.Query("api_key_id"); apiKeyIDStr != "" {
		id, err := strconv.ParseInt(apiKeyIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid api_key_id")
			return
		}
		apiKeyID = id
	}

	if accountIDStr := c.Query("account_id"); accountIDStr != "" {
		id, err := strconv.ParseInt(accountIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid account_id")
			return
		}
		accountID = id
	}

	if groupIDStr := c.Query("group_id"); groupIDStr != "" {
		id, err := strconv.ParseInt(groupIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid group_id")
			return
		}
		groupID = id
	}

	model := c.Query("model")
	billingMode := strings.TrimSpace(c.Query("billing_mode"))

	var requestType *int16
	var stream *bool
	if requestTypeStr := strings.TrimSpace(c.Query("request_type")); requestTypeStr != "" {
		parsed, err := service.ParseUsageRequestType(requestTypeStr)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		value := int16(parsed)
		requestType = &value
	} else if streamStr := c.Query("stream"); streamStr != "" {
		val, err := strconv.ParseBool(streamStr)
		if err != nil {
			response.BadRequest(c, "Invalid stream value, use true or false")
			return
		}
		stream = &val
	}

	var billingType *int8
	if billingTypeStr := c.Query("billing_type"); billingTypeStr != "" {
		val, err := strconv.ParseInt(billingTypeStr, 10, 8)
		if err != nil {
			response.BadRequest(c, "Invalid billing_type")
			return
		}
		bt := int8(val)
		billingType = &bt
	}

	// Parse date range
	var startTime, endTime *time.Time
	userTZ := c.Query("timezone") // Get user's timezone from request
	if startDateStr := c.Query("start_date"); startDateStr != "" {
		t, err := timezone.ParseInUserLocation("2006-01-02", startDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid start_date format, use YYYY-MM-DD")
			return
		}
		startTime = &t
	}

	if endDateStr := c.Query("end_date"); endDateStr != "" {
		t, err := timezone.ParseInUserLocation("2006-01-02", endDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid end_date format, use YYYY-MM-DD")
			return
		}
		// Use half-open range [start, end), move to next calendar day start (DST-safe).
		t = t.AddDate(0, 0, 1)
		endTime = &t
	}

	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "created_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}
	filters := usagestats.UsageLogFilters{
		UserID:      userID,
		APIKeyID:    apiKeyID,
		AccountID:   accountID,
		GroupID:     groupID,
		Model:       model,
		RequestType: requestType,
		Stream:      stream,
		BillingType: billingType,
		BillingMode: billingMode,
		StartTime:   startTime,
		EndTime:     endTime,
		ExactTotal:  exactTotal,
	}

	records, result, err := h.usageService.ListWithFilters(c.Request.Context(), params, filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.AdminUsageLog, 0, len(records))
	for i := range records {
		out = append(out, *dto.UsageLogFromServiceAdmin(&records[i]))
	}
	response.Paginated(c, out, result.Total, page, pageSize)
}

// GetReplayPackage returns a low-risk diagnostic package for Replay Lab.
// It intentionally does not replay requests or expose raw request bodies.
// GET /api/v1/admin/usage/:id/replay-package
func (h *UsageHandler) GetReplayPackage(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid usage log ID")
		return
	}

	log, err := h.usageService.GetByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	h.hydrateReplayUsageLog(c.Request.Context(), log)
	usageDTO := dto.UsageLogFromServiceAdmin(log)
	response.Success(c, ReplayLabPackageResponse{
		Usage:        usageDTO,
		Summary:      buildReplayLabSummary(log, usageDTO),
		Route:        buildReplayLabRouteInfo(log, usageDTO),
		Safety:       buildReplayLabSafetyInfo(log),
		CurlTemplate: buildReplayLabCurlTemplate(log, usageDTO),
		Checks:       buildReplayLabChecks(log, usageDTO),
		GeneratedAt:  time.Now(),
	})
}

func (h *UsageHandler) hydrateReplayUsageLog(ctx context.Context, log *service.UsageLog) {
	if log == nil {
		return
	}
	if h.adminService != nil && log.UserID > 0 {
		if user, err := h.adminService.GetUser(ctx, log.UserID); err == nil {
			log.User = user
		}
	}
	if h.apiKeyService != nil && log.APIKeyID > 0 {
		if apiKey, err := h.apiKeyService.GetByID(ctx, log.APIKeyID); err == nil {
			log.APIKey = apiKey
		}
	}
	if h.adminService != nil && log.AccountID > 0 {
		if account, err := h.adminService.GetAccount(ctx, log.AccountID); err == nil {
			log.Account = account
		}
	}
	if h.adminService != nil && log.GroupID != nil && *log.GroupID > 0 {
		if group, err := h.adminService.GetGroup(ctx, *log.GroupID); err == nil {
			log.Group = group
		}
	}
}

func buildReplayLabSummary(log *service.UsageLog, usageDTO *dto.AdminUsageLog) map[string]any {
	if log == nil || usageDTO == nil {
		return map[string]any{}
	}
	summary := map[string]any{
		"usage_log_id":          log.ID,
		"request_id":            log.RequestID,
		"user_id":               log.UserID,
		"api_key_id":            log.APIKeyID,
		"account_id":            log.AccountID,
		"model":                 usageDTO.Model,
		"upstream_model":        usageDTO.UpstreamModel,
		"model_mapping_chain":   usageDTO.ModelMappingChain,
		"inbound_endpoint":      ptrStringValue(log.InboundEndpoint),
		"upstream_endpoint":     ptrStringValue(log.UpstreamEndpoint),
		"request_type":          log.EffectiveRequestType().String(),
		"stream":                usageDTO.Stream,
		"duration_ms":           usageDTO.DurationMs,
		"first_token_ms":        usageDTO.FirstTokenMs,
		"input_tokens":          usageDTO.InputTokens,
		"output_tokens":         usageDTO.OutputTokens,
		"cache_creation_tokens": usageDTO.CacheCreationTokens,
		"cache_read_tokens":     usageDTO.CacheReadTokens,
		"total_tokens":          log.TotalTokens(),
		"actual_cost":           usageDTO.ActualCost,
		"created_at":            usageDTO.CreatedAt,
	}
	if log.User != nil {
		summary["user"] = map[string]any{"id": log.User.ID, "email": log.User.Email}
	}
	if log.APIKey != nil {
		summary["api_key"] = map[string]any{"id": log.APIKey.ID, "name": log.APIKey.Name}
	}
	if log.Account != nil {
		summary["account"] = map[string]any{
			"id":          log.Account.ID,
			"name":        log.Account.Name,
			"platform":    log.Account.Platform,
			"type":        log.Account.Type,
			"status":      log.Account.Status,
			"schedulable": log.Account.IsSchedulable(),
		}
	}
	return summary
}

func buildReplayLabRouteInfo(log *service.UsageLog, usageDTO *dto.AdminUsageLog) ReplayLabRouteInfo {
	route := ReplayLabRouteInfo{}
	if log == nil || usageDTO == nil {
		return route
	}
	route.InboundEndpoint = ptrStringValue(log.InboundEndpoint)
	route.UpstreamEndpoint = ptrStringValue(log.UpstreamEndpoint)
	route.RequestType = log.EffectiveRequestType().String()
	route.RequestedModel = usageDTO.Model
	route.UpstreamModel = usageDTO.UpstreamModel
	route.ModelMappingChain = usageDTO.ModelMappingChain
	route.ChannelID = usageDTO.ChannelID
	route.AccountID = log.AccountID
	route.GroupID = log.GroupID
	route.ProjectKey = log.ProjectKey
	route.ProjectLabel = log.ProjectLabel
	if log.Account != nil {
		route.AccountName = log.Account.Name
		route.AccountPlatform = log.Account.Platform
		route.AccountType = log.Account.Type
		route.AccountStatus = log.Account.Status
		schedulable := log.Account.IsSchedulable()
		route.AccountSchedulable = &schedulable
	}
	return route
}

func buildReplayLabSafetyInfo(log *service.UsageLog) ReplayLabSafetyInfo {
	reasons := []string{
		"usage log currently stores metadata only; original request body is not available",
		"real replay may create duplicated upstream cost, so this endpoint only returns a dry-run package",
	}
	if log != nil && log.EffectiveRequestType() == service.RequestTypeWSV2 {
		reasons = append(reasons, "WebSocket style requests need a separate session transcript before replay")
	}
	return ReplayLabSafetyInfo{
		CanReplay:            false,
		RawSnapshotAvailable: false,
		RequiresManualBody:   true,
		RiskLevel:            "safe_dry_run",
		Reasons:              reasons,
		RecommendedMode:      "copy_curl_template_with_manual_body",
	}
}

func buildReplayLabChecks(log *service.UsageLog, usageDTO *dto.AdminUsageLog) []ReplayLabCheck {
	checks := make([]ReplayLabCheck, 0, 6)
	add := func(key, label, status, message string) {
		checks = append(checks, ReplayLabCheck{Key: key, Label: label, Status: status, Message: message})
	}
	if log == nil || usageDTO == nil {
		add("usage_log", "日志记录", "blocked", "usage log missing")
		return checks
	}
	if strings.TrimSpace(log.RequestID) != "" {
		add("request_id", "请求 ID", "ok", "可用于关联网关日志和上游错误")
	} else {
		add("request_id", "请求 ID", "warn", "缺少 request_id，跨日志关联能力较弱")
	}
	if strings.TrimSpace(ptrStringValue(log.InboundEndpoint)) != "" {
		add("inbound_endpoint", "入口路径", "ok", ptrStringValue(log.InboundEndpoint))
	} else {
		add("inbound_endpoint", "入口路径", "warn", "历史日志缺少入口 endpoint，将使用通用模板")
	}
	if log.AccountID > 0 {
		if log.Account != nil && log.Account.IsSchedulable() {
			add("account", "上游账号", "ok", "原账号当前可调度")
		} else if log.Account != nil {
			add("account", "上游账号", "warn", "原账号当前不可调度，真实回放应改用健康账号")
		} else {
			add("account", "上游账号", "warn", "日志有账号 ID，但当前无法加载账号详情")
		}
	} else {
		add("account", "上游账号", "warn", "该日志没有绑定上游账号")
	}
	if usageDTO.UpstreamModel != nil && strings.TrimSpace(*usageDTO.UpstreamModel) != "" && *usageDTO.UpstreamModel != usageDTO.Model {
		add("model_mapping", "模型映射", "ok", "存在模型映射链，模板会保留用户请求模型")
	} else {
		add("model_mapping", "模型映射", "ok", "未发现模型映射或请求模型与上游模型一致")
	}
	add("raw_snapshot", "原始请求体", "blocked", "未保存原始请求体；只能生成手填 body 的 curl 模板")
	return checks
}

func buildReplayLabCurlTemplate(log *service.UsageLog, usageDTO *dto.AdminUsageLog) string {
	if log == nil || usageDTO == nil {
		return ""
	}
	endpoint := strings.TrimSpace(ptrStringValue(log.InboundEndpoint))
	if endpoint == "" {
		endpoint = "/v1/messages"
	}
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	body := buildReplayLabPlaceholderBody(log, usageDTO, endpoint)
	bodyJSON, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		bodyJSON = []byte(`{"model":"` + usageDTO.Model + `"}`)
	}
	return strings.Join([]string{
		"curl -sS -X POST \"$SUB2API_BASE_URL" + endpoint + "\" \\",
		"  -H \"Authorization: Bearer $SUB2API_KEY\" \\",
		"  -H \"Content-Type: application/json\" \\",
		"  -H \"X-Replay-Lab-Source: usage-log-" + strconv.FormatInt(log.ID, 10) + "\" \\",
		"  --data " + shellSingleQuote(string(bodyJSON)),
	}, "\n")
}

func buildReplayLabPlaceholderBody(log *service.UsageLog, usageDTO *dto.AdminUsageLog, endpoint string) map[string]any {
	body := map[string]any{
		"model": usageDTO.Model,
	}
	if log.EffectiveRequestType() == service.RequestTypeStream {
		body["stream"] = true
	}
	lowerEndpoint := strings.ToLower(endpoint)
	switch {
	case strings.Contains(lowerEndpoint, "responses"):
		body["input"] = "TODO: paste the original user input here"
	case strings.Contains(lowerEndpoint, "images"):
		body["prompt"] = "TODO: paste the original image prompt here"
	default:
		body["messages"] = []map[string]string{
			{"role": "user", "content": "TODO: paste the original user message here"},
		}
	}
	if usageDTO.ServiceTier != nil && strings.TrimSpace(*usageDTO.ServiceTier) != "" {
		body["service_tier"] = *usageDTO.ServiceTier
	}
	if usageDTO.ReasoningEffort != nil && strings.TrimSpace(*usageDTO.ReasoningEffort) != "" {
		body["reasoning_effort"] = *usageDTO.ReasoningEffort
	}
	return body
}

func ptrStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

// Stats handles getting usage statistics with filters
// GET /api/v1/admin/usage/stats
func (h *UsageHandler) Stats(c *gin.Context) {
	// Parse filters - same as List endpoint
	var userID, apiKeyID, accountID, groupID int64
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		id, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid user_id")
			return
		}
		userID = id
	}

	if apiKeyIDStr := c.Query("api_key_id"); apiKeyIDStr != "" {
		id, err := strconv.ParseInt(apiKeyIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid api_key_id")
			return
		}
		apiKeyID = id
	}

	if accountIDStr := c.Query("account_id"); accountIDStr != "" {
		id, err := strconv.ParseInt(accountIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid account_id")
			return
		}
		accountID = id
	}

	if groupIDStr := c.Query("group_id"); groupIDStr != "" {
		id, err := strconv.ParseInt(groupIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid group_id")
			return
		}
		groupID = id
	}

	model := c.Query("model")
	billingMode := strings.TrimSpace(c.Query("billing_mode"))

	var requestType *int16
	var stream *bool
	if requestTypeStr := strings.TrimSpace(c.Query("request_type")); requestTypeStr != "" {
		parsed, err := service.ParseUsageRequestType(requestTypeStr)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		value := int16(parsed)
		requestType = &value
	} else if streamStr := c.Query("stream"); streamStr != "" {
		val, err := strconv.ParseBool(streamStr)
		if err != nil {
			response.BadRequest(c, "Invalid stream value, use true or false")
			return
		}
		stream = &val
	}

	var billingType *int8
	if billingTypeStr := c.Query("billing_type"); billingTypeStr != "" {
		val, err := strconv.ParseInt(billingTypeStr, 10, 8)
		if err != nil {
			response.BadRequest(c, "Invalid billing_type")
			return
		}
		bt := int8(val)
		billingType = &bt
	}

	// Parse date range
	userTZ := c.Query("timezone")
	now := timezone.NowInUserLocation(userTZ)
	var startTime, endTime time.Time

	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	if startDateStr != "" && endDateStr != "" {
		var err error
		startTime, err = timezone.ParseInUserLocation("2006-01-02", startDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid start_date format, use YYYY-MM-DD")
			return
		}
		endTime, err = timezone.ParseInUserLocation("2006-01-02", endDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid end_date format, use YYYY-MM-DD")
			return
		}
		// 与 SQL 条件 created_at < end 对齐，使用次日 00:00 作为上边界（DST-safe）。
		endTime = endTime.AddDate(0, 0, 1)
	} else {
		period := c.DefaultQuery("period", "today")
		switch period {
		case "today":
			startTime = timezone.StartOfDayInUserLocation(now, userTZ)
		case "week":
			startTime = now.AddDate(0, 0, -7)
		case "month":
			startTime = now.AddDate(0, -1, 0)
		default:
			startTime = timezone.StartOfDayInUserLocation(now, userTZ)
		}
		endTime = now
	}

	// Build filters and call GetStatsWithFilters
	filters := usagestats.UsageLogFilters{
		UserID:      userID,
		APIKeyID:    apiKeyID,
		AccountID:   accountID,
		GroupID:     groupID,
		Model:       model,
		RequestType: requestType,
		Stream:      stream,
		BillingType: billingType,
		BillingMode: billingMode,
		StartTime:   &startTime,
		EndTime:     &endTime,
	}

	var stats *usagestats.UsageStats
	if parseBoolQueryWithDefault(c.Query("nocache"), false) {
		s, err := h.usageService.GetStatsWithFilters(c.Request.Context(), filters)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		stats = s
		c.Header("X-Usage-Stats-Cache", "bypass")
	} else {
		s, hit, err := h.getStatsCached(c.Request.Context(), filters)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		stats = s
		c.Header("X-Usage-Stats-Cache", cacheStatusValue(hit))
	}

	response.Success(c, stats)
}

// SearchUsers handles searching users by email keyword
// GET /api/v1/admin/usage/search-users
func (h *UsageHandler) SearchUsers(c *gin.Context) {
	keyword := c.Query("q")
	if keyword == "" {
		response.Success(c, []any{})
		return
	}

	// Limit to 30 results
	users, _, err := h.adminService.ListUsers(c.Request.Context(), 1, 30, service.UserListFilters{Search: keyword, IncludeDeleted: true}, "email", "asc")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Return simplified user list (only id and email)
	type SimpleUser struct {
		ID      int64  `json:"id"`
		Email   string `json:"email"`
		Deleted bool   `json:"deleted"`
	}

	result := make([]SimpleUser, len(users))
	for i, u := range users {
		result[i] = SimpleUser{
			ID:      u.ID,
			Email:   u.Email,
			Deleted: u.DeletedAt != nil,
		}
	}

	response.Success(c, result)
}

// SearchAPIKeys handles searching API keys by user
// GET /api/v1/admin/usage/search-api-keys
func (h *UsageHandler) SearchAPIKeys(c *gin.Context) {
	userIDStr := c.Query("user_id")
	keyword := c.Query("q")

	var userID int64
	if userIDStr != "" {
		id, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid user_id")
			return
		}
		userID = id
	}

	keys, err := h.apiKeyService.SearchAPIKeys(c.Request.Context(), userID, keyword, 30)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Return simplified API key list (only id and name)
	type SimpleAPIKey struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		UserID int64  `json:"user_id"`
	}

	result := make([]SimpleAPIKey, len(keys))
	for i, k := range keys {
		result[i] = SimpleAPIKey{
			ID:     k.ID,
			Name:   k.Name,
			UserID: k.UserID,
		}
	}

	response.Success(c, result)
}

// ListCleanupTasks handles listing usage cleanup tasks
// GET /api/v1/admin/usage/cleanup-tasks
func (h *UsageHandler) ListCleanupTasks(c *gin.Context) {
	if h.cleanupService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Usage cleanup service unavailable")
		return
	}
	operator := int64(0)
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok {
		operator = subject.UserID
	}
	page, pageSize := response.ParsePagination(c)
	logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 请求清理任务列表: operator=%d page=%d page_size=%d", operator, page, pageSize)
	params := pagination.PaginationParams{Page: page, PageSize: pageSize}
	tasks, result, err := h.cleanupService.ListTasks(c.Request.Context(), params)
	if err != nil {
		logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 查询清理任务列表失败: operator=%d page=%d page_size=%d err=%v", operator, page, pageSize, err)
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.UsageCleanupTask, 0, len(tasks))
	for i := range tasks {
		out = append(out, *dto.UsageCleanupTaskFromService(&tasks[i]))
	}
	logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 返回清理任务列表: operator=%d total=%d items=%d page=%d page_size=%d", operator, result.Total, len(out), page, pageSize)
	response.Paginated(c, out, result.Total, page, pageSize)
}

// CreateCleanupTask handles creating a usage cleanup task
// POST /api/v1/admin/usage/cleanup-tasks
func (h *UsageHandler) CreateCleanupTask(c *gin.Context) {
	if h.cleanupService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Usage cleanup service unavailable")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Unauthorized")
		return
	}

	var req CreateUsageCleanupTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	req.StartDate = strings.TrimSpace(req.StartDate)
	req.EndDate = strings.TrimSpace(req.EndDate)
	if req.StartDate == "" || req.EndDate == "" {
		response.BadRequest(c, "start_date and end_date are required")
		return
	}

	startTime, err := timezone.ParseInUserLocation("2006-01-02", req.StartDate, req.Timezone)
	if err != nil {
		response.BadRequest(c, "Invalid start_date format, use YYYY-MM-DD")
		return
	}
	endTime, err := timezone.ParseInUserLocation("2006-01-02", req.EndDate, req.Timezone)
	if err != nil {
		response.BadRequest(c, "Invalid end_date format, use YYYY-MM-DD")
		return
	}
	endTime = endTime.Add(24*time.Hour - time.Nanosecond)

	var requestType *int16
	stream := req.Stream
	if req.RequestType != nil {
		parsed, err := service.ParseUsageRequestType(*req.RequestType)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		value := int16(parsed)
		requestType = &value
		stream = nil
	}

	filters := service.UsageCleanupFilters{
		StartTime:   startTime,
		EndTime:     endTime,
		UserID:      req.UserID,
		APIKeyID:    req.APIKeyID,
		AccountID:   req.AccountID,
		GroupID:     req.GroupID,
		Model:       req.Model,
		RequestType: requestType,
		Stream:      stream,
		BillingType: req.BillingType,
	}

	var userID any
	if filters.UserID != nil {
		userID = *filters.UserID
	}
	var apiKeyID any
	if filters.APIKeyID != nil {
		apiKeyID = *filters.APIKeyID
	}
	var accountID any
	if filters.AccountID != nil {
		accountID = *filters.AccountID
	}
	var groupID any
	if filters.GroupID != nil {
		groupID = *filters.GroupID
	}
	var model any
	if filters.Model != nil {
		model = *filters.Model
	}
	var streamValue any
	if filters.Stream != nil {
		streamValue = *filters.Stream
	}
	var requestTypeName any
	if filters.RequestType != nil {
		requestTypeName = service.RequestTypeFromInt16(*filters.RequestType).String()
	}
	var billingType any
	if filters.BillingType != nil {
		billingType = *filters.BillingType
	}

	idempotencyPayload := struct {
		OperatorID int64                         `json:"operator_id"`
		Body       CreateUsageCleanupTaskRequest `json:"body"`
	}{
		OperatorID: subject.UserID,
		Body:       req,
	}
	executeAdminIdempotentJSON(c, "admin.usage.cleanup_tasks.create", idempotencyPayload, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 请求创建清理任务: operator=%d start=%s end=%s user_id=%v api_key_id=%v account_id=%v group_id=%v model=%v request_type=%v stream=%v billing_type=%v tz=%q",
			subject.UserID,
			filters.StartTime.Format(time.RFC3339),
			filters.EndTime.Format(time.RFC3339),
			userID,
			apiKeyID,
			accountID,
			groupID,
			model,
			requestTypeName,
			streamValue,
			billingType,
			req.Timezone,
		)

		task, err := h.cleanupService.CreateTask(ctx, filters, subject.UserID)
		if err != nil {
			logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 创建清理任务失败: operator=%d err=%v", subject.UserID, err)
			return nil, err
		}
		logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 清理任务已创建: task=%d operator=%d status=%s", task.ID, subject.UserID, task.Status)
		return dto.UsageCleanupTaskFromService(task), nil
	})
}

// CancelCleanupTask handles canceling a usage cleanup task
// POST /api/v1/admin/usage/cleanup-tasks/:id/cancel
func (h *UsageHandler) CancelCleanupTask(c *gin.Context) {
	if h.cleanupService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Usage cleanup service unavailable")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Unauthorized")
		return
	}
	idStr := strings.TrimSpace(c.Param("id"))
	taskID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || taskID <= 0 {
		response.BadRequest(c, "Invalid task id")
		return
	}
	logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 请求取消清理任务: task=%d operator=%d", taskID, subject.UserID)
	if err := h.cleanupService.CancelTask(c.Request.Context(), taskID, subject.UserID); err != nil {
		logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 取消清理任务失败: task=%d operator=%d err=%v", taskID, subject.UserID, err)
		response.ErrorFrom(c, err)
		return
	}
	logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 清理任务已取消: task=%d operator=%d", taskID, subject.UserID)
	response.Success(c, gin.H{"id": taskID, "status": service.UsageCleanupStatusCanceled})
}
