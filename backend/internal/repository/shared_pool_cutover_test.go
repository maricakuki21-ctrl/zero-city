package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/platform/cutover"
	"github.com/stretchr/testify/require"
)

func TestSharedPoolCutoverRepository_Transition_passes_complete_CAS(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo, err := NewSharedPoolCutoverRepository(db)
	require.NoError(t, err)

	expected := cutover.Expectation{AttemptID: "1b3d22fd-d4a4-46e4-b88c-25c7aa668f95", Phase: cutover.PhaseDraining, Epoch: 9, Version: 4}
	mock.ExpectQuery(regexp.QuoteMeta("FROM shared_pool_cutover_transition($1::uuid, $2, $3, $4, $5)")).
		WithArgs(expected.AttemptID, expected.Phase, expected.Version, expected.Epoch, cutover.PhaseFenced).
		WillReturnRows(sqlmock.NewRows([]string{
			"attempt_id", "phase", "authority_epoch", "version", "first_canonical_effect",
		}).AddRow(expected.AttemptID, cutover.PhaseFenced, int64(10), int64(5), false))

	state, err := repo.Transition(context.Background(), expected, cutover.PhaseFenced)
	require.NoError(t, err)
	require.Equal(t, cutover.PhaseFenced, state.Phase)
	require.Equal(t, int64(10), state.Epoch)
	require.Equal(t, int64(5), state.Version)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSharedPoolCutoverRepository_HandoffSnapshot_locks_authority_before_rows(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo, err := NewSharedPoolCutoverRepository(db)
	require.NoError(t, err)
	expected := cutover.Expectation{AttemptID: "590b56d2-1f13-43b7-82e7-7ee742890bb1", Phase: cutover.PhaseFenced, Epoch: 3, Version: 7}

	mock.ExpectBegin()
	mock.ExpectQuery("FROM shared_pool_cutover_control WHERE singleton = TRUE FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{
			"active_attempt_id", "phase", "authority_epoch", "version", "first_canonical_effect",
		}).AddRow(expected.AttemptID, expected.Phase, expected.Epoch, expected.Version, false))
	mock.ExpectQuery("SELECT jsonb_build_object").WillReturnRows(sqlmock.NewRows([]string{"snapshot"}).AddRow([]byte(`{
        "active_non_media":[{"identity":"reservation:1","payload":{"id":1}}],
        "holds":[{"identity":"hold:1","payload":{"id":1}}],
        "accepted_media_task_ids":[{"identity":"media-task:2","payload":{"id":2,"upstream_request_id":"up-2"}}],
        "terminal_unknown":[{"identity":"trace:3","payload":{"id":3}}],
        "unconsumed_outbox":[{"identity":"scheduler-outbox:4","payload":{"id":4}}]
    }`)))
	mock.ExpectCommit()

	snapshot, err := repo.HandoffSnapshot(context.Background(), expected)
	require.NoError(t, err)
	require.Len(t, snapshot.ActiveNonMedia, 1)
	require.Len(t, snapshot.Holds, 1)
	require.Len(t, snapshot.AcceptedMediaTaskIDs, 1)
	require.Len(t, snapshot.TerminalUnknown, 1)
	require.Len(t, snapshot.UnconsumedOutbox, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSharedPoolCutoverRepository_HandoffSnapshot_rejects_unfenced_authority_before_DB_access(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo, err := NewSharedPoolCutoverRepository(db)
	require.NoError(t, err)

	_, err = repo.HandoffSnapshot(context.Background(), cutover.Expectation{
		AttemptID: "590b56d2-1f13-43b7-82e7-7ee742890bb1",
		Phase:     cutover.PhaseImporting,
		Epoch:     3,
		Version:   8,
	})
	require.ErrorIs(t, err, cutover.ErrSnapshotClosed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSharedPoolCutoverRepository_BeginCanonicalEffect_rolls_back_closed_authority(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo, err := NewSharedPoolCutoverRepository(db)
	require.NoError(t, err)
	expected := cutover.Expectation{AttemptID: "dd2594ae-1088-48ff-85fa-d1e0fa0a09fa", Phase: cutover.PhaseVerified, Epoch: 4, Version: 8}

	mock.ExpectBegin()
	mock.ExpectQuery("shared_pool_cutover_mark_first_canonical_effect").
		WithArgs(expected.AttemptID, expected.Version, expected.Epoch).
		WillReturnError(cutover.ErrCanonicalEffectClosed)
	mock.ExpectRollback()

	tx, _, err := repo.BeginCanonicalEffect(context.Background(), expected)
	require.Nil(t, tx)
	require.ErrorIs(t, err, cutover.ErrCanonicalEffectClosed)
	require.NoError(t, mock.ExpectationsWereMet())
}
