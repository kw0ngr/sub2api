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
