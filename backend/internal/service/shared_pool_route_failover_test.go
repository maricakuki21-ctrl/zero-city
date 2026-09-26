package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSharedPoolRouteExclusionsKeepAccountAndPoolRoutesSeparate(t *testing.T) {
	ctx := context.Background()
	ctx = WithSharedPoolRouteExcluded(ctx, &SharedPoolAccessKey{ID: 41, AccountID: 17})
	ctx = WithSharedPoolRouteExcluded(ctx, &SharedPoolAccessKey{ID: 42, AccountID: 0})
	ctx = WithSharedPoolRouteExcluded(ctx, &SharedPoolAccessKey{ID: 41, AccountID: 17})

	accounts, accessKeys := SharedPoolRouteExclusions(ctx)
	require.Equal(t, []int64{17}, accounts)
	require.Equal(t, []int64{42}, accessKeys)
	require.Equal(t, 2, SharedPoolRouteFailoverCount(ctx))
	require.Equal(t, "request-1:route-failover-2", sharedPoolReservationAttemptBase(ctx, "request-1"))
}
