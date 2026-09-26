package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type antigravityIsolationCache struct {
	GeminiTokenCache
	deleted []string
	err     error
}

func (c *antigravityIsolationCache) DeleteAccessToken(_ context.Context, key string) error {
	c.deleted = append(c.deleted, key)
	return c.err
}

func TestAntigravityTokenIsolation_SharedProject(t *testing.T) {
	first := &Account{ID: 1, Credentials: map[string]any{"project_id": "shared"}}
	second := &Account{ID: 2, Credentials: map[string]any{"project_id": "shared"}}
	require.NotEqual(t, AntigravityTokenCacheKey(first), AntigravityTokenCacheKey(second))
	before := AntigravityTokenCacheKey(first)
	first.Credentials["project_id"] = "backfilled"
	require.Equal(t, before, AntigravityTokenCacheKey(first))
}

func TestAntigravityTokenIsolation_InvalidatesLegacyAndAccountKeys(t *testing.T) {
	for _, project := range []string{" shared ", "", "  "} {
		for _, failDelete := range []bool{false, true} {
			cache := &antigravityIsolationCache{}
			if failDelete {
				cache.err = errors.New("cache unavailable")
			}
			account := &Account{ID: 1, Type: AccountTypeOAuth, Platform: PlatformAntigravity,
				Credentials: map[string]any{"project_id": project}}
			err := NewCompositeTokenCacheInvalidator(cache).InvalidateToken(context.Background(), account)
			require.NoError(t, err)
			if project == " shared " {
				require.Equal(t, []string{"ag:shared", "ag:account:1"}, cache.deleted)
			} else {
				require.Equal(t, []string{"ag:account:1"}, cache.deleted)
			}
			require.NotContains(t, cache.deleted, "ag:account:2")
		}
	}
}
