//go:build r1integration

package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/lib/pq"
)

var r1MigrationOnce sync.Once
var r1MigrationErr error

func TestSharedPoolNativeOnboarding_MigrationReplayAndImmutableDisposition(t *testing.T) {
	db := openR1IntegrationDB(t)
	require.NoError(t, ApplyMigrations(context.Background(), db))
	require.NoError(t, ApplyMigrations(context.Background(), db))

	serviceUnderTest, repo, _ := newR1IntegrationServices(t, db)
	ownerID := createR1Owner(t, db)
	draft := createR1Draft(t, serviceUnderTest, ownerID, "migration-replay")
	replayed := createR1Draft(t, serviceUnderTest, ownerID, "migration-replay")
	require.Equal(t, draft.ID, replayed.ID)

	input := newR1AccountInput(t, draft.ID, ownerID, "account-replay", "secret-one")
	created, err := serviceUnderTest.OnboardSharedPoolNativeAccount(context.Background(), input)
	require.NoError(t, err)
	replayedAccount, err := serviceUnderTest.OnboardSharedPoolNativeAccount(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, created.ID, replayedAccount.ID)

	var dispositionID, nativeID int64
	require.NoError(t, db.QueryRow(`SELECT source_id, canonical_account_id FROM shared_pool_supply_dispositions WHERE source_kind='pool_account' AND source_id=$1`, created.ID).Scan(&dispositionID, &nativeID))
	require.Equal(t, created.ID, dispositionID)
	_, err = db.Exec(`UPDATE shared_pool_supply_dispositions SET reason_code='changed' WHERE source_kind='pool_account' AND source_id=$1`, created.ID)
	require.Error(t, err)
	_, err = db.Exec(`DELETE FROM shared_pool_supply_dispositions WHERE source_kind='pool_account' AND source_id=$1`, created.ID)
	require.Error(t, err)

	require.NoError(t, repo.DetachNativeBinding(context.Background(), draft.ID, ownerID, created.ID))
	var state string
	require.NoError(t, db.QueryRow(`SELECT native_binding_state FROM shared_pool_native_onboarding_progress WHERE source_id=$1`, created.ID).Scan(&state))
	require.Equal(t, "detached", state)
	var dispositionCount, groupCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM shared_pool_supply_dispositions WHERE source_kind='pool_account' AND source_id=$1 AND canonical_account_id=$2`, created.ID, nativeID).Scan(&dispositionCount))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM account_groups WHERE account_id=$1`, nativeID).Scan(&groupCount))
	require.Equal(t, 1, dispositionCount)
	require.Zero(t, groupCount)
	var sharedKey, sharedOAuth string
	require.NoError(t, db.QueryRow(`SELECT upstream_api_key, credentials_encrypted FROM shared_pool_accounts WHERE id=$1`, created.ID).Scan(&sharedKey, &sharedOAuth))
	require.Empty(t, sharedKey)
	require.Empty(t, sharedOAuth)
}

func TestSharedPoolNativeOnboarding_ConcurrentNativeReference(t *testing.T) {
	db := openR1IntegrationDB(t)
	serviceUnderTest, _, _ := newR1IntegrationServices(t, db)
	ownerID := createR1Owner(t, db)
	draft := createR1Draft(t, serviceUnderTest, ownerID, "concurrent")
	first := newR1AccountInput(t, draft.ID, ownerID, "account-concurrent", "secret-two")
	second := newR1AccountInput(t, draft.ID, ownerID, "account-concurrent", "secret-two")

	results := make(chan *service.SharedPoolAccount, 2)
	errorsCh := make(chan error, 2)
	var workers sync.WaitGroup
	for _, input := range []service.SharedPoolNativeAccountInput{first, second} {
		workers.Add(1)
		go func(value service.SharedPoolNativeAccountInput) {
			defer workers.Done()
			result, err := serviceUnderTest.OnboardSharedPoolNativeAccount(context.Background(), value)
			results <- result
			errorsCh <- err
		}(input)
	}
	workers.Wait()
	close(results)
	close(errorsCh)
	for err := range errorsCh {
		require.NoError(t, err)
	}
	var sharedAccountID int64
	for result := range results {
		require.NotNil(t, result)
		if sharedAccountID == 0 {
			sharedAccountID = result.ID
		}
		require.Equal(t, sharedAccountID, result.ID)
	}
	var nativeCount, dispositionCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM accounts WHERE extra->>'bizdecipher_supply_source'='pool_account' AND extra->>'bizdecipher_supply_id'=$1`, fmt.Sprint(sharedAccountID)).Scan(&nativeCount))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM shared_pool_supply_dispositions WHERE source_kind='pool_account' AND source_id=$1`, sharedAccountID).Scan(&dispositionCount))
	require.Equal(t, 1, nativeCount)
	require.Equal(t, 1, dispositionCount)
}

