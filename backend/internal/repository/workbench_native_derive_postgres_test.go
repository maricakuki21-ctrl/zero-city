package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestNativeWorkbenchDerivationPostgres(t *testing.T) {
	dsn := os.Getenv("MARKETPLACE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MARKETPLACE_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var database string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT current_database()`).Scan(&database))
	require.True(t, strings.HasPrefix(database, "bizdecipher_columns_acceptance_test_"), "disposable database only")
	require.NoError(t, ApplyMigrations(ctx, db))
	owner := insertMarketplaceTestUser(t, db, fmt.Sprintf("native-derive-%d@example.test", time.Now().UnixNano()))
	identity := workbench.Identity{ActorID: workbench.ActorID(owner)}
	digest := workbench.Digest(strings.Repeat("a", 64))
	group := int64(8)
	capability := workbench.Capability{ID: "native_derive", Version: service.WorkbenchNativeVersion, Digest: digest,
		CanonicalModelID: "test-model", CanonicalModelVersion: service.WorkbenchNativeVersion, Protocol: "chat"}
	record := service.WorkbenchCatalogRecord{Capability: capability,
		Quote: service.WorkbenchAcceptedQuote{ID: "native.signed-resource.never-a-quote-row", SHA256: digest,
			ModelID: "test-model", ModelVersion: service.WorkbenchNativeVersion, GroupID: group, ExpiresAt: time.Now().Add(time.Hour)},
		User:   &service.User{ID: owner, Status: service.StatusActive},
		APIKey: &service.APIKey{ID: 17, UserID: owner, GroupID: &group, Status: service.StatusActive}}
	catalog, err := service.NewWorkbenchCatalog(nativePGCatalog{record: record}, nil)
	require.NoError(t, err)
	executor := &nativePGExecutor{}
	repo := NewWorkbenchRuntimeRepository(db)
	store := ProvideWorkbenchRuntimeStore(repo)
	runtime, err := service.NewWorkbenchRuntime(service.WorkbenchRuntimeDependencies{Store: store, Catalog: catalog, Executor: executor})
	require.NoError(t, err)
	first, err := runtime.Launch(ctx, workbench.LaunchCommand{Identity: identity, IdempotencyKey: "native-derive-launch",
		Intent: "generate", CapabilityID: string(capability.ID), CapabilityVersion: capability.Version,
		CapabilityDigest: string(digest), CanonicalModelID: capability.CanonicalModelID, CanonicalModelVersion: capability.CanonicalModelVersion,
		AcceptedQuoteID: string(record.Quote.ID), AcceptedQuoteSHA: string(digest)})
	require.NoError(t, err)
	saved, err := runtime.Save(ctx, workbench.SaveCommand{Identity: identity, RunID: first.Run.ID, IdempotencyKey: "native-derive-save", ReplayLabel: "original"})
	require.NoError(t, err)
	savedAgain, err := runtime.Save(ctx, workbench.SaveCommand{Identity: identity, RunID: first.Run.ID, IdempotencyKey: "native-derive-save", ReplayLabel: "original"})
	require.NoError(t, err)
	require.True(t, savedAgain.Replayed)
	require.Equal(t, saved.Snapshot.ID, savedAgain.Snapshot.ID)
	forkCommand := workbench.ForkCommand{Identity: identity, RunID: first.Run.ID, IdempotencyKey: "native-derive-fork"}
	forked, err := runtime.Fork(ctx, forkCommand)
	require.NoError(t, err)
	retried, err := runtime.Fork(ctx, forkCommand)
	require.NoError(t, err)
	require.True(t, retried.Replayed)
	require.Equal(t, forked.Run.ID, retried.Run.ID)
	require.Equal(t, forked.SourceSnapshotID, retried.SourceSnapshotID)
	var snapshots int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM workbench_saved_snapshots WHERE owner_user_id=$1`, owner).Scan(&snapshots))
	require.Equal(t, 1, snapshots, "fork reuses the saved same-run input snapshot; retries create no orphan snapshots")
	require.Equal(t, saved.Snapshot.ID, forked.SourceSnapshotID)
	replayCommand := workbench.ReplayCommand{Identity: identity, ReplayID: saved.Snapshot.ID, IdempotencyKey: "native-derive-replay"}
	replayed, err := runtime.Replay(ctx, replayCommand)
	require.NoError(t, err)
	replayedAgain, err := runtime.Replay(ctx, replayCommand)
	require.NoError(t, err)
	require.True(t, replayedAgain.Replayed)
	require.Equal(t, replayed.Run.ID, replayedAgain.Run.ID)
	require.Equal(t, saved.Snapshot.ID, replayedAgain.SourceSnapshotID)
	require.Equal(t, 3, executor.calls, "one launch, one fork, one replay; no retry redispatch")
	for _, run := range []workbench.Run{forked.Run, replayed.Run} {
		require.Equal(t, workbench.StateSucceeded, run.State)
		require.Equal(t, record.Quote.ID, run.Input.AcceptedQuoteID)
		var quote, event, journal sql.NullString
		require.NoError(t, db.QueryRowContext(ctx, `SELECT accepted_quote_id,canonical_usage_event_id,ledger_journal_id FROM workbench_runs WHERE run_id=$1`, string(run.ID)).Scan(&quote, &event, &journal))
		require.False(t, quote.Valid || event.Valid || journal.Valid, "native token must not enter relational quote/settlement fields")
	}
	other := workbench.Identity{ActorID: identity.ActorID + 1}
	_, err = runtime.Fork(ctx, workbench.ForkCommand{Identity: other, RunID: first.Run.ID, IdempotencyKey: "wrong-owner-fork"})
	require.Error(t, err)
	_, err = runtime.Replay(ctx, workbench.ReplayCommand{Identity: other, ReplayID: saved.Snapshot.ID, IdempotencyKey: "wrong-owner-replay"})
	require.Error(t, err)
	require.Equal(t, 3, executor.calls)
	// Saving after a fork (and saving again with a new label) must not collide
	// on the input digest: distinct snapshots can share immutable run input.
	for _, label := range []string{"after-fork", "another-label"} {
		result, saveErr := runtime.Save(ctx, workbench.SaveCommand{Identity: identity, RunID: first.Run.ID,
			IdempotencyKey: label, ReplayLabel: label})
		require.NoError(t, saveErr)
		require.Equal(t, label, result.Snapshot.Label)
	}

	queued := first.Run.Clone()
	queued.ID = workbench.RunID(fmt.Sprintf("wbr_cancel%d", owner))
	queued.State = workbench.StateQueued
	queued.Version = 1
	queued.NextEventSeq = 1
	queued.Artifacts = nil
	queued.CanonicalRequestID = ""
	queued.TerminalAt = nil
	claim := workbench.OperationClaim{ID: workbench.OperationID(fmt.Sprintf("wbo_createcancel%d", owner)),
		OwnerID: identity.ActorID, RunID: queued.ID, Key: "create-cancel-fixture", Kind: workbench.OperationCreate, Fingerprint: workbench.RequestFingerprint(strings.Repeat("c", 64))}
	_, err = store.ClaimOperation(ctx, service.WorkbenchOperationRequest{Identity: identity, Claim: claim, Run: queued,
		Event: workbench.Event{ID: workbench.EventID(fmt.Sprintf("wbe_cancelqueued%d", owner)), RunID: queued.ID, Kind: workbench.EventRunQueued, State: workbench.StateQueued, CreatedAt: time.Now()}})
	require.NoError(t, err)
	cancelCommand := workbench.CancelCommand{Identity: identity, RunID: queued.ID, IdempotencyKey: "native-cancel-once", Reason: "stop"}
	cancelled, err := runtime.Cancel(ctx, cancelCommand)
	require.NoError(t, err)
	require.Equal(t, workbench.StateCancelRequested, cancelled.Run.State)
	cancelAgain, err := runtime.Cancel(ctx, cancelCommand)
	require.NoError(t, err)
	require.True(t, cancelAgain.Replayed)
	terminal, err := workbench.ApplyTransition(cancelAgain.Run, workbench.Transition{To: workbench.StateCancelled, At: time.Now()})
	require.NoError(t, err)
	_, err = store.TransitionRun(ctx, service.WorkbenchRunMutation{Identity: identity, Run: terminal,
		Event: workbench.Event{ID: workbench.EventID(fmt.Sprintf("wbe_cancelterminal%d", owner)), RunID: queued.ID, Kind: workbench.EventRunCancelled, State: workbench.StateCancelled, CreatedAt: time.Now()}})
	require.NoError(t, err)
	cancelAgain, err = runtime.Cancel(ctx, cancelCommand)
	require.NoError(t, err)
	require.True(t, cancelAgain.Replayed)
	require.Equal(t, workbench.StateCancelled, cancelAgain.Run.State)
	cancelCommand.Reason = "different payload"
	_, err = runtime.Cancel(ctx, cancelCommand)
	require.ErrorIs(t, err, workbench.ErrIdempotencyConflict)
}
