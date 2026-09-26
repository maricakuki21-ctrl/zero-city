package service

import (
	"errors"
	"net/url"

	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyurl"
)

var errInvalidSharedPoolProxy = errors.New("invalid shared pool proxy configuration")

func parseSharedPoolProxyURL(raw string) (string, *url.URL, error) {
	normalized, parsed, err := proxyurl.Parse(raw)
	if err != nil {
		return "", nil, errInvalidSharedPoolProxy
	}
	return normalized, parsed, nil
}

func newSharedPoolRuntimeProxy(raw string, proxyID int64) (string, *int64, *Proxy, error) {
	normalized, parsed, err := parseSharedPoolProxyURL(raw)
	if err != nil {
		return "", nil, nil, err
	}
	if parsed == nil {
		return "", nil, nil, nil
	}

	id := proxyID
	return normalized, &id, &Proxy{
		ID:         id,
		Name:       "shared-pool-runtime-proxy",
		Status:     StatusActive,
		runtimeURL: normalized,
	}, nil
}
