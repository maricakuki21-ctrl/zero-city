package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSharedPoolSafeAccountAliasIsStableAndOpaque(t *testing.T) {
	alias := SharedPoolSafeAccountAlias(991, 7731)
	require.Equal(t, alias, SharedPoolSafeAccountAlias(991, 7731))
	require.Regexp(t, `^共享线路 [0-9A-F]{6}$`, alias)
	require.NotContains(t, alias, "991")
	require.NotContains(t, alias, "7731")
	require.NotEqual(t, alias, SharedPoolSafeAccountAlias(991, 7732))
}

func TestNormalizeSharedPoolUsageTraceRejectsCredentialBearingEndpointAndRawStage(t *testing.T) {
	trace := SharedPoolUsageTrace{
		AccessKeyID:       4,
		PoolID:            8,
		UserID:            15,
		RequestID:         " req-safe ",
		Endpoint:          "https://secret.example/v1/responses?api_key=leak",
		FailureStage:      "upstream: secret raw error",
		SettlementOutcome: "unexpected",
		InputTokens:       -5,
		AccountAlias:      strings.Repeat("A", 80),
	}
	require.NoError(t, normalizeSharedPoolUsageTrace(&trace, true))
	require.Equal(t, "req-safe", trace.RequestID)
	require.Empty(t, trace.Endpoint)
	require.Empty(t, trace.FailureStage)
	require.Equal(t, SharedPoolSettlementUnknown, trace.SettlementOutcome)
	require.Zero(t, trace.InputTokens)
	require.Len(t, []rune(trace.AccountAlias), 64)
}

func TestNormalizeSharedPoolUsageTraceKeepsOnlyPathWithoutQuery(t *testing.T) {
	trace := SharedPoolUsageTrace{
		AccessKeyID: 4,
		PoolID:      8,
		UserID:      15,
		RequestID:   "req-path",
		Endpoint:    "/v1/responses?debug=true",
	}
	require.NoError(t, normalizeSharedPoolUsageTrace(&trace, false))
	require.Equal(t, "/v1/responses", trace.Endpoint)
	require.Equal(t, SharedPoolSettlementPending, trace.SettlementOutcome)
}
