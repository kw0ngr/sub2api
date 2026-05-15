package service

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"sort"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const toolNameRewriteKey = "claude_tool_name_rewrite"

var staticToolNameRewrites = map[string]string{
	"sessions_": "cc_sess_",
	"session_":  "cc_ses_",
}

var fakeToolNamePrefixes = []string{
	"analyze_", "compute_", "fetch_", "generate_", "lookup_", "modify_",
	"process_", "query_", "render_", "resolve_", "sync_", "update_",
	"validate_", "convert_", "extract_", "manage_", "monitor_", "parse_",
	"review_", "search_", "transform_", "handle_", "invoke_", "notify_",
}

const dynamicToolMapThreshold = 5

type ToolNameRewrite struct {
	Forward        map[string]string
	Reverse        map[string]string
	ReverseOrdered [][2]string
}

func buildDynamicToolMap(toolNames []string) map[string]string {
	if len(toolNames) <= dynamicToolMapThreshold {
		return nil
	}
	h := fnv.New64a()
	for i, name := range toolNames {
		if i > 0 {
			_, _ = h.Write([]byte{0})
		}
		_, _ = h.Write([]byte(name))
	}
	rng := rand.New(rand.NewSource(int64(h.Sum64())))
	available := make([]string, len(fakeToolNamePrefixes))
	copy(available, fakeToolNamePrefixes)
	rng.Shuffle(len(available), func(i, j int) { available[i], available[j] = available[j], available[i] })

	mapping := make(map[string]string, len(toolNames))
	for i, name := range toolNames {
		headLen := 3
		if len(name) < headLen {
			headLen = len(name)
		}
		mapping[name] = fmt.Sprintf("%s%s%02d", available[i%len(available)], name[:headLen], i)
	}
	return mapping
}

func sanitizeToolName(name string, dynamic map[string]string) string {
	if dynamic != nil {
		if fake, ok := dynamic[name]; ok {
			return fake
		}
	}
	for prefix, replacement := range staticToolNameRewrites {
		if strings.HasPrefix(name, prefix) {
			return replacement + name[len(prefix):]
		}
	}
	return name
}

func shouldMimicToolName(toolType string) bool {
	return toolType == "" || toolType == "function" || toolType == "custom"
}

func buildToolNameRewriteFromBody(body []byte) *ToolNameRewrite {
	tools := gjson.GetBytes(body, "tools")
	if !tools.IsArray() {
		return nil
	}

	mimicableNames := make([]string, 0)
	for _, tool := range tools.Array() {
		if !shouldMimicToolName(tool.Get("type").String()) {
			continue
		}
		if name := tool.Get("name").String(); name != "" {
			mimicableNames = append(mimicableNames, name)
		}
	}

	dynamic := buildDynamicToolMap(mimicableNames)
	rw := &ToolNameRewrite{
		Forward: make(map[string]string),
		Reverse: make(map[string]string),
	}
	for _, name := range mimicableNames {
		fake := sanitizeToolName(name, dynamic)
		if fake == name {
			continue
		}
		rw.Forward[name] = fake
		rw.Reverse[fake] = name
	}
	if len(rw.Forward) == 0 {
		return nil
	}

	rw.ReverseOrdered = make([][2]string, 0, len(rw.Reverse))
	for fake, real := range rw.Reverse {
		rw.ReverseOrdered = append(rw.ReverseOrdered, [2]string{fake, real})
	}
	sort.SliceStable(rw.ReverseOrdered, func(i, j int) bool {
		return len(rw.ReverseOrdered[i][0]) > len(rw.ReverseOrdered[j][0])
	})
	return rw
}

func applyToolNameRewriteToBody(body []byte, rw *ToolNameRewrite) []byte {
	if rw == nil || len(rw.Forward) == 0 {
		return body
	}

	tools := gjson.GetBytes(body, "tools")
	if tools.IsArray() {
		idx := -1
		tools.ForEach(func(_, tool gjson.Result) bool {
			idx++
			if !shouldMimicToolName(tool.Get("type").String()) {
				return true
			}
			name := tool.Get("name").String()
			fake, ok := rw.Forward[name]
			if !ok {
				return true
			}
			if next, err := sjson.SetBytes(body, fmt.Sprintf("tools.%d.name", idx), fake); err == nil {
				body = next
			}
			return true
		})
	}

	if toolChoice := gjson.GetBytes(body, "tool_choice"); toolChoice.Exists() && toolChoice.Get("type").String() == "tool" {
		if fake, ok := rw.Forward[toolChoice.Get("name").String()]; ok {
			if next, err := sjson.SetBytes(body, "tool_choice.name", fake); err == nil {
				body = next
			}
		}
	}

	messages := gjson.GetBytes(body, "messages")
	if messages.IsArray() {
		messages.ForEach(func(msgKey, msg gjson.Result) bool {
			msgIdx := int(msgKey.Num)
			content := msg.Get("content")
			if !content.IsArray() {
				return true
			}
			content.ForEach(func(blockKey, block gjson.Result) bool {
				blockIdx := int(blockKey.Num)
				if block.Get("type").String() != "tool_use" {
					return true
				}
				if fake, ok := rw.Forward[block.Get("name").String()]; ok {
					if next, err := sjson.SetBytes(body, fmt.Sprintf("messages.%d.content.%d.name", msgIdx, blockIdx), fake); err == nil {
						body = next
					}
				}
				return true
			})
			return true
		})
	}
	return body
}

func restoreToolNamesInBytes(data []byte, rw *ToolNameRewrite) []byte {
	if rw == nil {
		return data
	}
	for _, pair := range rw.ReverseOrdered {
		fake, real := pair[0], pair[1]
		if fake == "" || fake == real {
			continue
		}
		data = replaceAllBytes(data, fake, real)
	}
	return data
}

func replaceAllBytes(data []byte, from, to string) []byte {
	if len(data) == 0 || from == to || !strings.Contains(string(data), from) {
		return data
	}
	return []byte(strings.ReplaceAll(string(data), from, to))
}

func toolNameRewriteFromContext(c interface {
	Get(string) (any, bool)
}) *ToolNameRewrite {
	if c == nil {
		return nil
	}
	raw, ok := c.Get(toolNameRewriteKey)
	if !ok || raw == nil {
		return nil
	}
	rw, _ := raw.(*ToolNameRewrite)
	return rw
}

func reverseToolNamesIfPresent(c interface {
	Get(string) (any, bool)
}, chunk []byte) []byte {
	return restoreToolNamesInBytes(chunk, toolNameRewriteFromContext(c))
}
