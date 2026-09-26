//go:build embed

package web

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFrontendAvailableFromExternalRelease(t *testing.T) {
	data := t.TempDir()
	t.Setenv("DATA_DIR", data)
	public := filepath.Join(data, "public")
	if err := os.MkdirAll(public, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(public, "index.html"), []byte(`<html><div id="app"></div></html>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasEmbeddedFrontend() {
		t.Fatal("external release frontend was not enabled")
	}
	if _, err := NewFrontendServer(nil); err != nil {
		t.Fatalf("external frontend cannot be served: %v", err)
	}
}
