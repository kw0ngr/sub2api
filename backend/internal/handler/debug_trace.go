package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	debugTraceSchedulerLayerKey     = "debug_trace_scheduler_layer"
	debugTraceStickySessionHitKey   = "debug_trace_sticky_session_hit"
	debugTraceStickyPreviousHitKey  = "debug_trace_sticky_previous_hit"
	debugTraceAccountSwitchCountKey = "debug_trace_account_switch_count"
	debugTraceResponsePreviewLimit  = 16 * 1024
)

func setDebugTraceSchedulingContext(c *gin.Context, layer string, stickySessionHit, stickyPreviousHit bool) {
	if c == nil {
		return
	}
	if layer = strings.TrimSpace(layer); layer != "" {
		c.Set(debugTraceSchedulerLayerKey, layer)
	}
	c.Set(debugTraceStickySessionHitKey, stickySessionHit)
	c.Set(debugTraceStickyPreviousHitKey, stickyPreviousHit)
}

func setDebugTraceAccountSwitchCount(c *gin.Context, count int) {
	if c == nil || count <= 0 {
		return
	}
	c.Set(debugTraceAccountSwitchCountKey, count)
	if c.Request != nil {
		c.Request = c.Request.WithContext(service.WithAccountSwitchCount(c.Request.Context(), count, false))
	}
}

