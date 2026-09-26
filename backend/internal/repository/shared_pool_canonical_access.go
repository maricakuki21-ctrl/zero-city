package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Resolve product membership and published models without reading legacy
// credentials or selecting an account. Sub2 owns credentials and scheduling.
func (r *bizDecipherRepository) GetCanonicalSharedPoolAccessKeyByAPIKeyID(ctx context.Context, apiKeyID int64, model string) (*service.SharedPoolAccessKey, error) {
	var key service.SharedPoolAccessKey
	err := r.db.QueryRowContext(ctx, `
		SELECT k.id,k.pool_id,p.name,k.user_id,k.api_key_id,p.owner_id,
		       COALESCE(m.model_name,''),COALESCE(m.provider,''),
		       COALESCE(NULLIF(TRIM(m.upstream_model_name),''),m.model_name,''),
		       `+sharedPoolEffectiveRateMultiplierSQL("m.rate_multiplier", "p.rate_multiplier")+`,
		       COALESCE((
		         SELECT mc.model_name FROM model_catalog mc
		         WHERE mc.enabled=TRUE AND LOWER(mc.provider)=`+sharedPoolCanonicalProviderSQL("m.provider")+`
		           AND (LOWER(mc.model_name)=LOWER(m.model_name) OR EXISTS (
		             SELECT 1 FROM jsonb_array_elements_text(
		               CASE WHEN jsonb_typeof(mc.aliases)='array' THEN mc.aliases ELSE '[]'::jsonb END
		             ) alias(value) WHERE LOWER(alias.value)=LOWER(m.model_name)))
		         ORDER BY CASE WHEN LOWER(mc.model_name)=LOWER(m.model_name) THEN 0 ELSE 1 END,mc.sort_order,mc.id
		         LIMIT 1
		       ),'')
		FROM shared_pool_access_keys k
		JOIN api_keys ak ON ak.id=k.api_key_id AND ak.user_id=k.user_id
		JOIN users u ON u.id=k.user_id AND u.status='active' AND u.deleted_at IS NULL
		JOIN pool_seat_bindings seat ON seat.pool_id=k.pool_id AND seat.user_id=k.user_id AND seat.status='active'
		JOIN shared_pools p ON p.id=k.pool_id
		JOIN shared_pool_sub2_bindings b ON b.pool_id=p.id AND b.owner_id=p.owner_id AND b.lifecycle='active'
		JOIN groups g ON g.id=b.canonical_group_id AND g.status='active' AND g.deleted_at IS NULL
		LEFT JOIN LATERAL (
		  SELECT pm.* FROM shared_pool_models pm
		  WHERE pm.pool_id=p.id AND pm.enabled=TRUE AND pm.model_open=TRUE
		    AND (pm.model_name=$2 OR pm.model_aliases ? $2)
		  ORDER BY CASE WHEN pm.model_name=$2 THEN 0 ELSE 1 END,pm.sort_order,pm.id LIMIT 1
		) m ON TRUE
		WHERE k.api_key_id=$1 AND k.status='active' AND k.account_mode=TRUE
		  AND ak.status='active' AND ak.deleted_at IS NULL AND (ak.expires_at IS NULL OR ak.expires_at>NOW())
		  AND p.listed=TRUE AND p.lifecycle_state='operating' AND p.status IN ('healthy','limited')
		  AND p.governance_status NOT IN ('banned','suppressed')
		  AND ($2='' OR m.id IS NOT NULL)
		  AND ($2='' OR jsonb_array_length(k.allowed_models)=0 OR k.allowed_models ? $2 OR k.allowed_models ? m.model_name)
		  AND EXISTS (
		    SELECT 1 FROM shared_pool_supply_dispositions d
		    JOIN accounts a ON a.id=d.canonical_account_id AND a.deleted_at IS NULL AND a.status='active'
		    JOIN account_groups ag ON ag.account_id=a.id AND ag.group_id=b.canonical_group_id
		    WHERE d.pool_id=p.id AND d.owner_id=p.owner_id AND d.disposition='mapped'
		  )
		ORDER BY p.rank_weight DESC,p.id LIMIT 1`, apiKeyID, strings.TrimSpace(model)).Scan(
		&key.ID, &key.PoolID, &key.PoolName, &key.UserID, &key.APIKeyID, &key.OwnerID,
		&key.PublishedModelName, &key.Provider, &key.UpstreamModelName, &key.RateMultiplier, &key.CanonicalModelName,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrSharedPoolIdentityUnmapped
	}
	if err != nil {
		return nil, err
	}
	key.AccountMode = true
	_, err = r.db.ExecContext(ctx, `UPDATE pool_seat_bindings SET last_activity_at=NOW(),updated_at=NOW()
		WHERE pool_id=$1 AND user_id=$2 AND status='active'
		  AND (last_activity_at IS NULL OR last_activity_at<NOW()-INTERVAL '30 seconds')`, key.PoolID, key.UserID)
	return &key, err
}
