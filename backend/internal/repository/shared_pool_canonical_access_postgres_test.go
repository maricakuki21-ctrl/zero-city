package repository

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCanonicalSharedPoolNativeAccessPostgres(t *testing.T) {
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
	id := func(query string, args ...any) int64 {
		t.Helper()
		var value int64
		require.NoError(t, db.QueryRowContext(ctx, query, args...).Scan(&value))
		return value
	}
	exec := func(query string, args ...any) {
		t.Helper()
		_, err := db.ExecContext(ctx, query, args...)
		require.NoError(t, err)
	}
	owner := insertMarketplaceTestUser(t, db, "native-access-owner@example.test")
	member := insertMarketplaceTestUser(t, db, "native-access-member@example.test")
	group := id(`INSERT INTO groups(name,status) VALUES('native-access','active') RETURNING id`)
	account := id(`INSERT INTO accounts(name,platform,type,status) VALUES('native-access','openai','apikey','active') RETURNING id`)
	pool := id(`INSERT INTO shared_pools(owner_id,name,listed,status,lifecycle_state,native_onboarding_state)
		VALUES($1,'native-access',TRUE,'healthy','operating','supply_configuring') RETURNING id`, owner)
	localAccount := id(`INSERT INTO shared_pool_accounts(pool_id,owner_id,name) VALUES($1,$2,'metadata-only') RETURNING id`, pool, owner)
	exec(`INSERT INTO shared_pool_sub2_bindings(pool_id,owner_id,canonical_group_id,lifecycle) VALUES($1,$2,$3,'active')`, pool, owner, group)
	exec(`INSERT INTO shared_pool_supply_dispositions(source_kind,source_id,pool_id,owner_id,canonical_account_id,
		disposition,reason_code,credential_hash,source_checksum)
		VALUES('pool_account',$1,$2,$3,$4,'mapped','test',repeat('a',64),repeat('b',64))`, localAccount, pool, owner, account)
	exec(`INSERT INTO account_groups(account_id,group_id) VALUES($1,$2)`, account, group)
	keyID := id(`INSERT INTO api_keys(user_id,key,name,status) VALUES($1,'native-access-fixture','native-access','active') RETURNING id`, member)
	exec(`INSERT INTO shared_pool_access_keys(pool_id,user_id,api_key_id,status,account_mode) VALUES($1,$2,$3,'active',TRUE)`, pool, member, keyID)
	exec(`INSERT INTO pool_seat_bindings(pool_id,user_id) VALUES($1,$2)`, pool, member)
	exec(`INSERT INTO shared_pool_models(pool_id,model_name,provider,model_aliases,enabled,model_open)
		VALUES($1,'native-model','openai','["alias-model"]',TRUE,TRUE)`, pool)
	exec(`UPDATE shared_pools SET native_onboarding_state='billing_active',
		native_activated_config_version=config_version WHERE id=$1`, pool)
	r := &bizDecipherRepository{db: db}
	key, err := r.GetCanonicalSharedPoolAccessKeyByAPIKeyID(ctx, keyID, "alias-model")
	require.NoError(t, err)
	require.Equal(t, "native-model", key.PublishedModelName)
	require.Empty(t, key.UpstreamAPIKey)
	require.Zero(t, key.AccountID, "account selection belongs to Sub2")
	identity, err := r.ResolveSharedPoolCanonicalIdentity(ctx, service.SharedPoolIdentityQuery{PoolID: pool, OwnerID: owner})
	require.NoError(t, err)
	require.Equal(t, service.CanonicalGroupID(group), identity.GroupID)
	require.Nil(t, identity.AccountID)
	_, err = r.GetCanonicalSharedPoolAccessKeyByAPIKeyID(ctx, keyID, "not-published")
	require.ErrorIs(t, err, service.ErrSharedPoolIdentityUnmapped)
	for _, change := range []struct{ revoke, restore string }{
		{`UPDATE pool_seat_bindings SET status='released',released_at=NOW(),release_reason='legacy_release' WHERE pool_id=$1`, `UPDATE pool_seat_bindings SET status='active',released_at=NULL,release_reason='' WHERE pool_id=$1`},
		{`UPDATE shared_pool_access_keys SET status='disabled' WHERE pool_id=$1`, `UPDATE shared_pool_access_keys SET status='active' WHERE pool_id=$1`},
		{`UPDATE shared_pool_sub2_bindings SET lifecycle='quarantined' WHERE pool_id=$1`, `UPDATE shared_pool_sub2_bindings SET lifecycle='active' WHERE pool_id=$1`},
		{`UPDATE shared_pools SET listed=FALSE WHERE id=$1`, `UPDATE shared_pools SET listed=TRUE WHERE id=$1`},
		{`UPDATE shared_pool_access_keys SET allowed_models='["different-model"]' WHERE pool_id=$1`, `UPDATE shared_pool_access_keys SET allowed_models='[]' WHERE pool_id=$1`},
	} {
		exec(change.revoke, pool)
		_, err = r.GetCanonicalSharedPoolAccessKeyByAPIKeyID(ctx, keyID, "native-model")
		require.ErrorIs(t, err, service.ErrSharedPoolIdentityUnmapped)
		exec(change.restore, pool)
		exec(`UPDATE shared_pools SET native_onboarding_state='billing_active',listed=TRUE,
			status='healthy',lifecycle_state='operating',native_activated_config_version=config_version+1 WHERE id=$1`, pool)
		exec(`UPDATE groups SET status='active' WHERE id=$1`, group)
	}
	exec(`DELETE FROM account_groups WHERE account_id=$1 AND group_id=$2`, account, group)
	_, err = r.GetCanonicalSharedPoolAccessKeyByAPIKeyID(ctx, keyID, "native-model")
	require.ErrorIs(t, err, service.ErrSharedPoolIdentityUnmapped)
}
