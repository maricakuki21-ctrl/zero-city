package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestAdminPoolInspectionOwnerLookupUsesNoPublicVisibilityGate(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery(`SELECT owner_id FROM shared_pools WHERE id = $1`).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"owner_id"}).AddRow(int64(12)))
	owner, err := (&bizDecipherRepository{db: db}).GetSharedPoolOwnerForAdmin(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, int64(12), owner)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminPoolInspectionMissingPoolReturnsLookupError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery("SELECT owner_id FROM shared_pools").WithArgs(int64(7)).WillReturnError(sql.ErrNoRows)
	_, err = (&bizDecipherRepository{db: db}).GetSharedPoolOwnerForAdmin(context.Background(), 7)
	require.ErrorIs(t, err, sql.ErrNoRows)
	require.NoError(t, mock.ExpectationsWereMet())
}
