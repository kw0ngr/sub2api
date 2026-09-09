package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// testOpenAICompactConnection probes native remote compaction v2 through the
// existing account-test transport and persists its capability result in Extra.
func (s *AccountTestService) testOpenAICompactConnection(c *gin.Context, account *Account, testModelID string) error {
	ctx := c.Request.Context()
	authToken := ""
	apiURL := ""
	isOAuth := false

	switch {
	case account.IsOAuth():
		isOAuth = true
		authToken = account.GetOpenAIAccessToken()
		if authToken == "" {
			return s.sendErrorAndEnd(c, "No access token available")
		}
		apiURL = chatgptCodexAPIURL
	case account.Type == AccountTypeAPIKey:
		authToken = account.GetOpenAIApiKey()
		if authToken == "" {
			return s.sendErrorAndEnd(c, "No API key available")
		}
		baseURL := account.GetOpenAIBaseURL()
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		normalizedBaseURL, err := s.validateUpstreamBaseURL(baseURL)
		if err != nil {
			return s.sendErrorAndEnd(c, fmt.Sprintf("Invalid base URL: %s", err.Error()))
		}
		apiURL = buildOpenAIResponsesURL(normalizedBaseURL)
	default:
		return s.sendErrorAndEnd(c, fmt.Sprintf("Unsupported account type: %s", account.Type))
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	upstreamModelID := testModelID
	if isOAuth {
		upstreamModelID = normalizeOpenAIModelForUpstream(account, testModelID)
	}
	payloadBytes, err := json.Marshal(createOpenAICompactProbePayload(upstreamModelID, isOAuth))
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to create OpenAI compact probe payload")
	}
	s.sendEvent(c, TestEvent{Type: "test_start", Model: testModelID})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+authToken)
	ensureOpenAIRemoteCompactionV2BetaFeature(req.Header)
	probeSessionID := compactProbeSessionID(account.ID)
	req.Header.Set("session_id", probeSessionID)
	req.Header.Set("conversation_id", probeSessionID)

	if isOAuth {
		req.Host = "chatgpt.com"
		req.Header.Set("OpenAI-Beta", "responses=experimental")
		req.Header.Set("Originator", "codex_cli_rs")
		if customUA := account.GetOpenAIUserAgent(); customUA != "" {
			req.Header.Set("User-Agent", customUA)
		} else {
			req.Header.Set("User-Agent", codexCLIUserAgent)
		}
		if chatgptAccountID := account.GetChatGPTAccountID(); chatgptAccountID != "" {
			req.Header.Set("chatgpt-account-id", chatgptAccountID)
		}
	}

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, s.tlsFPProfileService.ResolveTLSProfile(account))
	if err != nil {
		s.persistOpenAICompactProbeUpdates(ctx, account, buildOpenAICompactProbeExtraUpdates(nil, nil, err, false, time.Now()))
		return s.sendErrorAndEnd(c, fmt.Sprintf("Request failed: %s", err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	compactionFound := readErr == nil && openAICompactProbeFoundCompactionItem(body)
	updates := buildOpenAICompactProbeExtraUpdates(resp, body, readErr, compactionFound, time.Now())
	if codexUpdates, snapshotErr := extractOpenAICodexProbeUpdates(resp); snapshotErr == nil {
		updates = mergeExtraUpdates(updates, codexUpdates)
	}
	s.persistOpenAICompactProbeUpdates(ctx, account, updates)

	if readErr != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Failed to read compact probe response: %s", readErr.Error()))
	}
	if resp.StatusCode != http.StatusOK {
		return s.sendErrorAndEnd(c, fmt.Sprintf("API returned %d: %s", resp.StatusCode, string(body)))
	}
	if !compactionFound {
		return s.sendErrorAndEnd(c, "Upstream returned 2xx without a compaction output item (native remote compaction v2 unsupported on this chain)")
	}

	s.sendEvent(c, TestEvent{Type: "content", Text: "Compact probe succeeded (native remote compaction v2)"})
	s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
	return nil
}

func (s *AccountTestService) persistOpenAICompactProbeUpdates(ctx context.Context, account *Account, updates map[string]any) {
	if s.accountRepo == nil || len(updates) == 0 {
		return
	}
	_ = s.accountRepo.UpdateExtra(ctx, account.ID, updates)
	mergeAccountExtra(account, updates)
}
