package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type modelCapabilityRepoStub struct {
	BizDecipherRepository
	lastID    int64
	lastInput ModelCatalogProfileUpdate
	entry     *ModelCatalogEntry
}

func (r *modelCapabilityRepoStub) UpdateModelCatalogProfile(
	_ context.Context,
	id int64,
	input ModelCatalogProfileUpdate,
) (*ModelCatalogEntry, error) {
	r.lastID = id
	r.lastInput = input
	return r.entry, nil
}

func TestUpdateModelCatalogProfileNormalizesAndForwards(t *testing.T) {
	orchestrator := true
	window := 400000
	repo := &modelCapabilityRepoStub{entry: &ModelCatalogEntry{ID: 7, AdapterKind: "openai_responses"}}
	svc := NewBizDecipherService(repo, nil, nil)

	entry, err := svc.UpdateModelCatalogProfile(context.Background(), 7, ModelCapabilityProfileInput{
		ModalitiesIn:  []string{" Text ", "IMAGE", "image"},
		ModalitiesOut: []string{"video"},
		AdapterKind:   " OpenAI_Responses ",
		ContextWindow: &window,
		Orchestrator:  &orchestrator,
		RuntimeRole:   "ORCHESTRATOR",
	})

	require.NoError(t, err)
	require.Equal(t, int64(7), repo.lastID)
	require.Equal(t, []string{"text", "image"}, repo.lastInput.ModalitiesIn)
	require.Equal(t, []string{"video"}, repo.lastInput.ModalitiesOut)
	require.Equal(t, "openai_responses", repo.lastInput.AdapterKind)
	require.Equal(t, "orchestrator", repo.lastInput.RuntimeRole)
	require.Equal(t, 400000, *repo.lastInput.ContextWindow)
	require.True(t, *repo.lastInput.Orchestrator)
	require.Equal(t, "openai_responses", entry.AdapterKind)
}

func TestUpdateModelCatalogProfileRejectsUnknownValues(t *testing.T) {
	repo := &modelCapabilityRepoStub{entry: &ModelCatalogEntry{ID: 7}}
	svc := NewBizDecipherService(repo, nil, nil)

	tests := []struct {
		name  string
		input ModelCapabilityProfileInput
	}{
		{name: "unknown modality", input: ModelCapabilityProfileInput{ModalitiesOut: []string{"hologram"}}},
		{name: "unknown adapter", input: ModelCapabilityProfileInput{AdapterKind: "telepathy"}},
		{name: "unknown role", input: ModelCapabilityProfileInput{RuntimeRole: "wizard"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := svc.UpdateModelCatalogProfile(context.Background(), 7, test.input)
			require.Error(t, err)
		})
	}
	require.Zero(t, repo.lastID)
}

func TestUpdateModelCatalogProfileRejectsOutOfRangeContextWindow(t *testing.T) {
	repo := &modelCapabilityRepoStub{entry: &ModelCatalogEntry{ID: 7}}
	svc := NewBizDecipherService(repo, nil, nil)

	negative := -1
	_, err := svc.UpdateModelCatalogProfile(context.Background(), 7, ModelCapabilityProfileInput{
		ContextWindow: &negative,
	})
	require.Error(t, err)

	huge := 20_000_000
	_, err = svc.UpdateModelCatalogProfile(context.Background(), 7, ModelCapabilityProfileInput{
		ContextWindow: &huge,
	})
	require.Error(t, err)
	require.Zero(t, repo.lastID)
}
