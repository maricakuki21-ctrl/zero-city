package repository

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type captureSharedPoolPricingQueryMatcher struct {
	actual *string
}

func (m captureSharedPoolPricingQueryMatcher) Match(_ string, actual string) error {
	*m.actual = actual
	return nil
}

func TestSaveSharedPoolCustomPriceVersionAppendOnlyAndOperationIdempotent(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &bizDecipherRepository{db: db}

	inputPrice := 2e-6
	outputPrice := 8e-6
	input := service.SaveSharedPoolCustomPriceInput{
		PoolID: 9, OwnerID: 7, ModelName: "owner-model", EndpointType: "chat",
		OperationID: "price-op-123", BillingMode: "token", InputPrice: &inputPrice,
		OutputPrice: &outputPrice, Multiplier: 1.25,
	}
	priceHash, err := sharedPoolPriceHash("owner-model", "chat", service.SharedPoolPricingSourceOwner, service.SharedPoolPriceComponents{BillingMode: "token", Currency: "USD", InputPrice: &inputPrice, OutputPrice: &outputPrice}, 1.25)
	require.NoError(t, err)
	now := time.Date(2026, 7, 19, 1, 2, 3, 0, time.UTC)

	mock.ExpectBegin()
	expectPricingModelLock(mock, 1)
	mock.ExpectQuery(`(?s)SELECT .*shared_pool_price_versions pv WHERE pv.endpoint_id = \$1 AND pv.operation_id = \$2`).
		WithArgs(int64(42), "price-op-123").
		WillReturnRows(priceVersionRows())
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(version_no\), 0\) \+ 1`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"version_no"}).AddRow(1))
	mock.ExpectQuery(`INSERT INTO shared_pool_price_versions`).
		WillReturnRows(priceVersionRows().AddRow(
			100, 9, 12, 42, "owner_custom", "token", "USD",
			inputPrice, outputPrice, nil, nil, nil, nil, nil, 1.25, nil, nil, now, priceHash, 2,
		))
	mock.ExpectExec(`UPDATE shared_pool_models`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE shared_pool_model_endpoints`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	created, err := repo.SaveSharedPoolCustomPriceVersion(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, int64(100), created.PriceVersionID)
	require.Equal(t, int64(2), created.ConfigVersion)

	// Reusing the same operation reads the original row and performs no INSERT,
	// UPDATE or version increment.
	mock.ExpectBegin()
	expectPricingModelLock(mock, 2)
	mock.ExpectQuery(`(?s)SELECT .*shared_pool_price_versions pv WHERE pv.endpoint_id = \$1 AND pv.operation_id = \$2`).
		WithArgs(int64(42), "price-op-123").
		WillReturnRows(priceVersionRows().AddRow(
			100, 9, 12, 42, "owner_custom", "token", "USD",
			inputPrice, outputPrice, nil, nil, nil, nil, nil, 1.25, nil, nil, now, created.PriceHash, 2,
		))
	mock.ExpectCommit()

	replayed, err := repo.SaveSharedPoolCustomPriceVersion(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, created.PriceVersionID, replayed.PriceVersionID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestResolveSharedPoolOfficialPricingUsesUnlockedCanonicalHandoff(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &bizDecipherRepository{db: db}

	expectOfficialPricingModel(mock, 1, false)
	quote, err := repo.ResolveSharedPoolPriceQuote(context.Background(), 9, "gpt-tiered", "chat")
	require.ErrorIs(t, err, service.ErrSharedPoolOfficialPriceRequired)
	require.Nil(t, quote)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestResolveSharedPoolCustomPriceUsesUnlockedReadPath(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &bizDecipherRepository{db: db}
	inputPrice, outputPrice := 2e-6, 8e-6
	now := time.Date(2026, 7, 19, 2, 3, 4, 0, time.UTC)

	expectOwnerPricingModel(mock, 2, 0, false)
	mock.ExpectQuery(`(?s)SELECT .*shared_pool_price_versions pv WHERE pv.endpoint_id = \$1 AND pv.source = \$2 AND pv.effective_from <= NOW\(\)`).
		WithArgs(int64(42), service.SharedPoolPricingSourceOwner).
		WillReturnRows(priceVersionRows().AddRow(
			202, 9, 12, 42, "owner_custom", "token", "USD", inputPrice, outputPrice,
			nil, nil, nil, nil, nil, 1.25, nil, nil, now, "owner-hash", 2,
		))

	quote, err := repo.ResolveSharedPoolPriceQuote(context.Background(), 9, "owner-model", "chat")
	require.NoError(t, err)
	require.Equal(t, int64(202), quote.PriceVersionID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListSharedPoolEndpointPricingReturnsIdentityAndImmutableVersionWithoutLegacyPriceQuery(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &bizDecipherRepository{db: db}
	now := time.Date(2026, 7, 19, 2, 3, 4, 0, time.UTC)
	inputPrice, outputPrice := 2e-6, 8e-6

	mock.ExpectQuery(`(?s)SELECT spm.id.*COALESCE\(mc_official.model_name, ''\) AS canonical_model_name.*FROM shared_pool_models`).
		WithArgs(int64(9), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"pool_model_id", "provider", "model_name", "display_name", "upstream_model_name", "model_aliases",
			"pricing_source", "pricing_status", "pricing_config_version", "rate_multiplier", "endpoint_id", "endpoint_type",
			"enabled", "gate_status", "media_probe_expires_at", "gate_checked_at", "endpoint_pricing_status", "endpoint_config_version", "canonical_model_name",
			"version_id", "version_pool_id", "version_model_id", "version_endpoint_id", "version_source", "billing_mode", "currency",
			"input_price", "output_price", "cache_read_price", "cache_write_price", "image_item_price", "video_second_price", "per_request_price", "multiplier",
			"minimum_charge", "maximum_charge", "effective_from", "price_hash", "version_config",
		}).AddRow(
			12, "openai", "published-alias", "Official Model", "official-model", []byte(`[]`),
			"official_catalog", "ready", 2, 1.25, 42, "chat", true, "passed", nil, now, "ready", 2, "official-model",
			101, 9, 12, 42, "official_catalog", "token", "USD", inputPrice, outputPrice, nil, nil, nil, nil, nil, 1.25, nil, nil, now, "immutable-hash", 2,
		))

	items, err := repo.ListSharedPoolModelEndpointPricing(context.Background(), 9, 7)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "official-model", items[0].CanonicalModelName)
	require.Equal(t, int64(101), items[0].CurrentPrice.PriceVersionID)
	require.Equal(t, "immutable-hash", items[0].CurrentPrice.PriceHash)
	require.NoError(t, mock.ExpectationsWereMet(), "a second channel_model_pricing query must never occur")
}

func TestListSharedPoolEndpointPricingReadsExpiryAgainstDatabaseTime(t *testing.T) {
	t.Parallel()
	var query string
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(captureSharedPoolPricingQueryMatcher{actual: &query}))
	require.NoError(t, err)
	defer db.Close()
	repo := &bizDecipherRepository{db: db}

	mock.ExpectQuery("realtime shared-pool media gate status").
		WithArgs(int64(9), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"unused"}))

	items, err := repo.ListSharedPoolModelEndpointPricing(context.Background(), 9, 7)
	require.NoError(t, err)
	require.Empty(t, items)
	require.NoError(t, mock.ExpectationsWereMet())

	for _, fragment := range []string{
		"spe.gate_status",
		"spe.media_probe_expires_at",
		"NOW() AS gate_checked_at",
	} {
		require.Truef(t, strings.Contains(query, fragment), "pricing query must contain %q\n%s", fragment, query)
	}
}

