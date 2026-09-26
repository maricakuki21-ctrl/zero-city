//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSharedPoolManagedKeySurvivesAuthSnapshotWithoutPrefix(t *testing.T) {
	svc := &APIKeyService{}
	apiKey := &APIKey{
		ID:                91,
		UserID:            7,
		Key:               "sk-legacy-normal-looking",
		Name:              "历史共享池 Key",
		Status:            StatusAPIKeyActive,
		SharedPoolManaged: true,
		User:              &User{ID: 7, Status: StatusActive},
	}

	snapshot := svc.snapshotFromAPIKey(context.Background(), apiKey)
	require.NotNil(t, snapshot)
	require.True(t, snapshot.SharedPoolManaged)

	restored := svc.snapshotToAPIKey(apiKey.Key, snapshot)
	require.True(t, restored.SharedPoolManaged)
	require.True(t, IsSharedPoolAPIKey(restored))
}

func TestSharedPoolKeyPrefixRemainsFailClosedFallback(t *testing.T) {
	require.True(t, IsSharedPoolAPIKey(&APIKey{Key: " sk-share-legacy "}))
	require.False(t, IsSharedPoolAPIKey(&APIKey{Key: "sk-official"}))
	require.False(t, IsSharedPoolAPIKey(nil))
}
