package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestGetUserCollectibleCardRarityReturnsOwnedValue(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &bizDecipherRepository{db: db}

	mock.ExpectQuery(`SELECT LOWER\(BTRIM\(rarity\)\).*checkin_collectible_cards`).
		WithArgs(int64(42), "card-dawn").
		WillReturnRows(sqlmock.NewRows([]string{"rarity"}).AddRow("legendary"))

	rarity, err := repo.GetUserCollectibleCardRarity(context.Background(), 42, " card-dawn ")
	require.NoError(t, err)
	require.Equal(t, "legendary", rarity)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSetPoolCardSkinRejectsZeroRowUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &bizDecipherRepository{db: db}

	mock.ExpectExec(`UPDATE shared_pools.*card_skin_key`).
		WithArgs("card-dawn", "legendary", int64(7), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = repo.SetPoolCardSkinTx(context.Background(), 7, 42, "card-dawn", "legendary")
	require.ErrorContains(t, err, "did not match")
	require.NoError(t, mock.ExpectationsWereMet())
}