func TestEffectiveSharedPoolEndpointGateStatusFailsExpiredMediaClosed(t *testing.T) {
	t.Parallel()
	// Pool verification mode is deliberately not an input: full-check and
	// professional-review pools share the same endpoint-level media expiry gate.
	checkedAt := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	future := sql.NullTime{Time: checkedAt.Add(time.Nanosecond), Valid: true}
	exactlyExpired := sql.NullTime{Time: checkedAt, Valid: true}
	alreadyExpired := sql.NullTime{Time: checkedAt.Add(-time.Nanosecond), Valid: true}
	missingExpiry := sql.NullTime{}

	tests := []struct {
		name       string
		endpoint   string
		gate       string
		expiresAt  sql.NullTime
		checkedAt  time.Time
		wantStatus string
	}{
		{name: "plain chat ignores media expiry", endpoint: service.SharedPoolEndpointChat, gate: "passed", expiresAt: missingExpiry, checkedAt: checkedAt, wantStatus: "passed"},
		{name: "plain responses ignores media expiry", endpoint: service.SharedPoolEndpointResponses, gate: "unverified", expiresAt: missingExpiry, checkedAt: checkedAt, wantStatus: "unverified"},
		{name: "image generation remains passed before expiry", endpoint: service.SharedPoolEndpointImageGeneration, gate: "passed", expiresAt: future, checkedAt: checkedAt, wantStatus: "passed"},
		{name: "image edit becomes stale exactly at expiry", endpoint: service.SharedPoolEndpointImageEdit, gate: "passed", expiresAt: exactlyExpired, checkedAt: checkedAt, wantStatus: "stale"},
		{name: "video becomes stale after expiry", endpoint: service.SharedPoolEndpointVideo, gate: "passed", expiresAt: alreadyExpired, checkedAt: checkedAt, wantStatus: "stale"},
		{name: "professional review cannot make missing video expiry valid", endpoint: service.SharedPoolEndpointVideo, gate: "passed", expiresAt: missingExpiry, checkedAt: checkedAt, wantStatus: "stale"},
		{name: "existing media failure remains failed", endpoint: service.SharedPoolEndpointImageGeneration, gate: "failed", expiresAt: future, checkedAt: checkedAt, wantStatus: "failed"},
		{name: "missing database clock fails passed media closed", endpoint: service.SharedPoolEndpointImageGeneration, gate: "passed", expiresAt: future, checkedAt: time.Time{}, wantStatus: "stale"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.wantStatus, effectiveSharedPoolEndpointGateStatus(tt.endpoint, tt.gate, tt.expiresAt, tt.checkedAt))
		})
	}
}

