package workbench

import (
	"reflect"
	"testing"
)

func TestContract_LaunchCommandCarriesOnlyCanonicalWorkbenchInputs(t *testing.T) {
	t.Parallel()

	got := fieldNames(reflect.TypeOf(LaunchCommand{}))
	want := []string{
		"AcceptedQuoteID", "AcceptedQuoteSHA", "CanonicalModelID", "CanonicalModelVersion",
		"CapabilityDigest", "CapabilityID", "CapabilityVersion", "IdempotencyKey", "Identity", "Intent",
		"RequiredProtocolFamily",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LaunchCommand fields = %v, want %v", got, want)
	}
}

func TestContract_ArtifactCarriesImmutableCanonicalLineage(t *testing.T) {
	t.Parallel()

	got := fieldNames(reflect.TypeOf(Artifact{}))
	want := []string{
		"AcceptedQuoteID", "AcceptedQuoteSHA", "AdapterDigest", "ArtifactDigest", "ArtifactID", "Body", "ByteSize",
		"CanonicalMediaBusinessEventID", "CanonicalRequestID", "CanonicalUsageEventID", "CapabilityDigest", "ContentType",
		"CreatedAt", "JournalID", "Kind", "MediaTaskID", "OwnerID", "RunID", "RunnerJobID", "StorageURI", "UpstreamTaskID",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Artifact fields = %v, want %v", got, want)
	}
}

func TestContract_RunCommandsCarryCanonicalReferencesNotRuntimeControls(t *testing.T) {
	t.Parallel()

	if got, want := fieldNames(reflect.TypeOf(RunCommand{})), []string{"IdempotencyKey", "Identity", "RunID"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("RunCommand fields = %v, want %v", got, want)
	}
	if got, want := fieldNames(reflect.TypeOf(ReplayCommand{})), []string{"IdempotencyKey", "Identity", "ReplayID"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ReplayCommand fields = %v, want %v", got, want)
	}
}

func fieldNames(value reflect.Type) []string {
	names := make([]string, 0, value.NumField())
	for index := range value.NumField() {
		names = append(names, value.Field(index).Name)
	}
	for left := range names {
		for right := left + 1; right < len(names); right++ {
			if names[right] < names[left] {
				names[left], names[right] = names[right], names[left]
			}
		}
	}
	return names
}
