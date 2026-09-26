package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/platform/corecontracts"
	platformledger "github.com/Wei-Shaw/sub2api/internal/platform/ledger"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const listPendingCanonicalSharedPoolUsageSQL = `
	SELECT event_id, request_id, api_key_id, payload, payload_sha256,
	       settlement_snapshot, settlement_snapshot_sha256
	FROM canonical_usage_outbox
	WHERE status = 'pending' AND settlement_retry_at <= NOW()
	ORDER BY settlement_retry_at, created_at, event_id
	LIMIT $1`

const markCanonicalSharedPoolUsageSettledSQL = `
	UPDATE canonical_usage_outbox
	SET status = 'published',
	    published_at = NOW()
	WHERE event_id = $1
	  AND status = 'pending'`

func (r *bizDecipherRepository) ListPendingCanonicalSharedPoolUsage(ctx context.Context, limit int) ([]service.CanonicalSharedPoolUsageClaim, error) {
	if r == nil || r.db == nil || limit <= 0 {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, listPendingCanonicalSharedPoolUsageSQL, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending canonical shared pool usage: %w", err)
	}
	defer func() { _ = rows.Close() }()

	claims := make([]service.CanonicalSharedPoolUsageClaim, 0, limit)
	for rows.Next() {
		var claim service.CanonicalSharedPoolUsageClaim
		var payload []byte
		var payloadSHA string
		var settlementPayload []byte
		var settlementSHA sql.NullString
		if err := rows.Scan(
			&claim.EventID, &claim.RequestID, &claim.APIKeyID, &payload, &payloadSHA,
			&settlementPayload, &settlementSHA,
		); err != nil {
			return nil, fmt.Errorf("scan canonical shared pool usage: %w", err)
		}
		var event corecontracts.CanonicalUsageFinalized
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, fmt.Errorf("decode canonical usage event %s: %w", claim.EventID, err)
		}
		actualSHA, err := platformledger.CanonicalUsagePayloadSHA256(event)
		if err != nil {
			return nil, fmt.Errorf("hash canonical usage event %s: %w", claim.EventID, err)
		}
		if !strings.EqualFold(strings.TrimSpace(actualSHA), strings.TrimSpace(payloadSHA)) {
			return nil, fmt.Errorf("canonical usage event %s payload hash mismatch", claim.EventID)
		}
		if claim.EventID != event.EventID() || claim.RequestID != event.RequestID() ||
			event.UsageReference() != fmt.Sprintf("sub2-usage:%d:%s", claim.APIKeyID, claim.RequestID) {
			return nil, fmt.Errorf("canonical usage event %s identity mismatch", claim.EventID)
		}
		if len(settlementPayload) > 0 {
			if !settlementSHA.Valid || strings.TrimSpace(settlementSHA.String) == "" {
				return nil, fmt.Errorf("canonical usage event %s settlement snapshot has no hash", claim.EventID)
			}
			var snapshot service.CanonicalUsageSettlementSnapshot
			if err := json.Unmarshal(settlementPayload, &snapshot); err != nil {
				return nil, fmt.Errorf("decode canonical usage settlement snapshot %s: %w", claim.EventID, err)
			}
			if err := snapshot.Validate(); err != nil {
				return nil, fmt.Errorf("validate canonical usage settlement snapshot %s: %w", claim.EventID, err)
			}
			actualSettlementSHA, err := service.CanonicalUsageSettlementSnapshotSHA256(&snapshot)
			if err != nil {
				return nil, fmt.Errorf("hash canonical usage settlement snapshot %s: %w", claim.EventID, err)
			}
			if !strings.EqualFold(strings.TrimSpace(actualSettlementSHA), strings.TrimSpace(settlementSHA.String)) {
				return nil, fmt.Errorf("canonical usage settlement snapshot %s hash mismatch", claim.EventID)
			}
			claim.Settlement = &snapshot
		} else if settlementSHA.Valid && strings.TrimSpace(settlementSHA.String) != "" {
			return nil, fmt.Errorf("canonical usage event %s settlement hash has no snapshot", claim.EventID)
		}
		claim.Event = event
		claims = append(claims, claim)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate canonical shared pool usage: %w", err)
	}
	return claims, nil
}

