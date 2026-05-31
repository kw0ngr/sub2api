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
	}, "1.2.3", "release")

	err := svc.PerformUpdate(context.Background())

	require.ErrorIs(t, err, ErrNoUpdateAvailable)
}
