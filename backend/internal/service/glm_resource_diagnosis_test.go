//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGLMUnavailableDiagnosisRespectsProtocolAndOtherHealthyRoutes(t *testing.T) {
	groupID := int64(19)
	openai := Account{ID: 2159, Platform: PlatformGLM, Type: AccountTypeAPIKey, Status: StatusError, Schedulable: false,
		ErrorMessage:  "GLM endpoint resource unavailable (1113): 余额不足",
		AccountGroups: []AccountGroup{{GroupID: groupID}},
		Credentials:   map[string]any{"compat_mode": GLMCompatModeOpenAI, "model_mapping": map[string]any{"glm-5.3": "glm-5.3"}}}
	anthropic := Account{ID: 1784, Platform: PlatformGLM, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		AccountGroups: []AccountGroup{{GroupID: groupID}},
		Credentials:   map[string]any{"compat_mode": GLMCompatModeAnthropic, "model_mapping": map[string]any{"glm-5.3": "glm-5.3"}}}
	repo := &mockAccountRepoForPlatform{accounts: []Account{openai, anthropic}}
	svc := &OpenAIGatewayService{accountRepo: repo, cfg: testConfig()}
	diag := svc.DiagnoseModelAvailabilityForPlatform(context.Background(), &groupID, "glm-5.3", PlatformGLM)
	require.True(t, diag.HasModelSupport)
	require.True(t, diag.ResourceUnavailable, "Anthropic capability must not hide the OpenAI endpoint outage")
	native := (&GatewayService{accountRepo: repo, cfg: testConfig()}).DiagnoseModelAvailabilityForPlatform(context.Background(), &groupID, "glm-5.3", PlatformGLM)
	require.True(t, native.HasModelSupport)
	require.False(t, native.ResourceUnavailable)
	openai.Status, openai.Schedulable, openai.ErrorMessage = StatusActive, true, ""
	repo.accounts = append(repo.accounts, openai)
	diag = svc.DiagnoseModelAvailabilityForPlatform(context.Background(), &groupID, "glm-5.3", PlatformGLM)
	require.False(t, diag.ResourceUnavailable, "an independent healthy OpenAI endpoint remains available")
}

func TestOpsPersistsGLMProviderCodeFromActualAttempt(t *testing.T) {
	var saved *OpsInsertErrorLogInput
	repo := &opsRepoMock{InsertErrorLogFn: func(_ context.Context, input *OpsInsertErrorLogInput) (int64, error) {
		saved = input
		return 1, nil
	}}
	svc := NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	entry := &OpsInsertErrorLogInput{ErrorPhase: "upstream", ErrorSource: "upstream_http",
		UpstreamErrors: []*OpsUpstreamErrorEvent{{UpstreamStatusCode: 429, Platform: PlatformGLM,
			Detail: `{"error":{"code":"1113","message":"余额不足","api_key":"secret"}}`}}}
	require.NoError(t, svc.RecordError(context.Background(), entry, nil))
	require.NotNil(t, saved.ProviderErrorCode)
	require.Equal(t, "1113", *saved.ProviderErrorCode)
	require.NotContains(t, *saved.UpstreamErrorsJSON, "secret")
	local := &OpsInsertErrorLogInput{ErrorPhase: "routing", ErrorSource: "gateway",
		ErrorBody: `{"error":{"code":"local_queue_full"}}`}
	require.NoError(t, svc.RecordError(context.Background(), local, nil))
	require.Nil(t, saved.ProviderErrorCode)
}