func (r *bizDecipherRepository) MarkCanonicalSharedPoolUsageSettled(ctx context.Context, eventID string) error {
	if r == nil || r.db == nil || eventID == "" {
		return nil
	}
	if _, err := r.db.ExecContext(ctx, markCanonicalSharedPoolUsageSettledSQL, eventID); err != nil {
		return fmt.Errorf("mark canonical shared pool usage settled: %w", err)
	}
	return nil
}

var _ service.CanonicalSharedPoolUsageRepository = (*bizDecipherRepository)(nil)

func (r *bizDecipherRepository) DeferCanonicalSharedPoolSettlement(ctx context.Context, eventID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE canonical_usage_outbox
		SET settlement_retry_at = NOW() + INTERVAL '15 minutes'
		WHERE event_id = $1 AND status = 'pending'`, eventID)
	return err
}

// Settlement resolves retained identities, never live routing eligibility.
// A disabled key or archived pool still owes for already finalized usage.
func (r *bizDecipherRepository) ResolveCanonicalSharedPoolSettlementIdentity(ctx context.Context, claim service.CanonicalSharedPoolUsageClaim) (*service.SharedPoolAccessKey, error) {
	var key service.SharedPoolAccessKey
	err := r.db.QueryRowContext(ctx, `
		SELECT k.id, k.pool_id, k.user_id,
		       CASE WHEN d.source_kind = 'pool_account' THEN d.source_id ELSE 0 END
		FROM shared_pool_sub2_bindings b
		JOIN shared_pool_supply_dispositions d ON d.pool_id = b.pool_id AND d.owner_id = b.owner_id
		JOIN shared_pool_access_keys k ON k.pool_id = b.pool_id
		JOIN shared_pools p ON p.id = b.pool_id AND p.owner_id = b.owner_id
		WHERE b.canonical_group_id = $1 AND d.canonical_account_id = $2
		  AND d.disposition = 'mapped' AND k.api_key_id = $3 AND k.user_id = $4
		ORDER BY k.id DESC LIMIT 1`,
		claim.Event.GroupID(), claim.Event.AccountID(), claim.APIKeyID, claim.Event.UserID(),
	).Scan(&key.ID, &key.PoolID, &key.UserID, &key.AccountID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &key, err
}

func (r *bizDecipherRepository) HasPostedCanonicalSharedPoolSettlement(ctx context.Context, input service.SharedPoolUsageInput) (bool, error) {
	var posted bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM shared_pool_balance_ledger l
			WHERE l.user_id = $1 AND l.pool_id = $2 AND COALESCE(l.account_id, 0) = $3
			  AND l.source_type = 'share_pool_usage' AND l.source_id = $4
			  AND l.status = 'posted' AND l.amount = -$5::numeric
			  AND l.price_version_id = $6 AND l.pricing_source_snapshot = $7
			  AND l.price_snapshot = $8::jsonb
			  AND (l.owner_payout_amount = 0 OR EXISTS (
				SELECT 1 FROM shared_pool_owner_earnings_ledger e
				WHERE e.pool_id = l.pool_id AND e.request_id = l.source_id
				  AND e.event_type = 'earning' AND e.status IN ('available', 'settled')
				  AND e.gross_amount = -l.amount AND e.net_amount = l.owner_payout_amount
				  AND e.platform_fee_amount = l.platform_fee_amount
				  AND e.price_version_id = l.price_version_id
			  ))
		)`, input.UserID, input.PoolID, input.AccountID, input.RequestID, input.Cost,
		input.PriceVersionID, input.PricingSource, string(input.PriceSnapshot)).Scan(&posted)
	return posted, err
}
