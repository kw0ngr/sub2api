package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type cancelBillingUserRepo struct {
	UserRepository
	cancel context.CancelFunc
}

func (r cancelBillingUserRepo) GetByID(ctx context.Context, id int64) (*User, error) {
	r.cancel()
	return nil, ctx.Err()
}

type cancelBillingSubscriptionRepo struct {
	UserSubscriptionRepository
	cancel context.CancelFunc
}

func (r cancelBillingSubscriptionRepo) GetActiveByUserIDAndGroupID(ctx context.Context, userID, groupID int64) (*UserSubscription, error) {
	r.cancel()
	return nil, ctx.Err()
}

func TestBillingClientCancellationDoesNotTripCircuit(t *testing.T) {
	for _, mode := range []string{"balance", "subscription"} {
		for _, halfOpen := range []bool{false, true} {
			t.Run(mode+map[bool]string{false: "/closed", true: "/half-open"}[halfOpen], func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				breaker := newBillingCircuitBreaker(config.CircuitBreakerConfig{Enabled: true, FailureThreshold: 1, HalfOpenRequests: 1})
				if halfOpen {
					breaker.state = billingCircuitHalfOpen
					breaker.halfOpenRemaining = 1
					breaker.halfOpenGeneration = 1
				}
				svc := &BillingCacheService{
					cfg: &config.Config{}, circuitBreaker: breaker,
					userRepo: cancelBillingUserRepo{cancel: cancel},
					subRepo:  cancelBillingSubscriptionRepo{cancel: cancel},
				}
				var group *Group
				var sub *UserSubscription
				if mode == "subscription" {
					group = &Group{ID: 17, SubscriptionType: SubscriptionTypeSubscription}
					sub = &UserSubscription{ID: 1}
				}
				err := svc.CheckBillingEligibility(ctx, &User{ID: 1}, nil, group, sub)
				require.ErrorIs(t, err, context.Canceled)
				require.NotErrorIs(t, err, ErrBillingServiceUnavailable)
				require.Zero(t, breaker.failures)
				require.NotEqual(t, billingCircuitOpen, breaker.state)
				allowed, _ := breaker.Allow()
				require.True(t, allowed, "another healthy client can still probe")
			})
		}
	}
}

func TestBillingAlreadyCanceledDoesNotConsumeCircuitPermit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc := &BillingCacheService{cfg: &config.Config{}, circuitBreaker: &billingCircuitBreaker{
		state: billingCircuitHalfOpen, halfOpenRequests: 1, halfOpenRemaining: 1,
	}}
	require.ErrorIs(t, svc.CheckBillingEligibility(ctx, nil, nil, nil, nil), context.Canceled)
	require.Equal(t, 1, svc.circuitBreaker.halfOpenRemaining)
}

func TestBillingRealFailureStillOpensCircuit(t *testing.T) {
	breaker := newBillingCircuitBreaker(config.CircuitBreakerConfig{Enabled: true, FailureThreshold: 1})
	breaker.OnFailure(errors.New("database unavailable"))
	require.Equal(t, billingCircuitOpen, breaker.state)
	allowed, _ := breaker.Allow()
	require.False(t, allowed)
}

func TestBillingCancellationOnlyReturnsItsOwnHalfOpenPermit(t *testing.T) {
	breaker := newBillingCircuitBreaker(config.CircuitBreakerConfig{Enabled: true, FailureThreshold: 1, HalfOpenRequests: 1})
	allowed, closedProbe := breaker.Allow()
	require.True(t, allowed)
	require.Zero(t, closedProbe)
	breaker.OnFailure(errors.New("database unavailable"))
	breaker.openedAt = time.Now().Add(-time.Hour)
	allowed, firstProbe := breaker.Allow()
	require.True(t, allowed)
	require.NotZero(t, firstProbe)
	breaker.OnCanceled(closedProbe)
	allowed, _ = breaker.Allow()
	require.False(t, allowed, "an old closed request cannot return the current probe")
	breaker.OnFailure(errors.New("first recovery failed"))
	breaker.openedAt = time.Now().Add(-time.Hour)
	allowed, secondProbe := breaker.Allow()
	require.True(t, allowed)
	require.NotEqual(t, firstProbe, secondProbe)
	breaker.OnCanceled(firstProbe)
	allowed, _ = breaker.Allow()
	require.False(t, allowed, "a stale recovery request cannot return a later generation's probe")
	breaker.OnCanceled(secondProbe)
	allowed, _ = breaker.Allow()
	require.True(t, allowed, "the current canceled probe returns its own permit")
}

func TestBillingExpiredRequestReturnsContextError(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	require.ErrorIs(t, (&BillingCacheService{}).CheckBillingEligibility(ctx, nil, nil, nil, nil), context.DeadlineExceeded)
}
