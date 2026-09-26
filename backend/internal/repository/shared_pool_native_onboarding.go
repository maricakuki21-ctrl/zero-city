package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func NewSharedPoolNativeOnboardingRepository(db *sql.DB) service.SharedPoolNativeOnboardingRepository {
	return &bizDecipherRepository{db: db}
}

func (r *bizDecipherRepository) CreateNativeDraft(ctx context.Context, input service.SharedPoolNativeDraftInput) (*service.SharedPool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var ownerLabel string
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(NULLIF(TRIM(bp.display_name), ''), NULLIF(TRIM(u.username), ''), u.email, 'shared')
		FROM users u
		LEFT JOIN biz_profiles bp ON bp.user_id = u.id
		WHERE u.id = $1 AND u.deleted_at IS NULL
		FOR SHARE OF u`, input.OwnerID).Scan(&ownerLabel); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrPoolForbidden
		}
		return nil, err
	}
	var poolID int64
	var fingerprint string
	err = tx.QueryRowContext(ctx, `SELECT id,create_request_fingerprint FROM shared_pools WHERE owner_id=$1 AND create_operation_id=$2 FOR UPDATE`, input.OwnerID, input.OperationID).Scan(&poolID, &fingerprint)
	if err == nil {
		if fingerprint != input.RequestFingerprint {
			return nil, service.ErrNativeOperationConflict
		}
		if err := ensureNativeCanonicalBindingTx(ctx, tx, poolID, input.OwnerID); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return r.GetOwnedSharedPool(ctx, poolID, input.OwnerID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var platformFee float64
	err = tx.QueryRowContext(ctx, `INSERT INTO shared_pools (
		owner_id,owner_label,name,description,status,listed,account_mode_enabled,
		upstream_base_url,upstream_api_key,create_operation_id,create_request_fingerprint,
		native_onboarding_state,lifecycle_state,source_kind
	) VALUES ($1,$2,$3,$4,'offline',FALSE,TRUE,'','',$5,$6,'draft','draft','owner_native_r1')
	ON CONFLICT (owner_id,create_operation_id) WHERE owner_id IS NOT NULL AND create_operation_id IS NOT NULL
	DO NOTHING RETURNING id,platform_fee_percent`, input.OwnerID, ownerLabel, input.Name, input.Description, input.OperationID, input.RequestFingerprint).Scan(&poolID, &platformFee)
	if errors.Is(err, sql.ErrNoRows) {
		if err := tx.QueryRowContext(ctx, `SELECT id,create_request_fingerprint FROM shared_pools WHERE owner_id=$1 AND create_operation_id=$2`, input.OwnerID, input.OperationID).Scan(&poolID, &fingerprint); err != nil {
			return nil, err
		}
		if fingerprint != input.RequestFingerprint {
			return nil, service.ErrNativeOperationConflict
		}
	} else if err != nil {
		return nil, err
	} else if err := insertSharedPoolSettlementRuleVersionTx(ctx, tx, poolID, 0, 0, platformFee, "bootstrap", input.OwnerID, time.Now().UTC().Truncate(time.Hour)); err != nil {
		return nil, err
	}
	if err := ensureNativeCanonicalBindingTx(ctx, tx, poolID, input.OwnerID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetOwnedSharedPool(ctx, poolID, input.OwnerID)
}

func (r *bizDecipherRepository) ClaimNativeBinding(ctx context.Context, input service.SharedPoolNativeAccountInput) (*service.SharedPoolNativeBinding, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	poolState, err := lockNativePoolOwner(ctx, tx, input.PoolID, input.OwnerID)
	if err != nil {
		return nil, err
	}
	if poolState == service.SharedPoolOnboardingLegacyExisting {
		return nil, service.ErrLegacyMigrationRequired
	}

	binding, err := queryNativeBindingByOperation(ctx, tx, input.PoolID, input.OwnerID, input.OperationID)
	if err == nil {
		if binding.RequestFingerprint != input.RequestFingerprint {
			return nil, service.ErrNativeOperationConflict
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return binding, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var sourceID int64
	err = tx.QueryRowContext(ctx, `INSERT INTO shared_pool_accounts (
		pool_id,owner_id,name,description,provider,auth_type,upstream_base_url,
		upstream_api_key,credentials_encrypted,schedulable,status,model_configs,cache_policy,routing_policy
	) VALUES ($1,$2,$3,$4,$5,$6,$7,'','',FALSE,'active','[]'::jsonb,'{}'::jsonb,'{}'::jsonb)
	RETURNING id`, input.PoolID, input.OwnerID, input.Name, input.Description, input.Provider, input.AuthType, input.UpstreamBaseURL).Scan(&sourceID)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO shared_pool_native_onboarding_progress (
		source_kind,source_id,pool_id,owner_id,native_binding_ref,native_operation_id,
		native_request_fingerprint,native_binding_state,native_binding_step
	) VALUES ('pool_account',$1,$2,$3,$4::uuid,$5,$6,'creating','native_account_create')`,
		sourceID, input.PoolID, input.OwnerID, input.BindingRef, input.OperationID, input.RequestFingerprint)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE shared_pools SET native_onboarding_state='supply_configuring',listed=FALSE,updated_at=NOW() WHERE id=$1 AND owner_id=$2`, input.PoolID, input.OwnerID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &service.SharedPoolNativeBinding{
		Account: service.SharedPoolAccount{
			ID: sourceID, PoolID: input.PoolID, OwnerID: input.OwnerID, Name: input.Name,
			Description: input.Description, Provider: input.Provider, AuthType: input.AuthType,
			UpstreamBaseURL: input.UpstreamBaseURL, Status: "active", Schedulable: false,
			NativeOperationID: input.OperationID, NativeBindingState: "creating", NativeBindingStep: "native_account_create",
			BillingActivationRequired: true,
		},
		BindingRef: input.BindingRef, OperationID: input.OperationID,
		RequestFingerprint: input.RequestFingerprint, State: "creating", Step: "native_account_create",
	}, nil
}

func (r *bizDecipherRepository) AttachNativeAccount(ctx context.Context, input service.SharedPoolNativeAccountInput, binding *service.SharedPoolNativeBinding, account *service.Account) (*service.SharedPoolAccount, error) {
	if binding == nil || account == nil {
		return nil, service.ErrNativeOperationConflict
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := lockNativePoolOwner(ctx, tx, input.PoolID, input.OwnerID); err != nil {
		return nil, err
	}
	var bindingRef, operationID, fingerprint string
	if err := tx.QueryRowContext(ctx, `SELECT native_binding_ref::text,native_operation_id,native_request_fingerprint FROM shared_pool_native_onboarding_progress WHERE source_kind='pool_account' AND source_id=$1 AND pool_id=$2 AND owner_id=$3 FOR UPDATE`, binding.Account.ID, input.PoolID, input.OwnerID).Scan(&bindingRef, &operationID, &fingerprint); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrPoolForbidden
		}
		return nil, err
	}
	if bindingRef != binding.BindingRef || operationID != input.OperationID || fingerprint != input.RequestFingerprint {
		return nil, service.ErrNativeOperationConflict
	}
	if fmt.Sprint(account.Extra["bizdecipher_supply_source"]) != "pool_account" || fmt.Sprint(account.Extra["bizdecipher_supply_id"]) != fmt.Sprint(binding.Account.ID) {
		return nil, service.ErrNativeOperationConflict
	}
	if err := insertOrVerifyNativeDisposition(ctx, tx, input, binding.Account.ID, account.ID); err != nil {
		return nil, err
	}
	if err := ensureNativeCanonicalBindingTx(ctx, tx, input.PoolID, input.OwnerID); err != nil {
		return nil, err
	}
	if err := attachNativeCanonicalMembershipTx(ctx, tx, input.PoolID, account.ID, account.Platform, account.Priority); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE shared_pool_native_onboarding_progress SET
		native_binding_state='attached',native_binding_step='model_discovery',native_binding_error_code='',native_binding_error_message='',
		native_account_observed_updated_at=$1,native_models='[]'::jsonb,native_models_verified_at=NULL,
		native_connection_status='unverified',native_connection_verified_at=NULL,updated_at=NOW()
		WHERE source_kind='pool_account' AND source_id=$2 AND pool_id=$3 AND owner_id=$4`, account.UpdatedAt, binding.Account.ID, input.PoolID, input.OwnerID); err != nil {
		return nil, err
	}
	if err := updateNativePoolState(ctx, tx, input.PoolID, input.OwnerID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetNativeBinding(ctx, input.PoolID, input.OwnerID, binding.Account.ID)
}

