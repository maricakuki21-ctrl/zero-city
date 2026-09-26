package repository

import (
	"context"
	"database/sql"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCommunityParticipationPrivateCommentsFailBeforeRead(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery(`SELECT district, channel FROM community_posts\s+WHERE id = \$1 AND private = FALSE AND deleted_at IS NULL AND status NOT IN`).
		WithArgs(int64(9)).WillReturnError(sql.ErrNoRows)
	_, err = (&bizDecipherRepository{db: db}).ListCommunityComments(context.Background(), 9, 50)
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCommunityParticipationFiltersUseBoundLocation(t *testing.T) {
	where, args := appendCommunityPostFilters("WHERE TRUE", nil, service.CommunityPostQuery{District: "workshop", Channel: "help-desk"})
	require.Equal(t, "WHERE TRUE AND p.district = $1 AND p.channel = $2", where)
	require.Equal(t, []any{"workshop", "help-desk"}, args)
}
