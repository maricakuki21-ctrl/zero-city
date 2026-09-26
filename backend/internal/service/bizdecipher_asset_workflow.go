package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
)

type AssetWorkflowStepResult struct {
	Index  int             `json:"index"`
	Action string          `json:"action"`
	State  string          `json:"state"`
	RunID  workbench.RunID `json:"run_id,omitempty"`
	Output string          `json:"output,omitempty"`
}

type AssetWorkflowState struct {
	RequestID  string                    `json:"request_id"`
	AssetID    int64                     `json:"asset_id"`
	Version    string                    `json:"version"`
	PlanDigest string                    `json:"plan_digest"`
	State      string                    `json:"state"`
	Total      int                       `json:"total"`
	Steps      []AssetWorkflowStepResult `json:"steps"`
	Output     string                    `json:"output,omitempty"`
	Error      string                    `json:"error,omitempty"`
}

// The repository serializes a single step and commits its output before returning.
// Model calls themselves retain the canonical runtime's durable idempotency keys.
type AssetWorkflowRepository interface {
	AdvanceAssetWorkflow(context.Context, int64, string, string, AssetWorkflowState,
		func(*AssetWorkflowState) error) (*AssetWorkflowState, error)
	GetAssetWorkflow(context.Context, int64, string) (*AssetWorkflowState, error)
	ListAssetWorkflows(context.Context, int64, int64, string) ([]AssetWorkflowState, error)
}

type AssetExecutionResponse struct {
	Run      *workbench.Run      `json:"run,omitempty"`
	Replayed bool                `json:"replayed,omitempty"`
	Workflow *AssetWorkflowState `json:"workflow,omitempty"`
}

func (s *BizDecipherService) ListAssetWorkflows(ctx context.Context, actor, assetID int64, version string) ([]AssetWorkflowState, error) {
	if _, err := s.GetAssetExecutionPlan(ctx, assetID, actor, version); err != nil {
		return nil, err
	}
	repo, ok := s.repo.(AssetWorkflowRepository)
	if !ok {
		return nil, ErrWorkbenchRuntimeUnavailable
	}
	return repo.ListAssetWorkflows(ctx, actor, assetID, version)
}

func (s *BizDecipherService) GetAssetWorkflow(ctx context.Context, actor, assetID int64, version, requestID string) (*AssetWorkflowState, error) {
	if _, err := s.GetAssetExecutionPlan(ctx, assetID, actor, version); err != nil {
		return nil, err
	}
	repo, ok := s.repo.(AssetWorkflowRepository)
	if !ok || !columnOperationPattern.MatchString(requestID) {
		return nil, ErrAssetCommerceInvalid
	}
	state, err := repo.GetAssetWorkflow(ctx, actor, requestID)
	if err != nil {
		return nil, err
	}
	if state.AssetID != assetID || state.Version != version {
		return nil, ErrAssetCommerceForbidden
	}
	return state, nil
}

