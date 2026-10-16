package service

import (
	"fmt"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/gin-gonic/gin"
)

// appendToLastUserMessage 在最后一条 user 消息 content 末尾追加文本
// （ladder 重试轮的策略注入点）。content 支持 string 与 parts 数组两形态。
func appendToLastUserMessage(body []byte, appendText string) ([]byte, error) {
	if appendText == "" {
		return body, nil
	}
	msgs := gjson.GetBytes(body, "messages").Array()
	lastUser := -1
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Get("role").String() == "user" {
			lastUser = i
			break
		}
	}
	if lastUser < 0 {
		return body, fmt.Errorf("no user message to append to")
	}
	path := fmt.Sprintf("messages.%d.content", lastUser)
	content := gjson.GetBytes(body, path)
	if content.Type == gjson.String {
		return sjson.SetBytes(body, path, content.String()+"\n\n"+appendText)
	}
	if content.IsArray() {
		// parts 数组：追加一个 text part
		newText := map[string]any{"type": "text", "text": appendText}
		return sjson.SetBytes(body, path+".-1", newText)
	}
	// 非常规形态：直接 string 化追加
	return sjson.SetBytes(body, path, content.String()+"\n\n"+appendText)
}

// getJailbreakSessionKey 从 gin context 提取会话 key（与 sticky session 同源）。
// 破限记忆按 分组+会话 隔离。
func getJailbreakSessionKey(c *gin.Context) string {
	if c == nil {
		return ""
	}
	v, ok := c.Get("jailbreak_session_key")
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}
