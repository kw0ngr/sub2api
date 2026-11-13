//go:build unit

package service

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestCurrentClaudePreservesSignedThinkingAndEffort(t *testing.T) {
	for _, model := range []string{"claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5", "claude-fable-5-2", "claude-mythos-5-2"} {
		body := []byte(`{"model":"` + model + `","output_config":{"effort":"xhigh"},"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"","signature":"upstream-signature"},{"type":"text","text":"progress"}]}]}`)
		require.Equal(t, body, FilterThinkingBlocksForModel(body, model))
		require.Equal(t, body, FilterThinkingBlocksForRetryModel(body, model))
		require.Equal(t, "xhigh", gjson.GetBytes(NormalizeClaudeOutputEffortInBody(body), "output_config.effort").String())
	}
}

func TestCurrentClaudeAliasValidationReturns400WithoutUpstream(t *testing.T) {
	for _, route := range deepSeekCompatTestRoutes() {
		body := strings.ReplaceAll(route.body, "deepseek-v4-flash", "client-alias")
		body = strings.ReplaceAll(body, `"max"`, `"none"`)
		account := &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Credentials: map[string]any{
			"model_mapping": map[string]any{"client-alias": "claude-fable-5-2"},
		}}
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		result, err := route.forward(&GatewayService{}, context.Background(), c, account, []byte(body), &ParsedRequest{})
		require.Error(t, err)
		require.Nil(t, result)
		require.Equal(t, 400, rec.Code)
		require.Contains(t, rec.Body.String(), "reasoning effort")
	}
}
