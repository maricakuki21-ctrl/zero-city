package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBizDecipherOpeningBalanceProjectionAppliesOnlyOnFirstVerifiedCanonicalOpen(t *testing.T) {
	content, err := FS.ReadFile("239_bizdecipher_opening_balance_import.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))

	triggerStart := strings.Index(sql, "create or replace function sync_bizdecipher_cutover_import_state()")
	require.NotEqual(t, -1, triggerStart)
	triggerSQL := sql[triggerStart:]

	require.Contains(t, triggerSQL, "elsif new.phase = 'canonical_open' and old.phase <> new.phase then")
	require.Contains(t, triggerSQL, "with opened_batches as (")
	require.Contains(t, triggerSQL, "set state = 'canonical_open'")
	require.Contains(t, triggerSQL, "and state = 'verified'")
	require.Contains(t, triggerSQL, "returning batch_id")
	require.Contains(t, triggerSQL, "from opened_batches batch")
	require.Contains(t, triggerSQL, "on conflict (account_id) do update")
	require.NotContains(t, triggerSQL, "from bizdecipher_opening_balance_batches batch\n        join bizdecipher_opening_balance_items")
}
