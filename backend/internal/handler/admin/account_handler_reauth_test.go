package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type applyOAuthAdminService struct {
	*stubAdminService
	account      *service.Account
	updateInput  *service.UpdateAccountInput
	extraUpdates map[string]any
}

func (s *applyOAuthAdminService) GetAccount(_ context.Context, _ int64) (*service.Account, error) {
	copyAccount := *s.account
	return &copyAccount, nil
}

func (s *applyOAuthAdminService) UpdateAccount(_ context.Context, _ int64, input *service.UpdateAccountInput) (*service.Account, error) {
	s.updateInput = input
	s.account.Type = input.Type
	s.account.Credentials = input.Credentials
	copyAccount := *s.account
	return &copyAccount, nil
}

func (s *applyOAuthAdminService) UpdateAccountExtra(_ context.Context, _ int64, updates map[string]any) error {
	s.extraUpdates = make(map[string]any, len(updates))
	for key, value := range updates {
		s.extraUpdates[key] = value
		s.account.Extra[key] = value
	}
	return nil
}

func (s *applyOAuthAdminService) ClearAccountError(_ context.Context, id int64) (*service.Account, error) {
	s.clearedAccountErrIDs = append(s.clearedAccountErrIDs, id)
	copyAccount := *s.account
	return &copyAccount, nil
}

type reauthTokenInvalidator struct {
	accountIDs []int64
}

func (r *reauthTokenInvalidator) InvalidateToken(_ context.Context, account *service.Account) error {
	if account != nil {
		r.accountIDs = append(r.accountIDs, account.ID)
	}
	return nil
}

func TestAccountHandlerApplyOAuthCredentials_MergesExtraAndInvalidatesCachedToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminSvc := &applyOAuthAdminService{
		stubAdminService: newStubAdminService(),
		account: &service.Account{
			ID:       33,
			Platform: service.PlatformOpenAI,
			Type:     service.AccountTypeOAuth,
			Status:   service.StatusError,
			Extra: map[string]any{
				"codex_cli_only": true,
				"openai_oauth_responses_websockets_v2_mode": "ctx_pool",
			},
		},
	}
	invalidator := &reauthTokenInvalidator{}
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, invalidator)

	router := gin.New()
	router.POST("/api/v1/admin/accounts/:id/apply-oauth-credentials", handler.ApplyOAuthCredentials)

	body, err := json.Marshal(map[string]any{
		"type":        "oauth",
		"credentials": map[string]any{"access_token": "new-token"},
		"extra":       map[string]any{"account_uuid": "acct-new"},
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/33/apply-oauth-credentials", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, adminSvc.updateInput)
	require.Nil(t, adminSvc.updateInput.Extra, "reauthorization must not replace the full Extra object")
	require.Equal(t, map[string]any{"account_uuid": "acct-new"}, adminSvc.extraUpdates)
	require.Equal(t, true, adminSvc.account.Extra["codex_cli_only"])
	require.Equal(t, "ctx_pool", adminSvc.account.Extra["openai_oauth_responses_websockets_v2_mode"])
	require.Equal(t, []int64{33}, adminSvc.clearedAccountErrIDs)
	require.Equal(t, []int64{33}, invalidator.accountIDs)
}
