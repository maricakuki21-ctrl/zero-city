package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

// v2 excludes legacy channel-table price snapshots. A namespace bump prevents
// a deployment from briefly serving a stale v1 price after BillingService
// becomes the only canonical source for shared-pool display pricing.
const sharedPoolDisplayCacheKeyPrefix = "bizdecipher:shared_pool:display:v2"

var errSharedPoolDisplayCacheUnavailable = errors.New("shared pool display cache unavailable")

type sharedPoolDisplayCache struct {
	rdb       *redis.Client
	keyPrefix string
}

type sharedPoolDisplayFilterCacheKey struct {
	Keyword         string  `json:"keyword"`
	Model           string  `json:"model"`
	Status          string  `json:"status"`
	View            string  `json:"view"`
	Lifecycle       string  `json:"lifecycle"`
	MinAvailability float64 `json:"min_availability"`
	SortBy          string  `json:"sort_by"`
	IncludeUnlisted bool    `json:"include_unlisted"`
	Limit           int     `json:"limit"`
}

func NewSharedPoolDisplayCache(rdb *redis.Client, cfg *config.Config) service.SharedPoolDisplayCache {
	prefix := "sub2api:"
	if cfg != nil {
		prefix = strings.TrimSpace(cfg.Dashboard.KeyPrefix)
	}
	if prefix != "" && !strings.HasSuffix(prefix, ":") {
		prefix += ":"
	}
	return &sharedPoolDisplayCache{rdb: rdb, keyPrefix: prefix}
}

func (c *sharedPoolDisplayCache) GetSharedPoolList(ctx context.Context, filter service.SharedPoolFilter) (*service.SharedPoolListView, error) {
	if c == nil || c.rdb == nil {
		return nil, errSharedPoolDisplayCacheUnavailable
	}
	data, err := c.rdb.Get(ctx, c.listKey(filter)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, service.ErrSharedPoolDisplayCacheMiss
		}
		return nil, err
	}
	var view service.SharedPoolListView
	if err := json.Unmarshal(data, &view); err != nil {
		return nil, err
	}
	return &view, nil
}

func (c *sharedPoolDisplayCache) SetSharedPoolList(ctx context.Context, filter service.SharedPoolFilter, view *service.SharedPoolListView, ttl time.Duration) error {
	if c == nil || c.rdb == nil {
		return errSharedPoolDisplayCacheUnavailable
	}
	if view == nil {
		return nil
	}
	data, err := json.Marshal(view)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, c.listKey(filter), data, ttl).Err()
}

func (c *sharedPoolDisplayCache) GetSharedPool(ctx context.Context, id int64) (*service.SharedPool, error) {
	if c == nil || c.rdb == nil {
		return nil, errSharedPoolDisplayCacheUnavailable
	}
	if id <= 0 {
		return nil, service.ErrSharedPoolDisplayCacheMiss
	}
	data, err := c.rdb.Get(ctx, c.detailKey(id)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, service.ErrSharedPoolDisplayCacheMiss
		}
		return nil, err
	}
	var pool service.SharedPool
	if err := json.Unmarshal(data, &pool); err != nil {
		return nil, err
	}
	return &pool, nil
}

func (c *sharedPoolDisplayCache) SetSharedPool(ctx context.Context, pool *service.SharedPool, ttl time.Duration) error {
	if c == nil || c.rdb == nil {
		return errSharedPoolDisplayCacheUnavailable
	}
	if pool == nil || pool.ID <= 0 {
		return nil
	}
	data, err := json.Marshal(pool)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, c.detailKey(pool.ID), data, ttl).Err()
}

func (c *sharedPoolDisplayCache) FlushSharedPoolDisplay(ctx context.Context) error {
	if c == nil || c.rdb == nil {
		return errSharedPoolDisplayCacheUnavailable
	}
	pattern := c.buildKey(sharedPoolDisplayCacheKeyPrefix + ":*")
	iter := c.rdb.Scan(ctx, 0, pattern, 100).Iterator()
	keys := make([]string, 0, 100)
	flush := func() error {
		if len(keys) == 0 {
			return nil
		}
		if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
			return err
		}
		keys = keys[:0]
		return nil
	}
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
		if len(keys) >= 100 {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := iter.Err(); err != nil {
		return err
	}
	return flush()
}

func (c *sharedPoolDisplayCache) listKey(filter service.SharedPoolFilter) string {
	payload, err := json.Marshal(normalizeSharedPoolDisplayFilterCacheKey(filter))
	if err != nil {
		return c.buildKey(sharedPoolDisplayCacheKeyPrefix + ":list:error")
	}
	sum := sha256.Sum256(payload)
	return c.buildKey(fmt.Sprintf("%s:list:%s", sharedPoolDisplayCacheKeyPrefix, hex.EncodeToString(sum[:])))
}

func (c *sharedPoolDisplayCache) detailKey(id int64) string {
	return c.buildKey(sharedPoolDisplayCacheKeyPrefix + ":detail:" + strconv.FormatInt(id, 10))
}

func (c *sharedPoolDisplayCache) buildKey(key string) string {
	if c == nil || c.keyPrefix == "" {
		return key
	}
	return c.keyPrefix + key
}

func normalizeSharedPoolDisplayFilterCacheKey(filter service.SharedPoolFilter) sharedPoolDisplayFilterCacheKey {
	return sharedPoolDisplayFilterCacheKey{
		Keyword:         strings.ToLower(strings.TrimSpace(filter.Keyword)),
		Model:           normalizeSharedPoolDisplayAllFilter(filter.Model),
		Status:          normalizeSharedPoolDisplayAllFilter(filter.Status),
		View:            normalizeSharedPoolDisplayView(filter.View),
		Lifecycle:       normalizeSharedPoolDisplayAllFilter(strings.ToLower(filter.Lifecycle)),
		MinAvailability: filter.MinAvailability,
		SortBy:          normalizeSharedPoolDisplaySort(filter.SortBy),
		IncludeUnlisted: filter.IncludeUnlisted,
		Limit:           clampSharedPoolDisplayCacheLimit(filter.Limit),
	}
}

func normalizeSharedPoolDisplayAllFilter(value string) string {
	value = strings.TrimSpace(value)
	if value == "all" {
		return ""
	}
	return value
}

func normalizeSharedPoolDisplayView(view string) string {
	switch strings.TrimSpace(strings.ToLower(view)) {
	case "observation":
		return "observation"
	default:
		return "public"
	}
}

func normalizeSharedPoolDisplaySort(sortBy string) string {
	switch strings.TrimSpace(sortBy) {
	case "availability", "rate", "latency", "users", "newest":
		return strings.TrimSpace(sortBy)
	case "score", "weight", "recommended", "":
		return "recommended"
	default:
		return "recommended"
	}
}

func clampSharedPoolDisplayCacheLimit(v int) int {
	if v <= 0 {
		return 50
	}
	if v > 200 {
		return 200
	}
	return v
}
