package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIGatewayService_CanonicalBindingUsesCanonicalAccountSlotWithOneConcurrentRequest(t *testing.T) {
	// Given: a canonical request bound to an account whose legacy value was wider.
	boundAccount := Account{
		ID:          47001,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 7,
	}
	service := &OpenAIGatewayService{
		accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{boundAccount}},
	}

	// When: canonical routing selects that already-bound Sub2 account.
	selection, decision, err := service.selectCanonicalBoundAccount(
		context.Background(),
		boundAccount.ID,
		nil,
		"gpt-5.1",
		nil,
		OpenAIUpstreamTransportHTTPSSE,
		"",
		"",
		false,
		PlatformOpenAI,
	)

	// Then: the existing account slot is used with a hard composite-key limit of one.
	require.NoError(t, err)
	require.Equal(t, "canonical_binding", decision.Layer)
	require.NotNil(t, selection)
	require.NotNil(t, selection.WaitPlan)
	require.Equal(t, boundAccount.ID, selection.WaitPlan.AccountID)
	require.Equal(t, 1, selection.WaitPlan.MaxConcurrency)
}
