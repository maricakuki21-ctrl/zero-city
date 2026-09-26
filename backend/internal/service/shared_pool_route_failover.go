package service

import (
	"context"
	"fmt"
	"strings"
)

type sharedPoolRouteExclusions struct {
	accountIDs   []int64
	accessKeyIDs []int64
}

type sharedPoolRouteExclusionsContextKey struct{}

// WithSharedPoolRouteExcluded records a route that returned a verified
// retryable upstream failure. The next lookup skips only that account (or the
// pool-level access key), so one combination key can fail over serially without
// ever broadcasting one request to several pools.
func WithSharedPoolRouteExcluded(ctx context.Context, accessKey *SharedPoolAccessKey) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	state, _ := ctx.Value(sharedPoolRouteExclusionsContextKey{}).(sharedPoolRouteExclusions)
	state = cloneSharedPoolRouteExclusions(state)
	if accessKey != nil {
		if accessKey.AccountID > 0 {
			state.accountIDs = appendUniquePositiveInt64(state.accountIDs, accessKey.AccountID)
		} else if accessKey.ID > 0 {
			state.accessKeyIDs = appendUniquePositiveInt64(state.accessKeyIDs, accessKey.ID)
		}
	}
	return context.WithValue(ctx, sharedPoolRouteExclusionsContextKey{}, state)
}

// SharedPoolRouteExclusions returns bounded copies for repository query
// construction. Callers must treat the result as read-only.
func SharedPoolRouteExclusions(ctx context.Context) (accountIDs, accessKeyIDs []int64) {
	if ctx == nil {
		return nil, nil
	}
	state, _ := ctx.Value(sharedPoolRouteExclusionsContextKey{}).(sharedPoolRouteExclusions)
	return append([]int64(nil), state.accountIDs...), append([]int64(nil), state.accessKeyIDs...)
}

// SharedPoolRouteFailoverCount is used to cap internal sequential attempts.
func SharedPoolRouteFailoverCount(ctx context.Context) int {
	accounts, accessKeys := SharedPoolRouteExclusions(ctx)
	return len(accounts) + len(accessKeys)
}

func sharedPoolReservationAttemptBase(ctx context.Context, base string) string {
	base = strings.TrimSpace(base)
	count := SharedPoolRouteFailoverCount(ctx)
	if count <= 0 {
		return base
	}
	return fmt.Sprintf("%s:route-failover-%d", base, count)
}

func cloneSharedPoolRouteExclusions(state sharedPoolRouteExclusions) sharedPoolRouteExclusions {
	return sharedPoolRouteExclusions{
		accountIDs:   append([]int64(nil), state.accountIDs...),
		accessKeyIDs: append([]int64(nil), state.accessKeyIDs...),
	}
}

func appendUniquePositiveInt64(values []int64, value int64) []int64 {
	if value <= 0 {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
