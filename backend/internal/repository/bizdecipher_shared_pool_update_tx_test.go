package repository

import (
	"context"
	"database/sql/driver"
	"errors"
	"regexp"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

const sharedPoolUpdateLockQuery = "SELECT upstream_api_key, hourly_seat_fee, hourly_min_usage_waiver, platform_fee_percent, account_mode_enabled, config_version, COALESCE(governance_status, 'normal') FROM shared_pools WHERE id = $1 AND owner_id = $2 FOR UPDATE"

func sharedPoolUpdateResultRow(configVersion int64) *sqlmock.Rows {
	columns := strings.Split(sharedPoolColumns, ", ")
	values := make([]driver.Value, len(columns))
	for i, column := range columns {
		switch column {
		case "id":
			values[i] = int64(7)
		case "owner_id":
			values[i] = int64(19)
		case "max_users", "current_users", "avg_latency_ms", "consecutive_probe_failures",
			"account_concurrency", "user_concurrency", "complaint_count", "like_count",
			"total_calls", "successful_calls", "failed_calls":
			values[i] = int64(0)
		case "featured_score", "rate_multiplier", "min_balance_admission", "hourly_seat_fee",
			"hourly_min_usage_waiver", "today_availability", "seven_day_availability",
			"owner_share_percent", "platform_fee_percent", "quality_score", "rank_weight",
			"reward_score", "penalty_score", "market_score":
			values[i] = float64(0)
		case "listed", "upstream_api_key <> ''", "account_mode_enabled", "owner_paused":
			values[i] = false
		case "card_skin_key", "card_skin_rarity", "last_probe_at", "last_probe_success",
			"last_successful_probe_at", "proxy_id", "archived_at", "archived_by":
			values[i] = nil
		case "status":
			values[i] = "healthy"
		case "governance_status":
			values[i] = "normal"
		case "lifecycle_state":
			values[i] = "operating"
		case "config_version":
			values[i] = configVersion
		default:
			values[i] = ""
		}
	}
	return sqlmock.NewRows(columns).AddRow(values...)
}

type sharedPoolConfigVersionScanner struct {
	t *testing.T
}

func (s sharedPoolConfigVersionScanner) Scan(dest ...any) error {
	s.t.Helper()
	require.Len(s.t, dest, len(strings.Split(sharedPoolColumns, ", ")))
	version, ok := dest[len(dest)-2].(*int64)
	require.True(s.t, ok, "config_version must precede native_onboarding_state")
	*version = 23
	state, ok := dest[len(dest)-1].(*string)
	require.True(s.t, ok, "native_onboarding_state must be the final shared-pool scan destination")
	*state = service.SharedPoolOnboardingReadyBillingBlocked
	return nil
}

func TestScanSharedPoolIncludesConfigVersion(t *testing.T) {
	pool, err := scanSharedPool(sharedPoolConfigVersionScanner{t: t})
	require.NoError(t, err)
	require.Equal(t, int64(23), pool.ConfigVersion)
	require.Equal(t, service.SharedPoolOnboardingReadyBillingBlocked, pool.NativeOnboardingState)
	require.True(t, pool.BillingActivationRequired)
}

func TestGetNativeBindingAuthorizesOwnerBeforeNativeJoin(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &bizDecipherRepository{db: db}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT 1 FROM shared_pools WHERE id=$1 AND owner_id=$2 AND lifecycle_state<>'archived'`)).
		WithArgs(int64(7), int64(19)).
		WillReturnRows(sqlmock.NewRows([]string{"owned"}))

	_, err = repo.GetNativeBinding(context.Background(), 7, 19, 33)

	require.ErrorIs(t, err, service.ErrPoolForbidden)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateSharedPoolTxRejectsStaleConfigVersion(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &bizDecipherRepository{db: db}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(sharedPoolUpdateLockQuery)).
		WithArgs(int64(7), int64(19)).
		WillReturnRows(sqlmock.NewRows([]string{
			"upstream_api_key", "hourly_seat_fee", "hourly_min_usage_waiver",
			"platform_fee_percent", "account_mode_enabled", "config_version", "governance_status",
		}).AddRow("stored-secret", 0.1, 1.0, 10.0, false, int64(8), "normal"))
	mock.ExpectRollback()

	_, err = repo.UpdateSharedPoolTx(context.Background(), 7, 19, service.UpdateSharedPoolInput{
		ExpectedConfigVersion: 7,
	})
	require.ErrorIs(t, err, service.ErrSharedPoolConcurrentUpdate)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateSharedPoolTxRejectsSuppressedListing(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &bizDecipherRepository{db: db}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(sharedPoolUpdateLockQuery)).
		WithArgs(int64(7), int64(19)).
		WillReturnRows(sqlmock.NewRows([]string{
			"upstream_api_key", "hourly_seat_fee", "hourly_min_usage_waiver",
			"platform_fee_percent", "account_mode_enabled", "config_version", "governance_status",
		}).AddRow("stored-secret", 0.1, 1.0, 10.0, false, int64(8), "suppressed"))
	mock.ExpectRollback()

	_, err = repo.UpdateSharedPoolTx(context.Background(), 7, 19, service.UpdateSharedPoolInput{
		ExpectedConfigVersion: 8,
		Listed:                true,
	})
	require.True(t, errors.Is(err, service.ErrSharedPoolGovernanceBlocked))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateSharedPoolTxReturnsConfigVersionAfterModelTriggers(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &bizDecipherRepository{db: db}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(sharedPoolUpdateLockQuery)).
		WithArgs(int64(7), int64(19)).
		WillReturnRows(sqlmock.NewRows([]string{
			"upstream_api_key", "hourly_seat_fee", "hourly_min_usage_waiver",
			"platform_fee_percent", "account_mode_enabled", "config_version", "governance_status",
		}).AddRow("stored-secret", 0.0, 0.0, 0.0, false, int64(8), "normal"))
	mock.ExpectQuery("UPDATE shared_pools SET").
		WillReturnRows(sharedPoolUpdateResultRow(9))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE shared_pool_models SET enabled = FALSE WHERE pool_id = $1")).
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO shared_pool_models").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE shared_pool_access_keys SET allowed_models = $1::jsonb, updated_at = NOW() WHERE pool_id = $2 AND status = 'active'")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE shared_pool_models SET rate_multiplier = $2 WHERE pool_id = $1 AND enabled = TRUE")).
		WithArgs(int64(7), 0.0001).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT config_version FROM shared_pools WHERE id = $1 AND owner_id = $2")).
		WithArgs(int64(7), int64(19)).
		WillReturnRows(sqlmock.NewRows([]string{"config_version"}).AddRow(int64(11)))
	mock.ExpectCommit()
	settlementColumns := []string{"pool_id", "hourly_seat_fee", "hourly_min_usage_waiver", "platform_fee_percent", "effective_from"}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT DISTINCT ON (pool_id)")).
		WithArgs(sqlmock.AnyArg(), int64(7)).
		WillReturnRows(sqlmock.NewRows(settlementColumns))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT DISTINCT ON (pool_id)")).
		WithArgs(sqlmock.AnyArg(), int64(7)).
		WillReturnRows(sqlmock.NewRows(settlementColumns))

	pool, err := repo.UpdateSharedPoolTx(context.Background(), 7, 19, service.UpdateSharedPoolInput{
		ExpectedConfigVersion: 8,
		RateMultiplier:        0.0001,
		RateMultiplierSet:     true,
		SyncModelRates:        true,
		ModelConfigsSet:       true,
		ModelConfigs: []service.SharedPoolModelInput{{
			Provider:       "openai",
			ModelName:      "gpt-5.6",
			RateMultiplier: 1,
			MaxConcurrency: 1,
			ModelOpen:      true,
		}},
	})
	require.NoError(t, err)
	require.Equal(t, int64(11), pool.ConfigVersion)
	require.Len(t, pool.ModelConfigs, 1)
	require.InDelta(t, 0.0001, pool.ModelConfigs[0].RateMultiplier, 1e-12)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSharedPoolModelUpdateRequested(t *testing.T) {
	require.False(t, sharedPoolModelUpdateRequested(service.UpdateSharedPoolInput{}))
	require.True(t, sharedPoolModelUpdateRequested(service.UpdateSharedPoolInput{ModelsSet: true}))
	require.True(t, sharedPoolModelUpdateRequested(service.UpdateSharedPoolInput{ModelConfigsSet: true}))
}

func TestSharedPoolModelRateSyncRequested(t *testing.T) {
	require.False(t, sharedPoolModelRateSyncRequested(service.UpdateSharedPoolInput{}))
	require.False(t, sharedPoolModelRateSyncRequested(service.UpdateSharedPoolInput{RateMultiplierSet: true}))
	require.False(t, sharedPoolModelRateSyncRequested(service.UpdateSharedPoolInput{SyncModelRates: true}))
	require.True(t, sharedPoolModelRateSyncRequested(service.UpdateSharedPoolInput{RateMultiplierSet: true, SyncModelRates: true}))
}
