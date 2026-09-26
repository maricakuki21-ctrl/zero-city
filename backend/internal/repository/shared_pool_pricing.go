package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const sharedPoolPriceVersionColumns = `
	pv.id, pv.pool_id, pv.pool_model_id, pv.endpoint_id, pv.source,
	pv.billing_mode, pv.currency, pv.input_price, pv.output_price,
	pv.cache_read_price, pv.cache_write_price, pv.image_item_price,
	pv.video_second_price, pv.per_request_price,
	pv.multiplier, pv.minimum_charge, pv.maximum_charge, pv.effective_from,
	pv.price_hash, pv.config_version`

type sharedPoolPricingModel struct {
	PoolModelID           int64
	PoolID                int64
	Provider              string
	ModelName             string
	DisplayName           string
	UpstreamModelName     string
	Aliases               []string
	PricingSource         string
	PricingStatus         string
	PricingConfigVersion  int64
	RateMultiplier        float64
	EndpointID            int64
	EndpointType          string
	EndpointEnabled       bool
	GateStatus            string
	EndpointPricingStatus string
	EndpointConfigVersion int64
	CanonicalModelName    string
	Official              bool
}

func normalizeSharedPoolProviderName(provider string) string {
	normalized := strings.ToLower(strings.TrimSpace(provider))
	switch normalized {
	case "openai_compatible", "openai-compatible", "openai compatible", "openai 兼容中转", "openai兼容中转", "openai 兼容", "openai兼容", "兼容中转":
		return "openai"
	case "grok", "x.ai":
		return "xai"
	case "gemini", "google gemini", "google_gemini", "google-gemini":
		return "google"
	case "claude":
		return "anthropic"
	default:
		return normalized
	}
}

// sharedPoolCanonicalProviderSQL mirrors normalizeSharedPoolProviderName for
// existing rows created before provider names were stored canonically. The
// column argument is always a repository-owned SQL identifier, never input.
func sharedPoolCanonicalProviderSQL(column string) string {
	normalized := "LOWER(BTRIM(" + column + "))"
	return `CASE ` + normalized + `
		WHEN 'openai_compatible' THEN 'openai'
		WHEN 'openai-compatible' THEN 'openai'
		WHEN 'openai compatible' THEN 'openai'
		WHEN 'openai 兼容中转' THEN 'openai'
		WHEN 'openai兼容中转' THEN 'openai'
		WHEN 'openai 兼容' THEN 'openai'
		WHEN 'openai兼容' THEN 'openai'
		WHEN '兼容中转' THEN 'openai'
		WHEN 'grok' THEN 'xai'
		WHEN 'x.ai' THEN 'xai'
		WHEN 'gemini' THEN 'google'
		WHEN 'google gemini' THEN 'google'
		WHEN 'google_gemini' THEN 'google'
		WHEN 'google-gemini' THEN 'google'
		WHEN 'claude' THEN 'anthropic'
		ELSE ` + normalized + `
	END`
}

func sharedPoolEffectiveRateMultiplierSQL(modelRateColumn, poolRateColumn string) string {
	return "COALESCE(NULLIF(" + modelRateColumn + ", 0), NULLIF(" + poolRateColumn + ", 0), 1)"
}

// SharedPoolPlatformFeePercent returns the fee rule currently visible to a
// buyer. The rule-version table is authoritative; the pool column is only a
// compatibility fallback for pools created before versioned settlement rules.
func (r *bizDecipherRepository) SharedPoolPlatformFeePercent(ctx context.Context, poolID int64) (float64, error) {
	if r == nil || r.db == nil || poolID <= 0 {
		return 0, nil
	}
	var fee float64
	err := r.db.QueryRowContext(ctx, `
		SELECT platform_fee_percent
		FROM shared_pool_settlement_rule_versions
		WHERE pool_id = $1 AND effective_from <= NOW()
		ORDER BY effective_from DESC, id DESC
		LIMIT 1`, poolID).Scan(&fee)
	if err == sql.ErrNoRows {
		err = r.db.QueryRowContext(ctx, `SELECT COALESCE(platform_fee_percent, 0) FROM shared_pools WHERE id = $1`, poolID).Scan(&fee)
	}
	if err != nil {
		return 0, err
	}
	if fee < 0 || fee > 100 {
		return 0, errors.New("invalid shared pool platform fee")
	}
	return fee, nil
}

