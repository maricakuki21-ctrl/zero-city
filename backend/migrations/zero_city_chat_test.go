package migrations_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestZeroCityChatMigration_isIdempotentAndCursorFriendly(t *testing.T) {
	// Given
	contents, err := os.ReadFile("255_zero_city_chat.sql")
	require.NoError(t, err)
	sql := string(contents)

	// When / Then
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS biz_chat_channels")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS biz_chat_messages")
	require.Contains(t, sql, "UNIQUE (channel_id, sender_user_id, client_message_id)")
	require.Contains(t, sql, "CHECK (char_length(body) BETWEEN 1 AND 2000)")
	require.Contains(t, sql, "ON CONFLICT (slug) DO NOTHING")
	require.NotContains(t, sql, "DROP TABLE")
}
