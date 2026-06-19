package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type cacheControlSettingRepoStub struct {
	values map[string]string
}

func (s *cacheControlSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (s *cacheControlSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}

func (s *cacheControlSettingRepoStub) Set(_ context.Context, key, value string) error {
	s.values[key] = value
	return nil
}

func (s *cacheControlSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		out[key] = s.values[key]
	}
	return out, nil
}

func (s *cacheControlSettingRepoStub) SetMultiple(_ context.Context, settings map[string]string) error {
	for key, value := range settings {
		s.values[key] = value
	}
	return nil
}

func (s *cacheControlSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	out := make(map[string]string, len(s.values))
	for key, value := range s.values {
		out[key] = value
	}
	return out, nil
}

func (s *cacheControlSettingRepoStub) Delete(_ context.Context, key string) error {
	delete(s.values, key)
	return nil
}

func TestSettingServiceParseSettingsDefaultsRewriteMessageCacheControlOff(t *testing.T) {
	svc := NewSettingService(nil, &config.Config{})

	settings := svc.parseSettings(map[string]string{})

	require.False(t, settings.RewriteMessageCacheControl)
	require.False(t, settings.EnableGLMZCodeStrongMimic)
}

func TestSettingServiceUpdateSettingsPersistsRewriteMessageCacheControl(t *testing.T) {
	repo := &cacheControlSettingRepoStub{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		RewriteMessageCacheControl: true,
	})

	require.NoError(t, err)
	require.Equal(t, "true", repo.values[SettingKeyRewriteMessageCacheControl])
}

func TestSettingServiceUpdateSettingsPersistsGLMZCodeStrongMimic(t *testing.T) {
	repo := &cacheControlSettingRepoStub{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		EnableGLMZCodeStrongMimic: true,
	})

	require.NoError(t, err)
	require.Equal(t, "true", repo.values[SettingKeyEnableGLMZCodeStrongMimic])
}

func TestSettingServiceIsGLMZCodeStrongMimicEnabledUsesGatewayCache(t *testing.T) {
	gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{})
	t.Cleanup(func() {
		gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{})
	})
	repo := &cacheControlSettingRepoStub{values: map[string]string{
		SettingKeyEnableGLMZCodeStrongMimic: "true",
	}}
	svc := NewSettingService(repo, &config.Config{})

	require.True(t, svc.IsGLMZCodeStrongMimicEnabled(context.Background()))
}
