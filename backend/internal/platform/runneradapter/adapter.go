package runneradapter

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidRequest    = errors.New("runneradapter: invalid request")
	ErrDenied            = errors.New("runneradapter: execution denied")
	ErrStaleArtifact     = errors.New("runneradapter: stale artifact")
	ErrMisleadingSuccess = errors.New("runneradapter: success without verified artifact")
)

type MediaKind string

const (
	KindImage    MediaKind = "image"
	KindVideo    MediaKind = "video"
	KindWorkflow MediaKind = "workflow"
)

type RunState string

const (
	StateQueued      RunState = "queued"
	StateRunning     RunState = "running"
	StateCompleted   RunState = "completed"
	StateCancelled   RunState = "cancelled"
	StateInterrupted RunState = "interrupted"
)

type Artifact struct {
	ID, RequestID, URI, SHA256 string
	CreatedAt                  time.Time
}
type Request struct {
	RequestID, IdempotencyKey string
	Kind                      MediaKind
	Sidecar, Prompt           string
	InputArtifacts            []string
	WorkspaceClean            bool
}
type SidecarSpec struct {
	Name, Version, Digest string
	Kinds                 []MediaKind
}
type SidecarResult struct {
	State      RunState
	Artifacts  []Artifact
	RunnerText string
}

type Sidecar interface {
	Execute(context.Context, Request) (SidecarResult, error)
	Cancel(context.Context, string) error
	Resume(context.Context, string) (SidecarResult, error)
}

type Adapter struct {
	sidecars map[string]registeredSidecar
	now      func() time.Time
}
type registeredSidecar struct {
	spec   SidecarSpec
	client Sidecar
}

func New(spec SidecarSpec, client Sidecar) (*Adapter, error) {
	if client == nil || !spec.valid() {
		return nil, fmt.Errorf("%w: sidecar registration", ErrInvalidRequest)
	}
	return &Adapter{sidecars: map[string]registeredSidecar{spec.Name: {spec: spec, client: client}}, now: time.Now}, nil
}

func (a *Adapter) Start(ctx context.Context, req Request) (SidecarResult, error) {
	if err := req.validate(); err != nil {
		return SidecarResult{}, err
	}
	reg, ok := a.sidecars[req.Sidecar]
	if !ok || !supports(reg.spec.Kinds, req.Kind) {
		return SidecarResult{}, fmt.Errorf("%w: sidecar or media kind not allowlisted", ErrDenied)
	}
	if err := contextErr(ctx); err != nil {
		return SidecarResult{}, err
	}
	started := a.now()
	result, err := reg.client.Execute(ctx, req)
	if err != nil {
		return SidecarResult{}, err
	}
	if result.State == StateCompleted {
		if err := verifyArtifacts(req, result.Artifacts, started, a.now()); err != nil {
			return SidecarResult{}, err
		}
	}
	return result, nil
}

func (a *Adapter) Cancel(ctx context.Context, sidecar, requestID string) error {
	if err := validateID(requestID); err != nil {
		return err
	}
	if err := contextErr(ctx); err != nil {
		return err
	}
	reg, ok := a.sidecars[sidecar]
	if !ok {
		return fmt.Errorf("%w: sidecar", ErrDenied)
	}
	return reg.client.Cancel(ctx, requestID)
}

func (a *Adapter) Resume(ctx context.Context, req Request) (SidecarResult, error) {
	if err := req.validate(); err != nil {
		return SidecarResult{}, err
	}
	reg, ok := a.sidecars[req.Sidecar]
	if !ok || !supports(reg.spec.Kinds, req.Kind) {
		return SidecarResult{}, fmt.Errorf("%w: sidecar or media kind not allowlisted", ErrDenied)
	}
	if err := contextErr(ctx); err != nil {
		return SidecarResult{}, err
	}
	started := a.now()
	result, err := reg.client.Resume(ctx, req.RequestID)
	if err != nil {
		return SidecarResult{}, err
	}
	if result.State == StateCompleted {
		if err := verifyArtifacts(req, result.Artifacts, started, a.now()); err != nil {
			return SidecarResult{}, err
		}
	}
	return result, nil
}

func (s SidecarSpec) valid() bool {
	if validateID(s.Name) != nil || s.Version == "" || strings.TrimSpace(s.Version) != s.Version || strings.ContainsAny(s.Version, "/\\\\\x00\r\n\t") || len(s.Kinds) == 0 {
		return false
	}
	if len(s.Digest) != 64 {
		return false
	}
	if _, err := hex.DecodeString(s.Digest); err != nil {
		return false
	}
	seen := make(map[MediaKind]struct{}, len(s.Kinds))
	for _, k := range s.Kinds {
		switch k {
		case KindImage, KindVideo, KindWorkflow:
		default:
			return false
		}
		if _, ok := seen[k]; ok {
			return false
		}
		seen[k] = struct{}{}
	}
	return true
}

func (r Request) validate() error {
	if err := validateID(r.RequestID); err != nil {
		return err
	}
	if err := validateID(r.IdempotencyKey); err != nil {
		return err
	}
	if r.Sidecar == "" || strings.TrimSpace(r.Sidecar) != r.Sidecar {
		return fmt.Errorf("%w: sidecar", ErrInvalidRequest)
	}
	if len(r.Prompt) > 32*1024 {
		return fmt.Errorf("%w: prompt too large", ErrInvalidRequest)
	}
	if !r.WorkspaceClean {
		return fmt.Errorf("%w: dirty worktree", ErrDenied)
	}
	switch r.Kind {
	case KindImage, KindVideo, KindWorkflow:
	default:
		return fmt.Errorf("%w: unsupported media kind", ErrInvalidRequest)
	}
	for _, id := range r.InputArtifacts {
		if err := validateID(id); err != nil {
			return err
		}
	}
	return nil
}

func verifyArtifacts(req Request, artifacts []Artifact, started, observed time.Time) error {
	if len(artifacts) == 0 {
		return ErrMisleadingSuccess
	}
	for _, artifact := range artifacts {
		if err := validateID(artifact.ID); err != nil {
			return err
		}
		if artifact.RequestID != req.RequestID || !validArtifactURI(artifact.URI) || !validDigest(artifact.SHA256) || artifact.CreatedAt.Before(started.Add(-time.Second)) || artifact.CreatedAt.After(observed.Add(5*time.Second)) {
			return ErrStaleArtifact
		}
	}
	return nil
}

func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
func validArtifactURI(value string) bool {
	return strings.HasPrefix(value, "sidecar://") && len(value) > len("sidecar://")
}

func validateID(value string) error {
	if value == "" || len(value) > 128 || strings.TrimSpace(value) != value {
		return fmt.Errorf("%w: canonical id", ErrInvalidRequest)
	}
	for _, c := range value {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || strings.ContainsRune("-_.:", c) {
			continue
		}
		return fmt.Errorf("%w: id characters", ErrInvalidRequest)
	}
	return nil
}
func supports(kinds []MediaKind, kind MediaKind) bool {
	for _, candidate := range kinds {
		if candidate == kind {
			return true
		}
	}
	return false
}
func contextErr(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
