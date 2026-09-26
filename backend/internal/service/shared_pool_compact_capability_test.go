package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type sharedPoolCompactCapabilityRepoStub struct {
	BizDecipherRepository
	ready                  bool
	err                    error
	calls                  int
	expectedPublishedModel string
	expectedUpstreamModel  string
}

func (r *sharedPoolCompactCapabilityRepoStub) HasSharedPoolOAuthCompactCapability(_ context.Context, poolID, accountID int64, publishedModel, upstreamModel string) (bool, error) {
	r.calls++
	if poolID != 9 || accountID != 27 {
		return false, errors.New("unexpected compact capability identity")
	}
	expectedPublishedModel := r.expectedPublishedModel
	if expectedPublishedModel == "" {
		expectedPublishedModel = "published-gpt-5.6"
	}
	expectedUpstreamModel := r.expectedUpstreamModel
	if expectedUpstreamModel == "" {
		expectedUpstreamModel = "gpt-5.6-codex"
	}
	if publishedModel != expectedPublishedModel || upstreamModel != expectedUpstreamModel {
		return false, errors.New("unexpected compact capability model binding")
	}
	return r.ready, r.err
}

func TestValidateSharedPoolCompactAccessRejectsAPIKeyWithoutCapabilityLookup(t *testing.T) {
	repo := &sharedPoolCompactCapabilityRepoStub{ready: true}
	gateway := &OpenAIGatewayService{bizDecipherService: NewBizDecipherService(repo, nil, nil)}

	err := gateway.ValidateSharedPoolCompactAccess(context.Background(), &SharedPoolAccessKey{
		PoolID: 9, AccountID: 27, AuthType: AccountTypeAPIKey,
		PublishedModelName: "published-gpt-5.6", UpstreamModelName: "gpt-5.6-codex",
	})

	require.ErrorIs(t, err, ErrSharedPoolCompactNotSupported)
	require.Zero(t, repo.calls)
}

func TestValidateSharedPoolCompactAccessRejectsUnverifiedOAuth(t *testing.T) {
	repo := &sharedPoolCompactCapabilityRepoStub{}
	gateway := &OpenAIGatewayService{bizDecipherService: NewBizDecipherService(repo, nil, nil)}

	err := gateway.ValidateSharedPoolCompactAccess(context.Background(), &SharedPoolAccessKey{
		PoolID: 9, AccountID: 27, AuthType: AccountTypeOAuth,
		PublishedModelName: "published-gpt-5.6", UpstreamModelName: "gpt-5.6-codex",
	})

	require.ErrorIs(t, err, ErrSharedPoolCompactNotSupported)
	require.Equal(t, 1, repo.calls)
}

func TestValidateSharedPoolCompactAccessAcceptsVerifiedOAuth(t *testing.T) {
	repo := &sharedPoolCompactCapabilityRepoStub{ready: true}
	gateway := &OpenAIGatewayService{bizDecipherService: NewBizDecipherService(repo, nil, nil)}

	err := gateway.ValidateSharedPoolCompactAccess(context.Background(), &SharedPoolAccessKey{
		PoolID: 9, AccountID: 27, AuthType: AccountTypeOAuth,
		PublishedModelName: "published-gpt-5.6", UpstreamModelName: "gpt-5.6-codex",
	})

	require.NoError(t, err)
	require.Equal(t, 1, repo.calls)
}

func TestValidateSharedPoolCompactAccessFailsClosedOnCapabilityStoreError(t *testing.T) {
	repo := &sharedPoolCompactCapabilityRepoStub{err: errors.New("database unavailable")}
	gateway := &OpenAIGatewayService{bizDecipherService: NewBizDecipherService(repo, nil, nil)}

	err := gateway.ValidateSharedPoolCompactAccess(context.Background(), &SharedPoolAccessKey{
		PoolID: 9, AccountID: 27, AuthType: AccountTypeOAuth,
		PublishedModelName: "published-gpt-5.6", UpstreamModelName: "gpt-5.6-codex",
	})

	require.Error(t, err)
	require.NotErrorIs(t, err, ErrSharedPoolCompactNotSupported)
	require.Contains(t, err.Error(), "database unavailable")
}

func TestValidateSharedPoolCompactAccessRejectsMissingModelBindingWithoutLookup(t *testing.T) {
	repo := &sharedPoolCompactCapabilityRepoStub{ready: true}
	gateway := &OpenAIGatewayService{bizDecipherService: NewBizDecipherService(repo, nil, nil)}

	err := gateway.ValidateSharedPoolCompactAccess(context.Background(), &SharedPoolAccessKey{
		PoolID: 9, AccountID: 27, AuthType: AccountTypeOAuth,
		UpstreamModelName: "gpt-5.6-codex",
	})

	require.ErrorIs(t, err, ErrSharedPoolCompactNotSupported)
	require.Zero(t, repo.calls)
}

func TestValidateSharedPoolCompactAccessUsesPublishedModelForDirectMapping(t *testing.T) {
	repo := &sharedPoolCompactCapabilityRepoStub{
		ready:                  true,
		expectedPublishedModel: "gpt-5.6",
		expectedUpstreamModel:  "gpt-5.6",
	}
	gateway := &OpenAIGatewayService{bizDecipherService: NewBizDecipherService(repo, nil, nil)}

	err := gateway.ValidateSharedPoolCompactAccess(context.Background(), &SharedPoolAccessKey{
		PoolID: 9, AccountID: 27, AuthType: AccountTypeOAuth,
		PublishedModelName: "gpt-5.6",
	})

	require.NoError(t, err)
	require.Equal(t, 1, repo.calls)
}
