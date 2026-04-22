package service

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultDebugTraceMaxEntries = 1000
	defaultDebugTraceTTL        = 15 * time.Minute
)

type DebugTrace struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`

	RequestID        string `json:"request_id,omitempty"`
	ClientRequestID  string `json:"client_request_id,omitempty"`
	Method           string `json:"method,omitempty"`
	Path             string `json:"path,omitempty"`
	InboundEndpoint  string `json:"inbound_endpoint,omitempty"`
	UpstreamEndpoint string `json:"upstream_endpoint,omitempty"`

	UserID    *int64 `json:"user_id,omitempty"`
	APIKeyID  *int64 `json:"api_key_id,omitempty"`
	AccountID *int64 `json:"account_id,omitempty"`
	GroupID   *int64 `json:"group_id,omitempty"`

	Platform      string `json:"platform,omitempty"`
	Model         string `json:"model,omitempty"`
	UpstreamModel string `json:"upstream_model,omitempty"`
	Stream        bool   `json:"stream"`
	UserAgent     string `json:"user_agent,omitempty"`

	StatusCode         int    `json:"status_code"`
	UpstreamStatusCode *int   `json:"upstream_status_code,omitempty"`
	ErrorType          string `json:"error_type,omitempty"`
	ErrorCode          string `json:"error_code,omitempty"`
	ErrorMessage       string `json:"error_message,omitempty"`

	ReasonCode string `json:"reason_code,omitempty"`
	ReasonHint string `json:"reason_hint,omitempty"`

	RequestBodyBytes            *int     `json:"request_body_bytes,omitempty"`
	RequestBodyPreviewJSON      *string  `json:"request_body_preview_json,omitempty"`
	RequestBodyPreviewStrategy  string   `json:"request_body_preview_strategy,omitempty"`
	RequestBodyPreviewTruncated bool     `json:"request_body_preview_truncated,omitempty"`
	RequestBodyTruncatedPaths   []string `json:"request_body_truncated_paths,omitempty"`

	ResponseBodyPreview   *string `json:"response_body_preview,omitempty"`
	ResponseBodyTruncated bool    `json:"response_body_truncated,omitempty"`
	RequestHeadersJSON    *string `json:"request_headers_json,omitempty"`

	FallbackTriggered  bool   `json:"fallback_triggered"`
	AccountSwitchCount *int   `json:"account_switch_count,omitempty"`
	SchedulerLayer     string `json:"scheduler_layer,omitempty"`
	StickySessionHit   *bool  `json:"sticky_session_hit,omitempty"`
	StickyPreviousHit  *bool  `json:"sticky_previous_hit,omitempty"`

	AuthLatencyMs      *int64 `json:"auth_latency_ms,omitempty"`
	RoutingLatencyMs   *int64 `json:"routing_latency_ms,omitempty"`
	UpstreamLatencyMs  *int64 `json:"upstream_latency_ms,omitempty"`
	ResponseLatencyMs  *int64 `json:"response_latency_ms,omitempty"`
	TimeToFirstTokenMs *int64 `json:"time_to_first_token_ms,omitempty"`

	UpstreamErrors []*OpsUpstreamErrorEvent `json:"upstream_errors,omitempty"`
}

type DebugTraceFilter struct {
	Limit        int
	RequestID    string
	Path         string
	Platform     string
	ReasonCode   string
	AccountID    *int64
	OnlyErrors   bool
	OnlyFallback bool
}

type debugTraceStore struct {
	mu         sync.RWMutex
	items      []*DebugTrace
	next       int
	count      int
	maxEntries int
	ttl        time.Duration
	seq        atomic.Uint64
}

func newDebugTraceStore(maxEntries int, ttl time.Duration) *debugTraceStore {
	if maxEntries <= 0 {
		maxEntries = defaultDebugTraceMaxEntries
	}
	if ttl <= 0 {
		ttl = defaultDebugTraceTTL
	}
	return &debugTraceStore{
		items:      make([]*DebugTrace, maxEntries),
		maxEntries: maxEntries,
		ttl:        ttl,
	}
}

var defaultDebugTraceStore = newDebugTraceStore(defaultDebugTraceMaxEntries, defaultDebugTraceTTL)

func RecordDebugTrace(trace *DebugTrace) {
	defaultDebugTraceStore.Add(trace)
}

func GetDebugTrace(id string) (*DebugTrace, bool) {
	return defaultDebugTraceStore.Get(id)
}

func ListDebugTraces(filter DebugTraceFilter) []*DebugTrace {
	return defaultDebugTraceStore.List(filter)
}

func ResetDebugTraceStoreForTest() {
	defaultDebugTraceStore = newDebugTraceStore(defaultDebugTraceMaxEntries, defaultDebugTraceTTL)
}

func (s *debugTraceStore) Add(trace *DebugTrace) {
	if s == nil || trace == nil {
		return
	}
	entry := cloneDebugTrace(trace)
	if entry == nil {
		return
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	if strings.TrimSpace(entry.ID) == "" {
		entry.ID = "dbg_" + strconv.FormatUint(s.seq.Add(1), 10)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.items[s.next] = entry
	s.next = (s.next + 1) % s.maxEntries
	if s.count < s.maxEntries {
		s.count++
	}
}

func (s *debugTraceStore) Get(id string) (*DebugTrace, bool) {
	if s == nil || strings.TrimSpace(id) == "" {
		return nil, false
	}
	now := time.Now()

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, item := range s.iterLocked() {
		if item == nil {
			continue
		}
		if now.Sub(item.CreatedAt) > s.ttl {
			continue
		}
		if item.ID == id {
			return cloneDebugTrace(item), true
		}
	}
	return nil, false
}

func (s *debugTraceStore) List(filter DebugTraceFilter) []*DebugTrace {
	if s == nil {
		return nil
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	now := time.Now()

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*DebugTrace, 0, min(limit, s.count))
	for _, item := range s.iterLocked() {
		if item == nil {
			continue
		}
		if now.Sub(item.CreatedAt) > s.ttl {
			continue
		}
		if !matchDebugTraceFilter(item, filter) {
			continue
		}
		result = append(result, cloneDebugTrace(item))
		if len(result) >= limit {
			break
		}
	}
	return result
}

func (s *debugTraceStore) iterLocked() []*DebugTrace {
	if s.count == 0 {
		return nil
	}
	out := make([]*DebugTrace, 0, s.count)
	for i := 0; i < s.count; i++ {
		idx := s.next - 1 - i
		if idx < 0 {
			idx += s.maxEntries
		}
		out = append(out, s.items[idx])
	}
	return out
}

func matchDebugTraceFilter(item *DebugTrace, filter DebugTraceFilter) bool {
	if item == nil {
		return false
	}
	if filter.OnlyErrors && item.StatusCode < 400 && len(item.UpstreamErrors) == 0 {
		return false
	}
	if filter.OnlyFallback && !item.FallbackTriggered {
		return false
	}
	if requestID := strings.TrimSpace(filter.RequestID); requestID != "" {
		if item.RequestID != requestID && item.ClientRequestID != requestID {
			return false
		}
	}
	if path := strings.TrimSpace(filter.Path); path != "" && !strings.Contains(strings.ToLower(item.Path), strings.ToLower(path)) {
		return false
	}
	if platform := strings.TrimSpace(filter.Platform); platform != "" && !strings.EqualFold(item.Platform, platform) {
		return false
	}
	if reason := strings.TrimSpace(filter.ReasonCode); reason != "" && !strings.EqualFold(item.ReasonCode, reason) {
		return false
	}
	if filter.AccountID != nil {
		if item.AccountID == nil || *item.AccountID != *filter.AccountID {
			return false
		}
	}
	return true
}

func cloneDebugTrace(in *DebugTrace) *DebugTrace {
	if in == nil {
		return nil
	}
	out := *in
	out.UserID = cloneInt64Ptr(in.UserID)
	out.APIKeyID = cloneInt64Ptr(in.APIKeyID)
	out.AccountID = cloneInt64Ptr(in.AccountID)
	out.GroupID = cloneInt64Ptr(in.GroupID)
	out.UpstreamStatusCode = cloneIntPtr(in.UpstreamStatusCode)
	out.RequestBodyBytes = cloneIntPtr(in.RequestBodyBytes)
	out.RequestBodyPreviewJSON = cloneStringPtr(in.RequestBodyPreviewJSON)
	out.ResponseBodyPreview = cloneStringPtr(in.ResponseBodyPreview)
	out.RequestHeadersJSON = cloneStringPtr(in.RequestHeadersJSON)
	out.AccountSwitchCount = cloneIntPtr(in.AccountSwitchCount)
	out.StickySessionHit = cloneBoolPtr(in.StickySessionHit)
	out.StickyPreviousHit = cloneBoolPtr(in.StickyPreviousHit)
	out.AuthLatencyMs = cloneInt64Ptr(in.AuthLatencyMs)
	out.RoutingLatencyMs = cloneInt64Ptr(in.RoutingLatencyMs)
	out.UpstreamLatencyMs = cloneInt64Ptr(in.UpstreamLatencyMs)
	out.ResponseLatencyMs = cloneInt64Ptr(in.ResponseLatencyMs)
	out.TimeToFirstTokenMs = cloneInt64Ptr(in.TimeToFirstTokenMs)
	if len(in.RequestBodyTruncatedPaths) > 0 {
		out.RequestBodyTruncatedPaths = append([]string(nil), in.RequestBodyTruncatedPaths...)
	}
	if len(in.UpstreamErrors) > 0 {
		out.UpstreamErrors = make([]*OpsUpstreamErrorEvent, 0, len(in.UpstreamErrors))
		for _, item := range in.UpstreamErrors {
			if item == nil {
				continue
			}
			copyItem := *item
			out.UpstreamErrors = append(out.UpstreamErrors, &copyItem)
		}
	}
	return &out
}

func cloneStringPtr(in *string) *string {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneIntPtr(in *int) *int {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneInt64Ptr(in *int64) *int64 {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneBoolPtr(in *bool) *bool {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
