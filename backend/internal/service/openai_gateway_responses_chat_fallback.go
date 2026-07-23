package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func shouldForwardResponsesViaChatCompletions(account *Account) bool {
	if account == nil || account.Type != AccountTypeAPIKey {
		return false
	}
	switch account.Platform {
	case PlatformOpenRouter, PlatformDeepSeek, PlatformGemini:
		return true
	case PlatformGLM:
		return account.IsGLMOpenAICompatible()
	case PlatformOpenAI:
		switch openai_compat.ResolveResponsesSupport(account.Extra) {
		case openai_compat.ResponsesSupportYes:
			return false
		case openai_compat.ResponsesSupportNo:
			return true
		default:
			// Unknown custom OpenAI-compatible accounts still start on the existing
			// HTTP /responses path so fork-local repair/retry logic (notably
			// invalid_encrypted_content cleanup) can run. They may fall back after an
			// endpoint-unsupported response; explicit probe/override false still uses
			// the bridge immediately via ResponsesSupportNo above.
			return false
		}
	case PlatformGrok:
		return false
	default:
		return false
	}
}

func shouldForwardAnthropicMessagesViaChatCompletions(account *Account) bool {
	if shouldForwardResponsesViaChatCompletions(account) {
		return true
	}
	return account != nil && account.Type == AccountTypeAPIKey && hasCustomNonOfficialOpenAIBaseURL(account)
}

func shouldRetryResponsesViaChatCompletionsAfterHTTPError(account *Account, statusCode int, upstreamMsg string, respBody []byte) bool {
	if account == nil || account.Type != AccountTypeAPIKey {
		return false
	}
	if openai_compat.ResolveResponsesSupport(account.Extra) != openai_compat.ResponsesSupportUnknown {
		return false
	}
	if !hasCustomNonOfficialOpenAIBaseURL(account) {
		return false
	}
	if isOpenAIModelNotFoundError(statusCode, upstreamMsg, respBody) {
		return false
	}
	return !isResponsesEndpointSupportedByStatus(statusCode)
}

func isOpenAIModelNotFoundError(statusCode int, upstreamMsg string, respBody []byte) bool {
	if statusCode != http.StatusNotFound {
		return false
	}
	code := strings.ToLower(strings.TrimSpace(extractUpstreamErrorCode(respBody)))
	if strings.Contains(code, "model") {
		return true
	}
	msg := strings.ToLower(strings.TrimSpace(upstreamMsg))
	return strings.Contains(msg, "model") && (strings.Contains(msg, "not found") ||
		strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "not supported") ||
		strings.Contains(msg, "unsupported"))
}

func isResponsesEndpointSupportedByStatus(status int) bool {
	switch status {
	case http.StatusNotFound, http.StatusMethodNotAllowed:
		return false
	default:
		return true
	}
}

func hasCustomNonOfficialOpenAIBaseURL(account *Account) bool {
	if account == nil || account.Platform != PlatformOpenAI {
		return false
	}
	raw := strings.TrimSpace(account.GetCredential("base_url"))
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return !strings.EqualFold(u.Hostname(), "api.openai.com")
}

// forwardResponsesViaRawChatCompletions serves /v1/responses clients through an
// upstream that only supports /v1/chat/completions. It is intentionally wired
// after account selection so existing account-pool, model mapping, failover,
// and billing paths keep working.
func (s *OpenAIGatewayService) forwardResponsesViaRawChatCompletions(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
) (*OpenAIForwardResult, error) {
	startTime := time.Now()

	var responsesReq apicompat.ResponsesRequest
	if err := json.Unmarshal(body, &responsesReq); err != nil {
		writeOpenAIResponsesFallbackError(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return nil, fmt.Errorf("parse responses request: %w", err)
	}
	originalModel := strings.TrimSpace(responsesReq.Model)
	if originalModel == "" {
		writeOpenAIResponsesFallbackError(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return nil, fmt.Errorf("missing model in request")
	}

	clientStream := responsesReq.Stream
	serviceTier := extractOpenAIServiceTierFromBody(body)
	customTools := apicompat.CustomToolNames(responsesReq.Tools)
	toolSearch := apicompat.HasToolSearchTool(responsesReq.Tools)
	namespaceTools := apicompat.NamespaceToolNames(responsesReq.Tools)

	chatReq, err := apicompat.ResponsesToChatCompletionsRequest(&responsesReq)
	if err != nil {
		writeOpenAIResponsesFallbackError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return nil, fmt.Errorf("convert responses to chat completions: %w", err)
	}

	billingModel := resolveOpenAIForwardModel(account, originalModel, "")
	upstreamModel := normalizeOpenAIModelForUpstream(account, billingModel)
	reasoningEffort := extractEffectiveOpenAIReasoningEffortFromBody(account, body, upstreamModel, billingModel, originalModel)
	chatReq.Model = upstreamModel
	chatReq.Stream = clientStream
	if clientStream {
		chatReq.StreamOptions = &apicompat.ChatStreamOptions{IncludeUsage: true}
	}

	chatBody, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal chat completions fallback request: %w", err)
	}
	if normalizedBody, normalized := NormalizeGLMOpenAIReasoningEffort(chatBody, upstreamModel); normalized {
		chatBody = normalizedBody
	}
	chatBody, err = s.applyOpenAIFastPolicyToBody(ctx, account, upstreamModel, chatBody)
	if err != nil {
		var blocked *OpenAIFastBlockedError
		if errors.As(err, &blocked) {
			writeOpenAIFastPolicyBlockedResponse(c, blocked)
		}
		return nil, err
	}
	if serviceTier == nil {
		serviceTier = extractOpenAIServiceTierFromBody(chatBody)
	}

	logger.L().Debug("openai responses: forwarding via chat completions bridge",
		zap.Int64("account_id", account.ID),
		zap.String("platform", account.Platform),
		zap.String("original_model", originalModel),
		zap.String("billing_model", billingModel),
		zap.String("upstream_model", upstreamModel),
		zap.Bool("stream", clientStream),
	)

	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, err
	}
	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	upstreamReq, err := s.buildUpstreamRequestOpenAICompatibleChatCompletions(upstreamCtx, c, account, chatBody, token)
	releaseUpstreamCtx()
	if err != nil {
		return nil, fmt.Errorf("build chat completions fallback request: %w", err)
	}

	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	setOpsUpstreamRequestBody(c, chatBody)
	resp, err := s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, false, upstreamModel)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody, upstreamMsg := s.readOpenAIUpstreamError(resp)
		if foErr := s.failoverOpenAIUpstreamHTTPError(ctx, c, account, resp, respBody, upstreamMsg, upstreamModel); foErr != nil {
			return nil, foErr
		}
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
		return s.handleErrorResponse(ctx, resp, c, account, chatBody, billingModel)
	}

	if clientStream {
		return s.streamChatCompletionsAsResponses(c, resp, originalModel, customTools, toolSearch, namespaceTools, billingModel, upstreamModel, reasoningEffort, serviceTier, startTime)
	}
	return s.bufferChatCompletionsAsResponses(c, resp, originalModel, customTools, toolSearch, namespaceTools, billingModel, upstreamModel, reasoningEffort, serviceTier, startTime)
}

