//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSharedPoolOAuthAccountCRUDAndRoutingContract(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)

	ownerID := mustCreateSharedPoolBillingUser(t, 0)
	userID := mustCreateSharedPoolBillingUser(t, 10)
	poolID := mustCreateSharedPoolBillingPool(t, ownerID, 0, 0)
	mustConfigureSharedPoolAccessKeyTestPool(t, poolID, "gpt-oauth-fixture")
	mustEnableSharedPoolAccountModeTestPool(t, poolID)
	mustCreateSharedPoolBillingSeat(t, poolID, userID, 0, 0, time.Now())

	expiresAt := time.Now().UTC().Add(time.Hour)
	fingerprint := fmt.Sprintf("fixture-fingerprint-%d", time.Now().UnixNano())
	created, err := repo.CreateSharedPoolAccount(ctx, service.SharedPoolAccountInput{
		PoolID:                poolID,
		OwnerID:               ownerID,
		Name:                  "OAuth fixture account",
		Provider:              service.PlatformOpenAI,
		AuthType:              service.AccountTypeOAuth,
		UpstreamBaseURL:       "https://chatgpt.com/backend-api/codex",
		CredentialsEncrypted:  "fixture-ciphertext",
		CredentialFingerprint: fingerprint,
		ExpiresAt:             &expiresAt,
		AutoPauseOnExpired:    true,
		AutoPauseOnExpiredSet: true,
		Schedulable:           true,
		SchedulableSet:        true,
		Status:                "testing",
		AccountWeight:         1,
		Priority:              100,
		AccountConcurrency:    2,
		UserConcurrency:       1,
		CachePolicy:           json.RawMessage(`{}`),
		RoutingPolicy:         json.RawMessage(`{}`),
		GateRequired:          true,
		ModelConfigs: []service.SharedPoolModelInput{{
			Provider:       service.PlatformOpenAI,
			ModelName:      "gpt-oauth-fixture",
			RateMultiplier: 1,
			MaxConcurrency: 2,
			ModelOpen:      true,
		}},
	})
	require.NoError(t, err)
	require.True(t, created.HasOAuthCredentials)
	require.False(t, created.HasUpstreamKey)
	require.True(t, created.Schedulable)
	require.NotNil(t, created.ExpiresAt)
	require.Equal(t, fingerprint, created.CredentialFingerprint)

	found, err := repo.FindSharedPoolAccountByFingerprint(ctx, poolID, ownerID, fingerprint)
	require.NoError(t, err)
	require.Equal(t, created.ID, found.ID)
	require.True(t, found.HasOAuthCredentials)

	_, err = integrationDB.ExecContext(ctx, `UPDATE shared_pool_accounts SET gate_passed = TRUE WHERE id = $1`, created.ID)
	require.NoError(t, err)

	accessKey, err := repo.CreateSharedPoolAccessKeyTx(
		ctx,
		poolID,
		userID,
		"OAuth routing fixture",
		fmt.Sprintf("sk-share-oauth-%d", time.Now().UnixNano()),
	)
	require.NoError(t, err)
	require.NotNil(t, accessKey)

	routed, err := repo.GetSharedPoolAccessKeyByAPIKeyID(ctx, accessKey.AccessKey.APIKeyID, "gpt-oauth-fixture")
	require.NoError(t, err)
	require.NotNil(t, routed, "valid OAuth account must be selectable by the unified shared key")
	require.Equal(t, created.ID, routed.AccountID)
	require.Equal(t, service.AccountTypeOAuth, routed.AuthType)
	require.Empty(t, routed.UpstreamAPIKey)
	require.Equal(t, "fixture-ciphertext", routed.OAuthCredentialsEncrypted)

	_, err = integrationDB.ExecContext(ctx, `UPDATE shared_pool_accounts SET expires_at = NOW() - INTERVAL '1 minute' WHERE id = $1`, created.ID)
	require.NoError(t, err)
	expiredRoute, err := repo.GetSharedPoolAccessKeyByAPIKeyID(ctx, accessKey.AccessKey.APIKeyID, "gpt-oauth-fixture")
	require.NoError(t, err)
	require.Nil(t, expiredRoute, "expired OAuth account must not be selected for routing")

	_, err = integrationDB.ExecContext(ctx, `UPDATE shared_pool_accounts SET expires_at = $2 WHERE id = $1`, created.ID, expiresAt)
	require.NoError(t, err)

	found.Name = "ignored"
	updated, err := repo.UpdateSharedPoolAccount(ctx, created.ID, service.SharedPoolAccountInput{
		PoolID:                poolID,
		OwnerID:               ownerID,
		Name:                  "OAuth fixture updated",
		Provider:              service.PlatformOpenAI,
		AuthType:              service.AccountTypeOAuth,
		UpstreamBaseURL:       "https://chatgpt.com/backend-api/codex",
		CredentialFingerprint: fingerprint,
		AutoPauseOnExpired:    true,
		AutoPauseOnExpiredSet: true,
		Schedulable:           false,
		SchedulableSet:        true,
		Status:                "limited",
		AccountWeight:         1,
		Priority:              100,
		AccountConcurrency:    2,
		UserConcurrency:       1,
		CachePolicy:           json.RawMessage(`{}`),
		RoutingPolicy:         json.RawMessage(`{}`),
		GateRequired:          true,
		ModelConfigs: []service.SharedPoolModelInput{{
			Provider:       service.PlatformOpenAI,
			ModelName:      "gpt-oauth-fixture",
			RateMultiplier: 1,
			MaxConcurrency: 2,
			ModelOpen:      true,
		}},
	})
	require.NoError(t, err)
	require.False(t, updated.Schedulable)
	require.True(t, updated.HasOAuthCredentials, "empty update ciphertext must preserve stored credentials")

	_, err = repo.CreateSharedPoolAccount(ctx, service.SharedPoolAccountInput{
		PoolID:                poolID,
		OwnerID:               ownerID,
		Name:                  "Duplicate OAuth fixture",
		Provider:              service.PlatformOpenAI,
		AuthType:              service.AccountTypeOAuth,
		UpstreamBaseURL:       "https://chatgpt.com/backend-api/codex",
		CredentialsEncrypted:  "another-fixture-ciphertext",
		CredentialFingerprint: fingerprint,
		AutoPauseOnExpired:    true,
		AutoPauseOnExpiredSet: true,
		Schedulable:           true,
		SchedulableSet:        true,
		Status:                "testing",
		AccountWeight:         1,
		Priority:              100,
		AccountConcurrency:    1,
		UserConcurrency:       1,
		CachePolicy:           json.RawMessage(`{}`),
		RoutingPolicy:         json.RawMessage(`{}`),
		GateRequired:          true,
		ModelConfigs: []service.SharedPoolModelInput{{
			Provider:       service.PlatformOpenAI,
			ModelName:      "gpt-oauth-fixture",
			RateMultiplier: 1,
			MaxConcurrency: 1,
			ModelOpen:      true,
		}},
	})
	require.Error(t, err, "pool-scoped stable identity index must reject duplicates")
}

