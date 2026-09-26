package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyFromService_MapsSharedResources(t *testing.T) {
	key := &service.APIKey{
		ID: 9, SharedPoolManaged: true,
		SharedPoolResources: []service.APIKeyPoolResource{
			{ID: 3, Name: "First pool", Status: "active"},
			{ID: 4, Name: "Second pool", Status: "disabled"},
		},
	}
	out := APIKeyFromService(key)
	require.True(t, out.SharedPoolManaged)
	require.Equal(t, key.SharedPoolResources, out.SharedPoolResources)
	require.False(t, APIKeyFromService(&service.APIKey{ID: 10}).SharedPoolManaged)
}
