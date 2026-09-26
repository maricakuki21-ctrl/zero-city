package main

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeneratedWireConstructsConcreteWorkbenchRuntime(t *testing.T) {
	// Given
	content, err := os.ReadFile("wire_gen.go")
	require.NoError(t, err)
	generated := string(content)

	// Then
	require.NotContains(t, generated, "service.ProvideWorkbenchRuntimeAdapter()")
	require.Contains(t, generated, "service.ProvideWorkbenchRuntimeAdapter(")
	require.Contains(t, generated, "repository.ProvideWorkbenchRuntimeStore(")
	require.Contains(t, generated, "repository.ProvideWorkbenchCatalogSource(")
	require.Contains(t, generated, "repository.ProvideWorkbenchSettlementSource(")
	require.Contains(t, generated, "service.ProvideWorkbenchCanonicalBridge(")
	require.False(t, strings.Contains(generated, "unavailableWorkbench"), "production Wire must not construct UnavailableAdapter")
}
