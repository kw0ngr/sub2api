package service

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	openAICompatibleFakeRateLimitPeekBytes   = 8 << 10
	openAICompatibleFakeRateLimitIDFreeBlock = "chatcmpl-freeblock"
	openAICompatibleFakeRateLimitIDFreeIPBlk = "chatcmpl-freeipblk"
	openAICompatibleFakeRateLimitText        = "当前网络的请求存在异常，已被临时限制访问"
)

// isOpenAICompatibleFakeRateLimitBody 识别兼容网关把 IP 封禁/风控页伪装成 HTTP 200 completion 的响应。
func isOpenAICompatibleFakeRateLimitBody(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	if bytes.Contains(body, []byte(openAICompatibleFakeRateLimitText)) ||
		(bytes.Contains(body, []byte("当前网络的请求存在异常")) && bytes.Contains(body, []byte("临时限制访问"))) {
		return true
	}
	if openAICompatibleFakeRateLimitID(gjson.GetBytes(body, "id").String()) {
		return true
	}
	for _, line := range bytes.Split(body, []byte("\n")) {
		payload, ok := extractOpenAISSEDataLine(string(bytes.TrimRight(line, "\r")))
		if !ok {
			continue
		}
		payload = strings.TrimSpace(payload)
		if payload == "" || payload == "[DONE]" {
			continue
		}
		if openAICompatibleFakeRateLimitID(gjson.Get(payload, "id").String()) {
			return true
		}
		if strings.Contains(payload, openAICompatibleFakeRateLimitText) {
			return true
		}
	}
	lower := strings.ToLower(string(body))
	return strings.Contains(lower, openAICompatibleFakeRateLimitIDFreeBlock) ||
		strings.Contains(lower, openAICompatibleFakeRateLimitIDFreeIPBlk)
}

func openAICompatibleFakeRateLimitID(id string) bool {
	id = strings.ToLower(strings.TrimSpace(id))
	return strings.HasPrefix(id, openAICompatibleFakeRateLimitIDFreeBlock) ||
		strings.HasPrefix(id, openAICompatibleFakeRateLimitIDFreeIPBlk)
}

func openAICompatibleFakeRateLimitMessage(body []byte) string {
	if bytes.Contains(body, []byte(openAICompatibleFakeRateLimitText)) {
		return openAICompatibleFakeRateLimitText
	}
	if msg := strings.TrimSpace(gjson.GetBytes(body, "choices.0.message.content").String()); strings.Contains(msg, "临时限制访问") {
		return msg
	}
	if msg := strings.TrimSpace(gjson.GetBytes(body, "choices.0.delta.content").String()); strings.Contains(msg, "临时限制访问") {
		return msg
	}
	return "upstream returned a fake success rate-limit page"
}

func peekHTTPResponseBody(resp *http.Response, n int) ([]byte, error) {
	if resp == nil || resp.Body == nil || n <= 0 {
		return nil, nil
	}
	orig := resp.Body
	buf := make([]byte, n)
	read, err := io.ReadFull(orig, buf)
	buf = buf[:read]
	switch {
	case err == io.EOF || err == io.ErrUnexpectedEOF:
		_ = orig.Close()
		resp.Body = io.NopCloser(bytes.NewReader(buf))
		return buf, nil
	case err != nil:
		return buf, err
	default:
		resp.Body = struct {
			io.Reader
			io.Closer
		}{
			Reader: io.MultiReader(bytes.NewReader(buf), orig),
			Closer: orig,
		}
		return buf, nil
	}
}

func (s *OpenAIGatewayService) failoverOpenAICompatibleFakeRateLimit(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	resp *http.Response,
	body []byte,
	upstreamModel string,
) *UpstreamFailoverError {
	if resp == nil || !isOpenAICompatibleFakeRateLimitBody(body) {
		return nil
	}
	originalStatus := resp.StatusCode
	resp.StatusCode = http.StatusTooManyRequests
	accountID := int64(0)
	platform := ""
	if account != nil {
		accountID = account.ID
		platform = account.Platform
	}
	slog.Warn("openai_compatible_fake_rate_limit",
		"account_id", accountID,
		"platform", platform,
		"original_status", originalStatus,
		"model", strings.TrimSpace(upstreamModel),
	)
	return s.failoverOpenAIUpstreamHTTPError(
		ctx,
		c,
		account,
		resp,
		body,
		openAICompatibleFakeRateLimitMessage(body),
		upstreamModel,
	)
}

func (s *OpenAIGatewayService) peekAndFailoverOpenAICompatibleFakeRateLimit(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	resp *http.Response,
	upstreamModel string,
) (*UpstreamFailoverError, error) {
	prefix, err := peekHTTPResponseBody(resp, openAICompatibleFakeRateLimitPeekBytes)
	if err != nil {
		return nil, err
	}
	return s.failoverOpenAICompatibleFakeRateLimit(ctx, c, account, resp, prefix, upstreamModel), nil
}
