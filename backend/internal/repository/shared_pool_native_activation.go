package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func ensureNativeCanonicalBindingTx(ctx context.Context, tx *sql.Tx, poolID, ownerID int64) error {
	var rate float64
	if err := tx.QueryRowContext(ctx, `SELECT rate_multiplier FROM shared_pools WHERE id=$1 AND owner_id=$2 FOR UPDATE`, poolID, ownerID).Scan(&rate); err != nil {
		return err
	}
	operationID := fmt.Sprintf("bizdecipher-pool-binding:%d", poolID)
	var groupID int64
	err := tx.QueryRowContext(ctx, `INSERT INTO groups
		(name,description,rate_multiplier,is_exclusive,status,duplicate_operation_id,platform)
		VALUES ($1,$2,$3,TRUE,'disabled',$4,'openai')
		ON CONFLICT (duplicate_operation_id) WHERE duplicate_operation_id IS NOT NULL AND deleted_at IS NULL
		DO UPDATE SET updated_at=groups.updated_at RETURNING id`, fmt.Sprintf("biz-pool-%d", poolID),
		fmt.Sprintf("Canonical Sub2 group for BizDecipher pool %d", poolID), rate, operationID).Scan(&groupID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO shared_pool_sub2_bindings
		(pool_id,owner_id,canonical_group_id,lifecycle) VALUES ($1,$2,$3,'quarantined')
		ON CONFLICT (pool_id) DO NOTHING`, poolID, ownerID, groupID)
	return err
}

func attachNativeCanonicalMembershipTx(ctx context.Context, tx *sql.Tx, poolID, accountID int64, platform string, priority int) error {
	var groupID int64
	if err := tx.QueryRowContext(ctx, `SELECT canonical_group_id FROM shared_pool_sub2_bindings WHERE pool_id=$1 FOR UPDATE`, poolID).Scan(&groupID); err != nil {
		return service.ErrSharedPoolIdentityUnmapped
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO account_groups (account_id,group_id,priority) VALUES ($1,$2,$3)
		ON CONFLICT (account_id,group_id) DO UPDATE SET priority=EXCLUDED.priority`, accountID, groupID, priority); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE groups SET platform=$2,updated_at=NOW() WHERE id=$1`, groupID, platform); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE shared_pool_sub2_bindings SET lifecycle='active',updated_at=NOW() WHERE pool_id=$1`, poolID)
	return err
}

