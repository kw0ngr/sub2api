//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrokAPIKeyHealthRequiresCompletedInference(t *testing.T) {
	for _, body := range []string{
		`{"data":[{"id":"grok-4.5"}]}`,
		`{"status":"incomplete"}`,
		`{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"hi"}]}]}`,
	} {
		t.Run(body, func(t *testing.T) {
			upstream := &glmProbeHTTPUpstream{responses: []*http.Response{{
				StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)),
			}}}
			svc := &AccountTestService{httpUpstream: upstream}
			account := &Account{Platform: PlatformGrok, Type: AccountTypeAPIKey,
				Credentials: map[string]any{
					"api_key": "xai-test", "base_url": "https://cli-chat-proxy.grok.com/v1",
					"headers": map[string]any{"x-grok-client-version": "0.2.93"},
				}}
			health, err := svc.CheckAPIKeyValidity(context.Background(), account)
			require.NoError(t, err)
			require.Equal(t, strings.Contains(body, `"completed"`), health.Valid)
			require.False(t, health.Invalid)
			require.Equal(t, http.MethodPost, upstream.requests[0].Method)
			require.Equal(t, "/v1/responses", upstream.requests[0].URL.Path)
			require.Equal(t, "0.2.93", upstream.requests[0].Header.Get("x-grok-client-version"))
		})
	}
}

type importAccountRepo struct {
	AccountRepository
	created *Account
}

func (r *importAccountRepo) Create(_ context.Context, a *Account) error {
	r.created = a
	return nil
}

func TestCreateAccountPreservesInitialSchedulingState(t *testing.T) {
	disabled, enabled := false, true
	for _, state := range []*bool{nil, &disabled, &enabled} {
		repo := &importAccountRepo{}
		svc := &adminServiceImpl{accountRepo: repo}
		a, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
			Platform: PlatformGemini, Type: AccountTypeAPIKey,
			SkipDefaultGroupBind: true, Schedulable: state,
		})
		require.NoError(t, err)
		require.Equal(t, state == nil || *state, a.Schedulable)
		require.Equal(t, a.Schedulable, repo.created.Schedulable, "quarantine must be persisted before group binding")
	}
}

func TestOpenRouterCreditsCannotRestoreKeyRejectedByInference(t *testing.T) {
	upstream := &queuedHTTPUpstream{responses: []*http.Response{
		newJSONResponse(200, `{"data":{"total_credits":20,"total_usage":0}}`),
		newJSONResponse(401, `{"error":{"message":"User not found."}}`),
	}}
	svc := &AccountTestService{httpUpstream: upstream}
	health, err := svc.CheckAPIKeyValidity(context.Background(), &Account{
		Platform: PlatformOpenRouter, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-or-test"},
	})
	require.NoError(t, err)
	require.False(t, health.Valid)
	require.True(t, health.Invalid)
	require.Equal(t, 401, health.StatusCode)
	require.NotNil(t, health.ProbeQuota)
}

func TestDeepSeekHealthUsesAvailabilityAndActualInference(t *testing.T) {
	for _, available := range []bool{true, false} {
		body := `{"is_available":true,"balance_infos":[{"total_balance":"0.20","currency":"CNY"}]}`
		if !available {
			body = `{"is_available":false,"balance_infos":[{"total_balance":"2","currency":"CNY"}]}`
		}
		upstream := &queuedHTTPUpstream{responses: []*http.Response{
			newJSONResponse(200, body),
			newJSONResponse(200, `{"choices":[{"message":{"content":"hi"}}]}`),
		}}
		svc := &AccountTestService{httpUpstream: upstream}
		health, err := svc.CheckAPIKeyValidity(context.Background(), &Account{
			Platform: PlatformDeepSeek, Type: AccountTypeAPIKey,
			Credentials: map[string]any{"api_key": "sk-test"},
		})
		require.NoError(t, err)
		require.Equal(t, available, health.Valid)
		require.Equal(t, !available, health.Invalid)
	}
}

func TestAPIKeyHealthKeepsDepletedBalancesDisabled(t *testing.T) {
	for _, platform := range []string{PlatformOpenRouter, PlatformDeepSeek} {
		for _, balance := range []string{"0", "-0.01"} {
			body := `{"data":{"total_credits":` + balance + `,"total_usage":0}}`
			if platform == PlatformDeepSeek {
				body = `{"balance_infos":[{"total_balance":"` + balance + `","currency":"USD"}]}`
			}
			svc := &AccountTestService{httpUpstream: &queuedHTTPUpstream{responses: []*http.Response{
				newJSONResponse(200, body),
			}}}
			health, err := svc.CheckAPIKeyValidity(context.Background(), &Account{
				Platform: platform, Type: AccountTypeAPIKey,
				Credentials: map[string]any{"api_key": "sk-test"},
			})
			require.NoError(t, err)
			require.False(t, health.Valid)
			require.True(t, health.Invalid)
		}
	}
}
