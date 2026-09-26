package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestAcceptCommunityCommentTxMarksAcceptedAndReturnsVisibleRuntimeComments(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &bizDecipherRepository{db: db}
	ctx := context.Background()
	postID := int64(101)
	commentID := int64(202)
	ownerID := int64(303)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT user_id\\s+FROM community_posts").
		WithArgs(postID).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(ownerID))
	mock.ExpectQuery("SELECT EXISTS\\(").
		WithArgs(commentID, postID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectExec("UPDATE community_comments\\s+SET status = CASE").
		WithArgs(postID, commentID).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery("UPDATE community_posts\\s+SET status = CASE").
		WithArgs(postID).
		WillReturnRows(newCommunityPostRows().AddRow(
			postID, ownerID, "", "support", "Need help", "Body", []byte(`["qa"]`), "tavern", "chat-hall", false,
			"answered", false, 0, 2, 9, "", "", "incident_support", "", "", "", "ask_help",
			[]byte(`[]`), []byte(`{}`), time.Now(), time.Now(),
		))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT c\\.id, c\\.post_id, c\\.user_id,").
		WithArgs(postID, 5).
		WillReturnRows(newCommunityCommentRows().
			AddRow(commentID, postID, int64(404), "helper", "Accepted answer", "operator", false, "accepted", time.Now(), time.Now()).
			AddRow(int64(203), postID, int64(405), "official", "Confirmed answer", "official", true, "confirmed", time.Now(), time.Now()))

	post, err := repo.AcceptCommunityCommentTx(ctx, postID, commentID, ownerID)
	require.NoError(t, err)
	require.Equal(t, "answered", post.Status)
	require.Len(t, post.Comments, 2)
	require.Equal(t, "accepted", post.Comments[0].Status)
	require.Equal(t, "confirmed", post.Comments[1].Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAcceptCommunityCommentTxRejectsNonOwner(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &bizDecipherRepository{db: db}
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT user_id\\s+FROM community_posts").
		WithArgs(int64(101)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(int64(999)))
	mock.ExpectRollback()

	post, err := repo.AcceptCommunityCommentTx(ctx, 101, 202, 303)
	require.Nil(t, post)
	require.ErrorIs(t, err, sql.ErrNoRows)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListCommunityCommentsKeepsRuntimeVisibleStatuses(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &bizDecipherRepository{db: db}
	ctx := context.Background()
	postID := int64(101)

	mock.ExpectQuery("c\\.status NOT IN \\('hidden', 'deleted'\\)").
		WithArgs(postID, 10).
		WillReturnRows(newCommunityCommentRows().
			AddRow(int64(1), postID, int64(11), "a", "visible", "", false, "visible", time.Now(), time.Now()).
			AddRow(int64(2), postID, int64(12), "b", "accepted", "", false, "accepted", time.Now(), time.Now()).
			AddRow(int64(3), postID, int64(13), "c", "confirmed", "", true, "confirmed", time.Now(), time.Now()).
			AddRow(int64(4), postID, int64(14), "d", "resolved", "", false, "resolved", time.Now(), time.Now()))

	comments, err := repo.ListCommunityComments(ctx, postID, 10)
	require.NoError(t, err)
	require.Len(t, comments, 4)
	require.Equal(t, []string{"visible", "accepted", "confirmed", "resolved"}, []string{comments[0].Status, comments[1].Status, comments[2].Status, comments[3].Status})
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateCommunityPostModerationCanPinPost(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &bizDecipherRepository{db: db}
	ctx := context.Background()
	pinned := true

	mock.ExpectQuery("UPDATE community_posts\\s+SET status = \\$2, pinned = \\$3").
		WithArgs(int64(101), "confirmed", true).
		WillReturnRows(newCommunityPostRows().AddRow(
			int64(101), int64(303), "", "support", "Need help", "Body", []byte(`["qa"]`), "tavern", "chat-hall", false,
			"confirmed", true, 0, 2, 9, "", "", "incident_support", "", "", "", "ask_help",
			[]byte(`[]`), []byte(`{}`), time.Now(), time.Now(),
		))

	post, err := repo.UpdateCommunityPostModeration(ctx, 101, "confirmed", &pinned)
	require.NoError(t, err)
	require.Equal(t, "confirmed", post.Status)
	require.True(t, post.Pinned)
	require.NoError(t, mock.ExpectationsWereMet())
}

func newCommunityPostRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "user_id", "author", "kind", "title", "body", "tags", "district", "channel", "private", "status", "pinned", "catches", "replies", "views",
		"source_type", "source_id", "scenario", "subject_type", "subject_id", "subject_title", "action_type", "evidence", "trust_signals", "created_at", "updated_at",
	})
}

func newCommunityCommentRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "post_id", "user_id", "author", "body", "helper_role", "official", "status", "created_at", "updated_at",
	})
}
