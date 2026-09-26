//go:build r1integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/lib/pq"
)

func TestNativePoolActivationPostgres_endToEnd(t *testing.T) {
	dsn := os.Getenv("NATIVE_ACTIVATION_DATABASE_URL")
	require.NotEmpty(t, dsn)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	var activationColumn string
	require.NoError(t, db.QueryRow(`SELECT column_name FROM information_schema.columns
		WHERE table_schema='public' AND table_name='shared_pools' AND column_name='native_activated_at'`).Scan(&activationColumn))
	require.Equal(t, "native_activated_at", activationColumn)
	driver := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	repo := &bizDecipherRepository{db: db}
	svc := service.NewBizDecipherService(repo, nil, nil)
	svc.SetSharedPoolNativeOnboarding(repo, NewSharedPoolNativeAccountRepository(client, db, nil))
	svc.SetSharedPoolNativeReadiness(repo, nativeActivationModels{"gpt-test"}, nativeActivationConnection{})
	ownerID := createNativeActivationUser(t, db, "owner")
	foreignID := createNativeActivationUser(t, db, "foreign")
	draftInput, err := service.NewSharedPoolNativeDraftInput(ownerID, "Native activation", "", fmt.Sprintf("draft-%d", time.Now().UnixNano()))
	require.NoError(t, err)
	pool, err := svc.CreateSharedPoolNativeDraft(context.Background(), draftInput)
	require.NoError(t, err)
	removeNativeDraftBinding(t, db, pool.ID)
	_, err = db.Exec(`UPDATE shared_pools SET rate_multiplier=1.7 WHERE id=$1`, pool.ID)
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		replayedDraft, replayErr := svc.CreateSharedPoolNativeDraft(context.Background(), draftInput)
		require.NoError(t, replayErr)
		require.Equal(t, pool.ID, replayedDraft.ID)
		requireNativeDraftBinding(t, db, pool.ID, 1.7)
	}
	removeNativeDraftBinding(t, db, pool.ID)
	accountInput, err := service.NewSharedPoolNativeAccountInput(service.SharedPoolNativeAccountRequest{
		PoolID: pool.ID, OwnerID: ownerID, OperationID: "account-1", Name: "native", Provider: service.PlatformOpenAI,
		AuthType: service.AccountTypeAPIKey, UpstreamBaseURL: "https://api.openai.com", APIKey: "test-secret-key",
	})
	require.NoError(t, err)
	sharedAccount, err := svc.OnboardSharedPoolNativeAccount(context.Background(), accountInput)
	require.NoError(t, err)
	requireNativeDraftBinding(t, db, pool.ID, 1.7)
	readinessInput := service.SharedPoolNativeReadinessInput{
		PoolID: pool.ID, OwnerID: ownerID, SharedAccountID: sharedAccount.ID,
		ExpectedConfigVersion: nativeActivationPoolVersion(t, db, pool.ID),
	}
	ready, err := svc.VerifySharedPoolNativeReadiness(context.Background(), readinessInput)
	require.NoError(t, err)
	require.Equal(t, "ready", ready.NativeBindingState)
	require.False(t, ready.NativeEvidenceStale)
	require.True(t, ready.BillingActivationRequired)
	require.False(t, ready.Schedulable)
	configureNativeActivationFixture(t, db, pool.ID, sharedAccount.ID)
	version := nativeActivationPoolVersion(t, db, pool.ID)
	input, err := service.NewSharedPoolNativeActivationInput(pool.ID, ownerID, "activate-1", version)
	require.NoError(t, err)

	activated, err := svc.ActivateSharedPoolNativeBilling(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, service.SharedPoolOnboardingBillingActive, activated.State)
	require.Equal(t, activated.ConfigVersion, activated.ActivatedConfigVersion)
	require.Greater(t, activated.ConfigVersion, version)
	replayed, err := svc.ActivateSharedPoolNativeBilling(context.Background(), input)
	require.NoError(t, err)
	require.True(t, replayed.AlreadyActive)
	wrongOwner, _ := service.NewSharedPoolNativeActivationInput(pool.ID, foreignID, "foreign", activated.ConfigVersion)
	_, err = svc.ActivateSharedPoolNativeBilling(context.Background(), wrongOwner)
	require.ErrorIs(t, err, service.ErrPoolForbidden)
	stale, _ := service.NewSharedPoolNativeActivationInput(pool.ID, ownerID, "stale", version)
	_, err = svc.ActivateSharedPoolNativeBilling(context.Background(), stale)
	require.ErrorIs(t, err, service.ErrNativeActivationStale)

	var cvBefore, avBefore int64
	require.NoError(t, db.QueryRow(`SELECT config_version,COALESCE(native_activated_config_version,0) FROM shared_pools WHERE id=$1`, pool.ID).Scan(&cvBefore, &avBefore))
	readinessInput.ExpectedConfigVersion = activated.ConfigVersion
	ready, err = svc.VerifySharedPoolNativeReadiness(context.Background(), readinessInput)
	require.NoError(t, err)
	var cvAfter, avAfter int64
	var stateAfter string
	require.NoError(t, db.QueryRow(`SELECT config_version,COALESCE(native_activated_config_version,0),native_onboarding_state FROM shared_pools WHERE id=$1`, pool.ID).Scan(&cvAfter, &avAfter, &stateAfter))
	var canonicalUpdated time.Time
	require.NoError(t, db.QueryRow(`SELECT a.updated_at FROM shared_pool_supply_dispositions d JOIN accounts a ON a.id=d.canonical_account_id WHERE d.pool_id=$1`, pool.ID).Scan(&canonicalUpdated))
	t.Logf("post-activation: config=%d->%d activated=%d->%d pool_state=%s stale=%v billing_required=%v schedulable=%v observed=%v canonical=%v", cvBefore, cvAfter, avBefore, avAfter, stateAfter, ready.NativeEvidenceStale, ready.BillingActivationRequired, ready.Schedulable, ready.NativeAccountObservedUpdatedAt, canonicalUpdated)
	require.False(t, ready.NativeEvidenceStale)
	require.False(t, ready.BillingActivationRequired)
	require.True(t, ready.Schedulable)
	require.Equal(t, service.SharedPoolOnboardingBillingActive, nativeActivationPoolState(t, db, pool.ID))

	svc.SetSharedPoolNativeReadiness(repo, nativeActivationModels{"gpt-test", "gpt-new"}, nativeActivationConnection{})
	ready, err = svc.VerifySharedPoolNativeReadiness(context.Background(), readinessInput)
	require.NoError(t, err)
	require.True(t, ready.NativeEvidenceStale)
	require.True(t, ready.BillingActivationRequired)
	require.False(t, ready.Schedulable)
	readinessInput.ExpectedConfigVersion = nativeActivationPoolVersion(t, db, pool.ID)
	ready, err = svc.VerifySharedPoolNativeReadiness(context.Background(), readinessInput)
	require.NoError(t, err)
	require.False(t, ready.NativeEvidenceStale)
	require.True(t, ready.BillingActivationRequired)
	reactivate, err := service.NewSharedPoolNativeActivationInput(pool.ID, ownerID, "activate-2", nativeActivationPoolVersion(t, db, pool.ID))
	require.NoError(t, err)
	_, err = svc.ActivateSharedPoolNativeBilling(context.Background(), reactivate)
	require.NoError(t, err)

	_, err = db.Exec(`UPDATE shared_pool_models SET enabled=FALSE WHERE pool_id=$1`, pool.ID)
	require.NoError(t, err)
	require.Equal(t, service.SharedPoolOnboardingReadyBillingBlocked, nativeActivationPoolState(t, db, pool.ID))
	var poolSchedulable, accountSchedulable bool
	require.NoError(t, db.QueryRow(`SELECT spa.schedulable,a.schedulable FROM shared_pool_accounts spa
		JOIN shared_pool_supply_dispositions d ON d.source_kind='pool_account' AND d.source_id=spa.id
		JOIN accounts a ON a.id=d.canonical_account_id WHERE spa.pool_id=$1`, pool.ID).Scan(&poolSchedulable, &accountSchedulable))
	require.False(t, poolSchedulable)
	require.False(t, accountSchedulable)
}