func (r *bizDecipherRepository) ListSharedPoolModelEndpointPricing(ctx context.Context, poolID, viewerID int64) ([]service.SharedPoolModelEndpointPricing, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT spm.id, spm.provider, spm.model_name,
		       COALESCE(NULLIF(spm.display_name, ''), NULLIF(mc_official.display_name, ''), spm.model_name),
		       spm.upstream_model_name, spm.model_aliases, spm.pricing_source,
		       spm.pricing_status, spm.pricing_config_version, `+sharedPoolEffectiveRateMultiplierSQL("spm.rate_multiplier", "sp.rate_multiplier")+`,
		       spe.id, spe.endpoint_type, spe.enabled, spe.gate_status,
		       spe.media_probe_expires_at, NOW() AS gate_checked_at,
		       spe.pricing_status, spe.config_version,
		       COALESCE(mc_official.model_name, '') AS canonical_model_name,
		       `+sharedPoolPriceVersionColumns+`
		FROM shared_pool_models spm
		JOIN shared_pools sp ON sp.id = spm.pool_id
		JOIN shared_pool_model_endpoints spe ON spe.pool_model_id = spm.id
		LEFT JOIN LATERAL (
			SELECT mc.model_name, mc.display_name
			FROM model_catalog mc
			WHERE mc.enabled = TRUE
			  AND LOWER(mc.provider) = `+sharedPoolCanonicalProviderSQL("spm.provider")+`
			  AND (
				LOWER(mc.model_name) = LOWER(spm.model_name)
				OR EXISTS (
					SELECT 1
					FROM jsonb_array_elements_text(
						CASE WHEN jsonb_typeof(mc.aliases) = 'array' THEN mc.aliases ELSE '[]'::jsonb END
					) AS official_alias(value)
					WHERE LOWER(official_alias.value) = LOWER(spm.model_name)
				)
			  )
			ORDER BY CASE WHEN LOWER(mc.model_name) = LOWER(spm.model_name) THEN 0 ELSE 1 END,
			         mc.sort_order, mc.id
			LIMIT 1
		) mc_official ON TRUE
		LEFT JOIN LATERAL (
			SELECT * FROM shared_pool_price_versions version
			WHERE version.endpoint_id = spe.id AND version.effective_from <= NOW()
			ORDER BY version.effective_from DESC, version.version_no DESC
			LIMIT 1
		) pv ON TRUE
		WHERE sp.id = $1
		  AND spm.enabled = TRUE
		  AND spe.endpoint_type IN ('chat', 'responses', 'image_generation', 'image_edit', 'video')
		  AND (
		    ($2 > 0 AND sp.owner_id = $2)
		    OR (sp.listed = TRUE AND sp.lifecycle_state = 'operating' AND sp.governance_status NOT IN ('banned', 'suppressed'))
		  )
		ORDER BY spm.sort_order, spm.id, spe.endpoint_type`, poolID, viewerID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.SharedPoolModelEndpointPricing, 0)
	for rows.Next() {
		var model sharedPoolPricingModel
		var aliasesRaw []byte
		var versionID, versionPoolID, versionModelID, versionEndpointID, versionConfig sql.NullInt64
		var versionSource, billingMode, currency, priceHash sql.NullString
		var inputPrice, outputPrice, cacheReadPrice, cacheWritePrice, imageItemPrice, videoSecondPrice, perRequestPrice sql.NullFloat64
		var multiplier, minimumCharge, maximumCharge sql.NullFloat64
		var effectiveFrom sql.NullTime
		var mediaProbeExpiresAt sql.NullTime
		var gateCheckedAt time.Time
		if err := rows.Scan(
			&model.PoolModelID, &model.Provider, &model.ModelName, &model.DisplayName,
			&model.UpstreamModelName, &aliasesRaw, &model.PricingSource,
			&model.PricingStatus, &model.PricingConfigVersion, &model.RateMultiplier,
			&model.EndpointID, &model.EndpointType, &model.EndpointEnabled, &model.GateStatus,
			&mediaProbeExpiresAt, &gateCheckedAt,
			&model.EndpointPricingStatus, &model.EndpointConfigVersion, &model.CanonicalModelName,
			&versionID, &versionPoolID, &versionModelID, &versionEndpointID, &versionSource,
			&billingMode, &currency, &inputPrice, &outputPrice, &cacheReadPrice, &cacheWritePrice,
			&imageItemPrice, &videoSecondPrice,
			&perRequestPrice, &multiplier, &minimumCharge, &maximumCharge, &effectiveFrom,
			&priceHash, &versionConfig,
		); err != nil {
			return nil, err
		}
		model.PoolID = poolID
		model.Aliases = scanStringArray(aliasesRaw)
		model.Official = strings.TrimSpace(model.CanonicalModelName) != ""
		model.GateStatus = effectiveSharedPoolEndpointGateStatus(model.EndpointType, model.GateStatus, mediaProbeExpiresAt, gateCheckedAt)
		item := service.SharedPoolModelEndpointPricing{
			PoolModelID: model.PoolModelID, Provider: model.Provider, ModelName: model.ModelName,
			DisplayName: model.DisplayName, PricingSource: model.PricingSource,
			PricingStatus: model.PricingStatus, PricingConfigVersion: model.PricingConfigVersion,
			EndpointID: model.EndpointID, EndpointType: model.EndpointType, Enabled: model.EndpointEnabled,
			GateStatus: model.GateStatus, EndpointPricingStatus: model.EndpointPricingStatus,
			ConfigVersion: model.EndpointConfigVersion, CanonicalModelName: model.CanonicalModelName,
			PoolID: model.PoolID, RateMultiplier: model.RateMultiplier,
		}
		if model.Official {
			item.PricingSource = service.SharedPoolPricingSourceOfficial
			if versionID.Valid && versionSource.String == service.SharedPoolPricingSourceOfficial {
				item.CurrentPrice = quoteFromNullableVersion(model, versionID, versionPoolID, versionModelID, versionEndpointID, versionSource, billingMode, currency, inputPrice, outputPrice, cacheReadPrice, cacheWritePrice, imageItemPrice, videoSecondPrice, perRequestPrice, multiplier, minimumCharge, maximumCharge, effectiveFrom, priceHash, versionConfig)
			}
		} else {
			item.PricingSource = service.SharedPoolPricingSourceOwner
			if versionID.Valid && versionSource.String == service.SharedPoolPricingSourceOwner {
				quote := quoteFromNullableVersion(model, versionID, versionPoolID, versionModelID, versionEndpointID, versionSource, billingMode, currency, inputPrice, outputPrice, cacheReadPrice, cacheWritePrice, imageItemPrice, videoSecondPrice, perRequestPrice, multiplier, minimumCharge, maximumCharge, effectiveFrom, priceHash, versionConfig)
				service.FinalizeSharedPoolPriceQuote(quote)
				item.CurrentPrice = quote
			}
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func effectiveSharedPoolEndpointGateStatus(endpointType, gateStatus string, mediaProbeExpiresAt sql.NullTime, checkedAt time.Time) string {
	endpointType = strings.ToLower(strings.TrimSpace(endpointType))
	gateStatus = strings.ToLower(strings.TrimSpace(gateStatus))
	switch endpointType {
	case service.SharedPoolEndpointImageGeneration, service.SharedPoolEndpointImageEdit, service.SharedPoolEndpointVideo:
		if gateStatus == "passed" && (!mediaProbeExpiresAt.Valid || checkedAt.IsZero() || !mediaProbeExpiresAt.Time.After(checkedAt)) {
			return "stale"
		}
	}
	return gateStatus
}

func (r *bizDecipherRepository) SaveSharedPoolCustomPriceVersion(ctx context.Context, input service.SaveSharedPoolCustomPriceInput) (*service.SharedPoolPriceQuote, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := ensureSharedPoolMediaPricingEndpoint(ctx, tx, input); err != nil {
		return nil, err
	}
	model, err := lockSharedPoolPricingModel(ctx, tx, input.PoolID, input.OwnerID, input.ModelName, input.EndpointType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrPoolForbidden
		}
		return nil, err
	}
	if model.Official {
		return nil, service.ErrSharedPoolOfficialPriceOnly
	}
	base := service.SharedPoolPriceComponents{
		BillingMode: input.BillingMode, Currency: "USD", InputPrice: input.InputPrice,
		OutputPrice: input.OutputPrice, CacheReadPrice: input.CacheReadPrice,
		CacheWritePrice: input.CacheWritePrice, ImageItemPrice: input.ImageItemPrice,
		VideoSecondPrice: input.VideoSecondPrice, PerRequestPrice: input.PerRequestPrice,
		MinimumCharge: input.MinimumCharge, MaximumCharge: input.MaximumCharge,
	}
	priceHash, err := sharedPoolPriceHash(model.ModelName, model.EndpointType, service.SharedPoolPricingSourceOwner, base, input.Multiplier)
	if err != nil {
		return nil, err
	}

	existing, err := selectSharedPoolPriceVersionByOperation(ctx, tx, model, input.OperationID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if existing != nil {
		if existing.PriceHash != priceHash {
			return nil, service.ErrSharedPoolPricingOperationID
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		service.FinalizeSharedPoolPriceQuote(existing)
		return existing, nil
	}

	var versionNo int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version_no), 0) + 1 FROM shared_pool_price_versions WHERE endpoint_id = $1`, model.EndpointID).Scan(&versionNo); err != nil {
		return nil, err
	}
	configVersion := model.EndpointConfigVersion + 1
	now := time.Now().UTC()
	quote, err := insertSharedPoolPriceVersion(ctx, tx, *model, versionNo, configVersion, input.OperationID, service.SharedPoolPricingSourceOwner, base, input.Multiplier, priceHash, input.OwnerID, now)
	if err != nil {
		return nil, err
	}
	customJSON, _ := json.Marshal(map[string]any{
		"endpoint_type": input.EndpointType, "billing_mode": input.BillingMode,
		"input_price": input.InputPrice, "output_price": input.OutputPrice,
		"cache_read_price": input.CacheReadPrice, "cache_write_price": input.CacheWritePrice,
		"image_item_price": input.ImageItemPrice, "video_second_price": input.VideoSecondPrice,
		"per_request_price": input.PerRequestPrice, "minimum_charge": input.MinimumCharge,
		"maximum_charge": input.MaximumCharge, "multiplier": input.Multiplier,
		"price_version_id": quote.PriceVersionID,
	})
	if _, err := tx.ExecContext(ctx, `UPDATE shared_pool_models
		SET pricing_source = 'owner_custom', pricing_status = 'ready',
		    pricing_config_version = pricing_config_version + 1,
		    custom_pricing = $2, pricing_updated_at = $3,
		    rate_multiplier = $4
		WHERE id = $1`, model.PoolModelID, customJSON, now, input.Multiplier); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE shared_pool_model_endpoints
		SET pricing_status = 'ready', config_version = $2, updated_at = $3
		WHERE id = $1`, model.EndpointID, configVersion, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	service.FinalizeSharedPoolPriceQuote(quote)
	return quote, nil
}

func ensureSharedPoolMediaPricingEndpoint(ctx context.Context, tx *sql.Tx, input service.SaveSharedPoolCustomPriceInput) error {
	var upstreamPath, adapter string
	asyncMode := false
	switch input.EndpointType {
	case service.SharedPoolEndpointImageGeneration:
		upstreamPath, adapter = "/v1/images/generations", "openai_compatible"
	case service.SharedPoolEndpointImageEdit:
		upstreamPath, adapter = "/v1/images/edits", "openai_compatible"
	case service.SharedPoolEndpointVideo:
		upstreamPath, adapter, asyncMode = "/v1/videos/generations", "grok_media", true
	default:
		return nil
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO shared_pool_model_endpoints (
			pool_model_id, endpoint_type, upstream_path, adapter, async_mode,
			enabled, gate_status, pricing_status
		)
		SELECT spm.id, $4, $5, $6, $7, TRUE, 'unverified', 'pending'
		FROM shared_pool_models spm
		JOIN shared_pools sp ON sp.id = spm.pool_id
		WHERE sp.id = $1 AND sp.owner_id = $2
		  AND sp.lifecycle_state <> 'archived'
		  AND spm.enabled = TRUE AND spm.model_open = TRUE
		  AND (LOWER(spm.model_name) = LOWER($3) OR spm.model_aliases ? $3)
		ON CONFLICT (pool_model_id, endpoint_type) DO NOTHING`,
		input.PoolID, input.OwnerID, strings.TrimSpace(input.ModelName), input.EndpointType,
		upstreamPath, adapter, asyncMode)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected == 0 {
		// A pre-existing row is expected on replay. Ownership is validated by the
		// subsequent locked model query; do not turn this into a false failure.
		return nil
	}
	return nil
}

