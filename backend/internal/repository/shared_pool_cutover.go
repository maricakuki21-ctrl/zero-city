package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/platform/cutover"
)

var ErrNilCutoverDatabase = errors.New("nil shared-pool cutover database")

type SharedPoolCutoverRepository struct {
	db *sql.DB
}

func NewSharedPoolCutoverRepository(db *sql.DB) (*SharedPoolCutoverRepository, error) {
	if db == nil {
		return nil, ErrNilCutoverDatabase
	}
	return &SharedPoolCutoverRepository{db: db}, nil
}

func (r *SharedPoolCutoverRepository) Load(ctx context.Context) (cutover.Authority, error) {
	return scanCutoverAuthority(r.db.QueryRowContext(ctx, `SELECT active_attempt_id::text, phase,
        authority_epoch, version, first_canonical_effect
        FROM shared_pool_cutover_control WHERE singleton = TRUE`))
}

func (r *SharedPoolCutoverRepository) Transition(
	ctx context.Context,
	expected cutover.Expectation,
	next cutover.Phase,
) (cutover.Authority, error) {
	row := r.db.QueryRowContext(ctx, `SELECT attempt_id::text, phase, authority_epoch, version,
        first_canonical_effect FROM shared_pool_cutover_transition($1::uuid, $2, $3, $4, $5)`,
		expected.AttemptID, expected.Phase, expected.Version, expected.Epoch, next)
	state, err := scanCutoverAuthority(row)
	if err != nil {
		return cutover.Authority{}, fmt.Errorf("transition shared-pool cutover: %w", err)
	}
	return state, nil
}

func (r *SharedPoolCutoverRepository) AbortAfterFence(
	ctx context.Context,
	expected cutover.Expectation,
	replacementAttemptID string,
) (cutover.AbortResult, error) {
	var failedAttemptID, activeAttemptID string
	var failedVersion, epoch, version int64
	err := r.db.QueryRowContext(ctx, `SELECT failed_attempt_id::text, failed_version,
        active_attempt_id::text, authority_epoch, version
        FROM shared_pool_cutover_abort_after_fence($1::uuid, $2, $3, $4, $5::uuid)`,
		expected.AttemptID, expected.Phase, expected.Version, expected.Epoch, replacementAttemptID).
		Scan(&failedAttemptID, &failedVersion, &activeAttemptID, &epoch, &version)
	if err != nil {
		return cutover.AbortResult{}, fmt.Errorf("abort shared-pool cutover: %w", err)
	}
	return cutover.AbortResult{
		Failed: cutover.Authority{
			AttemptID: failedAttemptID, Phase: cutover.PhaseAbortedBeforeActivation,
			Epoch: expected.Epoch, Version: failedVersion,
		},
		Replacement: cutover.Authority{
			AttemptID: activeAttemptID, Phase: cutover.PhaseLegacyOpen,
			Epoch: epoch, Version: version,
		},
	}, nil
}

func (r *SharedPoolCutoverRepository) AssertLegacyRuntimeWrite(ctx context.Context, epoch int64) error {
	if _, err := r.db.ExecContext(ctx, `SELECT assert_shared_pool_legacy_runtime_epoch($1)`, epoch); err != nil {
		return fmt.Errorf("authorize legacy runtime write: %w", err)
	}
	return nil
}

func (r *SharedPoolCutoverRepository) BeginCanonicalEffect(
	ctx context.Context,
	expected cutover.Expectation,
) (*sql.Tx, cutover.Authority, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, cutover.Authority{}, fmt.Errorf("begin canonical effect: %w", err)
	}
	state, err := scanCutoverAuthority(tx.QueryRowContext(ctx, `SELECT attempt_id::text, phase,
        authority_epoch, version, first_canonical_effect
        FROM shared_pool_cutover_mark_first_canonical_effect($1::uuid, $2, $3)`,
		expected.AttemptID, expected.Version, expected.Epoch))
	if err != nil {
		_ = tx.Rollback()
		return nil, cutover.Authority{}, fmt.Errorf("open canonical effect: %w", err)
	}
	return tx, state, nil
}

type cutoverRowScanner interface {
	Scan(dest ...any) error
}

func scanCutoverAuthority(row cutoverRowScanner) (cutover.Authority, error) {
	var state cutover.Authority
	if err := row.Scan(&state.AttemptID, &state.Phase, &state.Epoch, &state.Version, &state.FirstCanonicalEffect); err != nil {
		return cutover.Authority{}, err
	}
	return state, nil
}

type snapshotWire struct {
	ActiveNonMedia       []cutover.SnapshotItem `json:"active_non_media"`
	Holds                []cutover.SnapshotItem `json:"holds"`
	AcceptedMediaTaskIDs []cutover.SnapshotItem `json:"accepted_media_task_ids"`
	TerminalUnknown      []cutover.SnapshotItem `json:"terminal_unknown"`
	UnconsumedOutbox     []cutover.SnapshotItem `json:"unconsumed_outbox"`
}

func decodeSnapshot(raw []byte, authority cutover.Authority) (cutover.HandoffSnapshot, error) {
	var wire snapshotWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		return cutover.HandoffSnapshot{}, fmt.Errorf("decode cutover handoff snapshot: %w", err)
	}
	snapshot := cutover.HandoffSnapshot{
		Authority: authority, ActiveNonMedia: wire.ActiveNonMedia, Holds: wire.Holds,
		AcceptedMediaTaskIDs: wire.AcceptedMediaTaskIDs, TerminalUnknown: wire.TerminalUnknown,
		UnconsumedOutbox: wire.UnconsumedOutbox,
	}
	if err := snapshot.Validate(); err != nil {
		return cutover.HandoffSnapshot{}, err
	}
	return snapshot, nil
}