func (r *bizDecipherRepository) MarkNativeBindingAttention(ctx context.Context, poolID, ownerID, sourceID int64, code, message string) error {
	if len(message) > 500 {
		message = message[:500]
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := lockNativePoolOwner(ctx, tx, poolID, ownerID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE shared_pool_native_onboarding_progress SET native_binding_state='needs_attention',native_binding_step='retry_native_account',native_binding_error_code=$1,native_binding_error_message=$2,updated_at=NOW() WHERE source_kind='pool_account' AND source_id=$3 AND pool_id=$4 AND owner_id=$5`, strings.TrimSpace(code), strings.TrimSpace(message), sourceID, poolID, ownerID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return service.ErrPoolForbidden
	}
	if err := updateNativePoolState(ctx, tx, poolID, ownerID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *bizDecipherRepository) DetachNativeBinding(ctx context.Context, poolID, ownerID, sourceID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := lockNativePoolOwner(ctx, tx, poolID, ownerID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE shared_pool_native_onboarding_progress SET native_binding_state='detached',native_binding_step='detached',native_connection_status='unverified',native_connection_verified_at=NULL,updated_at=NOW() WHERE source_kind='pool_account' AND source_id=$1 AND pool_id=$2 AND owner_id=$3`, sourceID, poolID, ownerID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return service.ErrPoolForbidden
	}
	if err := retireSharedPoolSupplyTx(ctx, tx, poolID, sourceID); err != nil {
		return err
	}
	if err := updateNativePoolState(ctx, tx, poolID, ownerID); err != nil {
		return err
	}
	return tx.Commit()
}

func lockNativePoolOwner(ctx context.Context, tx *sql.Tx, poolID, ownerID int64) (string, error) {
	var state string
	err := tx.QueryRowContext(ctx, `SELECT native_onboarding_state FROM shared_pools WHERE id=$1 AND owner_id=$2 AND lifecycle_state<>'archived' FOR UPDATE`, poolID, ownerID).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return "", service.ErrPoolForbidden
	}
	return state, err
}

