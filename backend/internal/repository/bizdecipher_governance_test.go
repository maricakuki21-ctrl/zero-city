package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGovernanceGrantReadsCommittedIdentity(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM community_badges").WithArgs("host").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(4))
	mock.ExpectQuery("SELECT EXISTS").WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery("(?s)INSERT INTO community_user_badges.*RETURNING id").
		WithArgs(int64(4), int64(2), "completed room", int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectQuery("(?s)SELECT ub.id.*WHERE ub.id = ").WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id","key","name","description","user","reason","actor","at","revoked","revoked_reason"}).
			AddRow(9,"host","Host","Room host",2,"completed room",3,now,nil,""))
	mock.ExpectCommit()
	grant, err := (&bizDecipherRepository{db: db}).GrantCommunityBadgeTx(context.Background(), "host", 2, 3, "completed room")
	require.NoError(t, err)
	require.Equal(t, int64(9), grant.GrantID)
	require.Equal(t, int64(2), grant.UserID)
	require.Equal(t, int64(3), *grant.GrantedBy)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGovernanceAdoptionRejectsNonAdminBeforeDatabase(t *testing.T) {
	r := &bizDecipherRepository{}
	_, err := r.AdoptCommunityPollAsRuleTx(context.Background(), 1, 2, false)
	require.ErrorIs(t, err, service.ErrGovernanceForbidden)
}

func TestGovernanceAdoptionLocksPollAndRejectsOpen(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery("(?s)SELECT.*FOR UPDATE OF p, post").WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"owner", "title", "body", "closed"}).AddRow(2, "title", "body", false))
	mock.ExpectRollback()
	r := &bizDecipherRepository{db: db}
	_, err = r.AdoptCommunityPollAsRuleTx(context.Background(), 1, 2, true)
	require.ErrorIs(t, err, service.ErrGovernancePollNotClosed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGovernanceRevocationMissingBadgeDoesNotCommit(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery("(?s)UPDATE community_user_badges ub.*RETURNING ub.id").
		WithArgs("host", int64(2), int64(3), "reason").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectRollback()
	r := &bizDecipherRepository{db: db}
	_, err = r.RevokeCommunityBadgeTx(context.Background(), "host", 2, 3, "reason")
	require.ErrorIs(t, err, service.ErrGovernanceBadgeNotGranted)
	require.NoError(t, mock.ExpectationsWereMet())
}
