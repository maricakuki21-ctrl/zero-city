package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

var workbenchWorkspaceIDPattern = regexp.MustCompile(`^wbw_[0-9a-z]+$`)

type workbenchRuntimeStore struct {
	repo *WorkbenchRuntimeRepository
}

func ProvideWorkbenchRuntimeStore(repo *WorkbenchRuntimeRepository) service.WorkbenchRuntimeStore {
	return &workbenchRuntimeStore{repo: repo}
}

func workbenchNewID(prefix string) string {
	return prefix + strings.ReplaceAll(uuid.NewString(), "-", "")
}

func workbenchStoredVersion(run workbench.Run) uint64 {
	if run.Version == 0 {
		return 0
	}
	return run.Version - 1
}

func (s *workbenchRuntimeStore) LoadWorkspace(ctx context.Context, identity workbench.Identity) (workbench.Workspace, error) {
	if err := identity.Validate(); err != nil {
		return workbench.Workspace{}, err
	}
	id := workbench.WorkspaceID("wbw_" + strings.ToLower(strconv.FormatInt(int64(identity.ActorID), 10)))
	if !workbenchWorkspaceIDPattern.MatchString(string(id)) {
		return workbench.Workspace{}, service.ErrWorkbenchRuntimeUnavailable
	}
	workspace := workbench.Workspace{ID: id, OwnerID: identity.ActorID, Version: 1, HydratedAt: time.Now().UTC(), SavedSnapshots: []workbench.SavedSnapshot{}}
	var latest string
	err := s.repo.db.QueryRowContext(ctx, `SELECT run_id FROM workbench_runs
		WHERE owner_user_id=$1 ORDER BY created_at DESC,run_id DESC LIMIT 1`, int64(identity.ActorID)).Scan(&latest)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return workbench.Workspace{}, err
	}
	if latest != "" {
		run, err := s.LoadRun(ctx, identity, workbench.RunID(latest))
		if err != nil {
			return workbench.Workspace{}, err
		}
		workspace.CurrentRun = &run
	}
	rows, err := s.repo.db.QueryContext(ctx, `SELECT snapshot_id,run_id,owner_user_id,snapshot,snapshot_sha256,created_at
		FROM workbench_saved_snapshots WHERE owner_user_id=$1 ORDER BY created_at DESC,snapshot_id DESC LIMIT 50`, int64(identity.ActorID))
	if err != nil {
		return workbench.Workspace{}, err
	}
	defer rows.Close()
	for rows.Next() {
		snapshot, err := scanWorkbenchSnapshot(rows)
		if err != nil {
			return workbench.Workspace{}, err
		}
		workspace.SavedSnapshots = append(workspace.SavedSnapshots, snapshot)
	}
	return workspace, rows.Err()
}

func (s *workbenchRuntimeStore) LoadRun(ctx context.Context, identity workbench.Identity, runID workbench.RunID) (workbench.Run, error) {
	run, err := s.repo.LoadRun(ctx, identity, runID)
	if err != nil {
		return workbench.Run{}, err
	}
	rows, err := s.repo.db.QueryContext(ctx, `SELECT artifact_id,run_id,owner_user_id,artifact_kind,media_type,storage_uri,
		byte_size,digest_sha256,accepted_quote_id,canonical_request_id,canonical_usage_event_id,ledger_journal_id,
		canonical_media_business_event_id,runner_job_id,upstream_task_id,created_at,NULL::bytea
		FROM workbench_artifacts WHERE run_id=$1 AND owner_user_id=$2 AND content_bytes IS NOT NULL ORDER BY created_at,artifact_id`,
		string(runID), int64(identity.ActorID))
	if err != nil {
		return workbench.Run{}, err
	}
	defer rows.Close()
	for rows.Next() {
		artifact, err := scanWorkbenchArtifact(rows)
		if err != nil {
			return workbench.Run{}, err
		}
		run.Artifacts = append(run.Artifacts, artifact)
	}
	return run, rows.Err()
}

func (s *workbenchRuntimeStore) LoadArtifact(ctx context.Context, identity workbench.Identity, runID workbench.RunID, artifactID workbench.ArtifactID) (workbench.Artifact, error) {
	return s.repo.LoadArtifact(ctx, identity, runID, artifactID)
}

func (s *workbenchRuntimeStore) ReadEvents(ctx context.Context, identity workbench.Identity, runID workbench.RunID, cursor workbench.StreamCursor, limit uint32) (workbench.EventRead, error) {
	return s.repo.LoadEventRead(ctx, identity, runID, cursor, limit)
}

func (s *workbenchRuntimeStore) ClaimOperation(ctx context.Context, request service.WorkbenchOperationRequest) (service.WorkbenchOperationResult, error) {
	outcome, err := s.repo.CreateRun(ctx, WorkbenchCreateRunRequest{Identity: request.Identity, Claim: request.Claim, Run: request.Run, Event: request.Event})
	if err != nil {
		return service.WorkbenchOperationResult{}, err
	}
	workbench.NotifyRun(outcome.Run.ID)
	return service.WorkbenchOperationResult{Run: outcome.Run, Replayed: outcome.Replayed}, nil
}

func (s *workbenchRuntimeStore) TransitionRun(ctx context.Context, request service.WorkbenchRunMutation) (workbench.Run, error) {
	outcome, err := s.repo.ApplyOperation(ctx, WorkbenchOperationRequest{
		Identity: request.Identity, Claim: workbenchMutationClaim(request.Identity, request.Run, request.Event),
		ExpectedVersion: workbenchStoredVersion(request.Run), Run: request.Run, Event: request.Event,
	})
	if err != nil {
		return workbench.Run{}, err
	}
	workbench.NotifyRun(outcome.Run.ID)
	return outcome.Run, nil
}

func (s *workbenchRuntimeStore) CompleteRun(ctx context.Context, request service.WorkbenchExecutionCommit) (workbench.Run, error) {
	effects := WorkbenchOperationEffects{}
	if request.Artifact.ArtifactID != "" {
		artifact := request.Artifact
		effects.Artifact = &artifact
	}
	outcome, err := s.repo.ApplyOperation(ctx, WorkbenchOperationRequest{
		Identity: request.Identity, Claim: workbenchMutationClaim(request.Identity, request.Run, request.Event),
		ExpectedVersion: workbenchStoredVersion(request.Run), Run: request.Run, Event: request.Event, Effects: effects,
	})
	if err != nil {
		return workbench.Run{}, err
	}
	workbench.NotifyRun(outcome.Run.ID)
	return s.LoadRun(ctx, request.Identity, outcome.Run.ID)
}

func workbenchMutationClaim(identity workbench.Identity, run workbench.Run, event workbench.Event) workbench.OperationClaim {
	kind := workbench.OperationCreate
	switch event.Kind {
	case workbench.EventRunCancelled, workbench.EventCancelRequested:
		kind = workbench.OperationCancel
	case workbench.EventSnapshotSaved:
		kind = workbench.OperationSave
	}
	return workbench.OperationClaim{
		ID: workbench.OperationID(workbenchNewID("wbo_")), OwnerID: identity.ActorID, RunID: run.ID,
		Key: string(event.ID), Kind: kind, Fingerprint: workbench.RequestFingerprint(strings.Repeat("a", 64)),
	}
}

var _ service.WorkbenchRuntimeStore = (*workbenchRuntimeStore)(nil)
