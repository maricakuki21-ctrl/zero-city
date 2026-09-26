package cutover

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthority_Transition_increments_version_for_exact_legal_graph(t *testing.T) {
	state := mustAuthority(t, "attempt-1", 7)
	for _, next := range []Phase{PhaseDraining, PhaseFenced, PhaseImporting, PhaseVerified, PhaseCanonicalOpen, PhaseCanonicalRecovering, PhaseCanonicalOpen, PhaseComplete} {
		before := state.Version
		var err error
		state, err = state.Transition(state.Expectation(), next)
		require.NoError(t, err)
		require.Equal(t, before+1, state.Version)
	}
	require.Equal(t, int64(8), state.Epoch)
}

func TestAuthority_Transition_rejects_stale_or_illegal_without_mutation(t *testing.T) {
	state := mustAuthority(t, "attempt-1", 4)
	stale := state.Expectation()
	stale.Version--

	_, err := state.Transition(stale, PhaseDraining)
	require.ErrorIs(t, err, ErrStaleAuthority)
	_, err = state.Transition(state.Expectation(), PhaseCanonicalOpen)
	require.ErrorIs(t, err, ErrIllegalTransition)
	require.Equal(t, PhaseLegacyOpen, state.Phase)
	require.Equal(t, int64(1), state.Version)
}

func TestAuthority_AbortAfterFence_preserves_failed_attempt_and_raises_epoch(t *testing.T) {
	state := mustAuthority(t, "attempt-1", 11)
	for _, next := range []Phase{PhaseDraining, PhaseFenced, PhaseImporting} {
		var err error
		state, err = state.Transition(state.Expectation(), next)
		require.NoError(t, err)
	}

	result, err := state.AbortAfterFence(state.Expectation(), "attempt-2")
	require.NoError(t, err)
	require.Equal(t, PhaseAbortedBeforeActivation, result.Failed.Phase)
	require.Equal(t, "attempt-1", result.Failed.AttemptID)
	require.Equal(t, int64(12), result.Failed.Epoch)
	require.Equal(t, PhaseLegacyOpen, result.Replacement.Phase)
	require.Equal(t, "attempt-2", result.Replacement.AttemptID)
	require.Equal(t, int64(13), result.Replacement.Epoch)
	require.Equal(t, result.Failed.Version, result.Replacement.Version)

	_, err = state.AbortAfterFence(state.Expectation(), state.AttemptID)
	require.ErrorIs(t, err, ErrAttemptReuse)
}

func TestAuthority_Guards_prevent_old_epoch_and_preopen_canonical_effects(t *testing.T) {
	state := mustAuthority(t, "attempt-1", 3)
	require.NoError(t, state.AuthorizeLegacyRuntimeWrite(3))
	require.ErrorIs(t, state.AuthorizeLegacyRuntimeWrite(2), ErrStaleAuthority)
	_, err := state.MarkFirstCanonicalEffect(state.Expectation())
	require.ErrorIs(t, err, ErrCanonicalEffectClosed)

	state, err = state.Transition(state.Expectation(), PhaseDraining)
	require.NoError(t, err)
	state, err = state.Transition(state.Expectation(), PhaseFenced)
	require.NoError(t, err)
	require.Equal(t, int64(4), state.Epoch)
	require.ErrorIs(t, state.AuthorizeLegacyRuntimeWrite(3), ErrStaleAuthority)
	require.ErrorIs(t, state.AuthorizeLegacyRuntimeWrite(4), ErrLegacyRuntimeFenced)
}

func TestAuthority_MarkFirstCanonicalEffect_is_atomic_and_idempotent(t *testing.T) {
	state := mustAuthority(t, "attempt-1", 8)
	for _, next := range []Phase{PhaseDraining, PhaseFenced, PhaseImporting, PhaseVerified, PhaseCanonicalOpen} {
		var err error
		state, err = state.Transition(state.Expectation(), next)
		require.NoError(t, err)
	}

	marked, err := state.MarkFirstCanonicalEffect(state.Expectation())
	require.NoError(t, err)
	require.True(t, marked.FirstCanonicalEffect)
	require.Equal(t, state.Version+1, marked.Version)
	idempotent, err := marked.MarkFirstCanonicalEffect(marked.Expectation())
	require.NoError(t, err)
	require.Equal(t, marked, idempotent)
}

func mustAuthority(t *testing.T, attemptID string, epoch int64) Authority {
	t.Helper()
	state, err := NewAuthority(attemptID, epoch)
	require.NoError(t, err)
	return state
}

func TestErrors_are_distinguishable(t *testing.T) {
	require.False(t, errors.Is(ErrLegacyRuntimeFenced, ErrStaleAuthority))
}

func TestHandoffSnapshot_Validate_rejects_blank_or_cross_collection_duplicate_identity(t *testing.T) {
	snapshot := HandoffSnapshot{
		Holds:                []SnapshotItem{{Identity: "hold:1"}},
		AcceptedMediaTaskIDs: []SnapshotItem{{Identity: " hold:1 "}},
	}
	require.ErrorIs(t, snapshot.Validate(), ErrDuplicateSnapshotIdentity)

	snapshot = HandoffSnapshot{TerminalUnknown: []SnapshotItem{{Identity: " "}}}
	require.ErrorIs(t, snapshot.Validate(), ErrInvalidSnapshotIdentity)
}
