package service

import (
	"context"
	"errors"
	"net"
	"net/http"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type openAITransportErrorAccountRepo struct {
	AccountRepository
	calls []openAITransportErrorUnschedCall
}

type openAITransportErrorUnschedCall struct {
	AccountID int64
	Until     time.Time
	Reason    string
}

func (r *openAITransportErrorAccountRepo) SetTempUnschedulable(_ context.Context, id int64, until time.Time, reason string) error {
	r.calls = append(r.calls, openAITransportErrorUnschedCall{
		AccountID: id,
		Until:     until,
		Reason:    reason,
	})
	return nil
}

func TestClassifyOpenAITransportError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		persistent bool
	}{
		{name: "nil", err: nil, persistent: false},
		{name: "socks auth", err: errors.New("socks connect tcp: username/password authentication failed"), persistent: true},
		{name: "proxy auth", err: errors.New("407 Proxy Authentication Required"), persistent: true},
		{name: "connection refused typed", err: syscall.ECONNREFUSED, persistent: true},
		{name: "dns not found", err: &net.DNSError{IsNotFound: true, Err: "no such host"}, persistent: true},
		{name: "timeout transient", err: context.DeadlineExceeded, persistent: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.persistent, classifyOpenAITransportError(tt.err).Persistent)
		})
	}
}

func TestHandleOpenAIUpstreamTransportErrorFailoversAndTempUnschedulesPersistent(t *testing.T) {
	repo := &openAITransportErrorAccountRepo{}
	svc := &OpenAIGatewayService{accountRepo: repo}
	account := &Account{ID: 42, Name: "openai-proxy", Platform: PlatformOpenAI}

	err := svc.handleOpenAIUpstreamTransportError(
		context.Background(),
		nil,
		account,
		errors.New("socks connect tcp: username/password authentication failed"),
		false,
	)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.Contains(t, string(failoverErr.ResponseBody), "Upstream request failed")
	require.Len(t, repo.calls, 1)
	require.Equal(t, account.ID, repo.calls[0].AccountID)
	require.Contains(t, repo.calls[0].Reason, "authentication failed")
	require.True(t, repo.calls[0].Until.After(time.Now()))
}

func TestHandleOpenAIUpstreamTransportErrorCanceledDoesNotFailoverOrUnschedule(t *testing.T) {
	repo := &openAITransportErrorAccountRepo{}
	svc := &OpenAIGatewayService{accountRepo: repo}
	account := &Account{ID: 42, Name: "openai-proxy", Platform: PlatformOpenAI}

	err := svc.handleOpenAIUpstreamTransportError(context.Background(), nil, account, context.Canceled, false)

	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, repo.calls)
}

func TestHandleOpenAIUpstreamTransportErrorGrokBuildProbeDoesNotUnschedule(t *testing.T) {
	repo := &openAITransportErrorAccountRepo{}
	svc := &OpenAIGatewayService{accountRepo: repo}
	account := &Account{
		ID:       2108,
		Name:     "grok-build-probe",
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"grok-build": "grok-build-0.1"},
		},
	}

	err := svc.handleOpenAIUpstreamTransportError(
		context.Background(), nil, account, errors.New("socks connect tcp: connection refused"), false, "grok-build",
	)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.Empty(t, repo.calls)
}
