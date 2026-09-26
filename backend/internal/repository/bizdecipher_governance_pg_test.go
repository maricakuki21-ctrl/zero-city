package repository

import (
 "context"
 "database/sql"
 "fmt"
 "os"
 "path/filepath"
 "testing"
 "time"

 "github.com/Wei-Shaw/sub2api/internal/service"
 "github.com/stretchr/testify/require"
)

func TestGovernancePostgresLifecycle(t *testing.T) {
 dsn := os.Getenv("GOVERNANCE_TEST_DSN")
 if dsn == "" { t.Skip("GOVERNANCE_TEST_DSN required") }
 db, err := sql.Open("postgres", dsn)
 require.NoError(t, err)
 defer db.Close()
 db.SetMaxOpenConns(1)
 ctx := context.Background()
 schema := fmt.Sprintf("governance_test_%d", time.Now().UnixNano())
 _, err = db.ExecContext(ctx, "CREATE SCHEMA " + schema)
 require.NoError(t, err)
 defer func() {
  _, cleanupErr := db.ExecContext(ctx, "DROP SCHEMA " + schema + " CASCADE")
  require.NoError(t, cleanupErr)
 }()
 _, err = db.ExecContext(ctx, "SET search_path TO " + schema)
 require.NoError(t, err)
 _, err = db.ExecContext(ctx, `
 CREATE TABLE users (id BIGINT PRIMARY KEY, username TEXT, email TEXT, deleted_at TIMESTAMPTZ);
 CREATE TABLE biz_profiles (user_id BIGINT PRIMARY KEY, display_name TEXT);
 CREATE TABLE community_posts (id BIGINT PRIMARY KEY, user_id BIGINT, title TEXT, body TEXT, deleted_at TIMESTAMPTZ, private BOOLEAN, status TEXT);
 CREATE TABLE community_polls (id BIGINT PRIMARY KEY, post_id BIGINT, closed_at TIMESTAMPTZ, closes_at TIMESTAMPTZ);
 CREATE TABLE community_poll_options (id BIGINT PRIMARY KEY, poll_id BIGINT, label TEXT, position INT);
 CREATE TABLE community_poll_ballots (poll_id BIGINT, option_id BIGINT, user_id BIGINT);
 INSERT INTO users VALUES (1,'admin','admin@example.test',NULL),(2,'member','member@example.test',NULL);
 INSERT INTO community_posts VALUES (1,2,'Rule proposal','Published rule body',NULL,FALSE,'published');
 INSERT INTO community_polls VALUES (1,1,NOW(),NULL);
 INSERT INTO community_poll_options VALUES (1,1,'Yes',1),(2,1,'No',2);
 INSERT INTO community_poll_ballots VALUES (1,1,2);`)
 require.NoError(t, err)
 migrationPath := os.Getenv("GOVERNANCE_TEST_MIGRATION")
 if migrationPath == "" { migrationPath = filepath.Join("..", "..", "migrations", "252_zero_city_governance_rules_and_badges.sql") }
 migration, err := os.ReadFile(migrationPath)
 require.NoError(t, err)
 _, err = db.ExecContext(ctx, string(migration))
 require.NoError(t, err)
 repo := &bizDecipherRepository{db: db}
 grant, err := repo.GrantCommunityBadgeTx(ctx,"host",2,1,"completed room")
 require.NoError(t,err)
 require.Equal(t,int64(2),grant.UserID)
 _, err = repo.GrantCommunityBadgeTx(ctx,"host",2,1,"duplicate")
 require.ErrorIs(t,err,service.ErrGovernanceBadgeAlreadyHeld)
 owned, err := repo.ListUserBadges(ctx,2)
 require.NoError(t,err); require.Len(t,owned,1)
 revoked, err := repo.RevokeCommunityBadgeTx(ctx,"host",2,1,"correction")
 require.NoError(t,err); require.NotNil(t,revoked.RevokedAt)
 owned, err = repo.ListUserBadges(ctx,2)
 require.NoError(t,err); require.Empty(t,owned)
 _, err = repo.GrantCommunityBadgeTx(ctx,"host",2,1,"new grant")
 require.NoError(t,err)
 rule, err := repo.AdoptCommunityPollAsRuleTx(ctx,1,1,true)
 require.NoError(t,err)
 require.Equal(t,float64(1),rule.TallySnapshot["total_votes"])
 _, err = repo.AdoptCommunityPollAsRuleTx(ctx,1,1,true)
 require.ErrorIs(t,err,service.ErrGovernanceRuleConflict)
 rules, err := repo.ListGovernanceRules(ctx,20)
 require.NoError(t,err); require.Len(t,rules,1)
 revokedRule, err := repo.RevokeGovernanceRuleTx(ctx,rule.ID,1,"superseded")
 require.NoError(t,err); require.Equal(t,"superseded",revokedRule.RevokedReason)
 rules, err = repo.ListGovernanceRules(ctx,20)
 require.NoError(t,err); require.Empty(t,rules)
}
