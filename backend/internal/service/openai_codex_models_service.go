package service

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
)

var chatgptCodexModelsURL = "https://chatgpt.com/backend-api/codex/models"

const codexModelsManifestBodyLimit int64 = 8 << 20

type CodexModelsManifest struct {
	Body        []byte
	ETag        string
	NotModified bool
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

	modelSpecs, hasAPIKeyAccount := localCodexModelSpecsForAccounts(accounts)
	if !hasAPIKeyAccount || len(modelSpecs) == 0 {
		return nil, ErrNoAvailableAccounts
	}
	body, err := buildCodexModelsManifestBody(modelSpecs, nil)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusInternalServerError, "OPENAI_CODEX_MODELS_LOCAL_MANIFEST_FAILED", "encode local Codex models manifest: %v", err)
	}

	etag := localCodexModelsBodyETag(body)
	if localCodexModelsETagMatches(ifNoneMatch, etag) {
		return &CodexModelsManifest{ETag: etag, NotModified: true}, nil
	}
	return &CodexModelsManifest{Body: body, ETag: etag}, nil
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
	etag := resp.Header.Get("ETag")
	if convertedBody, converted := convertOpenAIModelListToCodexManifestForAccount(body, account); converted {
		body = convertedBody
		etag = localCodexModelsBodyETag(body)
		if localCodexModelsETagMatches(ifNoneMatch, etag) {
			return &CodexModelsManifest{ETag: etag, NotModified: true}, nil
		}
	}
	return &CodexModelsManifest{Body: body, ETag: etag}, nil
}
