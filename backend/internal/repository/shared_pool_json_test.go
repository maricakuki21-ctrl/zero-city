package repository

import (
	"database/sql/driver"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

type jsonTextArg string

func (expected jsonTextArg) Match(value driver.Value) bool {
	text, ok := value.(string)
	return ok && json.Valid([]byte(text)) && (expected == "" || text == string(expected))
}

func TestSharedPoolJSONBTextUsesTextAndDefaultsEmptySnapshots(t *testing.T) {
	for _, raw := range []json.RawMessage{nil, {}, []byte(" \t\r\n ")} {
		got, err := sharedPoolJSONBText(raw, "snapshot")
		require.NoError(t, err)
		require.Equal(t, `{}`, got)
	}

	got, err := sharedPoolJSONBText(json.RawMessage(`  {"price_version_id":55}  `), "snapshot")
	require.NoError(t, err)
	require.Equal(t, `{"price_version_id":55}`, got)
}

func TestSharedPoolJSONBTextRejectsMalformedSnapshots(t *testing.T) {
	got, err := sharedPoolJSONBText(json.RawMessage(`{"price_version_id":`), "shared pool price snapshot")
	require.Empty(t, got)
	require.ErrorContains(t, err, "shared pool price snapshot must be valid JSON")
}

func TestSharedPoolJSONBEqualUsesJSONBSemantics(t *testing.T) {
	require.True(t, sharedPoolJSONBEqual(
		[]byte(`{"price_version_id":55,"multiplier":1,"nested":{"ok":true}}`),
		[]byte(` { "nested": { "ok": true }, "multiplier": 1.0, "price_version_id": 5.5e1 } `),
	))
	require.False(t, sharedPoolJSONBEqual(
		[]byte(`{"price_version_id":55,"multiplier":1}`),
		[]byte(`{"price_version_id":55,"multiplier":2}`),
	))
	require.False(t, sharedPoolJSONBEqual([]byte(`{}`), nil))
}
