package middleware

import (
	"net/http"
	"testing"
)

func TestModelCatalogReadOnlyExemption(t *testing.T) {
	for _, tc := range []struct {
		method string
		path   string
		want   bool
	}{
		{http.MethodGet, "/v1/models", true},
		{http.MethodGet, "/models", true},
		{http.MethodPost, "/v1/models", false},
		{http.MethodPost, "/v1/chat/completions", false},
		{http.MethodPost, "/v1/images/generations", false},
		{http.MethodGet, "/v1/models/anything", false},
	} {
		if got := isModelCatalogRead(tc.method, tc.path); got != tc.want {
			t.Errorf("%s %s: got %v, want %v", tc.method, tc.path, got, tc.want)
		}
	}
}