func recordDebugTraceFromContext(c *gin.Context, status int, parsed parsedOpsError, responseBody []byte) {
	if c == nil || c.Request == nil {
		return
	}

	apiKey, _ := middleware2.GetAPIKeyFromContext(c)
	trace := &service.DebugTrace{
		CreatedAt:          timeNow(),
		Method:             c.Request.Method,
		Path:               c.Request.URL.Path,
		StatusCode:         status,
		ErrorType:          strings.TrimSpace(parsed.ErrorType),
		ErrorCode:          strings.TrimSpace(parsed.Code),
		ErrorMessage:       strings.TrimSpace(parsed.Message),
		RequestID:          responseHeaderValue(c, "X-Request-Id", "x-request-id"),
		RequestHeadersJSON: extractOpsRetryRequestHeaders(c),
		InboundEndpoint:    GetInboundEndpoint(c),
		UserAgent:          c.GetHeader("User-Agent"),
	}

	if clientRequestID, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string); strings.TrimSpace(clientRequestID) != "" {
		trace.ClientRequestID = clientRequestID
	}
	if model, ok := c.Get(opsModelKey); ok {
		if modelName, ok := model.(string); ok {
			trace.Model = strings.TrimSpace(modelName)
		}
	}
	if upstreamModel, ok := c.Get(opsUpstreamModelKey); ok {
		if modelName, ok := upstreamModel.(string); ok {
			trace.UpstreamModel = strings.TrimSpace(modelName)
		}
	}
	if streamV, ok := c.Get(opsStreamKey); ok {
		if stream, ok := streamV.(bool); ok {
			trace.Stream = stream
		}
	}
	if accountIDV, ok := c.Get(opsAccountIDKey); ok {
		if accountID, ok := accountIDV.(int64); ok && accountID > 0 {
			trace.AccountID = &accountID
		}
	}
	if apiKey != nil {
		trace.APIKeyID = &apiKey.ID
		if apiKey.User != nil {
			trace.UserID = &apiKey.User.ID
		}
		if apiKey.GroupID != nil {
			trace.GroupID = apiKey.GroupID
		}
		if apiKey.Group != nil && strings.TrimSpace(apiKey.Group.Platform) != "" {
			trace.Platform = apiKey.Group.Platform
		}
	}
	if trace.Platform == "" {
		trace.Platform = guessPlatformFromPath(c.Request.URL.Path)
	}
	trace.UpstreamEndpoint = GetUpstreamEndpoint(c, trace.Platform)

	if upstreamStatusCode := debugTraceUpstreamStatusCode(c); upstreamStatusCode != nil {
		trace.UpstreamStatusCode = upstreamStatusCode
	}
	if events := debugTraceUpstreamErrors(c); len(events) > 0 {
		trace.UpstreamErrors = events
		trace.FallbackTriggered = true
	}
	if layer, ok := c.Get(debugTraceSchedulerLayerKey); ok {
		if layerName, ok := layer.(string); ok {
			trace.SchedulerLayer = strings.TrimSpace(layerName)
		}
	}
	if v, ok := c.Get(debugTraceStickySessionHitKey); ok {
		if hit, ok := v.(bool); ok {
			trace.StickySessionHit = &hit
		}
	}
	if v, ok := c.Get(debugTraceStickyPreviousHitKey); ok {
		if hit, ok := v.(bool); ok {
			trace.StickyPreviousHit = &hit
		}
	}
	if v, ok := c.Get(debugTraceAccountSwitchCountKey); ok {
		if count, ok := v.(int); ok && count > 0 {
			trace.AccountSwitchCount = &count
			trace.FallbackTriggered = true
		}
	}
	if trace.AccountSwitchCount == nil {
		if count, ok := service.AccountSwitchCountFromContext(c.Request.Context()); ok && count > 0 {
			trace.AccountSwitchCount = &count
			trace.FallbackTriggered = true
		}
	}

	trace.AuthLatencyMs = getContextLatencyMs(c, service.OpsAuthLatencyMsKey)
	trace.RoutingLatencyMs = getContextLatencyMs(c, service.OpsRoutingLatencyMsKey)
	trace.UpstreamLatencyMs = getContextLatencyMs(c, service.OpsUpstreamLatencyMsKey)
	trace.ResponseLatencyMs = getContextLatencyMs(c, service.OpsResponseLatencyMsKey)
	trace.TimeToFirstTokenMs = getContextLatencyMs(c, service.OpsTimeToFirstTokenMsKey)

	if rawRequestBody := debugTraceRequestBody(c); len(rawRequestBody) > 0 {
		size := len(rawRequestBody)
		trace.RequestBodyBytes = &size
		if shouldCaptureDebugTraceBody(status, trace.UpstreamErrors, trace.AccountSwitchCount) {
			trace.RequestBodyPreviewJSON, trace.RequestBodyPreviewTruncated, trace.RequestBodyBytes, trace.RequestBodyTruncatedPaths, trace.RequestBodyPreviewStrategy = service.PrepareDebugTraceBodyPreview(rawRequestBody)
		}
	}

	if len(responseBody) > 0 {
		preview := truncateString(string(responseBody), debugTraceResponsePreviewLimit)
		trace.ResponseBodyPreview = &preview
		trace.ResponseBodyTruncated = len(responseBody) > len(preview)
	}

	trace.ReasonCode, trace.ReasonHint = classifyDebugTraceReason(status, parsed, trace)
	service.RecordDebugTrace(trace)
}

func shouldCaptureDebugTraceBody(status int, upstreamErrors []*service.OpsUpstreamErrorEvent, switchCount *int) bool {
	if status >= http.StatusBadRequest {
		return true
	}
	if len(upstreamErrors) > 0 {
		return true
	}
	return switchCount != nil && *switchCount > 0
}

func debugTraceRequestBody(c *gin.Context) []byte {
	if c == nil {
		return nil
	}
	v, ok := c.Get(opsRequestBodyKey)
	if !ok {
		return nil
	}
	raw, ok := v.([]byte)
	if !ok || len(raw) == 0 {
		return nil
	}
	return raw
}

func debugTraceUpstreamStatusCode(c *gin.Context) *int {
	if c == nil {
		return nil
	}
	v, ok := c.Get(service.OpsUpstreamStatusCodeKey)
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case int:
		if t > 0 {
			code := t
			return &code
		}
	case int64:
		if t > 0 {
			code := int(t)
			return &code
		}
	}
	return nil
}

