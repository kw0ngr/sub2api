package handler

import (
	"context"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBillingCancellationIsNotServiceUnavailable(t *testing.T) {
	for _, err := range []error{context.Canceled, context.DeadlineExceeded, fmt.Errorf("subscription lookup: %w", context.Canceled)} {
		status, code, _ := billingErrorDetails(err)
		require.Equal(t, 499, status)
		require.Equal(t, "client_canceled", code)
	}
	for _, cause := range []error{nil, context.DeadlineExceeded, context.Canceled} {
		status, code, _ := billingErrorDetails(service.ErrBillingServiceUnavailable.WithCause(cause))
		require.Equal(t, 503, status)
		require.Equal(t, "billing_service_error", code)
	}
}
