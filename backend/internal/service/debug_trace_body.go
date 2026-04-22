package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/tidwall/gjson"
)

const (
	debugTracePreviewGenericStringBytes = 4096
	debugTracePreviewChatStringBytes    = 2048
	debugTracePreviewMaxBytes           = 64 * 1024
	debugTracePreviewArrayHead          = 4
	debugTracePreviewArrayTail          = 2
)

// PrepareDebugTraceBodyPreview builds a lightweight structured preview for request tracing.
// It preserves most JSON fields, while truncating only the large text/chat fields.
func PrepareDebugTraceBodyPreview(raw []byte) (previewJSON *string, truncated bool, requestBodyBytes *int, truncatedPaths []string, strategy string) {
	if len(raw) == 0 {
		return nil, false, nil, nil, ""
	}
	size := len(raw)
	requestBodyBytes = &size

	if !gjson.ValidBytes(raw) {
		preview := map[string]any{
			"_trace_preview":        truncateUTF8WithMarker(string(raw), debugTracePreviewGenericStringBytes),
			"_trace_original_bytes": len(raw),
			"_trace_invalid_json":   true,
		}
		out, err := json.Marshal(preview)
		if err != nil {
			return nil, false, requestBodyBytes, nil, "raw_preview_v1"
		}
		s := string(out)
		return &s, len(raw) > debugTracePreviewGenericStringBytes, requestBodyBytes, nil, "raw_preview_v1"
	}

	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, false, requestBodyBytes, nil, ""
	}

	previewPayload, truncatedPaths, truncated := buildDebugTracePreviewValue(payload, "")
	out, err := json.Marshal(previewPayload)
	if err != nil {
		return nil, false, requestBodyBytes, truncatedPaths, "structured_preview_v1"
	}
	if len(out) > debugTracePreviewMaxBytes {
		summary := buildCompactDebugTraceBodySummary(raw, truncatedPaths)
		out, err = json.Marshal(summary)
		if err != nil {
			return nil, truncated, requestBodyBytes, truncatedPaths, "compact_summary_v1"
		}
		s := string(out)
		return &s, true, requestBodyBytes, truncatedPaths, "compact_summary_v1"
	}

	s := string(out)
	return &s, truncated, requestBodyBytes, truncatedPaths, "structured_preview_v1"
}

func buildDebugTracePreviewValue(value any, path string) (any, []string, bool) {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for k := range typed {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		out := make(map[string]any, len(typed))
		var truncatedPaths []string
		truncated := false
		for _, key := range keys {
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			childValue, childPaths, childTruncated := buildDebugTracePreviewValue(typed[key], childPath)
			out[key] = childValue
			truncatedPaths = append(truncatedPaths, childPaths...)
			truncated = truncated || childTruncated
		}
		return out, truncatedPaths, truncated
	case []any:
		items := typed
		var truncatedPaths []string
		truncated := false
		if shouldCompactDebugTraceArray(path) && len(items) > debugTracePreviewArrayHead+debugTracePreviewArrayTail {
			compact := make([]any, 0, debugTracePreviewArrayHead+debugTracePreviewArrayTail+1)
			for i := 0; i < debugTracePreviewArrayHead; i++ {
				childPath := fmt.Sprintf("%s[%d]", path, i)
				childValue, childPaths, childTruncated := buildDebugTracePreviewValue(items[i], childPath)
				compact = append(compact, childValue)
				truncatedPaths = append(truncatedPaths, childPaths...)
				truncated = truncated || childTruncated
			}
			compact = append(compact, map[string]any{"_trace_omitted_items": len(items) - debugTracePreviewArrayHead - debugTracePreviewArrayTail})
			for i := len(items) - debugTracePreviewArrayTail; i < len(items); i++ {
				childPath := fmt.Sprintf("%s[%d]", path, i)
				childValue, childPaths, childTruncated := buildDebugTracePreviewValue(items[i], childPath)
				compact = append(compact, childValue)
				truncatedPaths = append(truncatedPaths, childPaths...)
				truncated = truncated || childTruncated
			}
			truncatedPaths = append(truncatedPaths, path)
			return compact, dedupeStrings(truncatedPaths), true
		}

		out := make([]any, 0, len(items))
		for i, item := range items {
			childPath := fmt.Sprintf("%s[%d]", path, i)
			childValue, childPaths, childTruncated := buildDebugTracePreviewValue(item, childPath)
			out = append(out, childValue)
			truncatedPaths = append(truncatedPaths, childPaths...)
			truncated = truncated || childTruncated
		}
		return out, truncatedPaths, truncated
	case string:
		maxBytes := debugTracePreviewGenericStringBytes
		if isDebugTraceLargeTextPath(path) {
			maxBytes = debugTracePreviewChatStringBytes
		}
		if len(typed) <= maxBytes || !utf8.ValidString(typed) {
			return typed, nil, false
		}
		return truncateUTF8WithMarker(typed, maxBytes), []string{path}, true
	default:
		return value, nil, false
	}
}

func shouldCompactDebugTraceArray(path string) bool {
	path = strings.ToLower(strings.TrimSpace(path))
	switch path {
	case "messages", "input", "content", "tool_outputs":
		return true
	}
	return strings.HasSuffix(path, ".messages") ||
		strings.HasSuffix(path, ".input") ||
		strings.HasSuffix(path, ".content") ||
		strings.HasSuffix(path, ".tool_outputs")
}

func isDebugTraceLargeTextPath(path string) bool {
	path = strings.ToLower(strings.TrimSpace(path))
	if path == "" {
		return false
	}
	candidates := []string{
		".text",
		".content",
		".prompt",
		".instructions",
		".system",
		".output",
		".arguments",
		".stdout",
		".stderr",
		".diff",
		".patch",
	}
	for _, candidate := range candidates {
		if strings.HasSuffix(path, candidate) || strings.Contains(path, candidate+"[") {
			return true
		}
	}
	return strings.Contains(path, "messages[") ||
		strings.Contains(path, "input[") ||
		strings.Contains(path, "content[")
}

func buildCompactDebugTraceBodySummary(raw []byte, truncatedPaths []string) map[string]any {
	summary := map[string]any{
		"_trace_preview":        "compact_summary",
		"_trace_original_bytes": len(raw),
	}
	if model := strings.TrimSpace(gjson.GetBytes(raw, "model").String()); model != "" {
		summary["model"] = model
	}
	if stream := gjson.GetBytes(raw, "stream"); stream.Exists() {
		summary["stream"] = stream.Value()
	}
	if previousResponseID := strings.TrimSpace(gjson.GetBytes(raw, "previous_response_id").String()); previousResponseID != "" {
		summary["previous_response_id"] = previousResponseID
	}
	if promptCacheKey := strings.TrimSpace(gjson.GetBytes(raw, "prompt_cache_key").String()); promptCacheKey != "" {
		summary["prompt_cache_key"] = promptCacheKey
	}
	if messages := gjson.GetBytes(raw, "messages"); messages.Exists() && messages.IsArray() {
		summary["messages_count"] = len(messages.Array())
	}
	if input := gjson.GetBytes(raw, "input"); input.Exists() && input.IsArray() {
		summary["input_count"] = len(input.Array())
	}
	if len(truncatedPaths) > 0 {
		summary["_trace_truncated_paths"] = dedupeStrings(truncatedPaths)
	}
	return summary
}

func truncateUTF8WithMarker(value string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(value) <= maxBytes {
		return value
	}
	cut := value[:maxBytes]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return fmt.Sprintf("%s…<truncated %d bytes>", cut, len(value)-len(cut))
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
