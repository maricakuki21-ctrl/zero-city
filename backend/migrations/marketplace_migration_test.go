package migrations_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarketplaceMigration_hasParticipantAndIdempotencyConstraints(t *testing.T) {
	// Given
	contents, err := os.ReadFile("245_marketplace_listings_and_inquiries.sql")
	require.NoError(t, err)
	sql := string(contents)

	// When / Then
	require.Contains(t, sql, "UNIQUE (listing_id, initiator_user_id)")
	require.Contains(t, sql, "CHECK (initiator_user_id <> listing_owner_user_id)")
	require.Contains(t, sql, "UNIQUE (inquiry_id, sender_user_id, client_message_id)")
	require.Contains(t, sql, "CHECK (status IN ('published', 'archived', 'taken_down'))")
	require.Contains(t, sql, "CHECK (kind IN ('service', 'demand', 'talent'))")
	require.NotContains(t, sql, "CREATE TABLE marketplace_payments")
}