func TestSharedPoolLikesAreIdempotentAndUserScoped(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)

	ownerID := mustCreateSharedPoolBillingUser(t, 0)
	userA := mustCreateSharedPoolBillingUser(t, 10)
	userB := mustCreateSharedPoolBillingUser(t, 10)
	poolID := mustCreateSharedPoolBillingPool(t, ownerID, 0, 0)
	mustConfigureSharedPoolAccessKeyTestPool(t, poolID, "model-like")

	liked, err := repo.LikeSharedPoolTx(ctx, poolID, userA)
	require.NoError(t, err)
	require.Equal(t, 1, liked.LikeCount)

	likedAgain, err := repo.LikeSharedPoolTx(ctx, poolID, userA)
	require.NoError(t, err)
	require.Equal(t, 1, likedAgain.LikeCount)

	likedByB, err := repo.LikeSharedPoolTx(ctx, poolID, userB)
	require.NoError(t, err)
	require.Equal(t, 2, likedByB.LikeCount)

	likedIDs, err := repo.ListSharedPoolLikedIDs(ctx, userA, []int64{poolID})
	require.NoError(t, err)
	require.True(t, likedIDs[poolID])

	unliked, err := repo.UnlikeSharedPoolTx(ctx, poolID, userA)
	require.NoError(t, err)
	require.Equal(t, 1, unliked.LikeCount)

	likedIDs, err = repo.ListSharedPoolLikedIDs(ctx, userA, []int64{poolID})
	require.NoError(t, err)
	require.False(t, likedIDs[poolID])
}

