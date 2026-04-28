package handler

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type claudeFingerprintSettingRepoStub struct {
	values map[string]string
}

func (s *claudeFingerprintSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	return nil, service.ErrSettingNotFound
}

func (s *claudeFingerprintSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := s.values[key]; ok {
		return value, nil
	}
	return "", service.ErrSettingNotFound
}

func (s *claudeFingerprintSettingRepoStub) Set(_ context.Context, key, value string) error {
	if s.values == nil {
		s.values = make(map[string]string)
	}
	s.values[key] = value
	return nil
}

func (s *claudeFingerprintSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *claudeFingerprintSettingRepoStub) SetMultiple(_ context.Context, settings map[string]string) error {
	if s.values == nil {
		s.values = make(map[string]string)
	}
	for key, value := range settings {
		s.values[key] = value
	}
	return nil
}

func (s *claudeFingerprintSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	out := make(map[string]string, len(s.values))
	for key, value := range s.values {
		out[key] = value
	}
	return out, nil
}

func (s *claudeFingerprintSettingRepoStub) Delete(_ context.Context, key string) error {
	delete(s.values, key)
	return nil
}

func TestGatewayHandlerObserveClaudeCodeFingerprintCandidateIgnoresApplySwitch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &claudeFingerprintSettingRepoStub{
		values: map[string]string{
			service.SettingKeyEnableFingerprintUnification: "false",
		},
	}
	settingSvc := service.NewSettingService(repo, &config.Config{})
	h := &GatewayHandler{settingService: settingSvc}

	req := httptest.NewRequest("POST", "/v1/messages", nil)
	req.Header.Set("User-Agent", "claude-cli/2.1.93 (external, cli)")
	req.Header.Set("X-Stainless-Lang", "js")
	req.Header.Set("X-Stainless-Package-Version", "0.71.0")
	req.Header.Set("X-Stainless-OS", "Darwin")
	req.Header.Set("X-Stainless-Arch", "arm64")
	req.Header.Set("X-Stainless-Runtime", "node")
	req.Header.Set("X-Stainless-Runtime-Version", "v24.14.0")
	req = req.WithContext(service.SetClaudeCodeClient(req.Context(), true))

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = req

	h.observeClaudeCodeFingerprintCandidate(c, &service.APIKey{
		Name: "team-key",
		User: &service.User{Email: "dev@example.com"},
		Group: &service.Group{
			Name: "claude-team",
		},
	}, nil)

	library, err := settingSvc.GetClaudeCodeFingerprintLibrary(context.Background())
	require.NoError(t, err)
	require.Len(t, library.Profiles, 1)

	profile := library.Profiles[0]
	require.Equal(t, int64(0), profile.AccountID)
	require.Equal(t, "claude-cli/2.1.93 (external, cli)", profile.UserAgent)
	require.Equal(t, "0.71.0", profile.StainlessPackageVersion)
	require.Equal(t, "Darwin", profile.StainlessOS)
	require.Contains(t, profile.AccountName, "dev@example.com")
	require.Contains(t, profile.AccountName, "key:team-key")
	require.Contains(t, profile.AccountName, "group:claude-team")
}
