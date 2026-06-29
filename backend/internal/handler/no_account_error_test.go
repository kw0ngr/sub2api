package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type fakeModelAvailabilityDiagnoser struct {
	calls []fakeModelAvailabilityCall
	resp  service.ModelAvailabilityDiagnosis
}

type fakeModelAvailabilityCall struct {
	groupID  *int64
	model    string
	platform string
}

func (f *fakeModelAvailabilityDiagnoser) DiagnoseModelAvailabilityForPlatform(
	_ context.Context,
	groupID *int64,
	model string,
	platform string,
) service.ModelAvailabilityDiagnosis {
	f.calls = append(f.calls, fakeModelAvailabilityCall{groupID: groupID, model: model, platform: platform})
	return f.resp
}

func groupIDPtr(id int64) *int64 {
	return &id
}

func TestClassifyNoAccountError_Returns404WhenPoolHasNoModelSupport(t *testing.T) {
	diag := &fakeModelAvailabilityDiagnoser{
		resp: service.ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: false},
	}
	apiKey := &service.APIKey{GroupID: groupIDPtr(42)}

	cls := classifyNoAccountError(context.Background(), noAccountDiagnosisRequest{
		Diagnoser:    diag,
		APIKey:       apiKey,
		RoutingModel: "gpt-5.5",
		DisplayModel: "custom-gpt",
		Platform:     service.PlatformOpenAI,
	})

	require.Equal(t, http.StatusNotFound, cls.Status)
	require.Equal(t, "model_not_found", cls.ErrType)
	require.True(t, cls.ModelNotFound)
	require.Contains(t, cls.Message, "custom-gpt")
	require.Len(t, diag.calls, 1)
	require.Equal(t, "gpt-5.5", diag.calls[0].model)
	require.Equal(t, service.PlatformOpenAI, diag.calls[0].platform)
}

func TestClassifyNoAccountError_Keeps503WhenModelMayBeTemporarilyUnavailable(t *testing.T) {
	tests := []struct {
		name string
		resp service.ModelAvailabilityDiagnosis
	}{
		{name: "empty pool", resp: service.ModelAvailabilityDiagnosis{}},
		{name: "model supported", resp: service.ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diag := &fakeModelAvailabilityDiagnoser{resp: tt.resp}
			apiKey := &service.APIKey{GroupID: groupIDPtr(7)}

			cls := classifyNoAccountError(context.Background(), noAccountDiagnosisRequest{
				Diagnoser:    diag,
				APIKey:       apiKey,
				RoutingModel: "glm-5.2",
				DisplayModel: "glm-5.2",
				Platform:     service.PlatformGLM,
			})

			require.Equal(t, http.StatusServiceUnavailable, cls.Status)
			require.Equal(t, "api_error", cls.ErrType)
			require.False(t, cls.ModelNotFound)
		})
	}
}

func TestClassifyNoAccountError_SkipsDiagnosisWithoutGroupOrModel(t *testing.T) {
	diag := &fakeModelAvailabilityDiagnoser{
		resp: service.ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: false},
	}

	for _, req := range []noAccountDiagnosisRequest{
		{Diagnoser: diag, APIKey: &service.APIKey{}, RoutingModel: "gpt-5.5", Platform: service.PlatformOpenAI},
		{Diagnoser: diag, APIKey: &service.APIKey{GroupID: groupIDPtr(1)}, RoutingModel: " ", Platform: service.PlatformOpenAI},
	} {
		cls := classifyNoAccountError(context.Background(), req)
		require.Equal(t, http.StatusServiceUnavailable, cls.Status)
		require.False(t, cls.ModelNotFound)
	}
	require.Empty(t, diag.calls)
}
