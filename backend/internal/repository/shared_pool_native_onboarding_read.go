package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *bizDecipherRepository) GetNativeBinding(ctx context.Context, poolID, ownerID, sourceID int64) (*service.SharedPoolAccount, error) {
	var ownedPool int
	if err := r.db.QueryRowContext(ctx, `SELECT 1 FROM shared_pools WHERE id=$1 AND owner_id=$2 AND lifecycle_state<>'archived'`, poolID, ownerID).Scan(&ownedPool); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrPoolForbidden
		}
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, nativeBindingSelect+` WHERE p.pool_id=$1 AND p.owner_id=$2 AND p.source_id=$3`, poolID, ownerID, sourceID)
	account, _, err := scanNativeBinding(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrPoolForbidden
	}
	return account, err
}

func (r *bizDecipherRepository) ClaimNativeRepair(ctx context.Context, input service.SharedPoolNativeRepairInput) (*service.SharedPoolNativeRepairClaim, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var poolState string
	var configVersion int64
	if err := tx.QueryRowContext(ctx, `SELECT native_onboarding_state,config_version FROM shared_pools WHERE id=$1 AND owner_id=$2 AND lifecycle_state<>'archived' FOR UPDATE`, input.PoolID, input.OwnerID).Scan(&poolState, &configVersion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrPoolForbidden
		}
		return nil, err
	}
	if poolState == service.SharedPoolOnboardingLegacyExisting {
		return nil, service.ErrLegacyMigrationRequired
	}
	row := tx.QueryRowContext(ctx, nativeBindingSelect+` WHERE p.pool_id=$1 AND p.owner_id=$2 AND p.source_id=$3 FOR UPDATE OF p`, input.PoolID, input.OwnerID, input.SharedAccountID)
	account, progress, err := scanNativeBinding(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrPoolForbidden
	}
	if err != nil {
		return nil, err
	}
	if progress.BindingState == "detached" {
		return nil, service.ErrNativePoolImmutable
	}
	if progress.RepairOperationID == input.RepairOperationID {
		if progress.RepairFingerprint != input.RequestFingerprint {
			return nil, service.ErrNativeOperationConflict
		}
	} else {
		if configVersion != input.ExpectedConfigVersion {
			return nil, service.ErrSharedPoolConcurrentUpdate
		}
		if _, err := tx.ExecContext(ctx, `UPDATE shared_pool_native_onboarding_progress SET
			native_repair_operation_id=$1,native_repair_request_fingerprint=$2,native_repair_expected_config_version=$3,
			native_repair_state='pending',native_repair_started_at=NOW(),native_repair_completed_at=NULL,
			native_binding_step='repair_native_account',updated_at=NOW()
			WHERE source_kind='pool_account' AND source_id=$4 AND pool_id=$5 AND owner_id=$6`,
			input.RepairOperationID, input.RequestFingerprint, input.ExpectedConfigVersion, input.SharedAccountID, input.PoolID, input.OwnerID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &service.SharedPoolNativeRepairClaim{
		Binding: service.SharedPoolNativeBinding{
			Account: *account, BindingRef: progress.BindingRef, OperationID: account.NativeOperationID,
			RequestFingerprint: progress.CreateFingerprint, State: progress.BindingState, Step: progress.BindingStep,
		},
		ExpectedNativeUpdatedAt: progress.NativeUpdatedAt,
	}, nil
}

func (r *bizDecipherRepository) CompleteNativeRepair(ctx context.Context, input service.SharedPoolNativeRepairInput, claim *service.SharedPoolNativeRepairClaim, account *service.Account) (*service.SharedPoolAccount, error) {
	if claim == nil || account == nil {
		return nil, service.ErrNativeOperationConflict
	}
	if err := validateNativeAccountRepairEvidence(account, input.RepairOperationID, input.RequestFingerprint); err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := lockNativePoolOwner(ctx, tx, input.PoolID, input.OwnerID); err != nil {
		return nil, err
	}
	var operationID, fingerprint, repairState string
	var mappedAccountID int64
	if err := tx.QueryRowContext(ctx, `SELECT p.native_repair_operation_id,p.native_repair_request_fingerprint,p.native_repair_state,d.canonical_account_id
		FROM shared_pool_native_onboarding_progress p
		JOIN shared_pool_supply_dispositions d ON d.source_kind=p.source_kind AND d.source_id=p.source_id
		WHERE p.source_kind='pool_account' AND p.source_id=$1 AND p.pool_id=$2 AND p.owner_id=$3 FOR UPDATE OF p`, input.SharedAccountID, input.PoolID, input.OwnerID).
		Scan(&operationID, &fingerprint, &repairState, &mappedAccountID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrPoolForbidden
		}
		return nil, err
	}
	if operationID != input.RepairOperationID || fingerprint != input.RequestFingerprint || mappedAccountID != account.ID {
		return nil, service.ErrNativeOperationConflict
	}
	if repairState != "applied" {
		if _, err := tx.ExecContext(ctx, `UPDATE shared_pool_native_onboarding_progress SET
			native_repair_state='applied',native_repair_completed_at=NOW(),native_account_observed_updated_at=$1,
			native_binding_state='attached',native_binding_step='model_discovery',native_binding_error_code='',native_binding_error_message='',
			native_models='[]'::jsonb,native_models_verified_at=NULL,native_connection_status='unverified',native_connection_verified_at=NULL,
			native_verification_attempted_at=NULL,updated_at=NOW()
			WHERE source_kind='pool_account' AND source_id=$2 AND pool_id=$3 AND owner_id=$4`, account.UpdatedAt, input.SharedAccountID, input.PoolID, input.OwnerID); err != nil {
			return nil, err
		}
		if err := updateNativePoolState(ctx, tx, input.PoolID, input.OwnerID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetNativeBinding(ctx, input.PoolID, input.OwnerID, input.SharedAccountID)
}

type nativeBindingProgress struct {
	BindingRef        string
	CreateFingerprint string
	BindingState      string
	BindingStep       string
	RepairOperationID string
	RepairFingerprint string
	NativeUpdatedAt   time.Time
}

const nativeBindingSelect = `SELECT
	spa.id,spa.pool_id,spa.owner_id,spa.name,spa.description,spa.provider,spa.auth_type,spa.upstream_base_url,spa.status,spa.created_at,spa.updated_at,
	p.native_operation_id,p.native_binding_ref::text,p.native_request_fingerprint,p.native_binding_state,p.native_binding_step,
	p.native_binding_error_code,p.native_binding_error_message,p.native_models,p.native_models_verified_at,
	p.native_connection_status,p.native_connection_verified_at,p.native_account_observed_updated_at,
	COALESCE(p.native_repair_operation_id,''),COALESCE(p.native_repair_request_fingerprint,''),a.updated_at,
	NOT COALESCE(pool.native_onboarding_state='billing_active' AND pool.native_activated_config_version=pool.config_version,FALSE),
	COALESCE(spa.schedulable AND a.schedulable AND a.status='active' AND a.deleted_at IS NULL,FALSE)
	FROM shared_pool_native_onboarding_progress p
	JOIN shared_pool_accounts spa ON spa.id=p.source_id AND spa.pool_id=p.pool_id AND spa.owner_id=p.owner_id
	JOIN shared_pools pool ON pool.id=p.pool_id AND pool.owner_id=p.owner_id
	LEFT JOIN shared_pool_supply_dispositions d ON d.source_kind=p.source_kind AND d.source_id=p.source_id AND d.disposition='mapped'
	LEFT JOIN accounts a ON a.id=d.canonical_account_id`

func scanNativeBinding(row scanner) (*service.SharedPoolAccount, nativeBindingProgress, error) {
	var account service.SharedPoolAccount
	var progress nativeBindingProgress
	var models []byte
	var modelsAt, connectionAt, observedAt, nativeUpdatedAt sql.NullTime
	err := row.Scan(
		&account.ID, &account.PoolID, &account.OwnerID, &account.Name, &account.Description, &account.Provider, &account.AuthType,
		&account.UpstreamBaseURL, &account.Status, &account.CreatedAt, &account.UpdatedAt,
		&account.NativeOperationID, &progress.BindingRef, &progress.CreateFingerprint, &progress.BindingState, &progress.BindingStep,
		&account.NativeErrorCode, &account.NativeErrorMessage, &models, &modelsAt,
		&account.NativeConnectionStatus, &connectionAt, &observedAt,
		&progress.RepairOperationID, &progress.RepairFingerprint, &nativeUpdatedAt,
		&account.BillingActivationRequired, &account.Schedulable,
	)
	if err != nil {
		return nil, progress, err
	}
	_ = json.Unmarshal(models, &account.NativeModels)
	if account.NativeModels == nil {
		account.NativeModels = []string{}
	}
	if modelsAt.Valid {
		account.NativeModelsVerifiedAt = &modelsAt.Time
	}
	if connectionAt.Valid {
		account.NativeConnectionVerifiedAt = &connectionAt.Time
	}
	if observedAt.Valid {
		account.NativeAccountObservedUpdatedAt = &observedAt.Time
	}
	if nativeUpdatedAt.Valid {
		progress.NativeUpdatedAt = nativeUpdatedAt.Time
	}
	account.NativeBindingState = progress.BindingState
	account.NativeBindingStep = progress.BindingStep
	account.NativeEvidenceStale = !observedAt.Valid || !nativeUpdatedAt.Valid || !nativeUpdatedAt.Time.Equal(observedAt.Time)
	account.Schedulable = account.Schedulable && !account.BillingActivationRequired && !account.NativeEvidenceStale && progress.BindingState == "ready"
	return &account, progress, nil
}

func (r *bizDecipherRepository) hydrateNativeSharedPoolAccounts(ctx context.Context, accounts []service.SharedPoolAccount, poolID, ownerID int64) error {
	if len(accounts) == 0 {
		return nil
	}
	rows, err := r.db.QueryContext(ctx, nativeBindingSelect+` WHERE p.pool_id=$1 AND p.owner_id=$2`, poolID, ownerID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	byID := make(map[int64]*service.SharedPoolAccount, len(accounts))
	for index := range accounts {
		byID[accounts[index].ID] = &accounts[index]
	}
	for rows.Next() {
		native, _, scanErr := scanNativeBinding(rows)
		if scanErr != nil {
			return scanErr
		}
		if target := byID[native.ID]; target != nil {
			*target = mergeNativeSharedPoolAccount(*target, *native)
		}
	}
	return rows.Err()
}

func mergeNativeSharedPoolAccount(base, native service.SharedPoolAccount) service.SharedPoolAccount {
	base.NativeOperationID = native.NativeOperationID
	base.NativeBindingState = native.NativeBindingState
	base.NativeBindingStep = native.NativeBindingStep
	base.NativeErrorCode = native.NativeErrorCode
	base.NativeErrorMessage = native.NativeErrorMessage
	base.NativeModels = native.NativeModels
	base.NativeModelsVerifiedAt = native.NativeModelsVerifiedAt
	base.NativeConnectionStatus = native.NativeConnectionStatus
	base.NativeConnectionVerifiedAt = native.NativeConnectionVerifiedAt
	base.NativeAccountObservedUpdatedAt = native.NativeAccountObservedUpdatedAt
	base.NativeEvidenceStale = native.NativeEvidenceStale
	base.BillingActivationRequired = native.BillingActivationRequired
	base.HasUpstreamKey = false
	base.HasOAuthCredentials = false
	base.KeyPreview = ""
	base.Schedulable = native.Schedulable
	return base
}

func validateNativeAccountRepairEvidence(account *service.Account, operationID, fingerprint string) error {
	if account == nil || fmt.Sprint(account.Extra["shared_pool_repair_operation_id"]) != operationID || fmt.Sprint(account.Extra["shared_pool_repair_request_fingerprint"]) != fingerprint {
		return service.ErrNativeOperationConflict
	}
	return nil
}
