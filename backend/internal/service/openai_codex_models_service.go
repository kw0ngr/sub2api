package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	openaiapi "github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

var chatgptCodexModelsURL = "https://chatgpt.com/backend-api/codex/models"

const codexModelsManifestBodyLimit int64 = 8 << 20

type CodexModelsManifest struct {
	Body        []byte
	ETag        string
	NotModified bool
}

type localCodexReasoningLevel struct {
	Effort      string `json:"effort"`
	Description string `json:"description"`
}

type localCodexModel struct {
	Slug                          string                     `json:"slug"`
	DisplayName                   string                     `json:"display_name,omitempty"`
	Description                   string                     `json:"description,omitempty"`
	DefaultReasoningLevel         string                     `json:"default_reasoning_level,omitempty"`
	SupportedReasoningLevels      []localCodexReasoningLevel `json:"supported_reasoning_levels,omitempty"`
	Visibility                    string                     `json:"visibility"`
	SupportedInAPI                bool                       `json:"supported_in_api"`
	Priority                      int                        `json:"priority"`
	ContextWindow                 int                        `json:"context_window,omitempty"`
	MaxContextWindow              int                        `json:"max_context_window,omitempty"`
	EffectiveContextWindowPercent int                        `json:"effective_context_window_percent,omitempty"`
	InputModalities               []string                   `json:"input_modalities,omitempty"`
}

func (s *OpenAIGatewayService) SelectCodexModelsAccount(ctx context.Context, groupID *int64) (*Account, error) {
	return s.SelectCodexModelsAccountWithExclusions(ctx, groupID, nil)
}

// SelectCodexModelsAccountWithExclusions selects an OAuth account that can
// serve ChatGPT's rich Codex manifest while allowing the handler to fail over
// account-local transport and upstream failures.
func (s *OpenAIGatewayService) SelectCodexModelsAccountWithExclusions(ctx context.Context, groupID *int64, excludedIDs map[int64]struct{}) (*Account, error) {
	accounts, err := s.listSchedulableAccounts(ctx, groupID, PlatformOpenAI)
	if err != nil {
		return nil, err
	}
	for i := range accounts {
		if _, excluded := excludedIDs[accounts[i].ID]; excluded {
			continue
		}
		account := s.resolveFreshSchedulableOpenAIAccountForPlatform(ctx, &accounts[i], "", PlatformOpenAI)
		if account == nil {
			continue
		}
		if account.IsOpenAIOAuth() && strings.TrimSpace(account.GetOpenAIAccessToken()) != "" {
			return account, nil
		}
	}
	return nil, ErrNoAvailableAccounts
}