func TestCreateSharedPoolAccessKeyReusesUnifiedAPIKeyAcrossPools(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)

	ownerID := mustCreateSharedPoolBillingUser(t, 0)
	userID := mustCreateSharedPoolBillingUser(t, 10)
	poolA := mustCreateSharedPoolBillingPool(t, ownerID, 0, 0)
	poolB := mustCreateSharedPoolBillingPool(t, ownerID, 0, 0)
	mustConfigureSharedPoolAccessKeyTestPool(t, poolA, "model-alpha")
	mustConfigureSharedPoolAccessKeyTestPool(t, poolB, "model-beta")
	mustCreateSharedPoolBillingSeat(t, poolA, userID, 0, 0, time.Now())
	mustCreateSharedPoolBillingSeat(t, poolB, userID, 0, 0, time.Now())

	first, err := repo.CreateSharedPoolAccessKeyTx(ctx, poolA, userID, "Unified shared key", fmt.Sprintf("sk-share-unified-%d-a", time.Now().UnixNano()))
	require.NoError(t, err)
	require.NotNil(t, first)
	require.False(t, first.AlreadyHeld)
	require.NotEmpty(t, first.AccessKey.Key)
	require.True(t, first.AccessKey.AccountMode)

	second, err := repo.CreateSharedPoolAccessKeyTx(ctx, poolB, userID, "Unified shared key", fmt.Sprintf("sk-share-unified-%d-b", time.Now().UnixNano()))
	require.NoError(t, err)
	require.NotNil(t, second)
	require.False(t, second.AlreadyHeld)
	require.Empty(t, second.AccessKey.Key)
	require.Equal(t, first.AccessKey.APIKeyID, second.AccessKey.APIKeyID)
	require.True(t, second.AccessKey.AccountMode)

	activeBindings := sharedPoolBillingInt(t, `SELECT COUNT(*) FROM shared_pool_access_keys WHERE api_key_id = $1 AND status = 'active'`, first.AccessKey.APIKeyID)
	require.Equal(t, 2, activeBindings)

	repeated, err := repo.CreateSharedPoolAccessKeyTx(ctx, poolB, userID, "Unified shared key", fmt.Sprintf("sk-share-unified-%d-c", time.Now().UnixNano()))
	require.NoError(t, err)
	require.True(t, repeated.AlreadyHeld)
	require.Empty(t, repeated.AccessKey.Key)
	require.Equal(t, second.AccessKey.ID, repeated.AccessKey.ID)
}