func createNativeActivationUser(t *testing.T, db *sql.DB, label string) int64 {
	t.Helper()
	var id int64
	err := db.QueryRow(`INSERT INTO users(email,password_hash,role,status) VALUES($1,'hash','user','active') RETURNING id`, fmt.Sprintf("native-%s-%d@example.test", label, time.Now().UnixNano())).Scan(&id)
	require.NoError(t, err)
	return id
}

func removeNativeDraftBinding(t *testing.T, db *sql.DB, poolID int64) {
	t.Helper()
	var groupID int64
	require.NoError(t, db.QueryRow(`DELETE FROM shared_pool_sub2_bindings WHERE pool_id=$1 RETURNING canonical_group_id`, poolID).Scan(&groupID))
	_, err := db.Exec(`DELETE FROM groups WHERE id=$1`, groupID)
	require.NoError(t, err)
}

func requireNativeDraftBinding(t *testing.T, db *sql.DB, poolID int64, expectedRate float64) {
	t.Helper()
	var count int
	var rate float64
	require.NoError(t, db.QueryRow(`SELECT COUNT(*),MAX(g.rate_multiplier) FROM shared_pool_sub2_bindings b
		JOIN groups g ON g.id=b.canonical_group_id WHERE b.pool_id=$1`, poolID).Scan(&count, &rate))
	require.Equal(t, 1, count)
	require.Equal(t, expectedRate, rate)
}

