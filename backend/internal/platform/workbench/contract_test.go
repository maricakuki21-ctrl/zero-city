package workbench

import (
	"reflect"
	"testing"
)

func TestContract_CanonicalAdapterHasOnlyTypedReturns(t *testing.T) {
	t.Parallel()

	adapter := reflect.TypeOf((*CanonicalAdapter)(nil)).Elem()
	want := map[string][]reflect.Type{
		"Artifact":  {reflect.TypeOf(Artifact{})},
		"Cancel":    {reflect.TypeOf(CancelResult{})},
		"Events":    {reflect.TypeOf(EventStreamResult{})},
		"Fork":      {reflect.TypeOf(ForkResult{})},
		"Launch":    {reflect.TypeOf(LaunchResult{})},
		"Replay":    {reflect.TypeOf(ReplayResult{})},
		"Run":       {reflect.TypeOf(RunResult{})},
		"Save":      {reflect.TypeOf(SaveResult{})},
		"Workspace": {reflect.TypeOf(Workspace{})},
	}
	if adapter.NumMethod() != len(want) {
		t.Fatalf("CanonicalAdapter method count = %d, want %d", adapter.NumMethod(), len(want))
	}
	for name, resultTypes := range want {
		method, ok := adapter.MethodByName(name)
		if !ok {
			t.Fatalf("CanonicalAdapter.%s is missing", name)
		}
		if method.Type.NumOut() != 2 || method.Type.Out(0) != resultTypes[0] || method.Type.Out(1) != reflect.TypeOf((*error)(nil)).Elem() {
			t.Fatalf("CanonicalAdapter.%s returns %v, want (%v, error)", name, method.Type, resultTypes[0])
		}
	}
}

func TestContract_CommandResultsAreTyped(t *testing.T) {
	t.Parallel()

	values := []struct {
		name   string
		typeOf reflect.Type
	}{
		{name: "cancel_command", typeOf: reflect.TypeOf(CancelCommand{})},
		{name: "cancel_result", typeOf: reflect.TypeOf(CancelResult{})},
		{name: "fork_command", typeOf: reflect.TypeOf(ForkCommand{})},
		{name: "fork_result", typeOf: reflect.TypeOf(ForkResult{})},
		{name: "replay_command", typeOf: reflect.TypeOf(ReplayCommand{})},
		{name: "replay_result", typeOf: reflect.TypeOf(ReplayResult{})},
	}
	for _, value := range values {
		if value.typeOf.Kind() != reflect.Struct {
			t.Fatalf("%s must be a concrete struct", value.name)
		}
	}
}
