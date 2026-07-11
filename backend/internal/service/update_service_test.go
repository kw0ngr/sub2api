package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type updateServiceNoopCache struct{}

func (updateServiceNoopCache) GetUpdateInfo(context.Context) (string, error) {
	return "", errors.New("cache miss")
}

func (updateServiceNoopCache) SetUpdateInfo(context.Context, string, time.Duration) error {
	return nil
}

type updateServiceFakeGitHubClient struct {
	release *GitHubRelease
}

func (c updateServiceFakeGitHubClient) FetchLatestRelease(context.Context, string) (*GitHubRelease, error) {
	return c.release, nil
}

func (updateServiceFakeGitHubClient) DownloadFile(context.Context, string, string, int64) error {
	return nil
}

func (updateServiceFakeGitHubClient) FetchChecksumFile(context.Context, string) ([]byte, error) {
	return nil, nil
}

func TestUpdateService_PerformUpdateReturnsTypedNoUpdateError(t *testing.T) {
	svc := NewUpdateService(updateServiceNoopCache{}, updateServiceFakeGitHubClient{
		release: &GitHubRelease{
			TagName: "v1.2.3",
			Name:    "v1.2.3",
		},
	}, "1.2.3", "release", "kw0ngr/sub2api")

	err := svc.PerformUpdate(context.Background())

	require.ErrorIs(t, err, ErrNoUpdateAvailable)
}

type updateServiceRepoCaptureClient struct {
	repo string
}

func (c *updateServiceRepoCaptureClient) FetchLatestRelease(_ context.Context, repo string) (*GitHubRelease, error) {
	c.repo = repo
	return &GitHubRelease{TagName: "v9.9.9", Name: "v9.9.9"}, nil
}

func (updateServiceRepoCaptureClient) DownloadFile(context.Context, string, string, int64) error {
	return nil
}

func (updateServiceRepoCaptureClient) FetchChecksumFile(context.Context, string) ([]byte, error) {
	return nil, nil
}

func TestUpdateService_UsesConfiguredGitHubRepo(t *testing.T) {
	client := &updateServiceRepoCaptureClient{}
	svc := NewUpdateService(updateServiceNoopCache{}, client, "1.0.0", "release", "kw0ngr/sub2api")

	info, err := svc.CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, "kw0ngr/sub2api", client.repo)
	require.Equal(t, "kw0ngr/sub2api", info.GitHubRepo)
	require.True(t, info.HasUpdate)
}

func TestUpdateService_DefaultsGitHubRepoWhenEmpty(t *testing.T) {
	svc := NewUpdateService(updateServiceNoopCache{}, updateServiceFakeGitHubClient{
		release: &GitHubRelease{TagName: "v1.0.0"},
	}, "1.0.0", "release", "  ")
	require.Equal(t, defaultGitHubRepo, svc.GitHubRepo())
}
