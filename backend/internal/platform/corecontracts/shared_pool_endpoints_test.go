package corecontracts

import (
	"net/http"
	"strings"
	"testing"
)

func TestSharedPoolEndpointContract(t *testing.T) {
	tests := []struct {
		method, path string
		want         SharedPoolEndpointKind
	}{
		{http.MethodPost, "/v1/chat/completions", SharedPoolEndpointCanonical},
		{http.MethodPost, "/chat/completions", SharedPoolEndpointCanonical},
		{http.MethodPost, "/v1/responses", SharedPoolEndpointCanonical},
		{http.MethodPost, "/responses", SharedPoolEndpointCanonical},
		{http.MethodPost, "/backend-api/codex/responses", SharedPoolEndpointCanonical},
		{http.MethodPost, "/v1/responses/compact", SharedPoolEndpointCanonical},
		{http.MethodPost, "/responses/compact", SharedPoolEndpointCanonical},
		{http.MethodPost, "/backend-api/codex/responses/compact", SharedPoolEndpointCanonical},
		{http.MethodPost, "/v1/images/generations", SharedPoolEndpointCanonical},
		{http.MethodPost, "/images/generations", SharedPoolEndpointCanonical},
		{http.MethodPost, "/v1/images/edits", SharedPoolEndpointCanonical},
		{http.MethodPost, "/images/edits", SharedPoolEndpointCanonical},
		{http.MethodPost, "/v1/videos/generations", SharedPoolEndpointCanonical},
		{http.MethodPost, "/videos/generations", SharedPoolEndpointCanonical},
		{http.MethodGet, "/v1/videos/task-1", SharedPoolEndpointCanonical},
		{http.MethodGet, "/videos/task-1", SharedPoolEndpointCanonical},
		{http.MethodGet, "/v1/models", SharedPoolEndpointIntrospection},
		{http.MethodGet, "/models", SharedPoolEndpointIntrospection},
		{http.MethodGet, "/backend-api/codex/models", SharedPoolEndpointIntrospection},
		{http.MethodGet, "/v1/usage", SharedPoolEndpointIntrospection},
		{http.MethodGet, "/v1/sub2api/billing", SharedPoolEndpointIntrospection},
		{http.MethodPost, "/v1/messages", SharedPoolEndpointUnavailable},
		{http.MethodPost, "/v1/embeddings", SharedPoolEndpointUnavailable},
		{http.MethodPost, "/v1beta/models/a:generateContent", SharedPoolEndpointUnavailable},
		{http.MethodGet, "/v1/responses", SharedPoolEndpointUnavailable},
		{http.MethodPost, "/v1/responses/", SharedPoolEndpointUnavailable},
		{http.MethodPost, "/v1/responses/compact/other", SharedPoolEndpointUnavailable},
		{http.MethodPost, "/other/responses", SharedPoolEndpointUnavailable},
		{http.MethodDelete, "/v1/videos/task-1", SharedPoolEndpointUnavailable},
		{http.MethodGet, "/v1/images/tasks/task-1", SharedPoolEndpointUnavailable},
		{http.MethodGet, "/v1/videos/task-1/content", SharedPoolEndpointUnavailable},
		{http.MethodGet, "/v1/videos/", SharedPoolEndpointUnavailable},
		{http.MethodGet, "/v1/videos/..", SharedPoolEndpointUnavailable},
		{http.MethodGet, "/v1/videos/a\\b", SharedPoolEndpointUnavailable},
		{http.MethodGet, "/v1/videos/a\nb", SharedPoolEndpointUnavailable},
		{http.MethodGet, "/v1/videos/" + strings.Repeat("a", 513), SharedPoolEndpointUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			if got := ClassifySharedPoolEndpoint(tt.method, tt.path); got != tt.want {
				t.Fatalf("got endpoint kind %d, want %d", got, tt.want)
			}
		})
	}
}