func TestSharedPoolNativeOnboarding_RejectsFingerprintConflict(t *testing.T) {
	db := openR1IntegrationDB(t)
	serviceUnderTest, _, _ := newR1IntegrationServices(t, db)
	ownerID := createR1Owner(t, db)
	firstDraft, err := service.NewSharedPoolNativeDraftInput(ownerID, "first", "", "draft-conflict")
	require.NoError(t, err)
	_, err = serviceUnderTest.CreateSharedPoolNativeDraft(context.Background(), firstDraft)
	require.NoError(t, err)
	conflictingDraft, err := service.NewSharedPoolNativeDraftInput(ownerID, "second", "", "draft-conflict")
	require.NoError(t, err)
	_, err = serviceUnderTest.CreateSharedPoolNativeDraft(context.Background(), conflictingDraft)
	require.ErrorIs(t, err, service.ErrNativeOperationConflict)

	draft := createR1Draft(t, serviceUnderTest, ownerID, "account-conflict-pool")
	_, err = serviceUnderTest.OnboardSharedPoolNativeAccount(context.Background(), newR1AccountInput(t, draft.ID, ownerID, "account-conflict", "secret-a"))
	require.NoError(t, err)
	_, err = serviceUnderTest.OnboardSharedPoolNativeAccount(context.Background(), newR1AccountInput(t, draft.ID, ownerID, "account-conflict", "secret-b"))
	require.ErrorIs(t, err, service.ErrNativeOperationConflict)
}

