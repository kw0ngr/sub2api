package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConcurrencyErrorResponse_ClassifiesAcquireFailures(t *testing.T) {
	status, errType, message := concurrencyErrorResponse(&ConcurrencyError{SlotType: "account", IsTimeout: true}, "user")

	require.Equal(t, http.StatusTooManyRequests, status)
	require.Equal(t, "rate_limit_error", errType)
	require.Contains(t, message, "account")
}

func TestConcurrencyErrorResponse_ClassifiesClientCancel(t *testing.T) {
	status, errType, message := concurrencyErrorResponse(context.Canceled, "user")

	require.Equal(t, statusClientClosedRequest, status)
	require.Equal(t, "api_error", errType)
	require.Equal(t, "context canceled", message)
}

func TestConcurrencyErrorResponse_UnknownErrorUsesUnavailable(t *testing.T) {
	status, errType, message := concurrencyErrorResponse(errors.New("redis down"), "user")

	require.Equal(t, http.StatusServiceUnavailable, status)
	require.Equal(t, "api_error", errType)
	require.Contains(t, message, "temporarily unavailable")
}
