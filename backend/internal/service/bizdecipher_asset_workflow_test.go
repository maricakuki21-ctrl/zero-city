package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	"github.com/stretchr/testify/require"
)

type assetWorkflowRepoStub struct {
	*capabilityAssetPackageRepoStub
	mu     sync.Mutex
	state  *AssetWorkflowState
	digest string
}

func (r *assetWorkflowRepoStub) AdvanceAssetWorkflow(_ context.Context, _ int64, _, digest string,
	initial AssetWorkflowState, advance func(*AssetWorkflowState) error) (*AssetWorkflowState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == nil {
		r.state, r.digest = &initial, digest
	}
	if r.digest != digest {
		return nil, ErrAssetCommerceConflict
	}
	data, _ := json.Marshal(r.state)
	var next AssetWorkflowState
	_ = json.Unmarshal(data, &next)
	if err := advance(&next); err != nil {
		return nil, err
	}
	r.state = &next
	return &next, nil
}
func (r *assetWorkflowRepoStub) GetAssetWorkflow(context.Context, int64, string) (*AssetWorkflowState, error) {
	return r.state, nil
}
func (r *assetWorkflowRepoStub) ListAssetWorkflows(context.Context, int64, int64, string) ([]AssetWorkflowState, error) {
	if r.state == nil {
		return []AssetWorkflowState{}, nil
	}
	return []AssetWorkflowState{*r.state}, nil
}

type workflowAdapter struct {
	workbench.UnavailableAdapter
	commands        []workbench.LaunchCommand
	runs            map[string]workbench.Run
	artifactFailure bool
}

func (a *workflowAdapter) Launch(_ context.Context, command workbench.LaunchCommand) (workbench.LaunchResult, error) {
	if run, found := a.runs[command.IdempotencyKey]; found {
		return workbench.LaunchResult{Run: run, Replayed: true}, nil
	}
	if a.runs == nil {
		a.runs = map[string]workbench.Run{}
	}
	a.commands = append(a.commands, command)
	run := workbench.Run{ID: deterministicWorkbenchRunID(command.Identity, command.IdempotencyKey), State: workbench.StateSucceeded,
		Input: workbench.InputSnapshot{Intent: command.Intent, CapabilityID: workbench.CapabilityID(command.CapabilityID), CapabilityVersion: command.CapabilityVersion,
			CapabilityDigest: workbench.Digest(command.CapabilityDigest), CanonicalModelID: command.CanonicalModelID, CanonicalModelVersion: command.CanonicalModelVersion},
		Artifacts: []workbench.Artifact{{ArtifactID: "artifact"}}}
	a.runs[command.IdempotencyKey] = run
	return workbench.LaunchResult{Run: run}, nil
}
func (a *workflowAdapter) Run(_ context.Context, command workbench.RunCommand) (workbench.RunResult, error) {
	for _, run := range a.runs {
		if run.ID == command.RunID {
			return workbench.RunResult{Run: run}, nil
		}
	}
	return workbench.RunResult{}, workbench.ErrRunNotFound
}
func (a *workflowAdapter) Artifact(_ context.Context, command workbench.ArtifactCommand) (workbench.Artifact, error) {
	if a.artifactFailure {
		return workbench.Artifact{}, errors.New("temporary read failure")
	}
	return workbench.Artifact{Body: []byte("output:" + string(command.RunID))}, nil
}
func workflowFixture(t *testing.T, manifest string) (*BizDecipherService, *assetWorkflowRepoStub, *AssetExecutionPlan, workbench.LaunchCommand) {
	t.Helper()
	pkg := executableAssetPackage()
	pkg.Version.RuntimeKind = "workflow"
	pkg.Version.Manifest = []byte(manifest)
	repo := &assetWorkflowRepoStub{capabilityAssetPackageRepoStub: &capabilityAssetPackageRepoStub{
		asset: &CapabilityAsset{ID: 41, UserID: 9, PricingType: "free", Status: "listed"}, pkg: pkg,
	}}
	s := NewBizDecipherService(repo, nil, nil)
	plan, err := s.GetAssetExecutionPlan(context.Background(), 41, 7, "v1")
	require.NoError(t, err)
	command := workbench.LaunchCommand{Identity: workbench.Identity{ActorID: 7}, Intent: "original user input", IdempotencyKey: "workflow-operation-123",
		CapabilityID: "cap", CapabilityVersion: "v1", CapabilityDigest: "sha", CanonicalModelID: "model", CanonicalModelVersion: "v1",
		AcceptedQuoteID: "authorization", AcceptedQuoteSHA: "sha"}
	return s, repo, plan, command
}

const twoModelWorkflow = `{"schema":"asset-declarative/v1","permissions":["model.generate"],"steps":[{"action":"model.generate","prompt_file":"prompt.txt"},{"action":"model.generate","prompt_file":"prompt.txt"}]}`

