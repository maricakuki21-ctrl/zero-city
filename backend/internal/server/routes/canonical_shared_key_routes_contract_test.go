package routes

import (
	"bytes"
	"os"
	"testing"
)

func TestSharedKeyResponsesAndCodexRoutesUseCanonicalGateway(t *testing.T) {
	source, err := os.ReadFile("../../handler/openai_shared_pool_chat.go")
	if err != nil {
		t.Fatalf("non-canonical path: read shared route handler: %v", err)
	}
	for _, forbidden := range [][]byte{
		[]byte("WithSharedPoolRouteExcluded("),
		[]byte("SharedPoolRouteFailoverCount("),
	} {
		if bytes.Contains(source, forbidden) {
			t.Fatalf("non-canonical path: route path still owns %q", forbidden)
		}
	}
}
