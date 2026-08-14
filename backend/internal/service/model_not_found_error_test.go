package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsUpstreamModelNotFoundError_RecognizesExplicitModelOnly404(t *testing.T) {
	tests := []struct {
		name string
		body []byte
		want bool
	}{
		{name: "observed deepseek compact message", body: []byte(`{"error":{"message":"model: deepseek-v4-pro","type":"server_error"}}`), want: true},
		{name: "standard model not found", body: []byte(`{"error":{"code":"model_not_found","message":"model not found"}}`), want: true},
		{name: "code only model not found", body: []byte(`{"error":{"code":"model_not_found","message":"The requested resource was not found"}}`), want: true},
		{name: "openai model does not exist", body: []byte(`{"error":{"code":"model_not_found","message":"The model ` + "`gpt-5.6-sol`" + ` does not exist or you do not have access to it."}}`), want: true},
		{name: "gemini retired model", body: []byte(`{"error":{"code":404,"message":"This model models/gemini-2.0-flash is no longer available. Please update your code to use a newer model.","status":"NOT_FOUND"}}`), want: true},
		{name: "provider-qualified unknown model", body: []byte(`{"error":{"type":"invalid_request","message":"Unknown Umans model \"umans-glm-5.3\"."}}`), want: true},
		{name: "endpoint not found", body: []byte(`{"error":{"message":"endpoint not found"}}`), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isUpstreamModelNotFoundError(http.StatusNotFound, tt.body))
		})
	}

	require.False(t, isUpstreamModelNotFoundError(http.StatusBadRequest, []byte(`{"error":{"message":"model: deepseek-v4-pro"}}`)))
	require.True(t, isUpstreamModelNotFoundError(http.StatusBadRequest, []byte(`{"error":{"code":"model_not_found","message":"The model `+"`gpt-5.6-sol`"+` does not exist or you do not have access to it."}}`)))
}

func TestIsUpstreamModelNotFoundError_ResponsesFailureWrappedAs502(t *testing.T) {
	nested := []byte(`{"type":"response.failed","response":{"error":{"code":"model_not_found","type":"invalid_request_error","message":"Project does not have access to model gpt-5.6-sol"}}}`)

	require.Equal(t, "model_not_found", extractUpstreamErrorCode(nested))
	require.Equal(t, "invalid_request_error", extractUpstreamErrorType(nested))
	require.Equal(t, "Project does not have access to model gpt-5.6-sol", extractUpstreamErrorMessage(nested))
	require.True(t, isUpstreamModelNotFoundError(http.StatusBadGateway, nested))
	require.True(t, isUpstreamModelNotFoundError(http.StatusBadGateway, []byte(`{"error":{"code":"model_not_found","message":"no access"}}`)))

	// 5xx message heuristics alone are intentionally insufficient: only an
	// explicit structured model marker may override generic server-error health.
	require.False(t, isUpstreamModelNotFoundError(http.StatusBadGateway, []byte(`{"error":{"message":"model not found while upstream was unavailable"}}`)))
	require.False(t, isUpstreamModelNotFoundError(http.StatusBadGateway, []byte(`{"error":{"message":"endpoint not found"}}`)))
	require.False(t, isUpstreamModelNotFoundError(http.StatusOK, nested))
}
