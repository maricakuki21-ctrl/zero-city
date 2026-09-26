//go:build embed

package web

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
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
	server, err := NewFrontendServer(&mockSettingsProvider{settings: map[string]string{"site_name": "Test"}})
	if err != nil {
		t.Fatalf("external frontend cannot be served: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(public, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(public, "assets", "release.js"), []byte("console.log('release');"), 0o644); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(server.Middleware())
	for path, expected := range map[string]string{"/login": `id="app"`, "/assets/release.js": "console.log('release');"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != 200 || !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("external route %s: status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}