func TestEnsureSharedPoolOfficialPriceVersionFastPathDoesNotLock(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &bizDecipherRepository{db: db}
	inputPrice := 2e-6
	outputPrice := 8e-6
	base := service.SharedPoolPriceComponents{BillingMode: "token", Currency: "USD", InputPrice: &inputPrice, OutputPrice: &outputPrice}
	priceHash, err := sharedPoolPriceHash("gpt-tiered", "chat", service.SharedPoolPricingSourceOfficial, base, 1.25)
	require.NoError(t, err)
	now := time.Date(2026, 7, 19, 2, 3, 4, 0, time.UTC)

	expectOfficialPricingModel(mock, 3, false)
	expectLatestOfficialPriceVersion(mock, priceVersionRows().AddRow(
		303, 9, 12, 42, "official_catalog", "token", "USD",
		inputPrice, outputPrice, nil, nil, nil, nil, nil, 1.25, nil, nil, now, priceHash, 3,
	))

	quote, err := repo.EnsureSharedPoolOfficialPriceVersion(context.Background(), 9, "gpt-tiered", "chat", base)
	require.NoError(t, err)
	require.Equal(t, int64(303), quote.PriceVersionID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnsureSharedPoolOfficialPriceVersionAtoBtoACreatesThirdVersion(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &bizDecipherRepository{db: db}

	inputA, outputA := 2e-6, 8e-6
	inputB, outputB := 3e-6, 12e-6
	baseA := service.SharedPoolPriceComponents{BillingMode: "token", Currency: "USD", InputPrice: &inputA, OutputPrice: &outputA}
	baseB := service.SharedPoolPriceComponents{BillingMode: "token", Currency: "USD", InputPrice: &inputB, OutputPrice: &outputB}
	hashA, err := sharedPoolPriceHash("gpt-tiered", "chat", service.SharedPoolPricingSourceOfficial, baseA, 1.25)
	require.NoError(t, err)
	hashB, err := sharedPoolPriceHash("gpt-tiered", "chat", service.SharedPoolPricingSourceOfficial, baseB, 1.25)
	require.NoError(t, err)
	t1 := time.Date(2026, 7, 19, 1, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	t3 := t2.Add(time.Hour)

	// Initial A -> immutable version 1.
	expectOfficialVersionCreation(mock, 0, priceVersionRows(), 1, 101, baseA, hashA, t1)
	first, err := repo.EnsureSharedPoolOfficialPriceVersion(context.Background(), 9, "gpt-tiered", "chat", baseA)
	require.NoError(t, err)
	require.Equal(t, int64(101), first.PriceVersionID)

	// A -> B -> immutable version 2.
	latestA := priceVersionRows().AddRow(101, 9, 12, 42, "official_catalog", "token", "USD", inputA, outputA, nil, nil, nil, nil, nil, 1.25, nil, nil, t1, hashA, 1)
	expectOfficialVersionCreation(mock, 1, latestA, 2, 202, baseB, hashB, t2)
	second, err := repo.EnsureSharedPoolOfficialPriceVersion(context.Background(), 9, "gpt-tiered", "chat", baseB)
	require.NoError(t, err)
	require.Equal(t, int64(202), second.PriceVersionID)

	// B -> A must append version 3 instead of reusing historical version 1.
	latestB := priceVersionRows().AddRow(202, 9, 12, 42, "official_catalog", "token", "USD", inputB, outputB, nil, nil, nil, nil, nil, 1.25, nil, nil, t2, hashB, 2)
	expectOfficialVersionCreation(mock, 2, latestB, 3, 303, baseA, hashA, t3)
	third, err := repo.EnsureSharedPoolOfficialPriceVersion(context.Background(), 9, "gpt-tiered", "chat", baseA)
	require.NoError(t, err)
	require.Equal(t, int64(303), third.PriceVersionID)
	require.NotEqual(t, first.PriceVersionID, third.PriceVersionID)
	require.Equal(t, int64(3), third.ConfigVersion)
	require.NoError(t, mock.ExpectationsWereMet())
}

func expectOfficialVersionCreation(mock sqlmock.Sqlmock, currentConfig int64, latest *sqlmock.Rows, versionNo, versionID int64, base service.SharedPoolPriceComponents, priceHash string, effectiveFrom time.Time) {
	expectOfficialPricingModel(mock, currentConfig, false)
	expectLatestOfficialPriceVersion(mock, latest)
	mock.ExpectBegin()
	expectOfficialPricingModel(mock, currentConfig, true)
	// Return a fresh rows object for the lock-protected recheck.
	lockedLatest := priceVersionRows()
	if currentConfig > 0 {
		previousHash := priceHash
		previousInput, previousOutput := *base.InputPrice, *base.OutputPrice
		if versionNo == 2 {
			previousInput, previousOutput = 2e-6, 8e-6
			previousHash, _ = sharedPoolPriceHash("gpt-tiered", "chat", service.SharedPoolPricingSourceOfficial, service.SharedPoolPriceComponents{BillingMode: "token", Currency: "USD", InputPrice: &previousInput, OutputPrice: &previousOutput}, 1.25)
		} else if versionNo == 3 {
			previousInput, previousOutput = 3e-6, 12e-6
			previousHash, _ = sharedPoolPriceHash("gpt-tiered", "chat", service.SharedPoolPricingSourceOfficial, service.SharedPoolPriceComponents{BillingMode: "token", Currency: "USD", InputPrice: &previousInput, OutputPrice: &previousOutput}, 1.25)
		}
		lockedLatest.AddRow(versionID-101, 9, 12, 42, "official_catalog", "token", "USD", previousInput, previousOutput, nil, nil, nil, nil, nil, 1.25, nil, nil, effectiveFrom.Add(-time.Hour), previousHash, currentConfig)
	}
	expectLatestOfficialPriceVersion(mock, lockedLatest)
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(version_no\), 0\) \+ 1`).WithArgs(int64(42)).WillReturnRows(sqlmock.NewRows([]string{"version_no"}).AddRow(versionNo))
	mock.ExpectQuery(`INSERT INTO shared_pool_price_versions`).WillReturnRows(priceVersionRows().AddRow(
		versionID, 9, 12, 42, "official_catalog", "token", "USD",
		*base.InputPrice, *base.OutputPrice, nil, nil, nil, nil, nil, 1.25, nil, nil, effectiveFrom, priceHash, versionNo,
	))
	mock.ExpectExec(`UPDATE shared_pool_models`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE shared_pool_model_endpoints`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
}

func expectLatestOfficialPriceVersion(mock sqlmock.Sqlmock, rows *sqlmock.Rows) {
	mock.ExpectQuery(`(?s)SELECT .*shared_pool_price_versions pv WHERE pv.endpoint_id = \$1 AND pv.source = \$2 AND pv.effective_from <= NOW\(\)`).
		WithArgs(int64(42), service.SharedPoolPricingSourceOfficial).
		WillReturnRows(rows)
}

func expectOfficialPricingModel(mock sqlmock.Sqlmock, configVersion int64, forUpdate bool) {
	pattern := `(?s)SELECT spm.id.*LIMIT 1$`
	if forUpdate {
		pattern = `(?s)SELECT spm.id.*FOR UPDATE OF spm, spe$`
	}
	mock.ExpectQuery(pattern).
		WithArgs(int64(9), int64(0), "gpt-tiered", "chat").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "pool_id", "provider", "model_name", "display_name", "upstream_model_name", "model_aliases",
			"pricing_source", "pricing_status", "pricing_config_version", "rate_multiplier", "endpoint_id",
			"endpoint_type", "enabled", "gate_status", "endpoint_pricing_status", "endpoint_config_version", "official",
		}).AddRow(12, 9, "openai", "gpt-tiered", "GPT Tiered", "gpt-tiered", []byte(`[]`),
			"official_catalog", "ready", configVersion, 1.25, 42, "chat", true, "passed", "ready", configVersion, true))
}