func (s *BizDecipherService) AdvanceAssetExecution(ctx context.Context, adapter workbench.CanonicalAdapter,
	assetID int64, version, digest string, permissions []string, command workbench.LaunchCommand) (AssetExecutionResponse, error) {
	plan, err := s.GetAssetExecutionPlan(ctx, assetID, int64(command.Identity.ActorID), version)
	if err != nil {
		return AssetExecutionResponse{}, err
	}
	if plan.PlanDigest != digest || !slices.Equal(plan.Permissions, permissions) {
		return AssetExecutionResponse{}, ErrAssetCommerceConflict
	}
	if len(plan.Steps) == 1 && plan.Steps[0].Action == "model.generate" {
		result, err := s.LaunchAssetExecution(ctx, adapter, assetID, version, digest, permissions, command)
		return AssetExecutionResponse{Run: &result.Run, Replayed: result.Replayed}, err
	}
	if command.Identity.ActorID <= 0 || !columnOperationPattern.MatchString(command.IdempotencyKey) ||
		strings.TrimSpace(command.Intent) == "" || len(command.Intent) > 16000 {
		return AssetExecutionResponse{}, ErrAssetCommerceInvalid
	}
	repo, ok := s.repo.(AssetWorkflowRepository)
	if !ok || adapter == nil {
		return AssetExecutionResponse{}, ErrWorkbenchRuntimeUnavailable
	}
	// Bind resource, authorization, user input and exact package plan, not only a UI request ID.
	boundCommand := command
	if command.CapabilityVersion == WorkbenchNativeVersion {
		// Native tokens expire; the stable capability digest binds actor/key/group/model.
		// Renewing that same resource does not create a new workflow or rerun completed steps.
		boundCommand.AcceptedQuoteID, boundCommand.AcceptedQuoteSHA = "", ""
	}
	binding, _ := json.Marshal(struct {
		Plan    string
		Command workbench.LaunchCommand
	}{digest, boundCommand})
	hash := sha256.Sum256(binding)
	initial := AssetWorkflowState{RequestID: command.IdempotencyKey, AssetID: assetID, Version: version,
		PlanDigest: digest, State: "running", Total: len(plan.Steps), Steps: []AssetWorkflowStepResult{}}
	state, err := repo.AdvanceAssetWorkflow(ctx, int64(command.Identity.ActorID), command.IdempotencyKey,
		hex.EncodeToString(hash[:]), initial, func(state *AssetWorkflowState) error {
			if state.State == "succeeded" || state.State == "failed" {
				return nil
			}
			index := len(state.Steps)
			if index >= len(plan.Steps) {
				return ErrAssetCommerceConflict
			}
			step := plan.Steps[index]
			input := command.Intent
			if index > 0 {
				input = state.Steps[index-1].Output
			}
			result := AssetWorkflowStepResult{Index: index, Action: step.Action, State: "succeeded"}
			var output string
			switch step.Action {
			case "model.generate":
				next := command
				stepHash := sha256.Sum256([]byte(fmt.Sprintf("%d:%s:%s:%d", command.Identity.ActorID, command.IdempotencyKey, digest, index)))
				next.IdempotencyKey = "awf-" + hex.EncodeToString(stepHash[:])
				next.Intent = fmt.Sprintf("%s asset=%d version=%s package=%s plan=%s\n\nWorkflow step %d\nAuthor task instructions:\n%s\n\nInput from preceding step:\n%s",
					AssetExecutionIntentPrefix, assetID, version, plan.PackageDigest, digest, index+1, step.Prompt, input)
				next.Intent = strings.TrimSpace(next.Intent)
				next.RequiredProtocolFamily = "text"
				existing, loadErr := adapter.Run(ctx, workbench.RunCommand{Identity: command.Identity,
					RunID: deterministicWorkbenchRunID(command.Identity, next.IdempotencyKey)})
				run := workbench.LaunchResult{Run: existing.Run}
				if errors.Is(loadErr, workbench.ErrRunNotFound) {
					run, err = adapter.Launch(ctx, next)
					if err != nil {
						return err
					}
				} else if loadErr != nil {
					return loadErr
				} else if run.Run.Input.Intent != next.Intent || string(run.Run.Input.CapabilityID) != next.CapabilityID ||
					run.Run.Input.CapabilityVersion != next.CapabilityVersion || string(run.Run.Input.CapabilityDigest) != next.CapabilityDigest ||
					run.Run.Input.CanonicalModelID != next.CanonicalModelID || run.Run.Input.CanonicalModelVersion != next.CanonicalModelVersion {
					return ErrAssetCommerceConflict
				}
				result.RunID = run.Run.ID
				if run.Run.State == workbench.StateFailed || run.Run.State == workbench.StateCancelled {
					state.State, state.Error = "failed", "模型步骤已终止；未执行后续步骤。"
					result.State = string(run.Run.State)
					state.Steps = append(state.Steps, result)
					return nil
				}
				if run.Run.State != workbench.StateSucceeded {
					return nil
				}
				var parts []string
				for _, ref := range run.Run.Artifacts {
					artifact, err := adapter.Artifact(ctx, workbench.ArtifactCommand{Identity: command.Identity, RunID: run.Run.ID, ArtifactID: ref.ArtifactID})
					if err != nil {
						return err
					}
					parts = append(parts, string(artifact.Body))
				}
				output = strings.Join(parts, "\n\n")
			case "plugin.wasm":
				output, err = ExecuteAssetWASM(ctx, step.Module, input)
				if err != nil {
					// This pure sandbox has no external side effects. Capacity/timeouts may retry the same checkpoint.
					return err
				}
			}
			if len(output) > 65536 {
				state.State, state.Error = "failed", "步骤输出超过工作流上限；未截断内容或执行后续步骤。"
				result.State = "failed"
				state.Steps = append(state.Steps, result)
				return nil
			}
			result.Output = output
			state.Steps = append(state.Steps, result)
			state.Output = output
			if len(state.Steps) == state.Total {
				state.State = "succeeded"
			}
			return nil
		})
	return AssetExecutionResponse{Workflow: state}, err
}