func (s *OpenAIGatewayService) bufferChatCompletionsAsResponses(
	c *gin.Context,
	resp *http.Response,
	originalModel string,
	customTools map[string]bool,
	toolSearch bool,
	namespaceTools map[string]apicompat.NamespacedToolName,
	billingModel string,
	upstreamModel string,
	reasoningEffort *string,
	serviceTier *string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")
	ccResp, usage, err := s.readCCUpstreamJSONResponse(c, resp, writeOpenAIResponsesFallbackError)
	if err != nil {
		return nil, err
	}
	responsesResp := apicompat.ChatCompletionsResponseToResponses(ccResp, originalModel, customTools, toolSearch, namespaceTools)

	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	c.JSON(http.StatusOK, responsesResp)

	return &OpenAIForwardResult{
		RequestID:       requestID,
		Usage:           usage,
		Model:           originalModel,
		BillingModel:    billingModel,
		UpstreamModel:   upstreamModel,
		ReasoningEffort: reasoningEffort,
		ServiceTier:     serviceTier,
		Stream:          false,
		Duration:        time.Since(startTime),
	}, nil
}

func (s *OpenAIGatewayService) streamChatCompletionsAsResponses(
	c *gin.Context,
	resp *http.Response,
	originalModel string,
	customTools map[string]bool,
	toolSearch bool,
	namespaceTools map[string]apicompat.NamespacedToolName,
	billingModel string,
	upstreamModel string,
	reasoningEffort *string,
	serviceTier *string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")
	writeStreamHeaders := s.newStreamHeaderWriter(c, resp.Header)

	state := apicompat.NewChatCompletionsToResponsesStreamState(originalModel)
	state.CustomTools = customTools
	state.ToolSearchDeclared = toolSearch
	state.NamespaceTools = namespaceTools
	clientDisconnected := false

	writeEvents := func(events []apicompat.ResponsesStreamEvent) {
		if clientDisconnected || len(events) == 0 {
			return
		}
		writeStreamHeaders()
		for _, event := range events {
			sse, err := apicompat.ResponsesEventToSSE(event)
			if err != nil {
				logger.L().Warn("openai responses chat fallback: failed to marshal stream event",
					zap.Error(err),
					zap.String("request_id", requestID),
				)
				continue
			}
			if _, err := fmt.Fprint(c.Writer, sse); err != nil {
				clientDisconnected = true
				logger.L().Debug("openai responses chat fallback: client disconnected, continuing to drain upstream for billing",
					zap.Error(err),
					zap.String("request_id", requestID),
				)
				return
			}
		}
		c.Writer.Flush()
	}

	scan := s.scanCCStream(resp, "openai responses chat fallback", requestID, startTime, func(chunk *apicompat.ChatCompletionsChunk) {
		writeEvents(apicompat.ChatCompletionsChunkToResponsesEvents(chunk, state))
	})

	if scan.Err != nil {
		return &OpenAIForwardResult{
			RequestID:       requestID,
			Usage:           scan.Usage,
			Model:           originalModel,
			BillingModel:    billingModel,
			UpstreamModel:   upstreamModel,
			ReasoningEffort: reasoningEffort,
			ServiceTier:     serviceTier,
			Stream:          true,
			Duration:        time.Since(startTime),
			FirstTokenMs:    scan.FirstTokenMs,
		}, fmt.Errorf("stream usage incomplete: %w", scan.Err)
	}

	writeEvents(apicompat.FinalizeChatCompletionsResponsesStream(state))
	if !clientDisconnected {
		writeStreamHeaders()
		if _, err := fmt.Fprint(c.Writer, "data: [DONE]\n\n"); err != nil {
			clientDisconnected = true
		}
		if !clientDisconnected {
			c.Writer.Flush()
		}
	}
	if !scan.SawDone {
		logCCStreamMissingDoneSentinel("openai responses chat fallback", requestID)
	}

	return &OpenAIForwardResult{
		RequestID:       requestID,
		Usage:           scan.Usage,
		Model:           originalModel,
		BillingModel:    billingModel,
		UpstreamModel:   upstreamModel,
		ReasoningEffort: reasoningEffort,
		ServiceTier:     serviceTier,
		Stream:          true,
		Duration:        time.Since(startTime),
		FirstTokenMs:    scan.FirstTokenMs,
	}, nil
}