func (r *bizDecipherRepository) ResolveSharedPoolPriceQuote(ctx context.Context, poolID int64, modelName, endpointType string) (*service.SharedPoolPriceQuote, error) {
	model, err := readSharedPoolPricingModel(ctx, r.db, poolID, 0, modelName, endpointType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrSharedPoolPricingUnavailable
		}
		return nil, err
	}
	if !model.EndpointEnabled || !sharedPoolEndpointGateAllowsRouting(model.EndpointType, model.GateStatus) {
		return nil, service.ErrSharedPoolPricingUnavailable
	}
	if model.Official {
		// Official prices are intentionally resolved by BillingService, the same
		// canonical source used for normal gateway billing.
		return nil, service.ErrSharedPoolOfficialPriceRequired
	}
	quote, err := selectLatestSharedPoolPriceVersion(ctx, r.db, model, service.SharedPoolPricingSourceOwner)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrSharedPoolPricingUnavailable
		}
		return nil, err
	}
	service.FinalizeSharedPoolPriceQuote(quote)
	return quote, nil
}

func (r *bizDecipherRepository) EnsureSharedPoolOfficialPriceVersion(ctx context.Context, poolID int64, modelName, endpointType string, base service.SharedPoolPriceComponents) (*service.SharedPoolPriceQuote, error) {
	// Fast path: reads never acquire a row lock. The overwhelmingly common case
	// is that the latest immutable version already matches BillingService.
	model, err := readSharedPoolPricingModel(ctx, r.db, poolID, 0, modelName, endpointType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrSharedPoolPricingUnavailable
		}
		return nil, err
	}
	if !model.Official || !model.EndpointEnabled || !sharedPoolEndpointGateAllowsRouting(model.EndpointType, model.GateStatus) {
		return nil, service.ErrSharedPoolPricingUnavailable
	}
	priceHash, err := sharedPoolPriceHash(model.ModelName, model.EndpointType, service.SharedPoolPricingSourceOfficial, base, model.RateMultiplier)
	if err != nil {
		return nil, err
	}
	latest, err := selectLatestSharedPoolPriceVersion(ctx, r.db, model, service.SharedPoolPricingSourceOfficial)
	if err == nil && latest.PriceHash == priceHash {
		service.FinalizeSharedPoolPriceQuote(latest)
		return latest, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	// Slow path: serialize version creation, then re-read the latest row under
	// lock so concurrent requests cannot create duplicate versions.
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	model, err = lockSharedPoolPricingModel(ctx, tx, poolID, 0, modelName, endpointType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrSharedPoolPricingUnavailable
		}
		return nil, err
	}
	if !model.Official || !model.EndpointEnabled || !sharedPoolEndpointGateAllowsRouting(model.EndpointType, model.GateStatus) {
		return nil, service.ErrSharedPoolPricingUnavailable
	}
	priceHash, err = sharedPoolPriceHash(model.ModelName, model.EndpointType, service.SharedPoolPricingSourceOfficial, base, model.RateMultiplier)
	if err != nil {
		return nil, err
	}
	latest, err = selectLatestSharedPoolPriceVersion(ctx, tx, model, service.SharedPoolPricingSourceOfficial)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if latest != nil && latest.PriceHash == priceHash {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		service.FinalizeSharedPoolPriceQuote(latest)
		return latest, nil
	}
	var versionNo int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version_no), 0) + 1 FROM shared_pool_price_versions WHERE endpoint_id = $1`, model.EndpointID).Scan(&versionNo); err != nil {
		return nil, err
	}
	configVersion := model.EndpointConfigVersion + 1
	now := time.Now().UTC()
	operationID := fmt.Sprintf("official:%d:%s", versionNo, priceHash)
	quote, err := insertSharedPoolPriceVersion(ctx, tx, *model, versionNo, configVersion, operationID, service.SharedPoolPricingSourceOfficial, base, model.RateMultiplier, priceHash, 0, now)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE shared_pool_models SET pricing_source = 'official_catalog', pricing_status = 'ready', pricing_config_version = pricing_config_version + 1, pricing_updated_at = $2 WHERE id = $1`, model.PoolModelID, now); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE shared_pool_model_endpoints SET pricing_status = 'ready', config_version = $2, updated_at = $3 WHERE id = $1`, model.EndpointID, configVersion, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	service.FinalizeSharedPoolPriceQuote(quote)
	return quote, nil
}

func sharedPoolEndpointGateAllowsRouting(endpointType, gateStatus string) bool {
	switch endpointType {
	case service.SharedPoolEndpointImageGeneration, service.SharedPoolEndpointImageEdit, service.SharedPoolEndpointVideo:
		return strings.EqualFold(strings.TrimSpace(gateStatus), "passed")
	default:
		return true
	}
}

func lockSharedPoolPricingModel(ctx context.Context, tx *sql.Tx, poolID, ownerID int64, modelName, endpointType string) (*sharedPoolPricingModel, error) {
	return querySharedPoolPricingModel(ctx, tx, poolID, ownerID, modelName, endpointType, true)
}

type sharedPoolPricingQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func readSharedPoolPricingModel(ctx context.Context, q sharedPoolPricingQueryer, poolID, ownerID int64, modelName, endpointType string) (*sharedPoolPricingModel, error) {
	return querySharedPoolPricingModel(ctx, q, poolID, ownerID, modelName, endpointType, false)
}

func querySharedPoolPricingModel(ctx context.Context, q sharedPoolPricingQueryer, poolID, ownerID int64, modelName, endpointType string, forUpdate bool) (*sharedPoolPricingModel, error) {
	var model sharedPoolPricingModel
	var aliasesRaw []byte
	query := `
		SELECT spm.id, spm.pool_id, spm.provider, spm.model_name,
		       COALESCE(NULLIF(spm.display_name, ''), spm.model_name),
		       spm.upstream_model_name, spm.model_aliases, spm.pricing_source,
		       spm.pricing_status, spm.pricing_config_version, ` + sharedPoolEffectiveRateMultiplierSQL("spm.rate_multiplier", "sp.rate_multiplier") + `,
		       spe.id, spe.endpoint_type, spe.enabled, spe.gate_status,
		       spe.pricing_status, spe.config_version,
		       EXISTS (
		           SELECT 1 FROM model_catalog mc
		           WHERE mc.enabled = TRUE
		             AND LOWER(mc.provider) = ` + sharedPoolCanonicalProviderSQL("spm.provider") + `
		             AND (
		                 LOWER(mc.model_name) = LOWER(spm.model_name)
		                 OR EXISTS (
		                     SELECT 1
		                     FROM jsonb_array_elements_text(
		                         CASE WHEN jsonb_typeof(mc.aliases) = 'array' THEN mc.aliases ELSE '[]'::jsonb END
		                     ) AS official_alias(value)
		                     WHERE LOWER(official_alias.value) = LOWER(spm.model_name)
		                 )
		             )
		       ) AS official
		FROM shared_pool_models spm
		JOIN shared_pools sp ON sp.id = spm.pool_id
		JOIN shared_pool_model_endpoints spe ON spe.pool_model_id = spm.id
		WHERE sp.id = $1
		  AND ($2 = 0 OR sp.owner_id = $2)
		  AND sp.lifecycle_state <> 'archived'
		  AND spm.enabled = TRUE AND spm.model_open = TRUE
		  AND (LOWER(spm.model_name) = LOWER($3) OR spm.model_aliases ? $3)
		  AND spe.endpoint_type = $4
		  AND (
		      $4 NOT IN ('image_generation', 'image_edit', 'video')
		      OR (spe.gate_status = 'passed' AND spe.media_probe_expires_at > NOW())
		  )
		ORDER BY CASE WHEN LOWER(spm.model_name) = LOWER($3) THEN 0 ELSE 1 END, spm.sort_order
		LIMIT 1`
	if forUpdate {
		query += ` FOR UPDATE OF spm, spe`
	}
	err := q.QueryRowContext(ctx, query, poolID, ownerID, strings.TrimSpace(modelName), endpointType).Scan(
		&model.PoolModelID, &model.PoolID, &model.Provider, &model.ModelName, &model.DisplayName,
		&model.UpstreamModelName, &aliasesRaw, &model.PricingSource, &model.PricingStatus,
		&model.PricingConfigVersion, &model.RateMultiplier, &model.EndpointID, &model.EndpointType,
		&model.EndpointEnabled, &model.GateStatus, &model.EndpointPricingStatus,
		&model.EndpointConfigVersion, &model.Official,
	)
	if err != nil {
		return nil, err
	}
	model.Aliases = scanStringArray(aliasesRaw)
	if model.RateMultiplier <= 0 {
		model.RateMultiplier = 1
	}
	return &model, nil
}

func sharedPoolPriceHash(modelName, endpointType, source string, base service.SharedPoolPriceComponents, multiplier float64) (string, error) {
	return service.BuildSharedPoolPriceHash(modelName, endpointType, source, base, multiplier)
}

func insertSharedPoolPriceVersion(ctx context.Context, tx *sql.Tx, model sharedPoolPricingModel, versionNo, configVersion int64, operationID, source string, base service.SharedPoolPriceComponents, multiplier float64, priceHash string, createdBy int64, effectiveFrom time.Time) (*service.SharedPoolPriceQuote, error) {
	var quote service.SharedPoolPriceQuote
	var inputPrice, outputPrice, cacheReadPrice, cacheWritePrice, imageItemPrice, videoSecondPrice, perRequestPrice, minimumCharge, maximumCharge sql.NullFloat64
	err := tx.QueryRowContext(ctx, `INSERT INTO shared_pool_price_versions (
		pool_id, pool_model_id, endpoint_id, version_no, config_version, operation_id,
		source, billing_mode, currency, input_price, output_price, cache_read_price,
		cache_write_price, image_item_price, video_second_price, per_request_price,
		multiplier, minimum_charge, maximum_charge,
		effective_from, price_hash, created_by
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'USD',$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,NULLIF($21,0))
	RETURNING id, pool_id, pool_model_id, endpoint_id, source, billing_mode, currency,
	          input_price, output_price, cache_read_price, cache_write_price,
	          image_item_price, video_second_price, per_request_price,
	          multiplier, minimum_charge, maximum_charge,
	          effective_from, price_hash, config_version`,
		model.PoolID, model.PoolModelID, model.EndpointID, versionNo, configVersion, operationID,
		source, base.BillingMode, base.InputPrice, base.OutputPrice, base.CacheReadPrice,
		base.CacheWritePrice, base.ImageItemPrice, base.VideoSecondPrice, base.PerRequestPrice,
		multiplier, base.MinimumCharge, base.MaximumCharge, effectiveFrom, priceHash, createdBy,
	).Scan(&quote.PriceVersionID, &quote.PoolID, &quote.PoolModelID, &quote.EndpointID, &quote.PricingSource, &quote.BasePrice.BillingMode, &quote.BasePrice.Currency, &inputPrice, &outputPrice, &cacheReadPrice, &cacheWritePrice, &imageItemPrice, &videoSecondPrice, &perRequestPrice, &quote.Multiplier, &minimumCharge, &maximumCharge, &quote.EffectiveFrom, &quote.PriceHash, &quote.ConfigVersion)
	if err != nil {
		return nil, err
	}
	quote.ModelName = model.ModelName
	quote.EndpointType = model.EndpointType
	quote.PricingStatus = "ready"
	setSharedPoolPricePointers(&quote.BasePrice, inputPrice, outputPrice, cacheReadPrice, cacheWritePrice, imageItemPrice, videoSecondPrice, perRequestPrice, minimumCharge, maximumCharge)
	return &quote, nil
}

func selectSharedPoolPriceVersionByOperation(ctx context.Context, tx *sql.Tx, model *sharedPoolPricingModel, operationID string) (*service.SharedPoolPriceQuote, error) {
	return scanSharedPoolPriceVersion(tx.QueryRowContext(ctx, `SELECT `+sharedPoolPriceVersionColumns+` FROM shared_pool_price_versions pv WHERE pv.endpoint_id = $1 AND pv.operation_id = $2`, model.EndpointID, operationID), *model)
}

func selectLatestSharedPoolPriceVersion(ctx context.Context, q sharedPoolPricingQueryer, model *sharedPoolPricingModel, source string) (*service.SharedPoolPriceQuote, error) {
	return scanSharedPoolPriceVersion(q.QueryRowContext(ctx, `SELECT `+sharedPoolPriceVersionColumns+` FROM shared_pool_price_versions pv WHERE pv.endpoint_id = $1 AND pv.source = $2 AND pv.effective_from <= NOW() ORDER BY pv.effective_from DESC, pv.version_no DESC LIMIT 1`, model.EndpointID, source), *model)
}

func scanSharedPoolPriceVersion(row scanner, model sharedPoolPricingModel) (*service.SharedPoolPriceQuote, error) {
	var quote service.SharedPoolPriceQuote
	var inputPrice, outputPrice, cacheReadPrice, cacheWritePrice, imageItemPrice, videoSecondPrice, perRequestPrice, minimumCharge, maximumCharge sql.NullFloat64
	if err := row.Scan(&quote.PriceVersionID, &quote.PoolID, &quote.PoolModelID, &quote.EndpointID, &quote.PricingSource, &quote.BasePrice.BillingMode, &quote.BasePrice.Currency, &inputPrice, &outputPrice, &cacheReadPrice, &cacheWritePrice, &imageItemPrice, &videoSecondPrice, &perRequestPrice, &quote.Multiplier, &minimumCharge, &maximumCharge, &quote.EffectiveFrom, &quote.PriceHash, &quote.ConfigVersion); err != nil {
		return nil, err
	}
	quote.ModelName = model.ModelName
	quote.EndpointType = model.EndpointType
	quote.PricingStatus = "ready"
	setSharedPoolPricePointers(&quote.BasePrice, inputPrice, outputPrice, cacheReadPrice, cacheWritePrice, imageItemPrice, videoSecondPrice, perRequestPrice, minimumCharge, maximumCharge)
	return &quote, nil
}

func quoteFromNullableVersion(model sharedPoolPricingModel, versionID, poolID, modelID, endpointID sql.NullInt64, source, billingMode, currency sql.NullString, inputPrice, outputPrice, cacheReadPrice, cacheWritePrice, imageItemPrice, videoSecondPrice, perRequestPrice, multiplier, minimumCharge, maximumCharge sql.NullFloat64, effectiveFrom sql.NullTime, priceHash sql.NullString, configVersion sql.NullInt64) *service.SharedPoolPriceQuote {
	quote := &service.SharedPoolPriceQuote{PriceVersionID: versionID.Int64, PoolID: poolID.Int64, PoolModelID: modelID.Int64, EndpointID: endpointID.Int64, ModelName: model.ModelName, EndpointType: model.EndpointType, PricingSource: source.String, PricingStatus: "ready", ConfigVersion: configVersion.Int64, Multiplier: multiplier.Float64, EffectiveFrom: effectiveFrom.Time, PriceHash: priceHash.String}
	quote.BasePrice.BillingMode = billingMode.String
	quote.BasePrice.Currency = currency.String
	setSharedPoolPricePointers(&quote.BasePrice, inputPrice, outputPrice, cacheReadPrice, cacheWritePrice, imageItemPrice, videoSecondPrice, perRequestPrice, minimumCharge, maximumCharge)
	return quote
}

func setSharedPoolPricePointers(base *service.SharedPoolPriceComponents, inputPrice, outputPrice, cacheReadPrice, cacheWritePrice, imageItemPrice, videoSecondPrice, perRequestPrice, minimumCharge, maximumCharge sql.NullFloat64) {
	if inputPrice.Valid {
		value := inputPrice.Float64
		base.InputPrice = &value
	}
	if outputPrice.Valid {
		value := outputPrice.Float64
		base.OutputPrice = &value
	}
	if cacheReadPrice.Valid {
		value := cacheReadPrice.Float64
		base.CacheReadPrice = &value
	}
	if cacheWritePrice.Valid {
		value := cacheWritePrice.Float64
		base.CacheWritePrice = &value
	}
	if imageItemPrice.Valid {
		value := imageItemPrice.Float64
		base.ImageItemPrice = &value
	}
	if videoSecondPrice.Valid {
		value := videoSecondPrice.Float64
		base.VideoSecondPrice = &value
	}
	if perRequestPrice.Valid {
		value := perRequestPrice.Float64
		base.PerRequestPrice = &value
	}
	if minimumCharge.Valid {
		value := minimumCharge.Float64
		base.MinimumCharge = &value
	}
	if maximumCharge.Valid {
		value := maximumCharge.Float64
		base.MaximumCharge = &value
	}
}