func (r *bizDecipherRepository) ActivateNativePoolBilling(ctx context.Context, input service.SharedPoolNativeActivationInput) (*service.SharedPoolNativeActivation, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var state, operationID, fingerprint string
	var configVersion int64
	var activatedVersion sql.NullInt64
	var activatedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT native_onboarding_state,config_version,
		COALESCE(native_activation_operation_id,''),COALESCE(native_activation_fingerprint,''),
		native_activated_config_version,native_activated_at
		FROM shared_pools WHERE id=$1 AND owner_id=$2 AND lifecycle_state<>'archived' AND owner_paused=FALSE FOR UPDATE`, input.PoolID, input.OwnerID).
		Scan(&state, &configVersion, &operationID, &fingerprint, &activatedVersion, &activatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrPoolForbidden
	}
	if err != nil {
		return nil, err
	}
	if operationID == input.OperationID {
		if fingerprint != input.RequestFingerprint {
			return nil, service.ErrNativeOperationConflict
		}
		if state == service.SharedPoolOnboardingBillingActive {
			return commitNativeActivationReplay(tx, input.PoolID, configVersion, activatedVersion, activatedAt)
		}
	}
	if configVersion != input.ExpectedConfigVersion {
		return nil, service.ErrNativeActivationStale
	}
	if state != service.SharedPoolOnboardingReadyBillingBlocked {
		return nil, service.ErrNativeActivationInvalid
	}
	if err := ensureNativeCanonicalBindingTx(ctx, tx, input.PoolID, input.OwnerID); err != nil {
		return nil, err
	}
	if err := repairNativeCanonicalMappingsTx(ctx, tx, input.PoolID); err != nil {
		return nil, err
	}

	var groupID, accountID, sharedAccountID int64
	err = tx.QueryRowContext(ctx, `SELECT b.canonical_group_id,d.canonical_account_id,p.source_id
		FROM shared_pool_native_onboarding_progress p
		JOIN shared_pool_supply_dispositions d ON d.source_kind='pool_account' AND d.source_id=p.source_id AND d.pool_id=p.pool_id AND d.owner_id=p.owner_id AND d.disposition='mapped'
		JOIN shared_pool_sub2_bindings b ON b.pool_id=p.pool_id AND b.owner_id=p.owner_id AND b.lifecycle='active'
		JOIN groups g ON g.id=b.canonical_group_id AND g.deleted_at IS NULL
		JOIN accounts a ON a.id=d.canonical_account_id AND a.deleted_at IS NULL AND a.status='active'
		JOIN account_groups ag ON ag.account_id=a.id AND ag.group_id=b.canonical_group_id
		JOIN shared_pool_accounts spa ON spa.id=p.source_id AND spa.deleted_at IS NULL AND spa.status='active'
		JOIN shared_pool_models m ON m.pool_id=p.pool_id AND m.enabled=TRUE AND m.model_open=TRUE AND m.pricing_status='ready'
		WHERE p.pool_id=$1 AND p.owner_id=$2 AND p.native_binding_state='ready'
		AND p.native_connection_status=$3 AND jsonb_array_length(p.native_models)>0
		AND p.native_account_observed_updated_at=a.updated_at
		AND p.native_models ? COALESCE(NULLIF(m.upstream_model_name,''),m.model_name)
		AND EXISTS (SELECT 1 FROM shared_pool_model_endpoints e WHERE e.pool_model_id=m.id AND e.enabled=TRUE AND e.pricing_status='ready'
			AND (e.endpoint_type IN ('chat','responses') OR e.gate_status='passed')
			AND (m.pricing_source='official_catalog' OR EXISTS (SELECT 1 FROM shared_pool_price_versions pv WHERE pv.endpoint_id=e.id AND pv.effective_from<=NOW())))
		AND EXISTS (SELECT 1 FROM shared_pool_settlement_rule_versions sr WHERE sr.pool_id=p.pool_id AND sr.effective_from<=NOW())
		ORDER BY p.source_id,m.sort_order,m.id LIMIT 1`, input.PoolID, input.OwnerID, service.NativeConnectionAuthenticatedMetadataReachable).
		Scan(&groupID, &accountID, &sharedAccountID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrNativeActivationInvalid
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE accounts SET schedulable=TRUE,updated_at=NOW() WHERE id=$1`, accountID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE shared_pool_native_onboarding_progress p SET native_account_observed_updated_at=a.updated_at,updated_at=NOW()
		FROM accounts a WHERE p.pool_id=$1 AND p.source_kind='pool_account' AND p.source_id=$2 AND a.id=$3`, input.PoolID, sharedAccountID, accountID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE shared_pool_accounts SET schedulable=TRUE,updated_at=NOW() WHERE id=$1`, sharedAccountID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE groups SET status='active',updated_at=NOW() WHERE id=$1`, groupID); err != nil {
		return nil, err
	}
	// Account/group side effects may advance the canonical account timestamp.
	// Align the committed readiness evidence after all activation writes so an
	// immediately following readiness read is not falsely marked stale.
	if _, err := tx.ExecContext(ctx, `UPDATE shared_pool_native_onboarding_progress p SET native_account_observed_updated_at=a.updated_at
		FROM accounts a
		WHERE p.pool_id=$1 AND p.source_kind='pool_account' AND p.source_id=$2
		  AND a.id=$3`, input.PoolID, sharedAccountID, accountID); err != nil {
		return nil, err
	}
	var result service.SharedPoolNativeActivation
	err = tx.QueryRowContext(ctx, `UPDATE shared_pools SET native_onboarding_state='billing_active',listed=TRUE,status='healthy',lifecycle_state='operating',
		config_version=config_version+1,native_activation_operation_id=$2,native_activation_fingerprint=$3,
		native_activated_config_version=config_version+1,native_activated_at=NOW(),updated_at=NOW()
		WHERE id=$1 RETURNING id,native_onboarding_state,config_version,native_activated_config_version,native_activated_at`, input.PoolID, input.OperationID, input.RequestFingerprint).
		Scan(&result.PoolID, &result.State, &result.ConfigVersion, &result.ActivatedConfigVersion, &result.ActivatedAt)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &result, nil
}

func commitNativeActivationReplay(tx *sql.Tx, poolID, configVersion int64, activatedVersion sql.NullInt64, activatedAt sql.NullTime) (*service.SharedPoolNativeActivation, error) {
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result := &service.SharedPoolNativeActivation{PoolID: poolID, State: service.SharedPoolOnboardingBillingActive, ConfigVersion: configVersion, AlreadyActive: true}
	if activatedVersion.Valid {
		result.ActivatedConfigVersion = activatedVersion.Int64
	}
	if activatedAt.Valid {
		result.ActivatedAt = &activatedAt.Time
	}
	return result, nil
}

var _ service.SharedPoolNativeActivationRepository = (*bizDecipherRepository)(nil)

func repairNativeCanonicalMappingsTx(ctx context.Context, tx *sql.Tx, poolID int64) error {
	var groupID int64
	if err := tx.QueryRowContext(ctx, `SELECT canonical_group_id FROM shared_pool_sub2_bindings WHERE pool_id=$1 FOR UPDATE`, poolID).Scan(&groupID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO account_groups (account_id,group_id,priority)
		SELECT d.canonical_account_id,$2,a.priority FROM shared_pool_supply_dispositions d JOIN accounts a ON a.id=d.canonical_account_id
		WHERE d.pool_id=$1 AND d.source_kind='pool_account' AND d.disposition='mapped' ON CONFLICT (account_id,group_id) DO NOTHING`, poolID, groupID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE shared_pool_sub2_bindings SET lifecycle='active',updated_at=NOW() WHERE pool_id=$1`, poolID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE groups g SET platform=a.platform,updated_at=NOW() FROM shared_pool_supply_dispositions d JOIN accounts a ON a.id=d.canonical_account_id WHERE d.pool_id=$1 AND d.disposition='mapped' AND g.id=$2`, poolID, groupID)
	return err
}

