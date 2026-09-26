package migrations_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGovernancePollMigration_hasAtomicBallotAndClosureConstraints(t *testing.T) {
	contents, err := os.ReadFile("247_zero_city_governance_polls.sql")
	require.NoError(t, err)
	sql := string(contents)

	require.Contains(t, sql, "REFERENCES community_posts(id) ON DELETE CASCADE")
	require.Contains(t, sql, "UNIQUE (poll_id, position)")
	require.Contains(t, sql, "UNIQUE (poll_id, user_id)")
	require.Contains(t, sql, "FOREIGN KEY (option_id, poll_id)")
	require.Contains(t, sql, "closed_by_user_id")
	require.NotContains(t, sql, "credit")
}
