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

type nativePGCatalog struct {
	record service.WorkbenchCatalogRecord
}

func (s nativePGCatalog) ListWorkbenchCapabilities(context.Context, workbench.Identity) ([]workbench.Capability, error) {
	return []workbench.Capability{s.record.Capability}, nil
}
func (s nativePGCatalog) ResolveWorkbenchCatalog(context.Context, workbench.Identity, workbench.CapabilityID, workbench.QuoteID) (service.WorkbenchCatalogRecord, error) {
	return s.record, nil
}

type nativePGExecutor struct{ calls int }

func (s *nativePGExecutor) Execute(context.Context, service.WorkbenchCanonicalRequest) (service.WorkbenchCanonicalResult, error) {
	s.calls++
	return service.WorkbenchCanonicalResult{CanonicalRequestID: "gateway-provider-request",
		ContentType: "text/plain; charset=utf-8", Body: []byte("persisted native model output")}, nil
}
func (*nativePGExecutor) Cancel(context.Context, workbench.RunID) error { return nil }

func TestNativeWorkbenchPostgresPersistsOutputAndReplaysOnce(t *testing.T) {
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
	owner := insertMarketplaceTestUser(t, db, fmt.Sprintf("native-workbench-%d@example.test", time.Now().UnixNano()))
	identity := workbench.Identity{ActorID: workbench.ActorID(owner)}
	digest := workbench.Digest(strings.Repeat("a", 64))
	capability := workbench.Capability{ID: "native_test", Version: "native-metered-v1", Digest: digest,
		CanonicalModelID: "test-model", CanonicalModelVersion: "native-metered-v1", Protocol: "chat"}
	group := int64(8)
	record := service.WorkbenchCatalogRecord{
		Capability: capability,
		Quote: service.WorkbenchAcceptedQuote{ID: "native.signed-resource.not-a-quote-row", SHA256: digest,
			ModelID: "test-model", ModelVersion: "native-metered-v1", GroupID: group, ExpiresAt: time.Now().Add(time.Hour)},
		User:   &service.User{ID: owner, Status: service.StatusActive},
		APIKey: &service.APIKey{ID: 17, UserID: owner, GroupID: &group, Status: service.StatusActive},
	}
	catalog, err := service.NewWorkbenchCatalog(nativePGCatalog{record: record}, nil)
	require.NoError(t, err)
	executor := &nativePGExecutor{}
	store := ProvideWorkbenchRuntimeStore(NewWorkbenchRuntimeRepository(db))
	runtime, err := service.NewWorkbenchRuntime(service.WorkbenchRuntimeDependencies{Store: store, Catalog: catalog, Executor: executor})
	require.NoError(t, err)
	command := workbench.LaunchCommand{
		Identity: identity, IdempotencyKey: "native-pg-operation",
		Intent: "generate", CapabilityID: string(capability.ID), CapabilityVersion: capability.Version,
		CapabilityDigest: string(digest), CanonicalModelID: capability.CanonicalModelID,
		CanonicalModelVersion: capability.CanonicalModelVersion, AcceptedQuoteID: string(record.Quote.ID),
		AcceptedQuoteSHA: string(digest),
	}
	first, err := runtime.Launch(ctx, command)
	require.NoError(t, err)
	require.Equal(t, workbench.StateSucceeded, first.Run.State)
	require.Len(t, first.Run.Artifacts, 1, "completed result must include its persisted artifact")
	second, err := runtime.Launch(ctx, command)
	require.NoError(t, err)
	require.True(t, second.Replayed)
	require.Equal(t, 1, executor.calls, "a persisted retry never redispatches the model")
	require.Len(t, second.Run.Artifacts, 1)
	require.Nil(t, second.Run.ActualCost, "native charges are not fabricated canonical receipts")
	artifactID := second.Run.Artifacts[0].ArtifactID
	artifact, err := runtime.Artifact(ctx, workbench.ArtifactCommand{Identity: identity, RunID: first.Run.ID, ArtifactID: artifactID})
	require.NoError(t, err)
	require.Equal(t, "persisted native model output", string(artifact.Body))
	require.Empty(t, artifact.AcceptedQuoteID)
	require.Empty(t, artifact.CanonicalUsageEventID)
	require.Empty(t, artifact.JournalID)
	require.Equal(t, "gateway-provider-request", artifact.CanonicalRequestID)
	var quote, usage, journal sql.NullString
	require.NoError(t, db.QueryRowContext(ctx, `SELECT accepted_quote_id,canonical_usage_event_id,ledger_journal_id
		FROM workbench_artifacts WHERE artifact_id=$1`, string(artifactID)).Scan(&quote, &usage, &journal))
	require.False(t, quote.Valid || usage.Valid || journal.Valid, "resource authorization must never enter ledger foreign keys")
	saved, err := runtime.Save(ctx, workbench.SaveCommand{Identity: identity, RunID: first.Run.ID, IdempotencyKey: "save-native", ReplayLabel: "native result"})
	require.NoError(t, err)
	workspace, err := runtime.Workspace(ctx, identity)
	require.NoError(t, err)
	require.NotNil(t, workspace.CurrentRun)
	require.Equal(t, first.Run.ID, workspace.CurrentRun.ID)
	require.Len(t, workspace.CurrentRun.Artifacts, 1)
	require.Len(t, workspace.SavedSnapshots, 1)
	require.Equal(t, saved.Snapshot.ID, workspace.SavedSnapshots[0].ID)
	_, err = runtime.Artifact(ctx, workbench.ArtifactCommand{Identity: workbench.Identity{ActorID: identity.ActorID + 1},
		RunID: first.Run.ID, ArtifactID: artifactID})
	require.ErrorIs(t, err, workbench.ErrArtifactNotFound)
	_, err = db.ExecContext(ctx, `UPDATE workbench_runs SET canonical_request_id='forged',
		version=version+1,next_event_seq=next_event_seq+1 WHERE run_id=$1`, string(first.Run.ID))
	require.ErrorContains(t, err, "terminal lineage is immutable")
	_, err = db.ExecContext(ctx, `UPDATE workbench_runs SET cached_actual=99,
		version=version+1,next_event_seq=next_event_seq+1 WHERE run_id=$1`, string(first.Run.ID))
	require.ErrorContains(t, err, "terminal lineage is immutable")
	legacyID := workbench.ArtifactID(string(artifactID) + "legacy")
	_, err = db.ExecContext(ctx, `INSERT INTO workbench_artifacts
		(artifact_id,run_id,owner_user_id,artifact_kind,media_type,storage_uri,byte_size,digest_sha256)
		VALUES($1,$2,$3,'text','text/plain',$4,5,$5)`, string(legacyID), string(first.Run.ID), owner,
		"workbench://"+string(first.Run.ID)+"/legacy", strings.Repeat("b", 64))
	require.NoError(t, err)
	_, err = runtime.Artifact(ctx, workbench.ArtifactCommand{Identity: identity, RunID: first.Run.ID, ArtifactID: legacyID})
	require.ErrorIs(t, err, workbench.ErrArtifactNotFound, "legacy metadata-only output must not appear as a successful empty download")
}
