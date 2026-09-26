package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/platform/cutover"
)

func (r *SharedPoolCutoverRepository) HandoffSnapshot(
	ctx context.Context,
	expected cutover.Expectation,
) (cutover.HandoffSnapshot, error) {
	if expected.Phase != cutover.PhaseFenced {
		return cutover.HandoffSnapshot{}, cutover.ErrSnapshotClosed
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return cutover.HandoffSnapshot{}, fmt.Errorf("begin cutover handoff snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	authority, err := scanCutoverAuthority(tx.QueryRowContext(ctx, `SELECT active_attempt_id::text,
        phase, authority_epoch, version, first_canonical_effect
        FROM shared_pool_cutover_control WHERE singleton = TRUE FOR UPDATE`))
	if err != nil {
		return cutover.HandoffSnapshot{}, fmt.Errorf("lock cutover authority: %w", err)
	}
	if authority.Expectation() != expected {
		return cutover.HandoffSnapshot{}, cutover.ErrStaleAuthority
	}

	var raw []byte
	if err := tx.QueryRowContext(ctx, handoffSnapshotSQL).Scan(&raw); err != nil {
		return cutover.HandoffSnapshot{}, fmt.Errorf("read cutover handoff snapshot: %w", err)
	}
	snapshot, err := decodeSnapshot(raw, authority)
	if err != nil {
		return cutover.HandoffSnapshot{}, err
	}
	if err := tx.Commit(); err != nil {
		return cutover.HandoffSnapshot{}, fmt.Errorf("commit cutover handoff snapshot: %w", err)
	}
	return snapshot, nil
}

const handoffSnapshotSQL = `SELECT jsonb_build_object(
    'active_non_media', COALESCE((SELECT jsonb_agg(jsonb_build_object(
        'identity', 'reservation:' || r.id::text, 'payload', to_jsonb(r)) ORDER BY r.id)
        FROM shared_pool_usage_reservations r
        WHERE r.endpoint_type IN ('chat', 'responses')
          AND r.status IN ('reserved', 'forwarding', 'settlement_pending')), '[]'::jsonb),
    'holds', COALESCE((SELECT jsonb_agg(jsonb_build_object(
        'identity', 'hold:' || r.id::text, 'payload', to_jsonb(r)) ORDER BY r.id)
        FROM shared_pool_usage_reservations r
        WHERE r.status IN ('reserved', 'forwarding', 'settlement_pending', 'review_required')), '[]'::jsonb),
    'accepted_media_task_ids', COALESCE((SELECT jsonb_agg(jsonb_build_object(
        'identity', 'media-task:' || m.id::text, 'payload', jsonb_build_object(
            'id', m.id, 'reservation_id', m.reservation_id,
            'upstream_request_id', m.upstream_request_id, 'status', m.status)) ORDER BY m.id)
        FROM shared_pool_media_tasks m
        WHERE m.status IN ('submitted', 'processing')), '[]'::jsonb),
    'terminal_unknown', COALESCE((SELECT jsonb_agg(item ORDER BY identity)
        FROM (SELECT 'trace:' || t.id::text AS identity, jsonb_build_object(
            'identity', 'trace:' || t.id::text, 'payload', to_jsonb(t)) AS item
            FROM shared_pool_usage_traces t WHERE t.settlement_outcome = 'unknown'
            UNION ALL
            SELECT 'media-task:' || m.id::text, jsonb_build_object(
            'identity', 'media-task:' || m.id::text, 'payload', to_jsonb(m))
            FROM shared_pool_media_tasks m WHERE m.status = 'review_required') unknowns), '[]'::jsonb),
    'unconsumed_outbox', COALESCE((SELECT jsonb_agg(jsonb_build_object(
        'identity', 'scheduler-outbox:' || o.id::text, 'payload', to_jsonb(o)) ORDER BY o.id)
        FROM scheduler_outbox o), '[]'::jsonb)
)`
