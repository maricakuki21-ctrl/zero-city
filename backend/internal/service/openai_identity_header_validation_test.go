package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnforceCodexIdentityHeaders_RejectsUntrimmedControlBytes(t *testing.T) {
	for _, raw := range []string{
		"\ncodex_cli_rs/0.145.2 (Windows)",
		"codex_cli_rs/0.145.2 (Windows)\n",
		"codex_cli_rs/0.145.2 bad\x00",
	} {
		headers := http.Header{}
		headers.Set("originator", "codex_cli_rs")
		headers.Set("User-Agent", raw)
		enforceCodexIdentityHeaders(headers)
		require.Equal(t, codexCLIUserAgent, headers.Get("User-Agent"))
		require.Equal(t, "codex_cli_rs", headers.Get("originator"))
	}
}
