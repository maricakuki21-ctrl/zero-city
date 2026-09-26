package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type capabilityAssetUpdateRepoStub struct {
	BizDecipherRepository
	assetID int64
	userID  int64
	input   CapabilityAssetInput
	err     error
	calls   int
}

func (r *capabilityAssetUpdateRepoStub) UpdateCapabilityAsset(_ context.Context, assetID, userID int64, input CapabilityAssetInput) (*CapabilityAsset, error) {
	r.calls++
	r.assetID = assetID
	r.userID = userID
	r.input = input
	if r.err != nil {
		return nil, r.err
	}
	return &CapabilityAsset{ID: assetID, UserID: userID, Title: input.Title, Status: input.Status}, nil
}

func TestUpdateCapabilityAssetUpdatesOwnedDraftWithNormalizedFields(t *testing.T) {
	repo := &capabilityAssetUpdateRepoStub{}
	svc := NewBizDecipherService(repo, nil, nil)

	asset, err := svc.UpdateCapabilityAsset(context.Background(), 41, 17, CapabilityAssetInput{
		Title:       "  Draft title  ",
		Summary:     "  Draft summary  ",
		Description: "  Draft description  ",
		Status:      "",
		Tags:        []string{" ai ", "ai"},
	})

	require.NoError(t, err)
	require.Equal(t, int64(41), repo.assetID)
	require.Equal(t, int64(17), repo.userID)
	require.Equal(t, "Draft title", repo.input.Title)
	require.Equal(t, []string{"ai"}, repo.input.Tags)
	require.Equal(t, CapabilityAssetStatusDraft, repo.input.Status)
	require.Equal(t, CapabilityAssetStatusDraft, asset.Status)
}

func TestUpdateCapabilityAssetRejectsStatusElevation(t *testing.T) {
	for _, status := range []string{CapabilityAssetStatusPending, CapabilityAssetStatusListed, CapabilityAssetStatusRejected, CapabilityAssetStatusArchived, CapabilityAssetStatusDelisted} {
		t.Run(status, func(t *testing.T) {
			repo := &capabilityAssetUpdateRepoStub{}
			svc := NewBizDecipherService(repo, nil, nil)

			_, err := svc.UpdateCapabilityAsset(context.Background(), 41, 17, CapabilityAssetInput{
				Title: "title", Summary: "summary", Description: "description", Status: status,
			})

			require.EqualError(t, err, "asset status cannot be changed by draft edit")
			require.Zero(t, repo.calls)
		})
	}
}

func TestUpdateCapabilityAssetRejectsInvalidRequiredFields(t *testing.T) {
	repo := &capabilityAssetUpdateRepoStub{}
	svc := NewBizDecipherService(repo, nil, nil)

	_, err := svc.UpdateCapabilityAsset(context.Background(), 41, 17, CapabilityAssetInput{Title: "title", Description: "description"})

	require.EqualError(t, err, "asset summary is required")
	require.Zero(t, repo.calls)
}

func TestUpdateCapabilityAssetPreservesDraftNotFoundError(t *testing.T) {
	repo := &capabilityAssetUpdateRepoStub{err: ErrCapabilityAssetDraftNotFound}
	svc := NewBizDecipherService(repo, nil, nil)

	_, err := svc.UpdateCapabilityAsset(context.Background(), 41, 17, CapabilityAssetInput{
		Title: "title", Summary: "summary", Description: "description", Status: CapabilityAssetStatusDraft,
	})

	require.True(t, errors.Is(err, ErrCapabilityAssetDraftNotFound))
}