// BuildLocalCodexModelsManifest builds an authenticated, group-scoped Codex
// manifest from schedulable API key account model mappings. Official OpenAI API
// keys cannot call ChatGPT's OAuth-only manifest endpoint, but their configured
// model mappings are still authoritative for gateway routing. Keeping this
// fallback local also prevents an OAuth-wide cooldown from making API-key
// models disappear from Codex clients.
func (s *OpenAIGatewayService) BuildLocalCodexModelsManifest(ctx context.Context, groupID *int64, ifNoneMatch string) (*CodexModelsManifest, error) {
	if s == nil || s.accountRepo == nil {
		return nil, ErrNoAvailableAccounts
	}

	var (
		accounts []Account
		err      error
	)
	if groupID != nil {
		accounts, err = s.accountRepo.ListSchedulableByGroupIDAndPlatform(ctx, *groupID, PlatformOpenAI)
	} else {
		accounts, err = s.accountRepo.ListSchedulableByPlatform(ctx, PlatformOpenAI)
	}
	if err != nil {
		return nil, err
	}

	modelSet := make(map[string]struct{})
	hasAPIKeyAccount := false
	hasExplicitMapping := false
	for i := range accounts {
		account := &accounts[i]
		if !account.IsOpenAIApiKey() {
			continue
		}
		hasAPIKeyAccount = true
		mapping := account.GetModelMapping()
		if len(mapping) == 0 {
			continue
		}
		for modelID := range mapping {
			if modelID = strings.TrimSpace(modelID); modelID != "" && !strings.Contains(modelID, "*") {
				modelSet[modelID] = struct{}{}
				hasExplicitMapping = true
			}
		}
	}
	if !hasAPIKeyAccount {
		return nil, ErrNoAvailableAccounts
	}
	if !hasExplicitMapping {
		for _, modelID := range openaiapi.DefaultModelIDs() {
			modelSet[modelID] = struct{}{}
		}
	}
	if len(modelSet) == 0 {
		return nil, ErrNoAvailableAccounts
	}

	modelIDs := make([]string, 0, len(modelSet))
	for modelID := range modelSet {
		modelIDs = append(modelIDs, modelID)
	}
	sortLocalCodexModelIDs(modelIDs)

	models := make([]localCodexModel, 0, len(modelIDs))
	for i, modelID := range modelIDs {
		models = append(models, buildLocalCodexModel(modelID, i+1))
	}
	body, err := json.Marshal(struct {
		Models []localCodexModel `json:"models"`
	}{Models: models})
	if err != nil {
		return nil, infraerrors.Newf(http.StatusInternalServerError, "OPENAI_CODEX_MODELS_LOCAL_MANIFEST_FAILED", "encode local Codex models manifest: %v", err)
	}

	digest := sha256.Sum256(body)
	etag := fmt.Sprintf(`W/"sub2api-%x"`, digest[:16])
	if localCodexModelsETagMatches(ifNoneMatch, etag) {
		return &CodexModelsManifest{ETag: etag, NotModified: true}, nil
	}
	return &CodexModelsManifest{Body: body, ETag: etag}, nil
}

func sortLocalCodexModelIDs(modelIDs []string) {
	preferred := make(map[string]int, len(openaiapi.DefaultModels))
	for i, model := range openaiapi.DefaultModels {
		preferred[model.ID] = i
	}
	sort.Slice(modelIDs, func(i, j int) bool {
		leftRank, leftPreferred := preferred[modelIDs[i]]
		rightRank, rightPreferred := preferred[modelIDs[j]]
		if leftPreferred != rightPreferred {
			return leftPreferred
		}
		if leftPreferred && leftRank != rightRank {
			return leftRank < rightRank
		}
		return modelIDs[i] < modelIDs[j]
	})
}

func buildLocalCodexModel(modelID string, priority int) localCodexModel {
	displayName := modelID
	for _, model := range openaiapi.DefaultModels {
		if model.ID == modelID {
			displayName = model.DisplayName
			break
		}
	}

	entry := localCodexModel{
		Slug:           modelID,
		DisplayName:    displayName,
		Visibility:     "list",
		SupportedInAPI: true,
		Priority:       priority,
	}
	baseModel := openAIBaseModelIDForEffortSupport(modelID)
	if baseModel == "" {
		baseModel = strings.ToLower(strings.TrimSpace(modelID))
	}

	switch {
	case openAIModelSupportsMaxReasoning(baseModel):
		entry.DefaultReasoningLevel = "medium"
		if baseModel == "gpt-5.6-sol" {
			entry.DefaultReasoningLevel = "low"
			entry.Description = "Latest frontier agentic coding model."
		}
		entry.SupportedReasoningLevels = localCodexReasoningLevels("low", "medium", "high", "xhigh", "max")
		entry.ContextWindow = 272000
		entry.MaxContextWindow = 1000000
		entry.EffectiveContextWindowPercent = 95
		entry.InputModalities = []string{"text", "image"}
	case strings.HasPrefix(baseModel, "gpt-5"):
		entry.DefaultReasoningLevel = "medium"
		entry.SupportedReasoningLevels = localCodexReasoningLevels("low", "medium", "high", "xhigh")
	case strings.HasPrefix(baseModel, "o1"), strings.HasPrefix(baseModel, "o3"), strings.HasPrefix(baseModel, "o4"):
		entry.DefaultReasoningLevel = "medium"
		entry.SupportedReasoningLevels = localCodexReasoningLevels("low", "medium", "high")
	}
	return entry
}

