package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// TestChatRepositoryPostgres_sendReceiveAndIdempotency proves the real
// persistence loop behind the site-wide chat: a second account reads what the
// first account sent, the cursor backfills only newer rows, and replaying a
// client_message_id never duplicates or silently overwrites a message.
func TestChatRepositoryPostgres_sendReceiveAndIdempotency(t *testing.T) {
	dsn := os.Getenv("CHAT_TEST_DATABASE_URL")
	if dsn == "" && os.Getenv("CHAT_TEST_DATABASE") == "" {
		t.Skip("CHAT_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	require.NoError(t, db.PingContext(ctx))

	suffix := time.Now().UnixNano()
	senderID := insertMarketplaceTestUser(t, db, fmt.Sprintf("chat-sender-%d@example.test", suffix))
	readerID := insertMarketplaceTestUser(t, db, fmt.Sprintf("chat-reader-%d@example.test", suffix))
	clientMessageID := fmt.Sprintf("client-%d", suffix)
	secondClientMessageID := fmt.Sprintf("client-b-%d", suffix)
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM biz_chat_messages WHERE client_message_id IN ($1, $2)`, clientMessageID, secondClientMessageID)
		_, _ = db.Exec(`DELETE FROM users WHERE id IN ($1, $2)`, senderID, readerID)
	})

	repo := NewChatRepository(db)
	channel, err := repo.GetChatChannelBySlug(ctx, "lobby")
	require.NoError(t, err)
	require.Equal(t, service.ChatChannelKindPublic, channel.Kind)

	first, err := repo.AppendChatMessage(ctx, channel.ID, senderID, service.ChatMessageInput{ClientMessageID: clientMessageID, Body: "第一条真实消息"})
	require.NoError(t, err)
	require.NotZero(t, first.ID)
	require.Equal(t, "第一条真实消息", first.Body)
	require.NotEmpty(t, first.SenderName)

	replayed, err := repo.AppendChatMessage(ctx, channel.ID, senderID, service.ChatMessageInput{ClientMessageID: clientMessageID, Body: "第一条真实消息"})
	require.NoError(t, err)
	require.Equal(t, first.ID, replayed.ID, "identical replay must return the original row")

	_, err = repo.AppendChatMessage(ctx, channel.ID, senderID, service.ChatMessageInput{ClientMessageID: clientMessageID, Body: "被改写的内容"})
	require.ErrorIs(t, err, service.ErrChatMessageIdempotency)

	// The other account reads the history from a cursor of 0.
	initial, err := repo.ListChatMessages(ctx, channel.ID, 0, 50)
	require.NoError(t, err)
	require.True(t, containsChatMessage(initial, first.ID))

	second, err := repo.AppendChatMessage(ctx, channel.ID, readerID, service.ChatMessageInput{ClientMessageID: secondClientMessageID, Body: "收到"})
	require.NoError(t, err)

	backfill, err := repo.ListChatMessages(ctx, channel.ID, first.ID, 50)
	require.NoError(t, err)
	require.True(t, containsChatMessage(backfill, second.ID))
	require.False(t, containsChatMessage(backfill, first.ID), "cursor backfill must not repeat already-seen rows")

	var count int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM biz_chat_messages WHERE channel_id = $1 AND sender_user_id = $2 AND client_message_id = $3`, channel.ID, senderID, clientMessageID).Scan(&count))
	require.Equal(t, 1, count)
}

func containsChatMessage(items []service.ChatMessage, id int64) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}