func TestCreateSharedPoolAccessKeyDoesNotReuseOfficialGroupedKey(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)

	ownerID := mustCreateSharedPoolBillingUser(t, 0)
	userID := mustCreateSharedPoolBillingUser(t, 10)
	poolID := mustCreateSharedPoolBillingPool(t, ownerID, 0, 0)
	mustConfigureSharedPoolAccessKeyTestPool(t, poolID, "model-group-isolation")
	mustCreateSharedPoolBillingSeat(t, poolID, userID, 0, 0, time.Now())

	var groupID int64
	err := integrationDB.QueryRowContext(ctx, `
		INSERT INTO groups (name, description, status)
		VALUES ($1, 'official group fixture', 'active')
		RETURNING id`, fmt.Sprintf("Official Group Fixture %d", time.Now().UnixNano())).Scan(&groupID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM groups WHERE id = $1`, groupID)
	})

	var groupedAPIKeyID int64
	err = integrationDB.QueryRowContext(ctx, `
		INSERT INTO api_keys (user_id, key, name, group_id, status)
		VALUES ($1, $2, 'official grouped key', $3, 'active')
		RETURNING id`, userID, fmt.Sprintf("sk-share-grouped-%d", time.Now().UnixNano()), groupID).Scan(&groupedAPIKeyID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM api_keys WHERE id = $1`, groupedAPIKeyID)
	})

	created, err := repo.CreateSharedPoolAccessKeyTx(ctx, poolID, userID, "Unified shared key", fmt.Sprintf("sk-share-isolated-%d", time.Now().UnixNano()))
	require.NoError(t, err)
	require.NotNil(t, created)
	require.NotEqual(t, groupedAPIKeyID, created.AccessKey.APIKeyID, "a key already bound to an official group must never be reused")
	require.NotEmpty(t, created.AccessKey.Key, "a fresh ungrouped shared key should be returned")

	var stillGrouped int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT group_id FROM api_keys WHERE id = $1`, groupedAPIKeyID).Scan(&stillGrouped))
	require.Equal(t, groupID, stillGrouped)
}

func TestGetSharedPoolAccessKeyByAPIKeyIDRoutesSingleCredentialPool(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)

	ownerID := mustCreateSharedPoolBillingUser(t, 0)
	userID := mustCreateSharedPoolBillingUser(t, 10)
	poolID := mustCreateSharedPoolBillingPool(t, ownerID, 0, 0)
	mustConfigureSharedPoolAccessKeyTestPool(t, poolID, "model-single")
	mustCreateSharedPoolBillingSeat(t, poolID, userID, 0, 0, time.Now())
	_, err := integrationDB.ExecContext(ctx, `UPDATE shared_pools SET account_mode_enabled = FALSE, rate_multiplier = 1.8 WHERE id = $1`, poolID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE shared_pool_models SET rate_multiplier = 1.35 WHERE pool_id = $1 AND model_name = 'model-single'`, poolID)
	require.NoError(t, err)

	created, err := repo.CreateSharedPoolAccessKeyTx(ctx, poolID, userID, "Unified shared key", fmt.Sprintf("sk-share-single-%d", time.Now().UnixNano()))
	require.NoError(t, err)
	require.NotNil(t, created)
	require.True(t, created.AccessKey.AccountMode)

	routed, err := repo.GetSharedPoolAccessKeyByAPIKeyID(ctx, created.AccessKey.APIKeyID, "model-single")
	require.NoError(t, err)
	require.NotNil(t, routed)
	require.Equal(t, poolID, routed.PoolID)
	require.Zero(t, routed.AccountID)
	require.Equal(t, "https://shared-pool-routing.example.com", routed.UpstreamBaseURL)
	require.Equal(t, "upstream-test-key", routed.UpstreamAPIKey)
	require.Equal(t, "model-single", routed.PublishedModelName)
	require.InDelta(t, 1.35, routed.RateMultiplier, 0.000001, "pool model multiplier must override pool-wide multiplier")

	listed, err := repo.GetSharedPoolAccessKeyByAPIKeyID(ctx, created.AccessKey.APIKeyID, "")
	require.NoError(t, err)
	require.NotNil(t, listed)
	require.Equal(t, []string{"model-single"}, listed.AllowedModels)
}