func localCodexReasoningLevels(efforts ...string) []localCodexReasoningLevel {
	descriptions := map[string]string{
		"low":    "Fast responses with lighter reasoning",
		"medium": "Balances speed and reasoning depth for everyday tasks",
		"high":   "Greater reasoning depth for complex problems",
		"xhigh":  "Extra high reasoning depth for complex problems",
		"max":    "Maximum reasoning depth for the hardest problems",
	}
	levels := make([]localCodexReasoningLevel, 0, len(efforts))
	for _, effort := range efforts {
		levels = append(levels, localCodexReasoningLevel{Effort: effort, Description: descriptions[effort]})
	}
	return levels
}

func localCodexModelsETagMatches(ifNoneMatch, etag string) bool {
	normalize := func(value string) string {
		value = strings.TrimSpace(value)
		if len(value) >= 2 && strings.EqualFold(value[:2], "W/") {
			value = strings.TrimSpace(value[2:])
		}
		return value
	}
	want := normalize(etag)
	if want == "" {
		return false
	}
	for _, candidate := range strings.Split(ifNoneMatch, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || normalize(candidate) == want {
			return true
		}
	}
	return false
}

func (s *OpenAIGatewayService) FetchCodexModelsManifest(ctx context.Context, account *Account, clientVersion, ifNoneMatch string) (*CodexModelsManifest, error) {
	if account == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "OPENAI_CODEX_MODELS_ACCOUNT_REQUIRED", "account is required")
	}
	accessToken := strings.TrimSpace(account.GetOpenAIAccessToken())
	if accessToken == "" {
		return nil, infraerrors.New(http.StatusBadGateway, "OPENAI_CODEX_MODELS_TOKEN_MISSING", "account has no Codex backend access token")
	}

	clientVersion = strings.TrimSpace(clientVersion)
	if clientVersion == "" {
		clientVersion = openAICodexProbeVersion
	}
	requestURL := chatgptCodexModelsURL + "?client_version=" + url.QueryEscape(clientVersion)

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusInternalServerError, "OPENAI_CODEX_MODELS_REQUEST_FAILED", "create codex models request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Originator", "codex_cli_rs")
	req.Header.Set("Version", clientVersion)
	req.Header.Set("User-Agent", codexCLIUserAgent)
	if ifNoneMatch = strings.TrimSpace(ifNoneMatch); ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	if chatgptAccountID := account.GetChatGPTAccountID(); chatgptAccountID != "" {
		req.Header.Set("chatgpt-account-id", chatgptAccountID)
	}

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	client, err := httpclient.GetClient(httpclient.Options{
		ProxyURL:              proxyURL,
		Timeout:               15 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	})
	if err != nil {
		return nil, infraerrors.Newf(http.StatusInternalServerError, "OPENAI_CODEX_MODELS_PROXY_INVALID", "invalid proxy configuration: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "OPENAI_CODEX_MODELS_UPSTREAM_FAILED", "codex models manifest request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotModified {
		return &CodexModelsManifest{ETag: resp.Header.Get("ETag"), NotModified: true}, nil
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = resp.Status
		}
		return nil, infraerrors.Newf(http.StatusBadGateway, "OPENAI_CODEX_MODELS_UPSTREAM_FAILED", "codex models manifest upstream error %d: %s", resp.StatusCode, message)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, codexModelsManifestBodyLimit))
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "OPENAI_CODEX_MODELS_UPSTREAM_FAILED", "read codex models manifest response: %v", err)
	}
	return &CodexModelsManifest{Body: body, ETag: resp.Header.Get("ETag")}, nil
}