func queryNativeBindingByOperation(ctx context.Context, tx *sql.Tx, poolID, ownerID int64, operationID string) (*service.SharedPoolNativeBinding, error) {
	var binding service.SharedPoolNativeBinding
	var account service.SharedPoolAccount
	err := tx.QueryRowContext(ctx, `SELECT spa.id,spa.name,spa.description,spa.provider,spa.auth_type,spa.upstream_base_url,spa.status,
		p.native_binding_ref::text,p.native_operation_id,p.native_request_fingerprint,p.native_binding_state,p.native_binding_step
		FROM shared_pool_native_onboarding_progress p JOIN shared_pool_accounts spa ON spa.id=p.source_id
		WHERE p.pool_id=$1 AND p.owner_id=$2 AND p.native_operation_id=$3 FOR UPDATE OF p`, poolID, ownerID, operationID).
		Scan(&account.ID, &account.Name, &account.Description, &account.Provider, &account.AuthType, &account.UpstreamBaseURL, &account.Status,
			&binding.BindingRef, &binding.OperationID, &binding.RequestFingerprint, &binding.State, &binding.Step)
	if err != nil {
		return nil, err
	}
	account.PoolID = poolID
	account.OwnerID = ownerID
	account.Schedulable = false
	account.NativeOperationID = binding.OperationID
	account.NativeBindingState = binding.State
	account.NativeBindingStep = binding.Step
	account.BillingActivationRequired = true
	binding.Account = account
	return &binding, nil
}

func insertOrVerifyNativeDisposition(ctx context.Context, tx *sql.Tx, input service.SharedPoolNativeAccountInput, sourceID, accountID int64) error {
	var mappedAccountID int64
	err := tx.QueryRowContext(ctx, `SELECT canonical_account_id FROM shared_pool_supply_dispositions WHERE source_kind='pool_account' AND source_id=$1`, sourceID).Scan(&mappedAccountID)
	if err == nil {
		if mappedAccountID != accountID {
			return service.ErrNativeOperationConflict
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO shared_pool_supply_dispositions (
		source_kind,source_id,pool_id,owner_id,canonical_account_id,disposition,reason_code,credential_hash,source_checksum
	) VALUES ('pool_account',$1,$2,$3,$4,'mapped','owner_native_r1',$5,$6)`, sourceID, input.PoolID, input.OwnerID, accountID, input.CredentialDigest, input.RequestFingerprint)
	return err
}

func updateNativePoolState(ctx context.Context, tx *sql.Tx, poolID, ownerID int64) error {
	_, err := tx.ExecContext(ctx, `UPDATE shared_pools SET listed=FALSE,status='offline',lifecycle_state='draft',native_onboarding_state=CASE
		WHEN EXISTS (SELECT 1 FROM shared_pool_native_onboarding_progress p WHERE p.pool_id=$1 AND p.native_binding_state='ready') THEN 'supply_ready_billing_blocked'
		WHEN EXISTS (SELECT 1 FROM shared_pool_native_onboarding_progress p WHERE p.pool_id=$1 AND p.native_binding_state='needs_attention') THEN 'supply_needs_attention'
		WHEN EXISTS (SELECT 1 FROM shared_pool_native_onboarding_progress p WHERE p.pool_id=$1 AND p.native_binding_state<>'detached') THEN 'supply_configuring'
		ELSE 'draft' END,updated_at=NOW() WHERE id=$1 AND owner_id=$2`, poolID, ownerID)
	if err != nil {
		return err
	}
	return revokeNativeRuntimeTx(ctx, tx, poolID)
}
