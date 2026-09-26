package migrations

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTask19IsolatedOpeningBalanceRehearsals(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("..", "..", "deploy", "migration", "bizdecipher", "fixture.sql"))
	require.NoError(t, err)
	mig237, err := FS.ReadFile("237_shared_pool_cutover_control.sql")
	require.NoError(t, err)
	mig238, err := FS.ReadFile("238_bizdecipher_decimal_ledger.sql")
	require.NoError(t, err)
	mig239, err := FS.ReadFile("239_bizdecipher_opening_balance_import.sql")
	require.NoError(t, err)

	for i := 1; i <= 3; i++ {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			db := startWorkbenchPostgres(t)
			_, err := db.Exec(string(fixture))
			require.NoError(t, err)
			tx, err := db.Begin()
			require.NoError(t, err)
			_, err = tx.Exec(string(mig237))
			require.NoError(t, err)
			require.NoError(t, tx.Commit())
			_, err = db.Exec(string(mig238))
			require.NoError(t, err)
			_, err = db.Exec(string(mig239))
			require.NoError(t, err)

			var amount string
			require.NoError(t, db.QueryRow(`SELECT amount::text FROM legacy_opening_balances WHERE id = 'balance-small'`).Scan(&amount))
			require.Equal(t, "0.000100000000", amount)

			var fnCount int
			require.NoError(t, db.QueryRow(`SELECT count(*) FROM pg_proc WHERE proname IN ('bizdecipher_import_opening_balances','bizdecipher_verify_opening_balances')`).Scan(&fnCount))
			require.Equal(t, 2, fnCount)

			_, err = db.Exec(`SELECT 1 FROM shared_pool_cutover_attempts LIMIT 0`)
			require.NoError(t, err)
			_, err = db.Exec(`SELECT 1 FROM bizdecipher_ledger_accounts LIMIT 0`)
			require.NoError(t, err)
			_, err = db.Exec(`SELECT 1 FROM bizdecipher_opening_balance_batches LIMIT 0`)
			require.NoError(t, err)
		})
	}
}
