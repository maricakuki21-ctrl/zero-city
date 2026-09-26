package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// Run only against a disposable database. The suffix guard prevents accidental
// creation of test grants/users in the production database.
func TestTokenRewardsPostgresConcurrencyAndSettlement(t *testing.T) {
	dsn := os.Getenv("TOKEN_REWARD_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated reward database not configured")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	var dbname string
	require.NoError(t, db.QueryRow(`SELECT current_database()`).Scan(&dbname))
	require.Equal(t, "token_reward_qa", dbname)
	db.SetMaxOpenConns(20)
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	admin := insertMarketplaceTestUser(t, db, fmt.Sprintf("reward-admin-%d@test.invalid", suffix))
	_, err = db.Exec(`UPDATE users SET role='admin',balance=100 WHERE id=$1`, admin)
	require.NoError(t, err)
	var group, key int64
	require.NoError(t, db.QueryRow(`INSERT INTO groups(name) VALUES($1) RETURNING id`, fmt.Sprintf("reward-%d", suffix)).Scan(&group))
	require.NoError(t, db.QueryRow(`INSERT INTO api_keys(user_id,key,name,group_id,status) VALUES($1,$2,'Reward QA',$3,'active') RETURNING id`, admin, fmt.Sprintf("sk-reward-%d", suffix), group).Scan(&key))
	repo := NewTokenRewardRepository(db)
	_, err = repo.ValidateSponsor(ctx, key)
	require.NoError(t, err)
	resource := service.TokenRewardResource{ID: "qa-text", Model: "qa-model", KeyID: key}
	shares, err := service.AllocateTokenPacket(100_000_000, 64, "random")
	require.NoError(t, err)
	in := service.TokenPacketInput{ClientID: fmt.Sprintf("packet_%d", suffix), ResourceID: resource.ID, Total: 100_000_000, Portions: 64, Mode: "random", Blessing: "测试", ClaimHours: 24, UseHours: 168}
	packet, err := repo.CreateTokenPacket(ctx, "lobby", admin, in, resource, shares, "hash")
	require.NoError(t, err)
	replay, err := repo.CreateTokenPacket(ctx, "lobby", admin, in, resource, shares, "hash")
	require.NoError(t, err)
	require.Equal(t, packet.ID, replay.ID)
	_, err = repo.CreateTokenPacket(ctx, "lobby", admin, in, resource, shares, "other")
	require.Error(t, err)
	users := make([]int64, 200)
	for i := range users {
		users[i] = insertMarketplaceTestUser(t, db, fmt.Sprintf("reward-%d-%d@test.invalid", suffix, i))
	}
	type result struct {
		uid   int64
		grant *service.TokenGrant
		err   error
	}
	results := make(chan result, 200)
	var wg sync.WaitGroup
	for _, uid := range users {
		wg.Add(1)
		go func(uid int64) {
			defer wg.Done()
			g, e := repo.ClaimTokenPacket(ctx, packet.ID, uid)
			results <- result{uid, g, e}
		}(uid)
	}
	wg.Wait()
	close(results)
	var sum int64
	var count int
	var winner int64
	var original *service.TokenGrant
	for v := range results {
		if v.err == nil {
			sum += v.grant.Tokens
			count++
			winner = v.uid
			original = v.grant
		}
	}
	require.Equal(t, 64, count)
	require.Equal(t, int64(100_000_000), sum)
	g, err := repo.ClaimTokenPacket(ctx, packet.ID, winner)
	require.NoError(t, err)
	require.Equal(t, original.ID, g.ID)
	_, err = repo.ClaimTokenPacket(ctx, packet.ID, admin)
	require.Error(t, err)
	runIn := service.TokenRewardRunInput{ClientID: "run_123456", ResourceID: resource.ID, Prompt: "test", MaxOutput: 100}
	run, created, err := repo.ReserveTokenRun(ctx, winner, runIn, resource, 1000, "run-hash")
	require.NoError(t, err)
	require.True(t, created)
	again, created, err := repo.ReserveTokenRun(ctx, winner, runIn, resource, 1000, "run-hash")
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, run.ID, again.ID)
	_, _, err = repo.ReserveTokenRun(ctx, winner, runIn, resource, 1000, "different")
	require.Error(t, err)
	_, err = repo.FinishTokenRun(ctx, run.ID, "review", 0, "", "uncertain")
	require.NoError(t, err)
	w, err := repo.TokenRewardWallet(ctx, winner)
	require.NoError(t, err)
	require.Equal(t, int64(1000), w.Grants[0].Reserved)
	require.Zero(t, w.Grants[0].Used)
	_, err = repo.FinishTokenRun(ctx, run.ID, "succeeded", 70, "answer", "")
	require.NoError(t, err)
	_, err = repo.FinishTokenRun(ctx, run.ID, "succeeded", 70, "answer", "")
	require.NoError(t, err)
	w, err = repo.TokenRewardWallet(ctx, winner)
	require.NoError(t, err)
	require.Zero(t, w.Grants[0].Reserved)
	require.Equal(t, int64(70), w.Grants[0].Used)
	// Two concurrent attempts cannot consume the same remaining grant.
	results2 := make(chan error, 2)
	remaining := w.Grants[0].Tokens - w.Grants[0].Used
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ri := runIn
			ri.ClientID = fmt.Sprintf("parallel_%d", i)
			_, _, e := repo.ReserveTokenRun(ctx, winner, ri, resource, remaining, ri.ClientID)
			results2 <- e
		}(i)
	}
	wg.Wait()
	close(results2)
	success := 0
	for e := range results2 {
		if e == nil {
			success++
		}
	}
	require.Equal(t, 1, success)
	var balance float64
	require.NoError(t, db.QueryRow(`SELECT balance FROM users WHERE id=$1`, winner).Scan(&balance))
	require.Zero(t, balance)
	// Expiry does not revoke an already reserved request.
	_, err = db.Exec(`UPDATE biz_token_grants SET expires_at=NOW()-INTERVAL '1 second' WHERE user_id=$1`, winner)
	require.NoError(t, err)
	w, err = repo.TokenRewardWallet(ctx, winner)
	require.NoError(t, err)
	for _, v := range w.Runs {
		if v.Status == "pending" {
			_, err = repo.FinishTokenRun(ctx, v.ID, "succeeded", remaining+50, "", "overshoot paid by operator")
			require.NoError(t, err)
		}
	}
	w, err = repo.TokenRewardWallet(ctx, winner)
	require.NoError(t, err)
	require.Equal(t, w.Grants[0].Tokens, w.Grants[0].Used)
	require.Zero(t, w.Grants[0].Reserved)
	require.NoError(t, db.QueryRow(`SELECT balance FROM users WHERE id=$1`, winner).Scan(&balance))
	require.Zero(t, balance)
	// Real chat history includes the typed card, not a forged text marker.
	chat := NewChatRepository(db)
	channel, err := chat.GetChatChannelBySlug(ctx, "lobby")
	require.NoError(t, err)
	messages, err := chat.ListChatMessages(ctx, channel.ID, 0, 10)
	require.NoError(t, err)
	found := false
	for _, m := range messages {
		if m.ID == packet.MessageID {
			found = true
			require.Equal(t, packet.ID, m.TokenPacketID)
		}
	}
	require.True(t, found)
}
