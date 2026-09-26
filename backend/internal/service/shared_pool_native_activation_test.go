package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type nativeActivationRepoFake struct {
	BizDecipherRepository
	input SharedPoolNativeActivationInput
	err   error
}

func (f *nativeActivationRepoFake) ActivateNativePoolBilling(_ context.Context, input SharedPoolNativeActivationInput) (*SharedPoolNativeActivation, error) {
	f.input = input
	if f.err != nil {
		return nil, f.err
	}
	return &SharedPoolNativeActivation{PoolID: input.PoolID, State: SharedPoolOnboardingBillingActive}, nil
}

func TestSharedPoolNativeActivation_rejectsInvalidBoundaryInput(t *testing.T) {
	_, err := NewSharedPoolNativeActivationInput(1, 2, "", 7)
	require.ErrorIs(t, err, ErrOwnerNativeFieldRejected)
}

func TestSharedPoolNativeActivation_passesAtomicVersionAndIdentity(t *testing.T) {
	repo := &nativeActivationRepoFake{}
	svc := &BizDecipherService{repo: repo}
	input, err := NewSharedPoolNativeActivationInput(11, 22, "activate-1", 9)
	require.NoError(t, err)

	result, err := svc.ActivateSharedPoolNativeBilling(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, int64(11), result.PoolID)
	require.Equal(t, int64(9), repo.input.ExpectedConfigVersion)
	require.Len(t, repo.input.RequestFingerprint, 64)
}
