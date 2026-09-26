package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *bizDecipherRepository) PersistNativeReadiness(ctx context.Context, input service.SharedPoolNativeReadinessInput, update service.NativeReadinessUpdate) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var configVersion int64
	var nativeUpdatedAt sql.NullTime
	if err := tx.QueryRowContext(ctx, `SELECT config_version FROM shared_pools WHERE id=$1 AND owner_id=$2 AND lifecycle_state<>'archived' FOR UPDATE`, input.PoolID, input.OwnerID).Scan(&configVersion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrPoolForbidden
		}
		return err
	}
	if configVersion != input.ExpectedConfigVersion {
		return service.ErrSharedPoolConcurrentUpdate
	}
	if err := tx.QueryRowContext(ctx, `SELECT a.updated_at FROM shared_pool_supply_dispositions d JOIN accounts a ON a.id=d.canonical_account_id WHERE d.source_kind='pool_account' AND d.source_id=$1 AND d.pool_id=$2 AND d.owner_id=$3 AND d.disposition='mapped'`, input.SharedAccountID, input.PoolID, input.OwnerID).Scan(&nativeUpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrNativeIdentityDeleted
		}
		return err
	}
	if !nativeUpdatedAt.Valid || !nativeUpdatedAt.Time.Equal(update.ObservedNativeUpdatedAt) {
		return service.ErrSharedPoolConcurrentUpdate
	}
	modelsToPersist := update.Models
	if modelsToPersist == nil {
		modelsToPersist = []string{}
	}
	models, err := json.Marshal(modelsToPersist)
	if err != nil {
		return err
	}
	connectionStatus := update.ConnectionStatus
	if connectionStatus == "" {
		connectionStatus = "unverified"
	}
	result, err := tx.ExecContext(ctx, `UPDATE shared_pool_native_onboarding_progress SET
		native_models=$1::jsonb,
		native_models_verified_at=$2,
		native_connection_status=$3::text,
		native_connection_verified_at=$4,
		native_binding_error_code=$5,
		native_binding_error_message=$6,
		native_account_observed_updated_at=$7,
		native_verification_attempted_at=NOW(),
		native_binding_state=CASE WHEN $3::text=$8::text AND jsonb_array_length($1::jsonb)>0 THEN 'ready' ELSE 'needs_attention' END,
		native_binding_step=CASE WHEN $3::text=$8::text AND jsonb_array_length($1::jsonb)>0 THEN 'ready' ELSE 'connection_diagnostic' END,
		updated_at=NOW()
		WHERE source_kind='pool_account' AND source_id=$9 AND pool_id=$10 AND owner_id=$11 AND native_binding_state<>'detached'`, models, update.ModelsVerifiedAt, connectionStatus, update.ConnectionVerifiedAt, update.ErrorCode, update.ErrorMessage, update.ObservedNativeUpdatedAt, service.NativeConnectionAuthenticatedMetadataReachable, input.SharedAccountID, input.PoolID, input.OwnerID)
	if err != nil {
		return err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if updated == 0 {
		return service.ErrNativeIdentityDeleted
	}
	if err := materializeNativeModelsTx(ctx, tx, input.PoolID, input.OwnerID, input.SharedAccountID, models); err != nil {
		return err
	}
	if err := reconcileNativeReadinessStateTx(ctx, tx, input.PoolID, input.OwnerID); err != nil {
		return err
	}
	return tx.Commit()
}

var _ service.SharedPoolNativeReadinessRepository = (*bizDecipherRepository)(nil)

func materializeNativeModelsTx(ctx context.Context, tx *sql.Tx, poolID, ownerID, sourceID int64, models []byte) error {
	const query = `
	WITH discovered AS (
		SELECT DISTINCT BTRIM(value) AS model_name
		FROM jsonb_array_elements_text($4::jsonb)
		WHERE BTRIM(value) <> ''
	),
	enriched AS (
		SELECT d.model_name,
			COALESCE(mc.provider, spa.provider, 'openai') AS provider,
			(mc.model_name IS NOT NULL) AS official
		FROM discovered d
		JOIN shared_pool_accounts spa
			ON spa.id=$3 AND spa.pool_id=$1 AND spa.owner_id=$2 AND spa.deleted_at IS NULL
		LEFT JOIN LATERAL (
			SELECT model_name, provider
			FROM model_catalog
			WHERE enabled=TRUE
			  AND (
				LOWER(model_name)=LOWER(d.model_name)
				OR EXISTS (
					SELECT 1
					FROM jsonb_array_elements_text(
						CASE WHEN jsonb_typeof(aliases)='array' THEN aliases ELSE '[]'::jsonb END
					) alias_value
					WHERE LOWER(alias_value)=LOWER(d.model_name)
				)
			  )
			ORDER BY mainstream DESC, sort_order, id
			LIMIT 1
		) mc ON TRUE
	),
	upserted AS (
		INSERT INTO shared_pool_models (
			pool_id, provider, model_name, upstream_model_name, sort_order,
			enabled, rate_multiplier, rank_weight, model_open,
			pricing_source, pricing_status
		)
		SELECT $1, provider, model_name, model_name, ROW_NUMBER() OVER (ORDER BY model_name)-1,
			TRUE, 1, 100, official,
			CASE WHEN official THEN 'official_catalog' ELSE 'owner_custom' END,
			CASE WHEN official THEN 'ready' ELSE 'pending' END
		FROM enriched
		ON CONFLICT (pool_id, model_name) DO UPDATE SET
			provider = COALESCE(NULLIF(shared_pool_models.provider, ''), EXCLUDED.provider),
			upstream_model_name = COALESCE(NULLIF(shared_pool_models.upstream_model_name, ''), EXCLUDED.upstream_model_name)
		WHERE NULLIF(shared_pool_models.provider, '') IS NULL
		   OR NULLIF(shared_pool_models.upstream_model_name, '') IS NULL
		RETURNING id
	)
	SELECT COUNT(*) FROM upserted`
	var upsertedCount int
	if err := tx.QueryRowContext(ctx, query, poolID, ownerID, sourceID, models).Scan(&upsertedCount); err != nil {
		return err
	}
	// Endpoint rows are created by an AFTER INSERT trigger on shared_pool_models.
	// Data-modifying CTEs run against the same snapshot, so those rows are not
	// visible to a statement inside the CTE. Mark text-endpoint pricing ready in a
	// separate statement, and only when it actually changes.
	_, err := tx.ExecContext(ctx, `UPDATE shared_pool_model_endpoints endpoint
		SET pricing_status='ready', updated_at=NOW()
		FROM shared_pool_models model
		WHERE endpoint.pool_model_id=model.id
		  AND model.pool_id=$1
		  AND model.pricing_source='official_catalog'
		  AND model.pricing_status='ready'
		  AND endpoint.pricing_status IS DISTINCT FROM 'ready'
		  AND endpoint.endpoint_type IN ('chat','responses')`, poolID)
	return err
}
