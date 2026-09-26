package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestSharedPoolDisplayCacheKeyPrefix(t *testing.T) {
	cache := NewSharedPoolDisplayCache(nil, &config.Config{
		Dashboard: config.DashboardCacheConfig{KeyPrefix: "prod"},
	})
	impl, ok := cache.(*sharedPoolDisplayCache)
	require.True(t, ok)
	require.Equal(t, "prod:", impl.keyPrefix)

	cache = NewSharedPoolDisplayCache(nil, &config.Config{
		Dashboard: config.DashboardCacheConfig{KeyPrefix: "staging:"},
	})
	impl, ok = cache.(*sharedPoolDisplayCache)
	require.True(t, ok)
	require.Equal(t, "staging:", impl.keyPrefix)
}

func TestSharedPoolDisplayCacheListKeyNormalizesEquivalentFilters(t *testing.T) {
	cache := &sharedPoolDisplayCache{keyPrefix: "prod:"}
	base := cache.listKey(service.SharedPoolFilter{
		Keyword: "  GPT  ",
		Model:   "all",
		Status:  "all",
		SortBy:  "score",
		Limit:   0,
	})
	variant := cache.listKey(service.SharedPoolFilter{
		Keyword: "gpt",
		Model:   "",
		Status:  "",
		SortBy:  "recommended",
		Limit:   50,
	})
	require.Equal(t, base, variant)

	differentModel := cache.listKey(service.SharedPoolFilter{
		Keyword: "gpt",
		Model:   "gpt-5.5",
		SortBy:  "recommended",
		Limit:   50,
	})
	require.NotEqual(t, base, differentModel)

	currentLifecycle := cache.listKey(service.SharedPoolFilter{
		Keyword: "gpt", SortBy: "recommended", Limit: 50, Lifecycle: " current ",
	})
	archivedLifecycle := cache.listKey(service.SharedPoolFilter{
		Keyword: "gpt", SortBy: "recommended", Limit: 50, Lifecycle: "ARCHIVED",
	})
	allLifecycle := cache.listKey(service.SharedPoolFilter{
		Keyword: "gpt", SortBy: "recommended", Limit: 50, Lifecycle: "all",
	})
	require.NotEqual(t, currentLifecycle, archivedLifecycle, "admin lifecycle pages must never share a cached list")
	require.Equal(t, base, allLifecycle, "all and empty lifecycle have identical repository semantics")
}

func TestSharedPoolDisplayCacheReadWriteAndFlush(t *testing.T) {
	ctx := context.Background()
	srv := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	cache := NewSharedPoolDisplayCache(rdb, &config.Config{
		Dashboard: config.DashboardCacheConfig{KeyPrefix: "test"},
	})
	impl, ok := cache.(*sharedPoolDisplayCache)
	require.True(t, ok)

	filter := service.SharedPoolFilter{Keyword: "gpt", SortBy: "recommended", Limit: 10}
	_, err := cache.GetSharedPoolList(ctx, filter)
	require.True(t, errors.Is(err, service.ErrSharedPoolDisplayCacheMiss))

	view := &service.SharedPoolListView{
		Pools: []service.SharedPool{{ID: 7, Name: "Open Pool", Models: []string{"gpt-5.5"}}},
		Total: 1,
	}
	require.NoError(t, cache.SetSharedPoolList(ctx, filter, view, time.Minute))
	gotList, err := cache.GetSharedPoolList(ctx, filter)
	require.NoError(t, err)
	require.Equal(t, view.Total, gotList.Total)
	require.Equal(t, "Open Pool", gotList.Pools[0].Name)

	pool := &service.SharedPool{ID: 9, Name: "Detail Pool", Models: []string{"gpt-5.5"}}
	require.NoError(t, cache.SetSharedPool(ctx, pool, time.Minute))
	gotPool, err := cache.GetSharedPool(ctx, 9)
	require.NoError(t, err)
	require.Equal(t, pool.Name, gotPool.Name)

	require.NoError(t, rdb.Set(ctx, "test:unrelated:key", "keep", time.Minute).Err())
	require.NoError(t, cache.FlushSharedPoolDisplay(ctx))
	_, err = cache.GetSharedPoolList(ctx, filter)
	require.True(t, errors.Is(err, service.ErrSharedPoolDisplayCacheMiss))
	_, err = cache.GetSharedPool(ctx, 9)
	require.True(t, errors.Is(err, service.ErrSharedPoolDisplayCacheMiss))
	kept, err := rdb.Get(ctx, "test:unrelated:key").Result()
	require.NoError(t, err)
	require.Equal(t, "keep", kept)
	require.Equal(t, "test:"+sharedPoolDisplayCacheKeyPrefix+":detail:9", impl.detailKey(9))
}
