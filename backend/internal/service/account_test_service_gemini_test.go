//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestApplyTestConnectionAction_TemporaryGeminiFailuresDoNotMutateScheduling(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       []byte
	}{
		{
			name:       "model quota exhausted",
			statusCode: http.StatusTooManyRequests,
			body:       []byte(`{"error":{"code":429,"message":"Quota exceeded for model gemini-2.0-flash","status":"RESOURCE_EXHAUSTED"}}`),
		},
		{
			name:       "retired model",
			statusCode: http.StatusNotFound,
			body:       []byte(`{"error":{"code":404,"message":"This model models/gemini-2.0-flash is no longer available.","status":"NOT_FOUND"}}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &rateLimitAccountRepoStub{}
			account := &Account{
				ID:          300,
				Platform:    PlatformGemini,
				Type:        AccountTypeAPIKey,
				Status:      StatusActive,
				Schedulable: true,
			}

			applyTestConnectionAction(context.Background(), repo, account, tt.statusCode, http.Header{}, tt.body)

			require.Zero(t, repo.setErrorCalls)
			require.Zero(t, repo.rateLimitedCalls)
			require.Zero(t, repo.tempCalls)
		})
	}
}

func TestApplyTestConnectionAction_InvalidGeminiCredentialDisablesAccount(t *testing.T) {
	repo := &rateLimitAccountRepoStubWithSchedulable{}
	account := &Account{
		ID:          301,
		Platform:    PlatformGemini,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
	}
	body := []byte(`{"error":{"code":400,"message":"API key not valid. Please pass a valid API key.","status":"API_KEY_INVALID"}}`)

	applyTestConnectionAction(context.Background(), repo, account, http.StatusBadRequest, http.Header{}, body)

	require.Equal(t, 1, repo.setErrorCalls)
	require.Equal(t, 1, repo.setSchedulableCalls)
	require.False(t, repo.lastSchedulable)
	require.Zero(t, repo.rateLimitedCalls)
	require.Zero(t, repo.tempCalls)
}

func TestCreateGeminiTestPayload_ImageModel(t *testing.T) {
	t.Parallel()

	payload := createGeminiTestPayload("gemini-2.5-flash-image", "draw a tiny robot")

	var parsed struct {
		Contents []struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"contents"`
		GenerationConfig struct {
			ResponseModalities []string `json:"responseModalities"`
			ImageConfig        struct {
				AspectRatio string `json:"aspectRatio"`
			} `json:"imageConfig"`
		} `json:"generationConfig"`
	}

	require.NoError(t, json.Unmarshal(payload, &parsed))
	require.Len(t, parsed.Contents, 1)
	require.Len(t, parsed.Contents[0].Parts, 1)
	require.Equal(t, "draw a tiny robot", parsed.Contents[0].Parts[0].Text)
	require.Equal(t, []string{"TEXT", "IMAGE"}, parsed.GenerationConfig.ResponseModalities)
	require.Equal(t, "1:1", parsed.GenerationConfig.ImageConfig.AspectRatio)
}

func TestProcessGeminiStream_EmitsImageEvent(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	ctx, recorder := newTestContext()
	svc := &AccountTestService{}

	stream := strings.NewReader("data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"ok\"},{\"inlineData\":{\"mimeType\":\"image/png\",\"data\":\"QUJD\"}}]}}]}\n\ndata: [DONE]\n\n")

	err := svc.processGeminiStream(ctx, context.Background(), nil, stream)
	require.NoError(t, err)

	body := recorder.Body.String()
	require.Contains(t, body, "\"type\":\"content\"")
	require.Contains(t, body, "\"text\":\"ok\"")
	require.Contains(t, body, "\"type\":\"image\"")
	require.Contains(t, body, "\"image_url\":\"data:image/png;base64,QUJD\"")
	require.Contains(t, body, "\"mime_type\":\"image/png\"")
}