func configureNativeActivationFixture(t *testing.T, db *sql.DB, poolID, sharedAccountID int64) {
	t.Helper()
	var modelID int64
	require.NoError(t, db.QueryRow(`INSERT INTO shared_pool_models
		(pool_id,provider,model_name,upstream_model_name,enabled,model_open,pricing_source,pricing_status)
		VALUES($1,'openai','gpt-test','gpt-test',TRUE,TRUE,'official_catalog','ready')
		ON CONFLICT (pool_id,model_name) DO UPDATE SET enabled=TRUE,model_open=TRUE,pricing_source='official_catalog',pricing_status='ready'
		RETURNING id`, poolID).Scan(&modelID))
	_, err := db.Exec(`INSERT INTO shared_pool_model_endpoints
		(pool_model_id,endpoint_type,enabled,gate_status,pricing_status)
		VALUES($1,'chat',TRUE,'unverified','ready')
		ON CONFLICT (pool_model_id,endpoint_type) DO UPDATE
		SET enabled=EXCLUDED.enabled,gate_status=EXCLUDED.gate_status,pricing_status=EXCLUDED.pricing_status,updated_at=NOW()`, modelID)
	require.NoError(t, err)
}

// Provider metadata is stubbed; onboarding, readiness, triggers and activation
// run through the real service and PostgreSQL repository.
type nativeActivationModels []string

func (m nativeActivationModels) DiscoverNativeModels(context.Context, *service.Account) ([]string, error) {
	return []string(m), nil
}

type nativeActivationConnection struct{}

func (nativeActivationConnection) VerifyNativeConnection(context.Context, *service.Account) (service.NativeConnectionEvidence, error) {
	return service.NativeConnectionEvidence{Status: service.NativeConnectionAuthenticatedMetadataReachable}, nil
}

func nativeActivationPoolVersion(t *testing.T, db *sql.DB, poolID int64) int64 {
	t.Helper()
	var value int64
	require.NoError(t, db.QueryRow(`SELECT config_version FROM shared_pools WHERE id=$1`, poolID).Scan(&value))
	return value
}
func nativeActivationPoolState(t *testing.T, db *sql.DB, poolID int64) string {
	t.Helper()
	var value string
	require.NoError(t, db.QueryRow(`SELECT native_onboarding_state FROM shared_pools WHERE id=$1`, poolID).Scan(&value))
	return value
}
func nativeActivationAccountUpdatedAt(t *testing.T, db *sql.DB, poolID int64) time.Time {
	t.Helper()
	var value time.Time
	require.NoError(t, db.QueryRow(`SELECT a.updated_at FROM shared_pool_supply_dispositions d JOIN accounts a ON a.id=d.canonical_account_id WHERE d.pool_id=$1 AND d.source_kind='pool_account'`, poolID).Scan(&value))
	return value
}
