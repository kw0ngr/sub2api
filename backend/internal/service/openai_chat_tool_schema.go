package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// xAI 的 tools[].parameters 校验要求 root schema 是纯 object：root 挂
// anyOf/oneOf（哪怕同时标了 type:object，且分支无 type 字段）会被判为
// union 直接 400 [invalid_client_tool_schema]。OpenAI 对此宽容，因此部分
// agent 工具（如 tool_wait 用 oneOf 做「seconds 与 targets/timeout 互斥」）
// 只在 Grok 上炸。本兼容层只对 Grok 三方 key 生效，把 root 级 union 剥掉：
//   - anyOf/oneOf 分支里的 required 合并进 root required（并集，宽松化——
//     互斥语义降级为文档语义，交给 description 约束）；
//   - properties/additionalProperties 等其余字段原样保留；
//   - 嵌套层级的 anyOf/oneOf 不动（xAI 只校验 root）。
func normalizeGrokThirdPartyChatToolSchemas(account *Account, body []byte, chatReq *apicompat.ChatCompletionsRequest) ([]byte, bool, error) {
	if !isThirdPartyGrokAPIKey(account) {
		return body, false, nil
	}
	if !hasRootUnionToolSchema(chatReq, body) {
		return body, false, nil
	}

	out := body
	modified := false
	for i := range toolFunctionPaths(chatReq, body) {
		path := fmt.Sprintf("tools.%d.function.parameters", i)
		raw := gjson.GetBytes(out, path).Raw
		if raw == "" {
			continue
		}
		fixed, changed, err := stripRootUnionFromToolSchema([]byte(raw))
		if err != nil {
			return nil, false, fmt.Errorf("normalize grok tool schema at %s: %w", path, err)
		}
		if !changed {
			continue
		}
		var serr error
		out, serr = sjson.SetBytes(out, path, json.RawMessage(fixed))
		if serr != nil {
			return nil, false, fmt.Errorf("set normalized tool schema at %s: %w", path, serr)
		}
		modified = true
	}
	return out, modified, nil
}

// hasRootUnionToolSchema 粗判请求里是否有 root 带 anyOf/oneOf 的工具参数，
// 避免对干净请求做全量 JSON 改写（热路径零开销）。
func hasRootUnionToolSchema(chatReq *apicompat.ChatCompletionsRequest, body []byte) bool {
	count := toolFunctionPaths(chatReq, body)
	for _, i := range count {
		raw := gjson.GetBytes(body, fmt.Sprintf("tools.%d.function.parameters", i)).Raw
		if raw == "" {
			continue
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			continue
		}
		if _, has := m["oneOf"]; has {
			return true
		}
		if _, has := m["anyOf"]; has {
			return true
		}
	}
	return false
}

// toolFunctionPaths 返回 tools 下 function 型工具的下标。
func toolFunctionPaths(chatReq *apicompat.ChatCompletionsRequest, body []byte) []int {
	if chatReq != nil && len(chatReq.Tools) > 0 {
		idx := make([]int, 0, len(chatReq.Tools))
		for i := range chatReq.Tools {
			idx = append(idx, i)
		}
		return idx
	}
	n := len(gjson.GetBytes(body, "tools").Array())
	idx := make([]int, 0, n)
	for i := 0; i < n; i++ {
		idx = append(idx, i)
	}
	return idx
}

// stripRootUnionFromToolSchema 剥掉参数 schema root 级的 anyOf/oneOf。
// 分支里的 required 取并集并入 root（宽松化：原互斥约束降级为文档语义，
// xAI 只认 object root，宽松 required 不会引发上游校验失败）。
func stripRootUnionFromToolSchema(schema []byte) ([]byte, bool, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(schema, &m); err != nil {
		return schema, false, nil // 非 object root 原样返回，由上游报错
	}

	unionKey := ""
	for _, k := range []string{"oneOf", "anyOf"} {
		if _, ok := m[k]; ok {
			unionKey = k
			break
		}
	}
	if unionKey == "" {
		return schema, false, nil
	}

	var branches []map[string]json.RawMessage
	if err := json.Unmarshal(m[unionKey], &branches); err != nil {
		return schema, false, nil
	}

	// 分支 required 并集
	merged := map[string]bool{}
	var order []string
	for _, b := range branches {
		var req []string
		_ = json.Unmarshal(b["required"], &req)
		for _, r := range req {
			if !merged[r] {
				merged[r] = true
				order = append(order, r)
			}
		}
	}

	delete(m, unionKey)
	delete(m, "allOf") // root 级 allOf 常与 oneOf 配对出现，一并剥掉防同类拒收
	if strings.TrimSpace(string(m["type"])) == "" {
		m["type"] = json.RawMessage(`"object"`)
	}
	if len(order) > 0 {
		reqJSON, err := json.Marshal(order)
		if err != nil {
			return schema, false, err
		}
		m["required"] = reqJSON
	} else {
		delete(m, "required")
	}

	out, err := json.Marshal(m)
	if err != nil {
		return schema, false, err
	}
	return out, true, nil
}
