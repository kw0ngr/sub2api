package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	OpsOpenAICyberRedactedKey = "ops_openai_cyber_redacted"
	OpenAICyberPolicyCode     = "cyber_policy"
	OpenAICyberPolicyMessage  = "Request rejected by the upstream cybersecurity policy."
)

var errOpenAICyberPolicyRedacted = errors.New(OpenAICyberPolicyCode + ": " + OpenAICyberPolicyMessage)

// redactOpenAICyberPolicyResponse terminates an explicit provider denial before
// the generic error handlers can retry it or mark the selected account unhealthy.
func (s *OpenAIGatewayService) redactOpenAICyberPolicyResponse(c *gin.Context, account *Account, status int, payload []byte) error {
	if account == nil || account.Platform != PlatformOpenAI || s.settingService == nil || c == nil || c.Request == nil {
		return nil
	}
	if gjson.GetBytes(payload, "response.error.code").String() != OpenAICyberPolicyCode &&
		gjson.GetBytes(payload, "error.code").String() != OpenAICyberPolicyCode &&
		!((status >= 400 || gjson.GetBytes(payload, "type").String() == "error") && gjson.GetBytes(payload, "code").String() == OpenAICyberPolicyCode) {
		return nil
	}
	if !gjson.ValidBytes(payload) || !s.settingService.IsOpenAICyberSafetyRedactEnabled(c.Request.Context()) {
		return nil
	}

	StopOpenAICompactSSEKeepaliveCommitted(c)
	MarkResponseCommitted(c)
	c.Set(OpsOpenAICyberRedactedKey, true)
	body := gin.H{"error": gin.H{"type": "permission_error", "code": OpenAICyberPolicyCode, "message": OpenAICyberPolicyMessage}}
	path := strings.TrimRight(c.Request.URL.Path, "/")
	if strings.HasSuffix(path, "/messages") {
		body["type"] = "error"
	}
	if !c.Writer.Written() {
		c.Writer.Header().Del("Content-Length")
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.JSON(http.StatusForbidden, body)
	} else if strings.HasSuffix(path, "/responses") || strings.Contains(path, "/responses/") {
		writeOpenAISSEFailureMessage(c, status, OpenAICyberPolicyCode, OpenAICyberPolicyMessage, payload)
	} else {
		data, _ := json.Marshal(body)
		_, _ = fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n", data)
		c.Writer.Flush()
	}
	setOpsUpstreamError(c, status, OpenAICyberPolicyMessage, "")
	return errOpenAICyberPolicyRedacted
}
