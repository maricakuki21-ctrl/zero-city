package workbench

import "context"

type CanonicalAdapter interface {
	Workspace(ctx context.Context, identity Identity) (Workspace, error)
	Launch(ctx context.Context, command LaunchCommand) (LaunchResult, error)
	Run(ctx context.Context, command RunCommand) (RunResult, error)
	Cancel(ctx context.Context, command CancelCommand) (CancelResult, error)
	Save(ctx context.Context, command SaveCommand) (SaveResult, error)
	Replay(ctx context.Context, command ReplayCommand) (ReplayResult, error)
	Fork(ctx context.Context, command ForkCommand) (ForkResult, error)
	Artifact(ctx context.Context, command ArtifactCommand) (Artifact, error)
	Events(ctx context.Context, command EventsCommand) (EventStreamResult, error)
}

type UnavailableAdapter struct{}

func (UnavailableAdapter) Workspace(context.Context, Identity) (Workspace, error) {
	return Workspace{}, ErrCanonicalRuntimeUnavailable
}

func (UnavailableAdapter) Launch(context.Context, LaunchCommand) (LaunchResult, error) {
	return LaunchResult{}, ErrCanonicalRuntimeUnavailable
}

func (UnavailableAdapter) Run(context.Context, RunCommand) (RunResult, error) {
	return RunResult{}, ErrCanonicalRuntimeUnavailable
}

func (UnavailableAdapter) Cancel(context.Context, CancelCommand) (CancelResult, error) {
	return CancelResult{}, ErrCanonicalRuntimeUnavailable
}

func (UnavailableAdapter) Save(context.Context, SaveCommand) (SaveResult, error) {
	return SaveResult{}, ErrCanonicalRuntimeUnavailable
}

func (UnavailableAdapter) Replay(context.Context, ReplayCommand) (ReplayResult, error) {
	return ReplayResult{}, ErrCanonicalRuntimeUnavailable
}

func (UnavailableAdapter) Fork(context.Context, ForkCommand) (ForkResult, error) {
	return ForkResult{}, ErrCanonicalRuntimeUnavailable
}

func (UnavailableAdapter) Artifact(context.Context, ArtifactCommand) (Artifact, error) {
	return Artifact{}, ErrCanonicalRuntimeUnavailable
}

func (UnavailableAdapter) Events(context.Context, EventsCommand) (EventStreamResult, error) {
	return EventStreamResult{}, ErrCanonicalRuntimeUnavailable
}

var _ CanonicalAdapter = UnavailableAdapter{}