func revokeNativeRuntimeTx(ctx context.Context, tx *sql.Tx, poolID int64) error {
	if _, err := tx.ExecContext(ctx, `UPDATE shared_pool_accounts SET schedulable=FALSE,updated_at=NOW() WHERE pool_id=$1 AND deleted_at IS NULL AND schedulable=TRUE`, poolID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE accounts SET schedulable=FALSE,updated_at=NOW() WHERE schedulable=TRUE AND id IN (SELECT canonical_account_id FROM shared_pool_supply_dispositions WHERE pool_id=$1 AND source_kind='pool_account' AND disposition='mapped')`, poolID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE groups SET status='disabled',updated_at=NOW() WHERE id=(SELECT canonical_group_id FROM shared_pool_sub2_bindings WHERE pool_id=$1)`, poolID)
	return err
}

func reconcileNativeReadinessStateTx(ctx context.Context, tx *sql.Tx, poolID, ownerID int64) error {
	var state string
	var configVersion int64
	var activatedVersion sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT native_onboarding_state,config_version,native_activated_config_version FROM shared_pools WHERE id=$1 AND owner_id=$2 FOR UPDATE`, poolID, ownerID).Scan(&state, &configVersion, &activatedVersion); err != nil {
		return err
	}
	var ready bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM shared_pool_native_onboarding_progress WHERE pool_id=$1 AND native_binding_state='ready')`, poolID).Scan(&ready); err != nil {
		return err
	}
	if state == service.SharedPoolOnboardingBillingActive && ready && activatedVersion.Valid && activatedVersion.Int64 == configVersion {
		return nil
	}
	next := service.SharedPoolOnboardingSupplyConfiguring
	if ready {
		next = service.SharedPoolOnboardingReadyBillingBlocked
	} else {
		var attention bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM shared_pool_native_onboarding_progress WHERE pool_id=$1 AND native_binding_state='needs_attention')`, poolID).Scan(&attention); err != nil {
			return err
		}
		if attention {
			next = service.SharedPoolOnboardingSupplyNeedsAttention
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE shared_pools SET native_onboarding_state=$3,listed=FALSE,status='offline',lifecycle_state='draft',updated_at=NOW() WHERE id=$1 AND owner_id=$2`, poolID, ownerID, next); err != nil {
		return err
	}
	return revokeNativeRuntimeTx(ctx, tx, poolID)
}