func TestSharedPoolNativeOnboarding_SoftDeletedIdentityFence(t *testing.T) {
	db := openR1IntegrationDB(t)
	serviceUnderTest, repo, accountRepo := newR1IntegrationServices(t, db)
	ownerID := createR1Owner(t, db)
	draft := createR1Draft(t, serviceUnderTest, ownerID, "soft-delete")
	input := newR1AccountInput(t, draft.ID, ownerID, "soft-delete-account", "secret-three")
	binding, err := repo.ClaimNativeBinding(context.Background(), input)
	require.NoError(t, err)
	native := nativeAccountForR1Test(input, binding)
	require.NoError(t, accountRepo.CreateWithAccountGroups(context.Background(), native, nil))
	_, err = db.Exec(`UPDATE accounts SET deleted_at=NOW() WHERE id=$1`, native.ID)
	require.NoError(t, err)

	_, err = serviceUnderTest.OnboardSharedPoolNativeAccount(context.Background(), input)
	require.ErrorIs(t, err, service.ErrNativeIdentityDeleted)
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM accounts WHERE extra->>'bizdecipher_supply_id'=$1`, fmt.Sprint(binding.Account.ID)).Scan(&count))
	require.Equal(t, 1, count)
}

func TestSharedPoolNativeOnboarding_RepairCASAndCrashRecovery(t *testing.T) {
	db := openR1IntegrationDB(t)
	serviceUnderTest, repo, accountRepo := newR1IntegrationServices(t, db)
	ownerID := createR1Owner(t, db)
	draft := createR1Draft(t, serviceUnderTest, ownerID, "repair")
	created, err := serviceUnderTest.OnboardSharedPoolNativeAccount(context.Background(), newR1AccountInput(t, draft.ID, ownerID, "repair-account", "secret-old"))
	require.NoError(t, err)
	initialOperation := created.NativeOperationID
	version1 := currentR1PoolVersion(t, db, draft.ID)
	repair1 := newR1RepairInput(t, draft.ID, ownerID, created.ID, "repair-one", version1, "secret-new-one")

	failingRepo := &failOnceCompleteNativeRepo{SharedPoolNativeOnboardingRepository: repo}
	serviceWithFault := service.NewBizDecipherService(nil, nil, nil)
	serviceWithFault.SetSharedPoolNativeOnboarding(failingRepo, accountRepo)
	_, firstErr := serviceWithFault.RepairSharedPoolNativeAccount(context.Background(), repair1)
	require.Error(t, firstErr)
	recovered, retryErr := serviceWithFault.RepairSharedPoolNativeAccount(context.Background(), repair1)
	require.NoError(t, retryErr)
	require.Equal(t, initialOperation, recovered.NativeOperationID)
	listed, listErr := repo.(*bizDecipherRepository).ListSharedPoolAccounts(context.Background(), draft.ID, ownerID)
	require.NoError(t, listErr)
	require.Len(t, listed, 1)
	require.Equal(t, initialOperation, listed[0].NativeOperationID)
	require.Equal(t, "attached", listed[0].NativeBindingState)
	require.True(t, listed[0].BillingActivationRequired)

	version2 := currentR1PoolVersion(t, db, draft.ID)
	repair2 := newR1RepairInput(t, draft.ID, ownerID, created.ID, "repair-two", version2, "secret-new-two")
	_, err = serviceUnderTest.RepairSharedPoolNativeAccount(context.Background(), repair2)
	require.NoError(t, err)
	_, err = serviceUnderTest.RepairSharedPoolNativeAccount(context.Background(), repair1)
	require.ErrorIs(t, err, service.ErrSharedPoolConcurrentUpdate)

	account, deleted, err := accountRepo.GetBySharedPoolSource(context.Background(), created.ID)
	require.NoError(t, err)
	require.False(t, deleted)
	require.Equal(t, "secret-new-two", account.Credentials["api_key"])
	require.Equal(t, "repair-two", account.Extra["shared_pool_repair_operation_id"])
	var createOperation, createFingerprint string
	require.NoError(t, db.QueryRow(`SELECT native_operation_id,native_request_fingerprint FROM shared_pool_native_onboarding_progress WHERE source_id=$1`, created.ID).Scan(&createOperation, &createFingerprint))
	require.Equal(t, initialOperation, createOperation)
	require.NotEmpty(t, createFingerprint)
}

func TestSharedPoolNativeOnboarding_CompleteRepairRejectsMissingNativeEvidence(t *testing.T) {
	db := openR1IntegrationDB(t)
	serviceUnderTest, repo, accountRepo := newR1IntegrationServices(t, db)
	ownerID := createR1Owner(t, db)
	draft := createR1Draft(t, serviceUnderTest, ownerID, "repair-evidence")
	created, err := serviceUnderTest.OnboardSharedPoolNativeAccount(context.Background(), newR1AccountInput(t, draft.ID, ownerID, "repair-evidence-account", "secret-old"))
	require.NoError(t, err)
	input := newR1RepairInput(t, draft.ID, ownerID, created.ID, "repair-evidence-op", currentR1PoolVersion(t, db, draft.ID), "secret-new")
	claim, err := repo.ClaimNativeRepair(context.Background(), input)
	require.NoError(t, err)
	account, deleted, err := accountRepo.GetBySharedPoolSource(context.Background(), created.ID)
	require.NoError(t, err)
	require.False(t, deleted)

	_, err = repo.CompleteNativeRepair(context.Background(), input, claim, account)

	require.ErrorIs(t, err, service.ErrNativeOperationConflict)
	var repairState string
	require.NoError(t, db.QueryRow(`SELECT native_repair_state FROM shared_pool_native_onboarding_progress WHERE source_id=$1`, created.ID).Scan(&repairState))
	require.Equal(t, "pending", repairState)
}

type failOnceCompleteNativeRepo struct {
	service.SharedPoolNativeOnboardingRepository
	once sync.Once
}

func (r *failOnceCompleteNativeRepo) CompleteNativeRepair(ctx context.Context, input service.SharedPoolNativeRepairInput, claim *service.SharedPoolNativeRepairClaim, account *service.Account) (*service.SharedPoolAccount, error) {
	failed := false
	r.once.Do(func() { failed = true })
	if failed {
		return nil, errors.New("simulated crash after native commit")
	}
	return r.SharedPoolNativeOnboardingRepository.CompleteNativeRepair(ctx, input, claim, account)
}

func openR1IntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("INTEGRATION_DATABASE_URL"))
	require.NotEmpty(t, dsn, "INTEGRATION_DATABASE_URL is required for r1integration")
	dsnURL, err := url.Parse(dsn)
	require.NoError(t, err)
	require.Contains(t, []string{"postgres", "postgresql"}, dsnURL.Scheme)
	require.Equal(t, "127.0.0.1", dsnURL.Hostname())
	require.Equal(t, "35437", dsnURL.Port())
	require.Equal(t, "r1qa_app", dsnURL.User.Username())
	require.Equal(t, "/r1qa_20260906043015_94365fca", dsnURL.Path)
	require.Equal(t, "disable", dsnURL.Query().Get("sslmode"))
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, db.PingContext(ctx))
	var databaseName, roleName string
	var serverAddress sql.NullString
	var serverPort int
	var superuser, createDB, createRole bool
	require.NoError(t, db.QueryRowContext(ctx, `SELECT current_database(),current_user,inet_server_addr()::text,inet_server_port()`).Scan(&databaseName, &roleName, &serverAddress, &serverPort))
	require.Equal(t, "r1qa_20260906043015_94365fca", databaseName)
	require.Equal(t, "r1qa_app", roleName)
	require.Equal(t, 5432, serverPort)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT rolsuper,rolcreatedb,rolcreaterole FROM pg_roles WHERE rolname=current_user`).Scan(&superuser, &createDB, &createRole))
	require.False(t, superuser)
	require.False(t, createDB)
	require.False(t, createRole)
	r1MigrationOnce.Do(func() { r1MigrationErr = ApplyMigrations(ctx, db) })
	require.NoError(t, r1MigrationErr)
	return db
}

