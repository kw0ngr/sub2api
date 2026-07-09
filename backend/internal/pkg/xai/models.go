package xai

import (
	"net/url"
	"os"
	"strings"
)

const (
	DefaultBaseURL  = "https://api.x.ai/v1"
	DefaultTokenURL = "https://auth.x.ai/oauth2/token"
	DefaultClientID = "b1a00492-073a-47ea-816f-4c329264a828"
)

type Model struct {
	ID          string `json:"id"`
	Object      string `json:"object"`
	OwnedBy     string `json:"owned_by"`
	DisplayName string `json:"display_name,omitempty"`
}

var defaultModels = []Model{
	{ID: "grok-4.5", Object: "model", OwnedBy: "xai", DisplayName: "Grok 4.5"},
	{ID: "grok-4.3", Object: "model", OwnedBy: "xai", DisplayName: "Grok 4.3"},
	{ID: "grok-build-0.1", Object: "model", OwnedBy: "xai", DisplayName: "Grok Build 0.1"},
	{ID: "grok-composer-2.5-fast", Object: "model", OwnedBy: "xai", DisplayName: "Grok Composer 2.5 Fast"},
	{ID: "grok-4.20-0309-reasoning", Object: "model", OwnedBy: "xai", DisplayName: "Grok 4.20 Reasoning"},
	{ID: "grok-4.20-0309-non-reasoning", Object: "model", OwnedBy: "xai", DisplayName: "Grok 4.20 Non Reasoning"},
	{ID: "grok-4.20-multi-agent-0309", Object: "model", OwnedBy: "xai", DisplayName: "Grok 4.20 Multi Agent"},
}

func DefaultModels() []Model {
	out := make([]Model, len(defaultModels))
	copy(out, defaultModels)
	return out
}

func DefaultModelIDs() []string {
	models := DefaultModels()
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	return ids
}

func EffectiveTokenURL() string {
	if value := strings.TrimSpace(os.Getenv("XAI_OAUTH_TOKEN_URL")); value != "" {
		return value
	}
	return DefaultTokenURL
}

func EffectiveClientID(override string) string {
	if value := strings.TrimSpace(override); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("XAI_OAUTH_CLIENT_ID")); value != "" {
		return value
	}
	return DefaultClientID
}

func BuildRefreshForm(refreshToken string, clientID string) url.Values {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", EffectiveClientID(clientID))
	form.Set("refresh_token", strings.TrimSpace(refreshToken))
	return form
}
