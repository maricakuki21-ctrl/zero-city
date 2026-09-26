package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"testing/fstest"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestValidateMigrationExecutionMode(t *testing.T) {
	t.Run("事务迁移包含CONCURRENTLY会被拒绝", func(t *testing.T) {
		nonTx, err := validateMigrationExecutionMode("001_add_idx.sql", "CREATE INDEX CONCURRENTLY idx_a ON t(a);")
		require.False(t, nonTx)
		require.Error(t, err)
	})

	t.Run("notx迁移要求CREATE使用IF NOT EXISTS", func(t *testing.T) {
		nonTx, err := validateMigrationExecutionMode("001_add_idx_notx.sql", "CREATE INDEX CONCURRENTLY idx_a ON t(a);")
		require.False(t, nonTx)
		require.Error(t, err)
	})

	t.Run("notx迁移要求DROP使用IF EXISTS", func(t *testing.T) {
		nonTx, err := validateMigrationExecutionMode("001_drop_idx_notx.sql", "DROP INDEX CONCURRENTLY idx_a;")
		require.False(t, nonTx)
		require.Error(t, err)
	})

	t.Run("notx迁移禁止事务控制语句", func(t *testing.T) {
		nonTx, err := validateMigrationExecutionMode("001_add_idx_notx.sql", "BEGIN; CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_a ON t(a); COMMIT;")
		require.False(t, nonTx)
		require.Error(t, err)
	})

	t.Run("notx迁移禁止混用非CONCURRENTLY语句", func(t *testing.T) {
		nonTx, err := validateMigrationExecutionMode("001_add_idx_notx.sql", "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_a ON t(a); UPDATE t SET a = 1;")
		require.False(t, nonTx)
		require.Error(t, err)
	})

	t.Run("notx迁移允许幂等并发索引语句", func(t *testing.T) {
		nonTx, err := validateMigrationExecutionMode("001_add_idx_notx.sql", `
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_a ON t(a);
DROP INDEX CONCURRENTLY IF EXISTS idx_b;
`)
		require.True(t, nonTx)
		require.NoError(t, err)
	})
}