func debugTraceUpstreamErrors(c *gin.Context) []*service.OpsUpstreamErrorEvent {
	if c == nil {
		return nil
	}
	v, ok := c.Get(service.OpsUpstreamErrorsKey)
	if !ok {
		return nil
	}
	events, ok := v.([]*service.OpsUpstreamErrorEvent)
	if !ok || len(events) == 0 {
		return nil
	}
	out := make([]*service.OpsUpstreamErrorEvent, 0, len(events))
	for _, item := range events {
		if item == nil {
			continue
		}
		copyItem := *item
		out = append(out, &copyItem)
	}
	return out
}

func responseHeaderValue(c *gin.Context, keys ...string) string {
	if c == nil {
		return ""
	}
	for _, key := range keys {
		if value := strings.TrimSpace(c.Writer.Header().Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func classifyDebugTraceReason(status int, parsed parsedOpsError, trace *service.DebugTrace) (string, string) {
	if trace == nil {
		return "", ""
	}
	upstreamStatus := 0
	if trace.UpstreamStatusCode != nil {
		upstreamStatus = *trace.UpstreamStatusCode
	}
	lowerMessage := strings.ToLower(strings.TrimSpace(parsed.Message))
	if lowerMessage == "" {
		lowerMessage = strings.ToLower(strings.TrimSpace(trace.ErrorMessage))
	}
	if lowerMessage == "" && len(trace.UpstreamErrors) > 0 {
		last := trace.UpstreamErrors[len(trace.UpstreamErrors)-1]
		if last != nil {
			lowerMessage = strings.ToLower(strings.TrimSpace(last.Message + " " + last.Detail))
		}
	}

	switch {
	case status == http.StatusRequestEntityTooLarge || upstreamStatus == http.StatusRequestEntityTooLarge:
		return "request_body_too_large", "Request exceeded the configured body limit. Split the conversation or reduce tool output size."
	case strings.Contains(lowerMessage, "previous_response_id") && strings.Contains(lowerMessage, "response.id"):
		return "previous_response_id_invalid", "Use a response.id (resp_*) for continuation, not a message id."
	case strings.Contains(lowerMessage, "function_call_output") || strings.Contains(lowerMessage, "call_id") || strings.Contains(lowerMessage, "prompt_cache_key") || strings.Contains(lowerMessage, "session"):
		return "continuation_context_missing", "Continuation context is incomplete. Retry from a fresh turn or ensure prompt/session linkage fields are preserved."
	case strings.Contains(lowerMessage, "unsupported parameter") || strings.Contains(lowerMessage, "unknown parameter") || strings.Contains(lowerMessage, "not supported"):
		return "unsupported_upstream_parameter", "Upstream rejected one or more request fields. Compare the request preview with the target provider's supported schema."
	case upstreamStatus == http.StatusTooManyRequests || upstreamStatus == 529 || parsed.ErrorType == "rate_limit_error":
		return "upstream_rate_limited", "Upstream is rate limited. Wait for cooldown or switch to another account/provider."
	case strings.Contains(lowerMessage, "temporary cooldown") || strings.Contains(lowerMessage, "temp unsched") || strings.Contains(lowerMessage, "account_overloaded"):
		return "account_temp_cooldown", "Account entered a temporary cooldown. Retry later or inspect upstream errors for the triggering attempt."
	case strings.Contains(lowerMessage, "all available accounts exhausted"):
		return "fallback_exhausted", "Failover was attempted but no remaining account could serve the request."
	case strings.Contains(lowerMessage, "no available accounts"):
		return "no_available_accounts", "No schedulable account was available for the requested route/model."
	case parsed.ErrorType == "authentication_error":
		return "authentication_failed", "Credentials were rejected. Re-authenticate or verify the selected account is still valid."
	case parsed.ErrorType == "invalid_request_error":
		return "invalid_request", "Request shape was rejected before completion. Inspect the request preview and upstream error details."
	case status < http.StatusBadRequest && len(trace.UpstreamErrors) > 0:
		return "recovered_after_failover", "Request eventually succeeded after one or more upstream errors or retries."
	default:
		return "", ""
	}
}

func timeNow() (now time.Time) {
	return time.Now()
}
