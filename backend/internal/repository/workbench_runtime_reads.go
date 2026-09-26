package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
)

const workbenchSelectSnapshotByIDSQL = `SELECT snapshot.snapshot_id, snapshot.run_id, snapshot.owner_user_id,
	snapshot.snapshot, snapshot.snapshot_sha256, snapshot.created_at FROM workbench_saved_snapshots snapshot
	JOIN workbench_runs run ON run.run_id = snapshot.run_id
	WHERE snapshot.snapshot_id = $1 AND run.owner_user_id = $2`

func (r *WorkbenchRuntimeRepository) LoadEventRead(
	ctx context.Context,
	identity workbench.Identity,
	runID workbench.RunID,
	cursor workbench.StreamCursor,
	limit uint32,
) (workbench.EventRead, error) {
	if cursor.RunID != "" && cursor.RunID != runID {
		return workbench.EventRead{}, workbench.ErrCursorRunMismatch
	}
	run, err := r.LoadRun(ctx, identity, runID)
	if err != nil {
		return workbench.EventRead{}, err
	}
	if limit == 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, workbenchSelectEventsSQL, string(runID), int64(identity.ActorID), cursor.Seq, int(limit))
	if err != nil {
		return workbench.EventRead{}, fmt.Errorf("read workbench events: %w", err)
	}
	defer func() { _ = rows.Close() }()
	events := make([]workbench.Event, 0)
	for rows.Next() {
		event, scanErr := scanWorkbenchEvent(rows)
		if scanErr != nil {
			return workbench.EventRead{}, scanErr
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return workbench.EventRead{}, fmt.Errorf("iterate workbench events: %w", err)
	}
	return workbench.EventRead{Run: run, Events: events, Cursor: cursor, OldestAvailableSeq: 1}, nil
}

func (r *WorkbenchRuntimeRepository) LoadSnapshotByID(
	ctx context.Context,
	identity workbench.Identity,
	snapshotID workbench.SnapshotID,
) (workbench.SavedSnapshot, error) {
	if err := identity.Validate(); err != nil {
		return workbench.SavedSnapshot{}, err
	}
	snapshot, err := scanWorkbenchSnapshot(r.db.QueryRowContext(ctx, workbenchSelectSnapshotByIDSQL, string(snapshotID), int64(identity.ActorID)))
	if errors.Is(err, sql.ErrNoRows) {
		return workbench.SavedSnapshot{}, workbench.ErrSnapshotNotFound
	}
	return snapshot, err
}
