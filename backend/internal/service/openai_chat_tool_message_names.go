package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/tidwall/sjson"
)

func normalizeGrokThirdPartyChatToolMessageNames(account *Account, body []byte, chatReq *apicompat.ChatCompletionsRequest) ([]byte, bool, error) {
	if !isThirdPartyGrokAPIKey(account) {
		return body, false, nil
	}

	req := chatReq
	if req == nil {
		var parsed apicompat.ChatCompletionsRequest
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, false, fmt.Errorf("parse chat completions tool messages: %w", err)
		}
		req = &parsed
	}

	callNames := make(map[string]string)
	for _, msg := range req.Messages {
		for _, call := range msg.ToolCalls {
			callID := strings.TrimSpace(call.ID)
			name := strings.TrimSpace(call.Function.Name)
			if callID != "" && name != "" {
				callNames[callID] = name
			}
		}
	}

	fallbackName := firstChatToolName(req)
	if fallbackName == "" {
		fallbackName = "tool"
	}

	out := body
	modified := false
	for i, msg := range req.Messages {
		if strings.TrimSpace(msg.Role) != "tool" || strings.TrimSpace(msg.Name) != "" {
			continue
		}
		name := callNames[strings.TrimSpace(msg.ToolCallID)]
		if name == "" {
			name = fallbackName
		}
		var err error
		out, err = sjson.SetBytes(out, fmt.Sprintf("messages.%d.name", i), name)
		if err != nil {
			return nil, false, fmt.Errorf("set chat tool message name: %w", err)
		}
		modified = true
	}

	return out, modified, nil
}

func firstChatToolName(req *apicompat.ChatCompletionsRequest) string {
	if req == nil {
		return ""
	}
	for _, tool := range req.Tools {
		if tool.Function == nil {
			continue
		}
		if name := strings.TrimSpace(tool.Function.Name); name != "" {
			return name
		}
	}
	return ""
}
