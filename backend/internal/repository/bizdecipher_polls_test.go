package repository

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestVoteCommunityPollTx_rejectsClosedPollWhileHoldingPollLock(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := &bizDecipherRepository{db: db}

	mock.ExpectBegin()
	mock.ExpectQuery("FROM community_polls p.+FOR UPDATE").
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"owner_user_id", "is_closed"}).AddRow(int64(31), true))
	mock.ExpectRollback()

	_, err = repo.VoteCommunityPollTx(context.Background(), 9, 42, 77)

	require.ErrorIs(t, err, service.ErrCommunityPollClosed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCloseCommunityPollTx_rejectsWrongOwner(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := &bizDecipherRepository{db: db}

	mock.ExpectBegin()
	mock.ExpectQuery("FROM community_polls p.+FOR UPDATE").
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"owner_user_id", "is_closed"}).AddRow(int64(31), false))
	mock.ExpectRollback()

	_, err = repo.CloseCommunityPollTx(context.Background(), 9, 42, false)

	require.ErrorIs(t, err, service.ErrCommunityPollForbidden)
	require.NoError(t, mock.ExpectationsWereMet())
}
