//go:build unit

package repository

import (
	"context"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

const ownerArchiveReason = "池主归档：保留池子、账号、账本和历史记录"

func TestDeleteSharedPoolTxIgnoresDriftedCachedUserCount(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &bizDecipherRepository{db: db}

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT lifecycle_state\s+FROM shared_pools\s+WHERE id = \$1 AND owner_id = \$2\s+FOR UPDATE`).
		WithArgs(int64(17), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"lifecycle_state"}).AddRow("active"))
	mock.ExpectQuery(`SELECT COUNT\(1\) FROM pool_seat_bindings WHERE pool_id = \$1 AND status IN \('active', 'held'\)`).
		WithArgs(int64(17)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`(?s)SELECT EXISTS.*shared_pool_usage_reservations`).
		WithArgs(int64(17)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec(`(?s)WITH retired AS.*UPDATE accounts.*INSERT INTO scheduler_outbox`).
		WithArgs(int64(17), int64(0), "account_changed").
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(`(?s)WITH binding AS.*UPDATE shared_pool_sub2_bindings.*UPDATE groups.*INSERT INTO scheduler_outbox`).
		WithArgs(int64(17), "group_changed").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)UPDATE shared_pools\s+SET lifecycle_state = 'archived'.*current_users = 0`).
		WithArgs(int64(17), int64(9), ownerArchiveReason).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)INSERT INTO shared_pool_archive_events.*VALUES \(\$1, \$2, 'archive', \$3, 'archived', \$4\)`).
		WithArgs(int64(17), int64(9), "active", ownerArchiveReason).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`(?s)UPDATE shared_pool_accounts.*WHERE pool_id = \$1 AND deleted_at IS NULL`).
		WithArgs(int64(17)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(`(?s)UPDATE shared_pool_access_keys.*WHERE pool_id = \$1 AND status = 'active'`).
		WithArgs(int64(17)).
		WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectCommit()

	require.NoError(t, repo.DeleteSharedPoolTx(context.Background(), 17, 9))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteSharedPoolTxBlocksOnlyOnAuthoritativeActiveSeats(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &bizDecipherRepository{db: db}

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT lifecycle_state\s+FROM shared_pools`).
		WithArgs(int64(17), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"lifecycle_state"}).AddRow("active"))
	mock.ExpectQuery(`SELECT COUNT\(1\) FROM pool_seat_bindings`).
		WithArgs(int64(17)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectRollback()

	err = repo.DeleteSharedPoolTx(context.Background(), 17, 9)
	require.ErrorContains(t, err, "2 个活跃成员")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteSharedPoolTxFailsClosedWhenActiveSeatCountFails(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &bizDecipherRepository{db: db}
	countErr := errors.New("seat count unavailable")

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT lifecycle_state\s+FROM shared_pools`).
		WithArgs(int64(17), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"lifecycle_state"}).AddRow("active"))
	mock.ExpectQuery(`SELECT COUNT\(1\) FROM pool_seat_bindings`).
		WithArgs(int64(17)).
		WillReturnError(countErr)
	mock.ExpectRollback()

	require.ErrorIs(t, repo.DeleteSharedPoolTx(context.Background(), 17, 9), countErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteSharedPoolTxBlocksUnsettledUsage(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &bizDecipherRepository{db: db}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT lifecycle_state").WithArgs(int64(17), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"lifecycle_state"}).AddRow("suspended"))
	mock.ExpectQuery("SELECT COUNT").WithArgs(int64(17)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT EXISTS").WithArgs(int64(17)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectRollback()
	require.ErrorContains(t, repo.DeleteSharedPoolTx(context.Background(), 17, 9), "在途调用")
	require.NoError(t, mock.ExpectationsWereMet())
}