func TestGetSharedPoolAccessKeyByAPIKeyIDDoesNotFallbackToPoolCredentialsInAccountMode(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)

	ownerID := mustCreateSharedPoolBillingUser(t, 0)
	userID := mustCreateSharedPoolBillingUser(t, 10)
	poolID := mustCreateSharedPoolBillingPool(t, ownerID, 0, 0)
	mustConfigureSharedPoolAccessKeyTestPool(t, poolID, "model-account-only")
	mustEnableSharedPoolAccountModeTestPool(t, poolID)
	mustCreateSharedPoolBillingSeat(t, poolID, userID, 0, 0, time.Now())

	created, err := repo.CreateSharedPoolAccessKeyTx(ctx, poolID, userID, "Unified shared key", fmt.Sprintf("sk-share-account-only-%d", time.Now().UnixNano()))
	require.NoError(t, err)

	routed, err := repo.GetSharedPoolAccessKeyByAPIKeyID(ctx, created.AccessKey.APIKeyID, "model-account-only")
	require.NoError(t, err)
	require.Nil(t, routed)
}

func TestGetSharedPoolAccessKeyByAPIKeyIDMatchesRequestedModelAcrossUnifiedKeyPools(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)

	ownerID := mustCreateSharedPoolBillingUser(t, 0)
	userID := mustCreateSharedPoolBillingUser(t, 10)
	poolA := mustCreateSharedPoolBillingPool(t, ownerID, 0, 0)
	poolB := mustCreateSharedPoolBillingPool(t, ownerID, 0, 0)
	mustConfigureSharedPoolAccessKeyTestPool(t, poolA, "model-alpha")
	mustConfigureSharedPoolAccessKeyTestPool(t, poolB, "model-beta")
	mustCreateSharedPoolBillingSeat(t, poolA, userID, 0, 0, time.Now())
	mustCreateSharedPoolBillingSeat(t, poolB, userID, 0, 0, time.Now())

	first, err := repo.CreateSharedPoolAccessKeyTx(ctx, poolA, userID, "Unified shared key", fmt.Sprintf("sk-share-routing-%d-a", time.Now().UnixNano()))
	require.NoError(t, err)
	second, err := repo.CreateSharedPoolAccessKeyTx(ctx, poolB, userID, "Unified shared key", fmt.Sprintf("sk-share-routing-%d-b", time.Now().UnixNano()))
	require.NoError(t, err)
	require.Equal(t, first.AccessKey.APIKeyID, second.AccessKey.APIKeyID)

	routed, err := repo.GetSharedPoolAccessKeyByAPIKeyID(ctx, first.AccessKey.APIKeyID, "model-beta")
	require.NoError(t, err)
	require.NotNil(t, routed)
	require.Equal(t, poolB, routed.PoolID)
	require.Equal(t, second.AccessKey.ID, routed.ID)
	require.Equal(t, "https://shared-pool-routing.example.com", routed.UpstreamBaseURL)

	listed, err := repo.GetSharedPoolAccessKeyByAPIKeyID(ctx, first.AccessKey.APIKeyID, "")
	require.NoError(t, err)
	require.NotNil(t, listed)
	require.Equal(t, []string{"model-alpha", "model-beta"}, listed.AllowedModels)

	_, err = integrationDB.ExecContext(ctx, `UPDATE shared_pools SET governance_status = 'banned' WHERE id = $1`, poolB)
	require.NoError(t, err)

	blocked, err := repo.GetSharedPoolAccessKeyByAPIKeyID(ctx, first.AccessKey.APIKeyID, "model-beta")
	require.NoError(t, err)
	require.Nil(t, blocked)

	listed, err = repo.GetSharedPoolAccessKeyByAPIKeyID(ctx, first.AccessKey.APIKeyID, "")
	require.NoError(t, err)
	require.NotNil(t, listed)
	require.Equal(t, poolA, listed.PoolID)
	require.Equal(t, []string{"model-alpha"}, listed.AllowedModels)
}

