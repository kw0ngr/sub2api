package service

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleChatCompletionsErrorResponseOrFailover 统一的 4xx/5xx 出口：
// 可 failover 的错误返回 UpstreamFailoverError（handler 层切号重试），
// 否则透传上游错误响应给客户端。ladder 循环与常规路径共用。
func (s *OpenAIGatewayService) handleChatCompletionsErrorResponseOrFailover(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	upstreamModel string,
	resp *http.Response,
	respBody []byte,
) (*OpenAIForwardResult, error) {
	upstreamMsg := sanitizeUpstreamErrorMessage(extractUpstreamErrorMessage(respBody))
	if s.shouldFailoverOpenAIUpstreamResponseForAccount(account, resp.StatusCode, upstreamMsg, respBody, upstreamModel) {
		upstreamDetail := ""
		if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
			maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
			if maxBytes <= 0 {
				maxBytes = 2048
			}
			upstreamDetail = truncateString(string(respBody), maxBytes)
		}
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: resp.StatusCode,
			UpstreamRequestID:  resp.Header.Get("x-request-id"),
			Kind:               "failover",
			Message:            upstreamMsg,
			Detail:             upstreamDetail,
		})
		if s.rateLimitService != nil {
			s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody, upstreamModel)
		}
		_ = resp.Body.Close()
		return nil, &UpstreamFailoverError{
			StatusCode:             resp.StatusCode,
			ResponseBody:           respBody,
			ResponseHeaders:        resp.Header.Clone(),
			RetryableOnSameAccount: account.IsPoolMode() && isPoolModeRetryableStatus(resp.StatusCode),
		}
	}
	// body 已重置为 NopCloser(bytes.NewReader(respBody))；handleChatCompletionsErrorResponse 内部会消费
	return s.handleChatCompletionsErrorResponse(resp, c, account, upstreamModel)
}