func TestAssetWorkflowResumesDurableStepsWithoutDuplicateModelCharge(t *testing.T) {
	s, repo, plan, command := workflowFixture(t, twoModelWorkflow)
	adapter := &workflowAdapter{}
	call := func() (AssetExecutionResponse, error) {
		return s.AdvanceAssetExecution(context.Background(), adapter, 41, "v1", plan.PlanDigest, plan.Permissions, command)
	}
	first, err := call()
	require.NoError(t, err)
	require.Len(t, first.Workflow.Steps, 1)
	require.Equal(t, "running", first.Workflow.State)
	adapter.artifactFailure = true
	_, err = call()
	require.Error(t, err)
	require.Len(t, repo.state.Steps, 1)
	adapter.artifactFailure = false
	second, err := call()
	require.NoError(t, err)
	require.Equal(t, "succeeded", second.Workflow.State)
	require.Len(t, adapter.commands, 2)
	require.Contains(t, adapter.commands[1].Intent, first.Workflow.Output)
	require.NotEqual(t, adapter.commands[0].IdempotencyKey, adapter.commands[1].IdempotencyKey)
	replayed, err := call()
	require.NoError(t, err)
	require.Equal(t, second.Workflow.Output, replayed.Workflow.Output)
	require.Len(t, adapter.commands, 2)
	command.Intent = "changed"
	_, err = call()
	require.ErrorIs(t, err, ErrAssetCommerceConflict)
	repo.asset.Status = "delisted"
	_, err = call()
	require.Error(t, err)
}

func TestAssetWorkflowPluginCheckpointDoesNotFabricateModelRun(t *testing.T) {
	module, err := hex.DecodeString("0061736d0100000001040160000003020100070a01065f737461727400000a040102000b")
	require.NoError(t, err)
	pkg := executableAssetPackage()
	hash := sha256.Sum256(module)
	pkg.Files = append(pkg.Files, CapabilityAssetFile{Path: "tool.wasm", Content: module, SHA256: hex.EncodeToString(hash[:]), ByteSize: int64(len(module))})
	pkg.Version.PackageDigest = CapabilityAssetPackageDigest(pkg.Files)
	pkg.Version.Manifest = []byte(`{"schema":"asset-declarative/v1","permissions":["plugin.wasm"],"steps":[{"action":"plugin.wasm","module_file":"tool.wasm"}]}`)
	repo := &assetWorkflowRepoStub{capabilityAssetPackageRepoStub: &capabilityAssetPackageRepoStub{
		asset: &CapabilityAsset{ID: 41, UserID: 9, PricingType: "free", Status: "listed"}, pkg: pkg,
	}}
	s := NewBizDecipherService(repo, nil, nil)
	plan, err := s.GetAssetExecutionPlan(context.Background(), 41, 7, "v1")
	require.NoError(t, err)
	adapter := &workflowAdapter{}
	result, err := s.AdvanceAssetExecution(context.Background(), adapter, 41, "v1", plan.PlanDigest, plan.Permissions,
		workbench.LaunchCommand{Identity: workbench.Identity{ActorID: 7}, Intent: "plugin input", IdempotencyKey: "workflow-plugin-123"})
	require.NoError(t, err)
	require.Equal(t, "succeeded", result.Workflow.State)
	require.Len(t, result.Workflow.Steps, 1)
	require.Empty(t, result.Workflow.Steps[0].RunID)
	require.Nil(t, result.Run)
	require.Empty(t, adapter.commands)
	require.Equal(t, result.Workflow, repo.state)
}

func TestAssetWorkflowRejectsUndeclaredAndInvalidSteps(t *testing.T) {
	for _, manifest := range []string{
		`{"schema":"asset-declarative/v1","permissions":["model.generate","plugin.wasm"],"steps":[{"action":"model.generate","prompt_file":"prompt.txt"}]}`,
		`{"schema":"asset-declarative/v1","permissions":["model.generate"],"steps":[{"action":"model.generate","prompt_file":"prompt.txt","module_file":"x"}]}`,
		`{"schema":"asset-declarative/v1","permissions":["plugin.wasm"],"steps":[{"action":"plugin.wasm","module_file":"prompt.txt"}]}`,
		`{"schema":"asset-declarative/v1","permissions":["model.generate"],"steps":[]}`,
	} {
		pkg := executableAssetPackage()
		pkg.Version.Manifest = []byte(manifest)
		_, err := ParseAssetExecutionPackage(pkg)
		require.Error(t, err)
	}
}

func TestAssetWorkflowNativeAuthorizationRenewalKeepsCompletedSteps(t *testing.T) {
	s, _, plan, command := workflowFixture(t, twoModelWorkflow)
	command.CapabilityVersion = WorkbenchNativeVersion
	adapter := &workflowAdapter{artifactFailure: true}
	_, err := s.AdvanceAssetExecution(context.Background(), adapter, 41, "v1", plan.PlanDigest, plan.Permissions, command)
	require.Error(t, err)
	require.Len(t, adapter.commands, 1)
	command.AcceptedQuoteID, command.AcceptedQuoteSHA = "renewed", "renewed-sha"
	adapter.artifactFailure = false
	result, err := s.AdvanceAssetExecution(context.Background(), adapter, 41, "v1", plan.PlanDigest, plan.Permissions, command)
	require.NoError(t, err)
	require.Len(t, result.Workflow.Steps, 1)
	require.Len(t, adapter.commands, 1, "renewal reads the existing original-authorized model run")
	result, err = s.AdvanceAssetExecution(context.Background(), adapter, 41, "v1", plan.PlanDigest, plan.Permissions, command)
	require.NoError(t, err)
	require.Equal(t, "succeeded", result.Workflow.State)
	require.Len(t, adapter.commands, 2)
	require.Equal(t, "renewed", adapter.commands[1].AcceptedQuoteID)
	command.CapabilityID = "different-key"
	_, err = s.AdvanceAssetExecution(context.Background(), adapter, 41, "v1", plan.PlanDigest, plan.Permissions, command)
	require.ErrorIs(t, err, ErrAssetCommerceConflict)
}
