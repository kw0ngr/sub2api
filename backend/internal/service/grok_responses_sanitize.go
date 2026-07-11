package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var grokResponsesUnsupportedRecursiveFields = map[string]struct{}{
	"external_web_access": {},
}

var grokResponsesSupportedToolTypes = map[string]struct{}{
	"code_execution":     {},
	"code_interpreter":   {},
	"collections_search": {},
	"file_search":        {},
	"function":           {},
	"mcp":                {},
	"shell":              {},
	"web_search":         {},
	"x_search":           {},
}

func sanitizeGrokResponsesBody(body []byte, upstreamModel string) ([]byte, bool, error) {
	if !json.Valid(body) {
		return nil, false, fmt.Errorf("invalid grok responses json request body")
	}
	out, err := sjson.SetBytes(body, "model", strings.TrimSpace(upstreamModel))
	if err != nil {
		return nil, false, err
	}
	for _, field := range []string{"prompt_cache_retention", "safety_identifier"} {
		if gjson.GetBytes(out, field).Exists() {
			out, err = sjson.DeleteBytes(out, field)
			if err != nil {
				return nil, false, err
			}
		}
	}
	if strings.EqualFold(strings.TrimSpace(upstreamModel), "grok-4.5") {
		for _, field := range []string{"presence_penalty", "presencePenalty", "frequency_penalty", "frequencyPenalty", "stop"} {
			if gjson.GetBytes(out, field).Exists() {
				out, err = sjson.DeleteBytes(out, field)
				if err != nil {
					return nil, false, err
				}
			}
		}
	}
	out, err = sanitizeGrokResponsesUnsupportedFields(out)
	if err != nil {
		return nil, false, err
	}
	out, err = sanitizeGrokResponsesTools(out)
	if err != nil {
		return nil, false, err
	}
	return out, !bytes.Equal(out, body), nil
}

func sanitizeGrokResponsesUnsupportedFields(body []byte) ([]byte, error) {
	if !bytes.Contains(body, []byte(`"external_web_access"`)) {
		return body, nil
	}
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if !deleteGrokJSONFields(payload, grokResponsesUnsupportedRecursiveFields) {
		return body, nil
	}
	return json.Marshal(payload)
}

func deleteGrokJSONFields(value any, fields map[string]struct{}) bool {
	switch typed := value.(type) {
	case map[string]any:
		changed := false
		for field := range fields {
			if _, ok := typed[field]; ok {
				delete(typed, field)
				changed = true
			}
		}
		for _, child := range typed {
			if deleteGrokJSONFields(child, fields) {
				changed = true
			}
		}
		return changed
	case []any:
		changed := false
		for _, child := range typed {
			if deleteGrokJSONFields(child, fields) {
				changed = true
			}
		}
		return changed
	default:
		return false
	}
}

func sanitizeGrokResponsesTools(body []byte) ([]byte, error) {
	tools := gjson.GetBytes(body, "tools")
	if !tools.Exists() || !tools.IsArray() {
		return body, nil
	}
	rawTools := tools.Array()
	filteredTools := make([]json.RawMessage, 0, len(rawTools))
	for _, tool := range rawTools {
		toolType := strings.TrimSpace(tool.Get("type").String())
		if _, ok := grokResponsesSupportedToolTypes[toolType]; ok {
			filteredTools = append(filteredTools, json.RawMessage(tool.Raw))
		}
	}
	var err error
	if len(filteredTools) != len(rawTools) {
		if len(filteredTools) == 0 {
			body, err = sjson.DeleteBytes(body, "tools")
		} else {
			var encoded []byte
			encoded, err = json.Marshal(filteredTools)
			if err == nil {
				body, err = sjson.SetRawBytes(body, "tools", encoded)
			}
		}
		if err != nil {
			return nil, err
		}
	}
	toolChoice := gjson.GetBytes(body, "tool_choice")
	if toolChoice.Exists() && shouldDropGrokToolChoice(toolChoice, filteredTools) {
		body, err = sjson.DeleteBytes(body, "tool_choice")
		if err != nil {
			return nil, err
		}
	}
	return body, nil
}

func shouldDropGrokToolChoice(toolChoice gjson.Result, tools []json.RawMessage) bool {
	if len(tools) == 0 {
		return true
	}
	if !toolChoice.IsObject() {
		return false
	}
	choiceType := strings.TrimSpace(toolChoice.Get("type").String())
	if choiceType == "" {
		return false
	}
	if _, ok := grokResponsesSupportedToolTypes[choiceType]; !ok {
		return true
	}
	if choiceType != "function" {
		return false
	}
	choiceName := strings.TrimSpace(toolChoice.Get("name").String())
	if choiceName == "" {
		choiceName = strings.TrimSpace(toolChoice.Get("function.name").String())
	}
	if choiceName == "" {
		return false
	}
	for _, tool := range tools {
		var item struct {
			Type     string `json:"type"`
			Name     string `json:"name"`
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		}
		if err := json.Unmarshal(tool, &item); err != nil {
			continue
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = strings.TrimSpace(item.Function.Name)
		}
		if strings.TrimSpace(item.Type) == "function" && name == choiceName {
			return false
		}
	}
	return true
}
