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
	accounts, err := s.listSchedulableAccounts(ctx, groupID, PlatformOpenAI)
	if err != nil {
		return nil, err
	}
	for i := range accounts {
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
