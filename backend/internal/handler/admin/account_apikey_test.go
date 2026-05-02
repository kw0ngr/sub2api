package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func decodeAdminResponseData[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var envelope response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	raw, err := json.Marshal(envelope.Data)
	require.NoError(t, err)
	var out T
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

func newJSONTestContext(method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	return c, rec
}

func TestParseRawAPIKeyImportLines(t *testing.T) {
	total, lines, results, err := parseRawAPIKeyImportLines(`
# comment
sk-proj-123
sk-ant-456,https://api.anthropic.com
AIzaSy789
sk-or-v1-abc
deepseek,sk-deepseek-123
openrouter,sk-or-v1-custom,https://openrouter.ai/api/v1
bad-key
`)
	require.NoError(t, err)
	require.Equal(t, 7, total)
	require.Len(t, lines, 6)
	require.Len(t, results, 1)
	require.Equal(t, service.PlatformOpenAI, lines[0].Platform)
	require.Equal(t, service.PlatformAnthropic, lines[1].Platform)
	require.Equal(t, service.PlatformGemini, lines[2].Platform)
	require.Equal(t, service.PlatformOpenRouter, lines[3].Platform)
	require.Equal(t, service.PlatformDeepSeek, lines[4].Platform)
	require.Equal(t, service.PlatformOpenRouter, lines[5].Platform)
	require.Equal(t, "https://openrouter.ai/api/v1", lines[5].BaseURL)
	require.Contains(t, results[0].Error, "could not detect platform")
}

func TestParseRawAPIKeyImportLines_DetectsProviderFromBaseURL(t *testing.T) {
	total, lines, results, err := parseRawAPIKeyImportLines(`
sk-deepseek-compatible,https://api.deepseek.com
sk-openrouter-compatible,https://openrouter.ai/api/v1
`)
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Empty(t, results)
	require.Len(t, lines, 2)
	require.Equal(t, service.PlatformDeepSeek, lines[0].Platform)
	require.Equal(t, "https://api.deepseek.com", lines[0].BaseURL)
	require.Equal(t, service.PlatformOpenRouter, lines[1].Platform)
	require.Equal(t, "https://openrouter.ai/api/v1", lines[1].BaseURL)
}

func TestBuildAPIKeyIdentityUsesDefaultBaseURL(t *testing.T) {
	a := buildAPIKeyIdentity(service.PlatformOpenAI, "sk-proj-1", "")
	b := buildAPIKeyIdentity(service.PlatformOpenAI, "sk-proj-1", "https://api.openai.com/")
	require.Equal(t, a, b)
}

func TestBuildAPIKeyIdentityDoesNotExposeRawKey(t *testing.T) {
	identity := buildAPIKeyIdentity(service.PlatformOpenAI, "sk-proj-secret-123456", "https://api.openai.com/")
	require.NotContains(t, identity, "sk-proj-secret-123456")
	require.NotContains(t, identity, "api.openai.com")
	require.Len(t, identity, 64)
}

func TestDisableInvalidAPIKeyAccount_DisablesSchedulingWhenEnabled(t *testing.T) {
	adminSvc := newStubAdminService()
	handler := &AccountHandler{adminService: adminSvc}
	account := &service.Account{
		ID:          42,
		Platform:    service.PlatformOpenAI,
		Schedulable: true,
	}

	err := handler.disableInvalidAPIKeyAccount(context.Background(), account, "invalid api key")
	require.NoError(t, err)
	require.Len(t, adminSvc.setAccountErrCalls, 1)
	require.Equal(t, int64(42), adminSvc.setAccountErrCalls[0].id)
	require.Len(t, adminSvc.setSchedulableCalls, 1)
	require.Equal(t, int64(42), adminSvc.setSchedulableCalls[0].id)
	require.False(t, adminSvc.setSchedulableCalls[0].schedulable)
}

func TestDisableInvalidAPIKeyAccount_SkipsSchedulingUpdateWhenAlreadyDisabled(t *testing.T) {
	adminSvc := newStubAdminService()
	handler := &AccountHandler{adminService: adminSvc}
	account := &service.Account{
		ID:          43,
		Platform:    service.PlatformAnthropic,
		Schedulable: false,
	}

	err := handler.disableInvalidAPIKeyAccount(context.Background(), account, "invalid x-api-key")
	require.NoError(t, err)
	require.Len(t, adminSvc.setAccountErrCalls, 1)
	require.Equal(t, int64(43), adminSvc.setAccountErrCalls[0].id)
	require.Empty(t, adminSvc.setSchedulableCalls)
}

func TestRecoverValidAPIKeyAccount_ClearsErrorAndEnablesScheduling(t *testing.T) {
	// When health check confirms the key is valid via real chat completions,
	// status=error accounts should be fully recovered.
	adminSvc := newStubAdminService()
	handler := &AccountHandler{adminService: adminSvc}
	account := &service.Account{
		ID:          44,
		Status:      service.StatusError,
		Schedulable: false,
	}

	err := handler.recoverValidAPIKeyAccount(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, []int64{44}, adminSvc.clearedAccountErrIDs)
	require.Len(t, adminSvc.setSchedulableCalls, 1)
	require.Equal(t, int64(44), adminSvc.setSchedulableCalls[0].id)
	require.True(t, adminSvc.setSchedulableCalls[0].schedulable)
}

func TestRecoverValidAPIKeyAccount_EnablesSchedulingWithoutClearingActiveAccount(t *testing.T) {
	adminSvc := newStubAdminService()
	handler := &AccountHandler{adminService: adminSvc}
	account := &service.Account{
		ID:          45,
		Status:      service.StatusActive,
		Schedulable: false,
	}

	err := handler.recoverValidAPIKeyAccount(context.Background(), account)
	require.NoError(t, err)
	require.Empty(t, adminSvc.clearedAccountErrIDs)
	require.Len(t, adminSvc.setSchedulableCalls, 1)
	require.Equal(t, int64(45), adminSvc.setSchedulableCalls[0].id)
	require.True(t, adminSvc.setSchedulableCalls[0].schedulable)
}

func TestCheckAPIKeysHealth_StartAndStatusIncludeJobIDAndCompletion(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{{
		ID:          101,
		Name:        "openai-key",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
	}}
	handler := &AccountHandler{
		adminService:       adminSvc,
		accountTestService: &service.AccountTestService{},
	}

	c, rec := newJSONTestContext(http.MethodPost, "/admin/accounts/apikey-health-check", `{}`)
	handler.CheckAPIKeysHealth(c)
	require.Equal(t, http.StatusAccepted, rec.Code)

	start := decodeAdminResponseData[struct {
		Status    string `json:"status"`
		JobID     string `json:"job_id"`
		StartedAt string `json:"started_at"`
		Total     int    `json:"total"`
	}](t, rec)
	require.Equal(t, "running", start.Status)
	require.NotEmpty(t, start.JobID)
	require.NotEmpty(t, start.StartedAt)
	require.Equal(t, 1, start.Total)

	require.Eventually(t, func() bool {
		c, rec := newJSONTestContext(http.MethodGet, "/admin/accounts/apikey-health-check", "")
		handler.GetAPIKeysHealthStatus(c)
		require.Equal(t, http.StatusOK, rec.Code)
		status := decodeAdminResponseData[APIKeyHealthCheckStatusResponse](t, rec)
		return status.JobID == start.JobID &&
			status.Status == "completed" &&
			status.CompletedAt != nil &&
			status.Result != nil &&
			status.Result.Checked == 1 &&
			status.Result.Failed == 1
	}, time.Second, 10*time.Millisecond)
}

func TestRunHealthCheckBackground_RecoversWorkerPanic(t *testing.T) {
	handler := &AccountHandler{}
	jobID := "job-panic"
	now := time.Now()
	handler.hcState.mu.Lock()
	handler.hcState.running = true
	handler.hcState.jobID = jobID
	handler.hcState.startedAt = now
	handler.hcState.result = &APIKeyHealthCheckResult{
		Total:   1,
		Results: make([]APIKeyHealthCheckItem, 0, 1),
	}
	handler.hcState.mu.Unlock()

	handler.runHealthCheckBackground(jobID, []*service.Account{nil})

	handler.hcState.mu.RLock()
	defer handler.hcState.mu.RUnlock()
	require.False(t, handler.hcState.running)
	require.Equal(t, "completed", handler.hcState.status)
	require.NotNil(t, handler.hcState.completedAt)
	require.NotNil(t, handler.hcState.result)
	require.Equal(t, 1, handler.hcState.result.Checked)
	require.Equal(t, 1, handler.hcState.result.Failed)
	require.Len(t, handler.hcState.result.Results, 1)
	require.Contains(t, strings.ToLower(handler.hcState.result.Results[0].Error), "panic")
}

func TestImportRawAPIKeysValidateAfterImportStartsBackgroundHealthJob(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.accounts = nil
	handler := &AccountHandler{
		adminService:       adminSvc,
		accountTestService: &service.AccountTestService{},
	}

	c, rec := newJSONTestContext(http.MethodPost, "/admin/accounts/raw-import", `{
		"raw_text": "sk-proj-testkey123456",
		"validate_after_import": true
	}`)
	handler.ImportRawAPIKeys(c)
	require.Equal(t, http.StatusOK, rec.Code)

	result := decodeAdminResponseData[RawAPIKeyImportResult](t, rec)
	require.Equal(t, 1, result.TotalLines)
	require.Equal(t, 1, result.Created)
	require.Equal(t, 0, result.Checked, "import should not run health checks inside the HTTP request")
	require.True(t, result.HealthJobStarted)
	require.NotEmpty(t, result.HealthJobID)
	require.Empty(t, result.HealthJobError)
}