func TestGetSharedPoolAccessKeyByAPIKeyIDAcceptsPoolModelAlias(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)

	ownerID := mustCreateSharedPoolBillingUser(t, 0)
	userID := mustCreateSharedPoolBillingUser(t, 10)
	poolID := mustCreateSharedPoolBillingPool(t, ownerID, 0, 0)
	mustConfigureSharedPoolAccessKeyTestPool(t, poolID, "model-canonical")
	mustCreateSharedPoolBillingSeat(t, poolID, userID, 0, 0, time.Now())
	_, err := integrationDB.ExecContext(ctx, `UPDATE shared_pool_models
		SET model_aliases = '["model-pool-alias"]'::jsonb,
			upstream_model_name = ''
		WHERE pool_id = $1 AND model_name = 'model-canonical'`, poolID)
	require.NoError(t, err)

	created, err := repo.CreateSharedPoolAccessKeyTx(ctx, poolID, userID, "Unified shared key", fmt.Sprintf("sk-share-pool-alias-%d", time.Now().UnixNano()))
	require.NoError(t, err)

	routed, err := repo.GetSharedPoolAccessKeyByAPIKeyID(ctx, created.AccessKey.APIKeyID, "model-pool-alias")
	require.NoError(t, err)
	require.NotNil(t, routed)
	require.Equal(t, poolID, routed.PoolID)
	require.Equal(t, "model-canonical", routed.PublishedModelName)
	require.Equal(t, "model-canonical", routed.UpstreamModelName)
	require.Contains(t, routed.AllowedModels, "model-canonical")
	require.Contains(t, routed.AllowedModels, "model-pool-alias")

	_, err = integrationDB.ExecContext(ctx, `UPDATE shared_pool_models
		SET upstream_model_name = 'upstream-model-canonical'
		WHERE pool_id = $1 AND model_name = 'model-canonical'`, poolID)
	require.NoError(t, err)

	routed, err = repo.GetSharedPoolAccessKeyByAPIKeyID(ctx, created.AccessKey.APIKeyID, "model-pool-alias")
	require.NoError(t, err)
	require.NotNil(t, routed)
	require.Equal(t, "model-canonical", routed.PublishedModelName)
	require.Equal(t, "upstream-model-canonical", routed.UpstreamModelName)
}

func TestGetSharedPoolAccessKeyByAPIKeyIDAcceptsAccountModelAlias(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)

	ownerID := mustCreateSharedPoolBillingUser(t, 0)
	userID := mustCreateSharedPoolBillingUser(t, 10)
	poolID := mustCreateSharedPoolBillingPool(t, ownerID, 0, 0)
	mustConfigureSharedPoolAccessKeyTestPool(t, poolID, "model-account-canonical")
	mustEnableSharedPoolAccountModeTestPool(t, poolID)
	mustCreateSharedPoolBillingSeat(t, poolID, userID, 0, 0, time.Now())
	accountID := mustCreateSharedPoolAccountAlias(t, ownerID, poolID, "model-account-canonical", "model-account-alias", "")

	created, err := repo.CreateSharedPoolAccessKeyTx(ctx, poolID, userID, "Unified shared key", fmt.Sprintf("sk-share-account-alias-%d", time.Now().UnixNano()))
	require.NoError(t, err)

	routed, err := repo.GetSharedPoolAccessKeyByAPIKeyID(ctx, created.AccessKey.APIKeyID, "model-account-alias")
	require.NoError(t, err)
	require.NotNil(t, routed)
	require.Equal(t, poolID, routed.PoolID)
	require.Equal(t, "model-account-canonical", routed.PublishedModelName)
	require.Equal(t, "model-account-canonical", routed.UpstreamModelName)
	require.Contains(t, routed.AllowedModels, "model-account-canonical")
	require.Contains(t, routed.AllowedModels, "model-account-alias")

	_, err = integrationDB.ExecContext(ctx, `UPDATE shared_pool_accounts
		SET model_configs = jsonb_build_array(jsonb_build_object('model_name', $2::text, 'aliases', jsonb_build_array($3::text), 'upstream_model_name', $4::text, 'model_open', TRUE, 'rate_multiplier', 1, 'max_concurrency', 0))
		WHERE id = $1`, accountID, "model-account-canonical", "model-account-alias", "account-upstream-model")
	require.NoError(t, err)

	routed, err = repo.GetSharedPoolAccessKeyByAPIKeyID(ctx, created.AccessKey.APIKeyID, "model-account-alias")
	require.NoError(t, err)
	require.NotNil(t, routed)
	require.Equal(t, "model-account-canonical", routed.PublishedModelName)
	require.Equal(t, "account-upstream-model", routed.UpstreamModelName)
}

