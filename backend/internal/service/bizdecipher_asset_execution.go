package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
)

// AssetExecutionIntentPrefix marks canonical snapshots requiring entitlement-aware replay.
const AssetExecutionIntentPrefix = "[asset-declarative/v1]"

type AssetExecutionManifest struct {
	Schema      string               `json:"schema"`
	Permissions []string             `json:"permissions"`
	Steps       []AssetExecutionStep `json:"steps"`
}
type AssetExecutionStep struct {
	Action     string `json:"action"`
	PromptFile string `json:"prompt_file,omitempty"`
	ModuleFile string `json:"module_file,omitempty"`
}
type AssetExecutionPlanStep struct {
	Action     string `json:"action"`
	Prompt     string `json:"prompt,omitempty"`
	ModuleFile string `json:"module_file,omitempty"`
	Module     []byte `json:"-"`
}
type AssetExecutionPlan struct {
	AssetID       int64                    `json:"asset_id"`
	Version       string                   `json:"version"`
	PackageDigest string                   `json:"package_digest"`
	PlanDigest    string                   `json:"plan_digest"`
	Permissions   []string                 `json:"permissions"`
	Prompt        string                   `json:"prompt"`
	Steps         []AssetExecutionPlanStep `json:"steps"`
}

func ParseAssetExecutionPackage(pkg *CapabilityAssetPackage) (*AssetExecutionPlan, error) {
	if pkg == nil || pkg.Version.Status != CapabilityAssetVersionStatusPublished ||
		(pkg.Version.RuntimeKind != "declarative" && pkg.Version.RuntimeKind != "workflow") {
		return nil, ErrCapabilityAssetPackageInvalid
	}
	var manifest AssetExecutionManifest
	decoder := json.NewDecoder(bytes.NewReader(pkg.Version.Manifest))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return nil, ErrCapabilityAssetPackageInvalid
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, ErrCapabilityAssetPackageInvalid
	}
	if manifest.Schema != "asset-declarative/v1" || len(manifest.Steps) < 1 || len(manifest.Steps) > 8 {
		return nil, ErrCapabilityAssetPackageInvalid
	}
	if len(pkg.Files) == 0 || CapabilityAssetPackageDigest(pkg.Files) != pkg.Version.PackageDigest {
		return nil, ErrCapabilityAssetPackageConflict
	}
	files := map[string][]byte{}
	for _, file := range pkg.Files {
		hash := sha256.Sum256(file.Content)
		if hex.EncodeToString(hash[:]) != file.SHA256 || int64(len(file.Content)) != file.ByteSize {
			return nil, ErrCapabilityAssetPackageConflict
		}
		if _, exists := files[file.Path]; exists {
			return nil, ErrCapabilityAssetPackageInvalid
		}
		files[file.Path] = file.Content
	}
	permissions := map[string]bool{}
	for _, permission := range manifest.Permissions {
		if (permission != "model.generate" && permission != "plugin.wasm") || permissions[permission] {
			return nil, ErrCapabilityAssetPackageInvalid
		}
		permissions[permission] = true
	}
	used := map[string]bool{}
	steps := make([]AssetExecutionPlanStep, 0, len(manifest.Steps))
	for _, step := range manifest.Steps {
		if !permissions[step.Action] {
			return nil, ErrCapabilityAssetPackageInvalid
		}
		used[step.Action] = true
		next := AssetExecutionPlanStep{Action: step.Action}
		switch step.Action {
		case "model.generate":
			path, err := normalizeCapabilityAssetPath(step.PromptFile)
			if err != nil || step.ModuleFile != "" {
				return nil, ErrCapabilityAssetPackageInvalid
			}
			next.Prompt = string(files[path])
			if strings.TrimSpace(next.Prompt) == "" || len(next.Prompt) > 16000 || !utf8.ValidString(next.Prompt) {
				return nil, ErrCapabilityAssetPackageInvalid
			}
		case "plugin.wasm":
			path, err := normalizeCapabilityAssetPath(step.ModuleFile)
			if err != nil || step.PromptFile != "" {
				return nil, ErrCapabilityAssetPackageInvalid
			}
			next.ModuleFile, next.Module = path, files[path]
			if len(next.Module) < 8 || len(next.Module) > 4<<20 || !bytes.Equal(next.Module[:4], []byte{0, 'a', 's', 'm'}) {
				return nil, ErrCapabilityAssetPackageInvalid
			}
		default:
			return nil, ErrCapabilityAssetPackageInvalid
		}
		steps = append(steps, next)
	}
	if len(used) != len(permissions) {
		return nil, ErrCapabilityAssetPackageInvalid
	}
	plan := &AssetExecutionPlan{AssetID: pkg.Version.AssetID, Version: pkg.Version.Version,
		PackageDigest: pkg.Version.PackageDigest, Permissions: manifest.Permissions, Prompt: steps[0].Prompt, Steps: steps}
	data, _ := json.Marshal(plan)
	digest := sha256.Sum256(data)
	plan.PlanDigest = hex.EncodeToString(digest[:])
	return plan, nil
}

func (s *BizDecipherService) GetAssetExecutionPlan(ctx context.Context, assetID, viewer int64, version string) (*AssetExecutionPlan, error) {
	if viewer <= 0 {
		return nil, ErrAssetCommerceForbidden
	}
	pkg, err := s.DownloadCapabilityAssetPackage(ctx, assetID, viewer, version)
	if err != nil {
		return nil, err
	}
	return ParseAssetExecutionPackage(pkg)
}

func (s *BizDecipherService) LaunchAssetExecution(ctx context.Context, adapter workbench.CanonicalAdapter,
	assetID int64, version, planDigest string, permissions []string, command workbench.LaunchCommand) (workbench.LaunchResult, error) {
	plan, err := s.GetAssetExecutionPlan(ctx, assetID, int64(command.Identity.ActorID), version)
	if err != nil {
		return workbench.LaunchResult{}, err
	}
	if plan.PlanDigest != planDigest || len(permissions) != 1 || permissions[0] != "model.generate" {
		return workbench.LaunchResult{}, ErrAssetCommerceConflict
	}
	if len(plan.Steps) != 1 || plan.Steps[0].Action != "model.generate" {
		return workbench.LaunchResult{}, ErrAssetCommerceInvalid
	}
	if len(strings.TrimSpace(command.Intent)) == 0 || len(command.Intent) > 16000 || !columnOperationPattern.MatchString(command.IdempotencyKey) {
		return workbench.LaunchResult{}, ErrAssetCommerceInvalid
	}
	for _, value := range []string{command.CapabilityID, command.CapabilityVersion, command.CapabilityDigest,
		command.CanonicalModelID, command.CanonicalModelVersion, command.AcceptedQuoteID, command.AcceptedQuoteSHA} {
		if strings.TrimSpace(value) == "" {
			return workbench.LaunchResult{}, ErrAssetCommerceInvalid
		}
	}
	if adapter == nil {
		return workbench.LaunchResult{}, ErrWorkbenchRuntimeUnavailable
	}
	// Author text is a user prompt, never system instructions, tools, executable code or URLs.
	command.Intent = fmt.Sprintf("%s asset=%d version=%s package=%s plan=%s\n\nAuthor task instructions:\n%s\n\nUser input:\n%s",
		AssetExecutionIntentPrefix, assetID, version, plan.PackageDigest, plan.PlanDigest, plan.Prompt, command.Intent)
	command.RequiredProtocolFamily = "text"
	return adapter.Launch(ctx, command)
}
