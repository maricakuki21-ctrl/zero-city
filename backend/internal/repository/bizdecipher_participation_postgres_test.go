package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCommunityParticipationPostgresClosedLoop(t *testing.T) {
	dsn := os.Getenv("MARKETPLACE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MARKETPLACE_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	ctx := context.Background()
	var name string
	require.NoError(t, db.QueryRowContext(ctx, "SELECT current_database()").Scan(&name))
	require.True(t, strings.HasPrefix(name, "bizdecipher_columns_acceptance_test_"))
	require.NoError(t, ApplyMigrations(ctx, db))
	member := insertMarketplaceTestUser(t, db, "p0-member@example.test")
	admin := insertMarketplaceTestUser(t, db, "p0-admin@example.test")
	_, err = db.ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, admin)
	require.NoError(t, err)
	r := &bizDecipherRepository{db: db}
	s := service.NewBizDecipherService(r, nil, nil)
	in := service.CommunityPostInput{Title: "P0 forum", Body: "body", Kind: "card"}
	post, err := s.CreateCommunityPost(ctx, member, in)
	require.NoError(t, err)
	require.Equal(t, "tavern", post.District)
	in.District, in.Channel = "workshop", "help-desk"
	_, err = s.CreateCommunityPost(ctx, member, in)
	require.ErrorIs(t, err, service.ErrCommunityParticipationRequired)
	for i := 0; i < 2; i++ {
		_, err = s.SetCommunityParticipation(ctx, admin, member, 1, "Verified external work")
		require.NoError(t, err)
	}
	var history int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM community_participation_history WHERE user_id=$1`, member).Scan(&history))
	require.Equal(t, 1, history)
	in.TrustSignals = json.RawMessage(`{"official":true}`)
	qualified, err := s.CreateCommunityPost(ctx, member, in)
	require.NoError(t, err)
	require.Equal(t, "workshop", qualified.District)
	require.JSONEq(t, `{}`, string(qualified.TrustSignals))
	items, err := s.ListCommunityPosts(ctx, service.CommunityPostQuery{District: "workshop", Channel: "help-desk", Limit: 50})
	require.NoError(t, err)
	require.Len(t, items, 1)
	comment, err := s.CreateCommunityComment(ctx, qualified.ID, member, service.CommunityCommentInput{Body: "reply", HelperRole: "admin"})
	require.NoError(t, err)
	require.Equal(t, "resident", comment.HelperRole)
	_, err = s.SetCommunityParticipation(ctx, admin, member, 0, "Qualification revoked")
	require.NoError(t, err)
	_, err = s.CreateCommunityComment(ctx, qualified.ID, member, service.CommunityCommentInput{Body: "blocked"})
	require.ErrorIs(t, err, service.ErrCommunityParticipationRequired)
	_, err = r.UpdateCommunityPostModeration(ctx, qualified.ID, "hidden", nil)
	require.NoError(t, err)
	_, err = s.ListCommunityComments(ctx, qualified.ID, 50)
	require.Error(t, err)
	_, err = db.ExecContext(ctx, `UPDATE community_posts SET private=TRUE WHERE id=$1`, post.ID)
	require.NoError(t, err)
	_, err = s.ListCommunityComments(ctx, post.ID, 50)
	require.Error(t, err)
}
