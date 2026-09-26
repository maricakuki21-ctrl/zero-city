package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestCommunityPollRepositoryPostgres_serializesOneBallotAndClosure(t *testing.T) {
	dsn := os.Getenv("GOVERNANCE_POLL_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("GOVERNANCE_POLL_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	ctx := context.Background()
	suffix := time.Now().UnixNano()
	ownerID := insertGovernancePollTestUser(t, db, fmt.Sprintf("poll-owner-%d@example.test", suffix))
	voterID := insertGovernancePollTestUser(t, db, fmt.Sprintf("poll-voter-%d@example.test", suffix))
	otherID := insertGovernancePollTestUser(t, db, fmt.Sprintf("poll-other-%d@example.test", suffix))
	repo := &bizDecipherRepository{db: db}
	poll, err := repo.CreateCommunityPollTx(ctx, ownerID, service.CommunityPollInput{
		Title: "Concurrency", Body: "Choose one", Options: []string{"A", "B"},
	})
	require.NoError(t, err)

	start := make(chan struct{})
	results := make([]error, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	for index, option := range poll.Options {
		go func() {
			defer wait.Done()
			<-start
			_, results[index] = repo.VoteCommunityPollTx(ctx, poll.ID, voterID, option.ID)
		}()
	}
	close(start)
	wait.Wait()

	successes, conflicts := 0, 0
	for _, result := range results {
		switch {
		case result == nil:
			successes++
		case errors.Is(result, service.ErrCommunityPollAlreadyVoted):
			conflicts++
		default:
			require.NoError(t, result)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)

	current, err := repo.getCommunityPoll(ctx, poll.ID, voterID, false)
	require.NoError(t, err)
	require.Equal(t, 1, current.TotalVotes)
	_, err = repo.VoteCommunityPollTx(ctx, poll.ID, voterID, current.ViewerOptionID)
	require.NoError(t, err, "same-option retry must be idempotent")
	_, err = repo.CloseCommunityPollTx(ctx, poll.ID, otherID, false)
	require.ErrorIs(t, err, service.ErrCommunityPollForbidden)
	closed, err := repo.CloseCommunityPollTx(ctx, poll.ID, ownerID, false)
	require.NoError(t, err)
	require.Equal(t, "closed", closed.Status)
	_, err = repo.VoteCommunityPollTx(ctx, poll.ID, otherID, poll.Options[0].ID)
	require.ErrorIs(t, err, service.ErrCommunityPollClosed)
}

func insertGovernancePollTestUser(t *testing.T, db *sql.DB, email string) int64 {
	t.Helper()
	var userID int64
	err := db.QueryRow(`INSERT INTO users (email, password_hash, role, status) VALUES ($1, 'test', 'user', 'active') RETURNING id`, email).Scan(&userID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO biz_profiles (user_id, display_name) VALUES ($1, $2)`, userID, email)
	require.NoError(t, err)
	return userID
}
