package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPlayerAnnouncementPostgresLifecycle(t *testing.T) {
	dsn := os.Getenv("GOVERNANCE_POLL_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("GOVERNANCE_POLL_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	ctx := context.Background()
	repo := &bizDecipherRepository{db: db}
	users := make([]int64, 3)
	for i := range users {
		users[i] = insertGovernancePollTestUser(t, db, fmt.Sprintf("announcement-%d-%d@example.test", time.Now().UnixNano(), i))
	}
	defer func() {
		for _, id := range users {
			_, _ = db.Exec(`DELETE FROM users WHERE id = $1`, id)
		}
	}()
	deadline := time.Now().Add(time.Hour)
	poll, err := repo.CreateCommunityPollTx(ctx, users[0], service.CommunityPollInput{
		Title: "Player announcement lifecycle", Body: "Canonical content", ProposalKind: "announcement", Options: []string{"支持发布", "暂不发布"},
		ClosesAt: &deadline, MinimumVotes: 3, SupportPercent: 60, DisplayDays: 7,
	})
	require.NoError(t, err)
	for _, user := range users {
		_, err = repo.VoteCommunityPollTx(ctx, poll.ID, user, poll.Options[0].ID)
		require.NoError(t, err)
	}
	_, err = repo.VoteCommunityPollTx(ctx, poll.ID, users[1], poll.Options[0].ID)
	require.NoError(t, err)
	_, err = repo.VoteCommunityPollTx(ctx, poll.ID, users[1], poll.Options[1].ID)
	require.ErrorIs(t, err, service.ErrCommunityPollAlreadyVoted)
	_, err = db.Exec(`UPDATE community_polls SET closes_at = NOW() - INTERVAL '1 hour' WHERE id = $1`, poll.ID)
	require.NoError(t, err)
	current, err := repo.getCommunityPoll(ctx, poll.ID, users[0], false)
	require.NoError(t, err)
	require.Equal(t, "published", current.Decision)
	require.Equal(t, 3, current.TotalVotes)
	for i := 0; i < 2; i++ {
		items, err := repo.ListPlayerAnnouncements(ctx)
		require.NoError(t, err)
		count := 0
		for _, item := range items {
			if item.ID == poll.ID {
				count++
			}
		}
		require.Equal(t, 1, count, "publication must not duplicate on retry")
	}
	_, err = repo.VoteCommunityPollTx(ctx, poll.ID, users[1], poll.Options[0].ID)
	require.ErrorIs(t, err, service.ErrCommunityPollClosed)
	_, err = db.Exec(`UPDATE community_posts SET status = 'hidden' WHERE id = $1`, poll.PostID)
	require.NoError(t, err)
	items, err := repo.ListPlayerAnnouncements(ctx)
	require.NoError(t, err)
	for _, item := range items {
		require.NotEqual(t, poll.ID, item.ID)
	}
	_, err = repo.VoteCommunityPollTx(ctx, poll.ID, users[1], poll.Options[0].ID)
	require.ErrorIs(t, err, service.ErrCommunityPollNotFound)
	_, err = db.Exec(`UPDATE community_posts SET status = 'rejected' WHERE id = $1`, poll.PostID)
	require.NoError(t, err)
	items, err = repo.ListPlayerAnnouncements(ctx)
	require.NoError(t, err)
	for _, item := range items {
		require.NotEqual(t, poll.ID, item.ID)
	}
	_, err = db.Exec(`UPDATE community_posts SET status = 'open' WHERE id = $1`, poll.PostID)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE community_polls SET closes_at = NOW() - INTERVAL '8 days' WHERE id = $1`, poll.ID)
	require.NoError(t, err)
	current, err = repo.getCommunityPoll(ctx, poll.ID, users[0], false)
	require.NoError(t, err)
	require.Equal(t, "expired", current.Decision)
	_, err = repo.CloseCommunityPollTx(ctx, poll.ID, users[1], false)
	require.ErrorIs(t, err, service.ErrCommunityPollForbidden)
	current, err = repo.CloseCommunityPollTx(ctx, poll.ID, users[0], false)
	require.NoError(t, err)
	require.Equal(t, "withdrawn", current.Decision)
	current, err = repo.CloseCommunityPollTx(ctx, poll.ID, users[0], false)
	require.NoError(t, err)
	require.Equal(t, "withdrawn", current.Decision)
}
