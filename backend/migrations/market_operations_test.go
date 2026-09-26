package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarketOperationsMigrationsStayScoped(t *testing.T) {
	data, err := FS.ReadFile("276_correct_tron_usdt_contract.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(data))
	require.Contains(t, sql, "where key = 'payment_crypto_contract_address'")
	require.Contains(t, sql, "and value = 'txlaq63xg1nazckpwkhvzw7csemlmeqcdj'")
	require.NotContains(t, sql, "update users")
	require.NotContains(t, sql, "update payment_orders")
	require.NotContains(t, sql, "payment_crypto_wallet_confirmed_address")

	data, err = FS.ReadFile("277_marketplace_onboarding.sql")
	require.NoError(t, err)
	sql = strings.ToLower(string(data))
	require.Contains(t, sql, "primary key (user_id, version)")
	require.Contains(t, sql, "references users(id)")
	require.Contains(t, sql, "check (intent in ('hire', 'sell', 'browse'))")

	data, err = FS.ReadFile("278_shared_pool_owner_pause.sql")
	require.NoError(t, err)
	sql = strings.ToLower(string(data))
	require.Contains(t, sql, "owner_paused boolean not null default false")
	require.Contains(t, sql, "before insert or update")
	require.Contains(t, sql, "new.listed := false")
	require.Contains(t, sql, "new.lifecycle_state <> 'archived'")
	require.NotContains(t, sql, "delete from")
	require.NotContains(t, sql, "update users")
}