func TestCreateSharedPoolAccessKeyTxUpgradesLegacyActiveBindingToAccountMode(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)

	ownerID := mustCreateSharedPoolBillingUser(t, 0)
	userID := mustCreateSharedPoolBillingUser(t, 10)
	poolID := mustCreateSharedPoolBillingPool(t, ownerID, 0, 0)
	mustConfigureSharedPoolAccessKeyTestPool(t, poolID, "legacy-model")
	mustCreateSharedPoolBillingSeat(t, poolID, userID, 0, 0, time.Now())
	accessKeyID := mustCreateSharedPoolBillingAccessKey(t, poolID, userID)

	var apiKeyID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT api_key_id FROM shared_pool_access_keys WHERE id = $1`, accessKeyID).Scan(&apiKeyID))
	_, err := integrationDB.ExecContext(ctx, `UPDATE shared_pool_access_keys SET account_mode = FALSE WHERE id = $1`, accessKeyID)
	require.NoError(t, err)

	result, err := repo.CreateSharedPoolAccessKeyTx(ctx, poolID, userID, "Unified shared key", fmt.Sprintf("sk-share-legacy-%d", time.Now().UnixNano()))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.AlreadyHeld)
	require.Equal(t, accessKeyID, result.AccessKey.ID)
	require.True(t, result.AccessKey.AccountMode)
	require.Equal(t, []string{"legacy-model"}, result.AccessKey.AllowedModels)

	var accountMode bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT account_mode FROM shared_pool_access_keys WHERE id = $1`, accessKeyID).Scan(&accountMode))
	require.True(t, accountMode)
	var allowedModels []byte
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT allowed_models FROM shared_pool_access_keys WHERE id = $1`, accessKeyID).Scan(&allowedModels))
	require.JSONEq(t, `["legacy-model"]`, string(allowedModels))

	routed, err := repo.GetSharedPoolAccessKeyByAPIKeyID(ctx, apiKeyID, "legacy-model")
	require.NoError(t, err)
	require.NotNil(t, routed)
	require.Equal(t, poolID, routed.PoolID)
	require.Equal(t, accessKeyID, routed.ID)
}

func TestSharedPoolAccessKeyCleanupKeepsAPIKeyWithOtherActiveBindings(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)

	ownerID := mustCreateSharedPoolBillingUser(t, 0)
	userID := mustCreateSharedPoolBillingUser(t, 10)
	poolA := mustCreateSharedPoolBillingPool(t, ownerID, 0, 0)
	poolB := mustCreateSharedPoolBillingPool(t, ownerID, 0, 0)
	mustConfigureSharedPoolAccessKeyTestPool(t, poolA, "model-alpha")
	mustConfigureSharedPoolAccessKeyTestPool(t, poolB, "model-beta")
	mustCreateSharedPoolBillingSeat(t, poolA, userID, 0, 0, time.Now())
	mustCreateSharedPoolBillingSeat(t, poolB, userID, 0, 0, time.Now())

	first, err := repo.CreateSharedPoolAccessKeyTx(ctx, poolA, userID, "Unified shared key", fmt.Sprintf("sk-share-cleanup-%d-a", time.Now().UnixNano()))
	require.NoError(t, err)
	second, err := repo.CreateSharedPoolAccessKeyTx(ctx, poolB, userID, "Unified shared key", fmt.Sprintf("sk-share-cleanup-%d-b", time.Now().UnixNano()))
	require.NoError(t, err)
	require.Equal(t, first.AccessKey.APIKeyID, second.AccessKey.APIKeyID)

	require.NoError(t, repo.LeaveSharedPoolTx(ctx, poolA, userID))

	apiKeyStatus := sharedPoolBillingString(t, `SELECT status FROM api_keys WHERE id = $1`, first.AccessKey.APIKeyID)
	require.Equal(t, service.StatusActive, apiKeyStatus)
	poolAStatus := sharedPoolBillingString(t, `SELECT status FROM shared_pool_access_keys WHERE id = $1`, first.AccessKey.ID)
	require.Equal(t, service.StatusDisabled, poolAStatus)
	poolBStatus := sharedPoolBillingString(t, `SELECT status FROM shared_pool_access_keys WHERE id = $1`, second.AccessKey.ID)
	require.Equal(t, service.StatusActive, poolBStatus)
	seatStatus := sharedPoolBillingString(t, `SELECT status FROM pool_seat_bindings WHERE pool_id = $1 AND user_id = $2 ORDER BY id DESC LIMIT 1`, poolA, userID)
	require.Equal(t, "released", seatStatus)

	require.NoError(t, repo.DeleteSharedPoolTx(ctx, poolA, ownerID))
	notDeleted := sharedPoolBillingInt(t, `SELECT COUNT(*) FROM api_keys WHERE id = $1 AND deleted_at IS NULL`, first.AccessKey.APIKeyID)
	require.Equal(t, 1, notDeleted)
}

func mustConfigureSharedPoolAccessKeyTestPool(t *testing.T, poolID int64, model string) {
	t.Helper()
	ctx := context.Background()
	_, err := integrationDB.ExecContext(ctx, `UPDATE shared_pools
		SET upstream_base_url = 'https://shared-pool-routing.example.com',
			upstream_api_key = 'upstream-test-key',
			account_mode_enabled = FALSE,
			listed = TRUE,
			status = 'healthy'
		WHERE id = $1`, poolID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO shared_pool_models (pool_id, model_name, display_name, sort_order, enabled, model_open, rate_multiplier)
		VALUES ($1, $2, $2, 1, TRUE, TRUE, 1)
		ON CONFLICT (pool_id, model_name) DO UPDATE SET enabled = TRUE, model_open = TRUE`, poolID, model)
	require.NoError(t, err)
}

func mustEnableSharedPoolAccountModeTestPool(t *testing.T, poolID int64) {
	t.Helper()
	_, err := integrationDB.ExecContext(context.Background(), `UPDATE shared_pools SET account_mode_enabled = TRUE WHERE id = $1`, poolID)
	require.NoError(t, err)
}

func mustCreateSharedPoolAccountAlias(t *testing.T, ownerID, poolID int64, model, alias, upstreamModel string) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	err := integrationDB.QueryRowContext(ctx, `INSERT INTO shared_pool_accounts (
		pool_id, owner_id, name, upstream_base_url, upstream_api_key, status, gate_required, gate_passed, model_configs
	) VALUES (
		$1, $2, 'alias account', 'https://shared-pool-account-routing.example.com', 'account-upstream-test-key', 'active', FALSE, TRUE,
		jsonb_build_array(jsonb_build_object('model_name', $3::text, 'aliases', jsonb_build_array($4::text), 'upstream_model_name', $5::text, 'model_open', TRUE, 'rate_multiplier', 1, 'max_concurrency', 0))
	) RETURNING id`, poolID, ownerID, model, alias, upstreamModel).Scan(&id)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM shared_pool_accounts WHERE id = $1`, id)
	})
	return id
}