func TestApplyMigrationsFS_NonTransactionalMigration(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	prepareMigrationsBootstrapExpectations(mock)
	mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
		WithArgs("001_add_idx_notx.sql").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_t_a ON t\\(a\\)").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations \\(filename, checksum\\) VALUES \\(\\$1, \\$2\\)").
		WithArgs("001_add_idx_notx.sql", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	expectMigrationsSessionCleanup(mock)

	fsys := fstest.MapFS{
		"001_add_idx_notx.sql": &fstest.MapFile{
			Data: []byte("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_t_a ON t(a);"),
		},
	}

	err = applyMigrationsFS(context.Background(), db, fsys)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyMigrationsFS_NonTransactionalMigration_MultiStatements(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	prepareMigrationsBootstrapExpectations(mock)
	mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
		WithArgs("001_add_multi_idx_notx.sql").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_t_a ON t\\(a\\)").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_t_b ON t\\(b\\)").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations \\(filename, checksum\\) VALUES \\(\\$1, \\$2\\)").
		WithArgs("001_add_multi_idx_notx.sql", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	expectMigrationsSessionCleanup(mock)

	fsys := fstest.MapFS{
		"001_add_multi_idx_notx.sql": &fstest.MapFile{
			Data: []byte(`
-- first
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_t_a ON t(a);
-- second
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_t_b ON t(b);
`),
		},
	}

	err = applyMigrationsFS(context.Background(), db, fsys)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyMigrationsFS_NonTransactionalMigration_LatestAPIKeyIPIndexDropsInvalidIndexBeforeRetry(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	prepareMigrationsBootstrapExpectations(mock)
	mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
		WithArgs(latestAPIKeyIPIndexMigration).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT EXISTS \\(").
		WithArgs(latestAPIKeyIPIndex).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectExec("DROP INDEX CONCURRENTLY IF EXISTS idx_usage_logs_api_key_latest_ip").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_api_key_latest_ip").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations \\(filename, checksum\\) VALUES \\(\\$1, \\$2\\)").
		WithArgs(latestAPIKeyIPIndexMigration, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	expectMigrationsSessionCleanup(mock)

	fsys := fstest.MapFS{
		latestAPIKeyIPIndexMigration: &fstest.MapFile{
			Data: []byte(`
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_api_key_latest_ip
    ON usage_logs (api_key_id, created_at DESC, id DESC)
    INCLUDE (ip_address)
    WHERE ip_address IS NOT NULL AND ip_address <> '';
`),
		},
	}

	err = applyMigrationsFS(context.Background(), db, fsys)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyMigrationsFS_SharedPoolNonTransactionalIndexesDropInvalidIndexesBeforeRetry(t *testing.T) {
	tests := []struct {
		name       string
		migration  string
		indexes    []string
		statements []string
	}{
		{
			name:      "price version indexes",
			migration: sharedPoolPriceVersionIndexesMigration,
			indexes:   sharedPoolPriceVersionIndexes,
			statements: []string{
				"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_shared_pool_owner_earnings_price_version ON shared_pool_owner_earnings_ledger(price_version_id) WHERE price_version_id IS NOT NULL",
				"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_shared_pool_balance_ledger_price_version ON shared_pool_balance_ledger(price_version_id) WHERE price_version_id IS NOT NULL",
				"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_shared_pool_usage_reservations_price_version ON shared_pool_usage_reservations(price_version_id) WHERE price_version_id IS NOT NULL",
			},
		},
		{
			name:      "routing hot path indexes",
			migration: sharedPoolRoutingHotPathIndexesMigration,
			indexes:   sharedPoolRoutingHotPathIndexes,
			statements: []string{
				"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_shared_pool_models_aliases_open_gin ON shared_pool_models USING GIN (model_aliases) WHERE enabled = TRUE AND model_open = TRUE",
				"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_shared_pool_accounts_schedulable_route ON shared_pool_accounts (pool_id, priority, full_check_score DESC, account_weight DESC, total_calls, last_used_at, id) WHERE deleted_at IS NULL AND schedulable = TRUE AND status IN ('active', 'limited', 'testing')",
			},
		},
		{
			name:      "compact capability index",
			migration: sharedPoolCompactCapabilityIndexMigration,
			indexes:   []string{sharedPoolCompactCapabilityIndex},
			statements: []string{
				"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_shared_pool_probe_jobs_compact_capability ON shared_pool_probe_jobs (pool_id, account_id, config_version, (LOWER(BTRIM(model_name))), (LOWER(BTRIM(upstream_model_name))), finished_at DESC NULLS LAST, created_at DESC, id DESC) WHERE status = 'succeeded'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()

			prepareMigrationsBootstrapExpectations(mock)
			mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
				WithArgs(tt.migration).
				WillReturnError(sql.ErrNoRows)
			for _, indexName := range tt.indexes {
				mock.ExpectQuery("SELECT EXISTS \\(").
					WithArgs(indexName).
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				mock.ExpectExec("DROP INDEX CONCURRENTLY IF EXISTS " + indexName).
					WillReturnResult(sqlmock.NewResult(0, 0))
			}
			for i := range tt.statements {
				mock.ExpectExec("CREATE INDEX CONCURRENTLY IF NOT EXISTS " + tt.indexes[i]).
					WillReturnResult(sqlmock.NewResult(0, 0))
			}
			mock.ExpectExec("INSERT INTO schema_migrations \\(filename, checksum\\) VALUES \\(\\$1, \\$2\\)").
				WithArgs(tt.migration, sqlmock.AnyArg()).
				WillReturnResult(sqlmock.NewResult(1, 1))
			expectMigrationsSessionCleanup(mock)

			content := ""
			for _, statement := range tt.statements {
				content += statement + ";\n"
			}
			fsys := fstest.MapFS{tt.migration: &fstest.MapFile{Data: []byte(content)}}

			err = applyMigrationsFS(context.Background(), db, fsys)
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestApplyMigrationsFS_SharedPoolNonTransactionalIndexesRecoverAfterInterruptedBuild(t *testing.T) {
	statements := []string{
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_shared_pool_owner_earnings_price_version ON shared_pool_owner_earnings_ledger(price_version_id) WHERE price_version_id IS NOT NULL",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_shared_pool_balance_ledger_price_version ON shared_pool_balance_ledger(price_version_id) WHERE price_version_id IS NOT NULL",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_shared_pool_usage_reservations_price_version ON shared_pool_usage_reservations(price_version_id) WHERE price_version_id IS NOT NULL",
	}
	content := ""
	for _, statement := range statements {
		content += statement + ";\n"
	}
	fsys := fstest.MapFS{
		sharedPoolPriceVersionIndexesMigration: &fstest.MapFile{Data: []byte(content)},
	}

	interruptedErr := errors.New("simulated concurrent index build interruption")
	func() {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		prepareMigrationsBootstrapExpectations(mock)
		mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
			WithArgs(sharedPoolPriceVersionIndexesMigration).
			WillReturnError(sql.ErrNoRows)
		for _, indexName := range sharedPoolPriceVersionIndexes {
			mock.ExpectQuery("SELECT EXISTS \\(").
				WithArgs(indexName).
				WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
		}
		mock.ExpectExec("CREATE INDEX CONCURRENTLY IF NOT EXISTS " + sharedPoolPriceVersionIndexes[0]).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec("CREATE INDEX CONCURRENTLY IF NOT EXISTS " + sharedPoolPriceVersionIndexes[1]).
			WillReturnError(interruptedErr)
		expectMigrationsSessionCleanup(mock)

		err = applyMigrationsFS(context.Background(), db, fsys)
		require.ErrorIs(t, err, interruptedErr)
		require.Contains(t, err.Error(), "non-tx statement 2")
		require.NoError(t, mock.ExpectationsWereMet())
	}()

	func() {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		prepareMigrationsBootstrapExpectations(mock)
		// The failed run must not have recorded the migration as complete.
		mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
			WithArgs(sharedPoolPriceVersionIndexesMigration).
			WillReturnError(sql.ErrNoRows)
		for i, indexName := range sharedPoolPriceVersionIndexes {
			invalid := i == 1
			mock.ExpectQuery("SELECT EXISTS \\(").
				WithArgs(indexName).
				WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(invalid))
			if invalid {
				mock.ExpectExec("DROP INDEX CONCURRENTLY IF EXISTS " + indexName).
					WillReturnResult(sqlmock.NewResult(0, 0))
			}
		}
		for i := range statements {
			mock.ExpectExec("CREATE INDEX CONCURRENTLY IF NOT EXISTS " + sharedPoolPriceVersionIndexes[i]).
				WillReturnResult(sqlmock.NewResult(0, 0))
		}
		mock.ExpectExec("INSERT INTO schema_migrations \\(filename, checksum\\) VALUES \\(\\$1, \\$2\\)").
			WithArgs(sharedPoolPriceVersionIndexesMigration, sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))
		expectMigrationsSessionCleanup(mock)

		err = applyMigrationsFS(context.Background(), db, fsys)
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	}()
}

func TestApplyMigrationsFS_PaymentOrdersOutTradeNoUniqueMigration_FailsFastOnDuplicatePrecheck(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	prepareMigrationsBootstrapExpectations(mock)
	mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
		WithArgs("120_enforce_payment_orders_out_trade_no_unique_notx.sql").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT out_trade_no, COUNT\\(\\*\\) AS duplicate_count FROM payment_orders").
		WillReturnRows(sqlmock.NewRows([]string{"out_trade_no", "duplicate_count"}).AddRow("dup-out-trade-no", 2))
	expectMigrationsSessionCleanup(mock)

	fsys := fstest.MapFS{
		"120_enforce_payment_orders_out_trade_no_unique_notx.sql": &fstest.MapFile{
			Data: []byte(`
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS paymentorder_out_trade_no_unique
    ON payment_orders (out_trade_no)
    WHERE out_trade_no <> '';

DROP INDEX CONCURRENTLY IF EXISTS paymentorder_out_trade_no;
`),
		},
	}

	err = applyMigrationsFS(context.Background(), db, fsys)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate out_trade_no")
	require.Contains(t, err.Error(), "dup-out-trade-no")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyMigrationsFS_PaymentOrdersOutTradeNoUniqueMigration_DropsInvalidIndexBeforeRetry(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	prepareMigrationsBootstrapExpectations(mock)
	mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
		WithArgs("120_enforce_payment_orders_out_trade_no_unique_notx.sql").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT out_trade_no, COUNT\\(\\*\\) AS duplicate_count FROM payment_orders").
		WillReturnRows(sqlmock.NewRows([]string{"out_trade_no", "duplicate_count"}))
	mock.ExpectQuery("SELECT EXISTS \\(").
		WithArgs("paymentorder_out_trade_no_unique").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectExec("DROP INDEX CONCURRENTLY IF EXISTS paymentorder_out_trade_no_unique").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS paymentorder_out_trade_no_unique").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DROP INDEX CONCURRENTLY IF EXISTS paymentorder_out_trade_no").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations \\(filename, checksum\\) VALUES \\(\\$1, \\$2\\)").
		WithArgs("120_enforce_payment_orders_out_trade_no_unique_notx.sql", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	expectMigrationsSessionCleanup(mock)

	fsys := fstest.MapFS{
		"120_enforce_payment_orders_out_trade_no_unique_notx.sql": &fstest.MapFile{
			Data: []byte(`
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS paymentorder_out_trade_no_unique
    ON payment_orders (out_trade_no)
    WHERE out_trade_no <> '';

DROP INDEX CONCURRENTLY IF EXISTS paymentorder_out_trade_no;
`),
		},
	}

	err = applyMigrationsFS(context.Background(), db, fsys)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyMigrationsFS_SchedulerOutboxPendingDedupKeyMigration_DropsInvalidIndexBeforeRetry(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	prepareMigrationsBootstrapExpectations(mock)
	mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
		WithArgs("153_scheduler_outbox_pending_dedup_key_index_notx.sql").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT EXISTS \\(").
		WithArgs("idx_scheduler_outbox_pending_dedup_key").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectExec("DROP INDEX CONCURRENTLY IF EXISTS idx_scheduler_outbox_pending_dedup_key").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_scheduler_outbox_pending_dedup_key").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations \\(filename, checksum\\) VALUES \\(\\$1, \\$2\\)").
		WithArgs("153_scheduler_outbox_pending_dedup_key_index_notx.sql", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	expectMigrationsSessionCleanup(mock)

	fsys := fstest.MapFS{
		"153_scheduler_outbox_pending_dedup_key_index_notx.sql": &fstest.MapFile{
			Data: []byte(`
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_scheduler_outbox_pending_dedup_key
    ON scheduler_outbox (dedup_key)
    WHERE dedup_key IS NOT NULL;
`),
		},
	}

	err = applyMigrationsFS(context.Background(), db, fsys)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyMigrationsFS_TransactionalMigration(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	// The advisory lock and all migration work must share one session. This also
	// proves startup cannot self-deadlock when deployments cap the pool at one.
	db.SetMaxOpenConns(1)

	prepareMigrationsBootstrapExpectations(mock)
	mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
		WithArgs("001_add_col.sql").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectExec("ALTER TABLE t ADD COLUMN name TEXT").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations \\(filename, checksum\\) VALUES \\(\\$1, \\$2\\)").
		WithArgs("001_add_col.sql", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	expectMigrationsSessionCleanup(mock)

	fsys := fstest.MapFS{
		"001_add_col.sql": &fstest.MapFile{
			Data: []byte("ALTER TABLE t ADD COLUMN name TEXT;"),
		},
	}

	err = applyMigrationsFS(context.Background(), db, fsys)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func prepareMigrationsBootstrapExpectations(mock sqlmock.Sqlmock) {
	mock.ExpectExec(migrationsSessionSetLockTimeoutSQL).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT pg_try_advisory_lock\\(\\$1\\)").
		WithArgs(migrationsAdvisoryLockID).
		WillReturnRows(sqlmock.NewRows([]string{"pg_try_advisory_lock"}).AddRow(true))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS schema_migrations").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT EXISTS \\(").
		WithArgs("schema_migrations").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery("SELECT EXISTS \\(").
		WithArgs("atlas_schema_revisions").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM atlas_schema_revisions").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
}

func expectMigrationsSessionCleanup(mock sqlmock.Sqlmock) {
	mock.ExpectExec("SELECT pg_advisory_unlock\\(\\$1\\)").
		WithArgs(migrationsAdvisoryLockID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(migrationsSessionResetLockTimeoutSQL).
		WillReturnResult(sqlmock.NewResult(0, 0))
}
