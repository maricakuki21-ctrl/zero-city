package corecontracts

import (
	"net/http"
	"strings"
)

type SharedPoolEndpointKind uint8

const (
	SharedPoolEndpointUnavailable SharedPoolEndpointKind = iota
	SharedPoolEndpointIntrospection
	SharedPoolEndpointCanonical
)

// ClassifySharedPoolEndpoint is shared by authentication and canonical routing.
// A route is enabled only when its authorization and settlement path is ready;
// a registered native endpoint alone is not sufficient. Match exactly, without
// trimming or substring matching that could admit unverified subroutes.
func ClassifySharedPoolEndpoint(method, path string) SharedPoolEndpointKind {
	switch method {
	case http.MethodPost:
		switch path {
		case "/v1/chat/completions", "/chat/completions",
			"/v1/responses", "/responses", "/backend-api/codex/responses",
			"/v1/responses/compact", "/responses/compact", "/backend-api/codex/responses/compact",
			"/v1/images/generations", "/images/generations",
			"/v1/images/edits", "/images/edits",
			"/v1/videos/generations", "/videos/generations":
			return SharedPoolEndpointCanonical
		}
	case http.MethodGet:
		switch path {
		case "/v1/models", "/models", "/backend-api/codex/models",
			"/v1/usage", "/v1/sub2api/billing":
			return SharedPoolEndpointIntrospection
		}
		for _, prefix := range []string{"/v1/videos/", "/videos/"} {
			if !strings.HasPrefix(path, prefix) {
				continue
			}
			id := strings.TrimPrefix(path, prefix)
			if id != "" && len(id) <= 512 && id != "." && id != ".." &&
				!strings.ContainsAny(id, "/\\\r\n\t") {
				return SharedPoolEndpointCanonical
			}
		}
	}
	return SharedPoolEndpointUnavailable
}
