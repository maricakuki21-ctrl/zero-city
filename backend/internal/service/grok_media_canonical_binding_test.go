package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/platform/corecontracts"
	"github.com/Wei-Shaw/sub2api/internal/platform/mediatask"
	"github.com/stretchr/testify/require"
)

type canonicalMediaTaskBindingRepositoryStub struct {
	UsageBillingRepository
	claimCalled     bool
	acceptErr       error
	acceptCalled    bool
	retryOpenAtSave bool
	runtime         *CanonicalGatewayRuntime
	retryable       error
}

func (s *canonicalMediaTaskBindingRepositoryStub) ClaimCanonicalMediaTask(context.Context, mediatask.ClaimBindingInput) (mediatask.Binding, bool, error) {
	s.claimCalled = true
	return mediatask.Binding{}, false, errors.New("claim is not expected")
}

func (s *canonicalMediaTaskBindingRepositoryStub) AcceptCanonicalMediaTask(_ context.Context, _ mediatask.CreateContext, _ string) (mediatask.Binding, error) {
	s.acceptCalled = true
	s.retryOpenAtSave = s.runtime.CanRetry(s.retryable)
	return mediatask.Binding{}, s.acceptErr
}

func (s *canonicalMediaTaskBindingRepositoryStub) ResolveCanonicalMediaTask(context.Context, int64, int64, string) (mediatask.Binding, error) {
	return mediatask.Binding{}, errors.New("resolve is not expected")
}

func TestCanonicalMediaTaskAcceptancePersistsBeforeRetryCloses(t *testing.T) {
	tests := []struct {
		name       string
		persistErr error
	}{
		{name: "durable accept succeeds"},
		{name: "durable accept fails after upstream acceptance", persistErr: errors.New("database unavailable")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			retryable := errors.New("retryable")
			runtime, err := NewCanonicalGatewayRuntime(CanonicalGatewayRuntimeInput{
				GroupID:       17,
				BillingPolicy: corecontracts.BillingPolicyBizDecipherLedger,
			}, func(err error) bool { return errors.Is(err, retryable) })
			require.NoError(t, err)
			createContext, err := mediatask.NewCreateContext("media:11:client-1", "client-1", []byte(`{"prompt":"clip"}`))
			require.NoError(t, err)
			repo := &canonicalMediaTaskBindingRepositoryStub{acceptErr: tt.persistErr, runtime: runtime, retryable: retryable}
			svc := &OpenAIGatewayService{usageBillingRepo: repo}
			ctx := mediatask.WithCreateContext(WithCanonicalGatewayRuntime(context.Background(), runtime), createContext)

			// When
			err = svc.acceptCanonicalGrokMediaTask(ctx, GrokMediaEndpointVideosGenerations, "upstream-task-1")

			// Then
			require.True(t, repo.acceptCalled)
			require.True(t, repo.retryOpenAtSave)
			require.False(t, runtime.CanRetry(retryable))
			if tt.persistErr != nil {
				require.ErrorIs(t, err, tt.persistErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCanonicalMediaTaskClaimRejectsMissingIdentityBeforeRepository(t *testing.T) {
	// Given
	repo := &canonicalMediaTaskBindingRepositoryStub{}
	svc := &OpenAIGatewayService{usageBillingRepo: repo}

	// When
	_, claimed, err := svc.ClaimCanonicalGrokMediaTask(context.Background(), mediatask.ClaimBindingInput{})

	// Then
	require.ErrorIs(t, err, ErrCanonicalMediaTaskConflict)
	require.False(t, claimed)
	require.False(t, repo.claimCalled)
}
