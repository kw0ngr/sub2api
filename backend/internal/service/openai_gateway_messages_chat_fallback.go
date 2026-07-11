package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// forwardAnthropicViaRawChatCompletions serves /v1/messages clients through an
// OpenAI-compatible upstream that only supports /v1/chat/completions.
func (s *OpenAIGatewayService) forwardAnthropicViaRawChatCompletions(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	defaultMappedModel string,
) (*OpenAIForwardResult, error) {
	startTime := time.Now()

	var anthropicReq apicompat.AnthropicRequest
	if err := json.Unmarshal(body, &anthropicReq); err != nil {
		writeAnthropicError(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return nil, fmt.Errorf("parse anthropic request: %w", err)
	}
	originalModel := strings.TrimSpace(anthropicReq.Model)
	if originalModel == "" {
		writeAnthropicError(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return nil, fmt.Errorf("missing model in request")
	}
	applyOpenAICompatModelNormalization(&anthropicReq)
	clientStream := anthropicReq.Stream

	responsesReq, err := apicompat.AnthropicToResponses(&anthropicReq)
	if err != nil {
		writeAnthropicError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return nil, fmt.Errorf("convert anthropic to responses: %w", err)
	}

	billingModel := resolveOpenAIForwardModel(account, anthropicReq.Model, defaultMappedModel)
	upstreamModel := normalizeOpenAIModelForUpstream(account, billingModel)
	responsesReq.Model = upstreamModel

	chatReq, err := apicompat.ResponsesToChatCompletionsRequest(responsesReq)
	if err != nil {
		writeAnthropicError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return nil, fmt.Errorf("convert responses to chat completions: %w", err)
	}
	chatReq.Stream = clientStream
	if clientStream {
		chatReq.StreamOptions = &apicompat.ChatStreamOptions{IncludeUsage: true}
	}

	reasoningEffort := extractOpenAIReasoningEffortFromBody(body, upstreamModel, billingModel, originalModel)
	serviceTier := extractOpenAIServiceTierFromBody(body)

	chatBody, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal chat completions request: %w", err)
	}
	if normalizedBody, normalized := NormalizeGLMOpenAIReasoningEffort(chatBody, upstreamModel); normalized {
		chatBody = normalizedBody
	}

	logger.L().Debug("openai messages: forwarding via chat completions bridge",
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
		return nil, fmt.Errorf("build chat completions request: %w", err)
	}

	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	setOpsUpstreamRequestBody(c, chatBody)
	resp, err := s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, false)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody, upstreamMsg := s.readOpenAIUpstreamError(resp)
		if foErr := s.failoverOpenAIUpstreamHTTPError(ctx, c, account, resp, respBody, upstreamMsg, upstreamModel); foErr != nil {
			return nil, foErr
		}
		return s.handleAnthropicErrorResponse(resp, c, account, billingModel)
	}

	if clientStream {
		return s.streamChatCompletionsAsAnthropic(c, resp, originalModel, billingModel, upstreamModel, reasoningEffort, serviceTier, startTime)
	}
	return s.bufferChatCompletionsAsAnthropic(c, resp, originalModel, billingModel, upstreamModel, reasoningEffort, serviceTier, startTime)
}

func (s *OpenAIGatewayService) bufferChatCompletionsAsAnthropic(
	c *gin.Context,
	resp *http.Response,
	originalModel string,
	billingModel string,
	upstreamModel string,
	reasoningEffort *string,
	serviceTier *string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")
	ccResp, usage, err := s.readCCUpstreamJSONResponse(c, resp, writeAnthropicError)
	if err != nil {
		return nil, err
	}
	responsesResp := apicompat.ChatCompletionsResponseToResponses(ccResp, originalModel, nil, false, nil)
	anthropicResp := apicompat.ResponsesToAnthropic(responsesResp, originalModel)

	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	c.JSON(http.StatusOK, anthropicResp)

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

func (s *OpenAIGatewayService) streamChatCompletionsAsAnthropic(
	c *gin.Context,
	resp *http.Response,
	originalModel string,
	billingModel string,
	upstreamModel string,
	reasoningEffort *string,
	serviceTier *string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")
	writeStreamHeaders := s.newStreamHeaderWriter(c, resp.Header)

	ccState := apicompat.NewChatCompletionsToResponsesStreamState(originalModel)
	anthropicState := apicompat.NewResponsesEventToAnthropicState()
	anthropicState.Model = originalModel
	clientDisconnected := false

	emitChunk := func(chunk *apicompat.ChatCompletionsChunk) {
		responsesEvents := apicompat.ChatCompletionsChunkToResponsesEvents(chunk, ccState)
		for _, rEvent := range responsesEvents {
			anthropicEvents := apicompat.ResponsesEventToAnthropicEvents(&rEvent, anthropicState)
			if clientDisconnected {
				continue
			}
			for _, aEvt := range anthropicEvents {
				sse, err := apicompat.ResponsesAnthropicEventToSSE(aEvt)
				if err != nil {
					continue
				}
				writeStreamHeaders()
				if _, err := fmt.Fprint(c.Writer, sse); err != nil {
					clientDisconnected = true
					break
				}
			}
		}
		if !clientDisconnected && len(responsesEvents) > 0 {
			c.Writer.Flush()
		}
	}

	scan := s.scanCCStream(resp, "openai messages chat fallback", requestID, startTime, emitChunk)
	usage := scan.Usage
	if scan.Err != nil {
		return &OpenAIForwardResult{
			RequestID:        requestID,
			Usage:            usage,
			Model:            originalModel,
			BillingModel:     billingModel,
			UpstreamModel:    upstreamModel,
			ReasoningEffort:  reasoningEffort,
			ServiceTier:      serviceTier,
			Stream:           true,
			Duration:         time.Since(startTime),
			FirstTokenMs:     scan.FirstTokenMs,
			ClientDisconnect: clientDisconnected,
		}, fmt.Errorf("stream usage incomplete: %w", scan.Err)
	}

	finalEvents := apicompat.FinalizeChatCompletionsResponsesStream(ccState)
	for _, rEvent := range finalEvents {
		if rEvent.Response != nil && rEvent.Response.Usage != nil {
			usage = copyOpenAIUsageFromResponsesUsage(rEvent.Response.Usage)
		}
		if clientDisconnected {
			continue
		}
		anthropicEvents := apicompat.ResponsesEventToAnthropicEvents(&rEvent, anthropicState)
		for _, aEvt := range anthropicEvents {
			sse, err := apicompat.ResponsesAnthropicEventToSSE(aEvt)
			if err != nil {
				continue
			}
			writeStreamHeaders()
			if _, err := fmt.Fprint(c.Writer, sse); err != nil {
				clientDisconnected = true
				break
			}
		}
	}
	if !clientDisconnected {
		c.Writer.Flush()
	}
	if !scan.SawDone {
		logCCStreamMissingDoneSentinel("openai messages chat fallback", requestID)
	}

	return &OpenAIForwardResult{
		RequestID:        requestID,
		Usage:            usage,
		Model:            originalModel,
		BillingModel:     billingModel,
		UpstreamModel:    upstreamModel,
		ReasoningEffort:  reasoningEffort,
		ServiceTier:      serviceTier,
		Stream:           true,
		Duration:         time.Since(startTime),
		FirstTokenMs:     scan.FirstTokenMs,
		ClientDisconnect: clientDisconnected,
	}, nil
}
