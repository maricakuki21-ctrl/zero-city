package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestSub2SessionWindowAtomicOwnership(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := &gatewayCache{rdb: client}
	ctx := context.Background()
	previous, err := cache.ClaimOpenAIResponsesSessionWindow(ctx, 7, "scope", []byte("first"), time.Minute)
	require.NoError(t, err)
	require.Empty(t, previous)
	previous, err = cache.ClaimOpenAIResponsesSessionWindow(ctx, 7, "scope", []byte("second"), time.Minute)
	require.NoError(t, err)
	require.Equal(t, "first", string(previous))

	owned, err := cache.CompareAndRefreshOpenAIResponsesSessionWindow(ctx, 7, "scope", []byte("first"), time.Hour)
	require.NoError(t, err)
	require.False(t, owned)
	deleted, err := cache.CompareAndDeleteOpenAIResponsesSessionWindow(ctx, 7, "scope", []byte("first"))
	require.NoError(t, err)
	require.False(t, deleted, "late old-owner cleanup must not delete the replacement")
	previous, err = cache.ClaimOpenAIResponsesSessionWindow(ctx, 8, "scope", []byte("other-group"), time.Hour)
	require.NoError(t, err)
	require.Empty(t, previous)

	server.FastForward(45 * time.Second)
	owned, err = cache.CompareAndRefreshOpenAIResponsesSessionWindow(ctx, 7, "scope", []byte("second"), time.Minute)
	require.NoError(t, err)
	require.True(t, owned)
	server.FastForward(30 * time.Second)
	require.True(t, server.Exists(buildOpenAIResponsesSessionWindowKey(7, "scope")))
	deleted, err = cache.CompareAndDeleteOpenAIResponsesSessionWindow(ctx, 7, "scope", []byte("second"))
	require.NoError(t, err)
	require.True(t, deleted)
	require.True(t, server.Exists(buildOpenAIResponsesSessionWindowKey(8, "scope")))
	server.FastForward(time.Hour)
	owned, err = cache.CompareAndRefreshOpenAIResponsesSessionWindow(ctx, 8, "scope", []byte("other-group"), time.Hour)
	require.NoError(t, err)
	require.False(t, owned, "refresh must not recreate an expired claim")
}

func TestSub2SessionWindowRejectsInvalidClaims(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := &gatewayCache{rdb: client}
	ctx := context.Background()
	_, err := cache.ClaimOpenAIResponsesSessionWindow(ctx, 0, "scope", []byte("owner"), time.Minute)
	require.Error(t, err)
	_, err = cache.ClaimOpenAIResponsesSessionWindow(ctx, 7, "scope", nil, time.Minute)
	require.Error(t, err)
	_, err = cache.ClaimOpenAIResponsesSessionWindow(ctx, 7, " ", []byte("owner"), time.Minute)
	require.Error(t, err)
	_, err = cache.ClaimOpenAIResponsesSessionWindow(ctx, 7, "scope", []byte("owner"), time.Nanosecond)
	require.Error(t, err)
	_, err = cache.CompareAndRefreshOpenAIResponsesSessionWindow(ctx, 7, "scope", []byte("owner"), time.Nanosecond)
	require.Error(t, err)
	_, err = cache.CompareAndDeleteOpenAIResponsesSessionWindow(ctx, 7, "", []byte("owner"))
	require.Error(t, err)
	require.Empty(t, server.Keys())
}
