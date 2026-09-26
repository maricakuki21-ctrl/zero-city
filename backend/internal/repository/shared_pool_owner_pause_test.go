package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSharedPoolOwnerPauseRejectsForeignAndStaleWrites(t *testing.T) {
	for _, name := range []string{"foreign", "stale"} {
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			repo := &bizDecipherRepository{db: db}
			mock.ExpectBegin()
			q := mock.ExpectQuery("SELECT config_version,owner_paused FROM shared_pools WHERE id=\\$1 AND owner_id=\\$2").
				WithArgs(int64(7), int64(19))
			if name == "foreign" {
				q.WillReturnError(sql.ErrNoRows)
			} else {
				q.WillReturnRows(sqlmock.NewRows([]string{"config_version", "owner_paused"}).AddRow(12, false))
			}
			mock.ExpectRollback()
			_, err = repo.SetSharedPoolOwnerPauseTx(context.Background(), 7, 19, 11, true)
			if name == "foreign" {
				require.ErrorIs(t, err, service.ErrPoolForbidden)
			} else {
				require.ErrorIs(t, err, service.ErrSharedPoolConcurrentUpdate)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSharedPoolOwnerPausePreservesMembersAndCredentials(t *testing.T) {
	for _, paused := range []bool{false, true} {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		repo := &bizDecipherRepository{db: db}
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT config_version,owner_paused").
			WithArgs(int64(7), int64(19)).WillReturnRows(sqlmock.NewRows([]string{"config_version", "owner_paused"}).AddRow(11, !paused))
		mock.ExpectExec("UPDATE shared_pools SET owner_paused=\\$3,listed=FALSE").
			WithArgs(int64(7), int64(19), paused).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery("SELECT .* FROM shared_pools WHERE id=\\$1 AND owner_id=\\$2").
			WithArgs(int64(7), int64(19)).WillReturnRows(sharedPoolUpdateResultRow(12))
		mock.ExpectCommit()
		_, err = repo.SetSharedPoolOwnerPauseTx(context.Background(), 7, 19, 11, paused)
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
		db.Close()
	}
}
