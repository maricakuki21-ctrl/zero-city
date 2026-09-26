package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBizDecipherDecimalLedgerMigrationDefinesOneBalancedPersistentTruth(t *testing.T) {
	content, err := FS.ReadFile("238_bizdecipher_decimal_ledger.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))

	for _, fragment := range []string{
		"create table if not exists bizdecipher_ledger_journals",
		"event_id text not null unique",
		"idempotency_key text not null unique",
		"amount numeric(24,12)",
		"deferrable initially deferred",
		"create or replace function bizdecipher_assert_complete_journal()",
		"select count(*) from bizdecipher_ledger_entries",
		"before update or delete on bizdecipher_ledger_journals",
		"before update or delete on bizdecipher_ledger_entries",
		"create table if not exists bizdecipher_ledger_outbox_receipts",
		"durable consumer inbox receipts for canonicalusagefinalized outbox events",
		"references bizdecipher_ledger_journals(id) deferrable initially deferred",
		"before update or delete on bizdecipher_ledger_outbox_receipts",
		"post a reversal instead",
		"create table if not exists bizdecipher_ledger_holds",
		"create table if not exists bizdecipher_ledger_cutover_pending",
		"aborted_before_activation",
		"where state = 'canonical_open'",
		"create or replace function bizdecipher_rebuild_ledger_projections()",
		"sum(amount) as balance",
		"where state = 'reserved'",
		"create or replace view bizdecipher_owner_earnings",
	} {
		require.Contains(t, sql, fragment)
	}
	require.NotContains(t, sql, "double precision")
	require.NotContains(t, sql, " real ")
	require.NotContains(t, sql, "epsilon")
}