func expectPricingModelLock(mock sqlmock.Sqlmock, configVersion int64) {
	expectOwnerPricingModel(mock, configVersion, 7, true)
}

func expectOwnerPricingModel(mock sqlmock.Sqlmock, configVersion, ownerID int64, forUpdate bool) {
	pattern := `(?s)SELECT spm.id.*LIMIT 1$`
	if forUpdate {
		pattern = `(?s)SELECT spm.id.*FOR UPDATE OF spm, spe$`
	}
	mock.ExpectQuery(pattern).
		WithArgs(int64(9), ownerID, "owner-model", "chat").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "pool_id", "provider", "model_name", "display_name", "upstream_model_name", "model_aliases",
			"pricing_source", "pricing_status", "pricing_config_version", "rate_multiplier", "endpoint_id",
			"endpoint_type", "enabled", "gate_status", "endpoint_pricing_status", "endpoint_config_version", "official",
		}).AddRow(12, 9, "openai", "owner-model", "Owner Model", "owner-model", []byte(`[]`),
			"owner_custom", "ready", configVersion, 1.25, 42, "chat", true, "passed", "ready", configVersion, false))
}

func priceVersionRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "pool_id", "pool_model_id", "endpoint_id", "source", "billing_mode", "currency",
		"input_price", "output_price", "cache_read_price", "cache_write_price", "image_item_price", "video_second_price", "per_request_price",
		"multiplier", "minimum_charge", "maximum_charge", "effective_from", "price_hash", "config_version",
	})
}