func newR1IntegrationServices(t *testing.T, db *sql.DB) (*service.BizDecipherService, service.SharedPoolNativeOnboardingRepository, service.SharedPoolNativeAccountRepository) {
	t.Helper()
	driver := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(driver))
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	repo := NewSharedPoolNativeOnboardingRepository(db)
	accountRepo := NewSharedPoolNativeAccountRepository(client, db, nil)
	serviceUnderTest := service.NewBizDecipherService(nil, nil, nil)
	serviceUnderTest.SetSharedPoolNativeOnboarding(repo, accountRepo)
	return serviceUnderTest, repo, accountRepo
}

func createR1Owner(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	var id int64
	email := fmt.Sprintf("r1-owner-%d@example.com", time.Now().UnixNano())
	require.NoError(t, db.QueryRow(`INSERT INTO users(email,password_hash,role,balance,concurrency,status) VALUES($1,'hash','user',0,1,'active') RETURNING id`, email).Scan(&id))
	return id
}

func createR1Draft(t *testing.T, serviceUnderTest *service.BizDecipherService, ownerID int64, operationID string) *service.SharedPool {
	t.Helper()
	input, err := service.NewSharedPoolNativeDraftInput(ownerID, "R1 "+operationID, "", operationID)
	require.NoError(t, err)
	pool, err := serviceUnderTest.CreateSharedPoolNativeDraft(context.Background(), input)
	require.NoError(t, err)
	return pool
}

func newR1AccountInput(t *testing.T, poolID, ownerID int64, operationID, secret string) service.SharedPoolNativeAccountInput {
	t.Helper()
	input, err := service.NewSharedPoolNativeAccountInput(service.SharedPoolNativeAccountRequest{
		PoolID: poolID, OwnerID: ownerID, OperationID: operationID, Name: operationID, Provider: service.PlatformOpenAI,
		AuthType: service.AccountTypeAPIKey, UpstreamBaseURL: "https://api.openai.com", APIKey: secret,
	})
	require.NoError(t, err)
	return input
}

func newR1RepairInput(t *testing.T, poolID, ownerID, accountID int64, operationID string, version int64, secret string) service.SharedPoolNativeRepairInput {
	t.Helper()
	input, err := service.NewSharedPoolNativeRepairInput(service.SharedPoolNativeRepairRequest{
		PoolID: poolID, OwnerID: ownerID, SharedAccountID: accountID, RepairOperationID: operationID,
		ExpectedConfigVersion: version, Provider: service.PlatformOpenAI, AuthType: service.AccountTypeAPIKey,
		UpstreamBaseURL: "https://api.openai.com", APIKey: secret,
	})
	require.NoError(t, err)
	return input
}

func currentR1PoolVersion(t *testing.T, db *sql.DB, poolID int64) int64 {
	t.Helper()
	var version int64
	require.NoError(t, db.QueryRow(`SELECT config_version FROM shared_pools WHERE id=$1`, poolID).Scan(&version))
	return version
}

func nativeAccountForR1Test(input service.SharedPoolNativeAccountInput, binding *service.SharedPoolNativeBinding) *service.Account {
	return &service.Account{
		Name: input.Name, Platform: input.Provider, Type: input.AuthType,
		Credentials: map[string]any{"api_key": input.APIKey, "base_url": input.UpstreamBaseURL},
		Extra: map[string]any{
			"bizdecipher_supply_source": "pool_account", "bizdecipher_supply_id": binding.Account.ID,
			"shared_pool_binding_ref": binding.BindingRef, "shared_pool_request_fingerprint": input.RequestFingerprint,
		},
		Concurrency: 1, Priority: 100, Status: service.StatusActive, Schedulable: false,
	}
}
