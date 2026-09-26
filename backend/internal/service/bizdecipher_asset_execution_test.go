package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	"github.com/stretchr/testify/require"
)

func executableAssetPackage() *CapabilityAssetPackage {
	content := []byte("Summarize user input.")
	hash := sha256.Sum256(content)
	files := []CapabilityAssetFile{{Path: "prompt.txt", Content: content, ByteSize: int64(len(content)), SHA256: hex.EncodeToString(hash[:])}}
	return &CapabilityAssetPackage{Version: CapabilityAssetVersion{
		AssetID: 41, Version: "v1", RuntimeKind: "declarative", Status: "published",
		Manifest:      []byte(`{"schema":"asset-declarative/v1","permissions":["model.generate"],"steps":[{"action":"model.generate","prompt_file":"prompt.txt"}]}`),
		PackageDigest: CapabilityAssetPackageDigest(files),
	}, Files: files}
}
func TestAssetExecutionStrictProtocol(t *testing.T) {
	pkg := executableAssetPackage()
	plan, err := ParseAssetExecutionPackage(pkg)
	require.NoError(t, err)
	require.Equal(t, "Summarize user input.", plan.Prompt)
	require.Len(t, plan.PlanDigest, 64)
	for _, manifest := range []string{
		`{"schema":"asset-declarative/v1","permissions":["network"],"steps":[{"action":"model.generate","prompt_file":"prompt.txt"}]}`,
		`{"schema":"asset-declarative/v1","permissions":["model.generate"],"steps":[{"action":"shell","prompt_file":"prompt.txt"}]}`,
		`{"schema":"asset-declarative/v1","permissions":["model.generate"],"steps":[{"action":"model.generate","prompt_file":"https://evil.test"}]}`,
		`{"schema":"asset-declarative/v1","permissions":["model.generate"],"steps":[{"action":"model.generate","prompt_file":"prompt.txt"}],"url":"https://evil.test"}`,
	} {
		p := executableAssetPackage()
		p.Version.Manifest = []byte(manifest)
		_, err := ParseAssetExecutionPackage(p)
		require.Error(t, err)
	}
	pkg.Files[0].Content = []byte("tampered")
	_, err = ParseAssetExecutionPackage(pkg)
	require.ErrorIs(t, err, ErrCapabilityAssetPackageConflict)
}

type assetExecutionAdapter struct {
	workbench.UnavailableAdapter
	calls   int
	command workbench.LaunchCommand
}

func (a *assetExecutionAdapter) Launch(_ context.Context, command workbench.LaunchCommand) (workbench.LaunchResult, error) {
	a.calls++
	a.command = command
	return workbench.LaunchResult{Run: workbench.Run{ID: "asset-run-1"}}, nil
}
func TestAssetExecutionLaunchBindingAndRevocation(t *testing.T) {
	repo := &capabilityAssetPackageRepoStub{
		asset: &CapabilityAsset{ID: 41, UserID: 9, PricingType: "free", Status: "listed"},
		pkg:   executableAssetPackage(),
	}
	s := NewBizDecipherService(repo, nil, nil)
	plan, err := s.GetAssetExecutionPlan(context.Background(), 41, 7, "v1")
	require.NoError(t, err)
	adapter := &assetExecutionAdapter{}
	command := workbench.LaunchCommand{Identity: workbench.Identity{ActorID: 7}, Intent: "input", IdempotencyKey: "asset-execution-operation-1",
		CapabilityID: "canonical", CapabilityVersion: "v1", CapabilityDigest: "cap-sha",
		CanonicalModelID: "model", CanonicalModelVersion: "v1", AcceptedQuoteID: "q1", AcceptedQuoteSHA: "sha"}
	result, err := s.LaunchAssetExecution(context.Background(), adapter, 41, "v1", plan.PlanDigest, []string{"model.generate"}, command)
	require.NoError(t, err)
	require.Equal(t, workbench.RunID("asset-run-1"), result.Run.ID)
	require.True(t, strings.HasPrefix(adapter.command.Intent, AssetExecutionIntentPrefix))
	require.Contains(t, adapter.command.Intent, plan.PackageDigest)
	require.Equal(t, "q1", adapter.command.AcceptedQuoteID)
	require.Equal(t, "text", adapter.command.RequiredProtocolFamily)
	_, err = s.LaunchAssetExecution(context.Background(), adapter, 41, "v1", "stale", []string{"model.generate"}, command)
	require.ErrorIs(t, err, ErrAssetCommerceConflict)
	_, err = s.LaunchAssetExecution(context.Background(), adapter, 41, "v1", plan.PlanDigest, nil, command)
	require.ErrorIs(t, err, ErrAssetCommerceConflict)
	repo.asset.PricingType = "paid"
	_, err = s.LaunchAssetExecution(context.Background(), adapter, 41, "v1", plan.PlanDigest, []string{"model.generate"}, command)
	require.ErrorIs(t, err, ErrCapabilityAssetPackageForbidden)
	repo.asset.PricingType = "free"
	repo.pkg.Version.Status = "revoked"
	_, err = s.LaunchAssetExecution(context.Background(), adapter, 41, "v1", plan.PlanDigest, []string{"model.generate"}, command)
	require.ErrorIs(t, err, ErrCapabilityAssetPackageNotFound)
	require.Equal(t, 1, adapter.calls)
}
