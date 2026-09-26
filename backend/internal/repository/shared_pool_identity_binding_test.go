package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestResolveSharedPoolCanonicalIdentityReturnsBoundGroupAndAccount(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery("SELECT (.+) FROM shared_pool_sub2_bindings").
		WithArgs(int64(41), int64(7), "pool_account", int64(19)).
		WillReturnRows(sqlmock.NewRows([]string{
			"pool_id", "owner_id", "canonical_group_id", "canonical_account_id", "lifecycle",
		}).AddRow(int64(41), int64(7), int64(101), int64(202), "active"))

	repo := sharedPoolIdentityResolver{db: db}
	identity, err := repo.ResolveSharedPoolCanonicalIdentity(context.Background(), service.SharedPoolIdentityQuery{
		PoolID:  41,
		OwnerID: 7,
		Supply: &service.SharedPoolSupplyReference{
			Kind: service.SharedPoolSupplyPoolAccount,
			ID:   19,
		},
	})
	require.NoError(t, err)
	require.Equal(t, service.CanonicalGroupID(101), identity.GroupID)
	require.NotNil(t, identity.AccountID)
	require.Equal(t, service.CanonicalAccountID(202), *identity.AccountID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestResolveSharedPoolCanonicalIdentityRejectsMissingBinding(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery("SELECT (.+) FROM shared_pool_sub2_bindings").
		WithArgs(int64(77), int64(9), "", int64(0)).
		WillReturnError(sql.ErrNoRows)

	repo := sharedPoolIdentityResolver{db: db}
	_, err = repo.ResolveSharedPoolCanonicalIdentity(context.Background(), service.SharedPoolIdentityQuery{
		PoolID:  77,
		OwnerID: 9,
	})
	require.ErrorIs(t, err, service.ErrSharedPoolIdentityUnmapped)
	require.NoError(t, mock.ExpectationsWereMet())
}
