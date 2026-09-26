package service

import (
	"context"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAssetWASIBounds(t *testing.T) {
	_, err := ExecuteAssetWASM(context.Background(), []byte("not wasm"), "input")
	require.ErrorIs(t, err, ErrCapabilityAssetPackageInvalid)
	_, err = ExecuteAssetWASM(context.Background(), []byte{1}, strings.Repeat("x", assetPluginOutputLimit+1))
	require.ErrorIs(t, err, ErrCapabilityAssetPackageInvalid)
	out := &assetPluginOutput{}
	_, err = out.Write(make([]byte, assetPluginOutputLimit))
	require.NoError(t, err)
	_, err = out.Write([]byte{1})
	require.Error(t, err)
	require.True(t, out.exceeded)
	require.Equal(t, assetPluginOutputLimit, out.Len())
}

func TestAssetWASIStartAndCancellation(t *testing.T) {
	// (module (func (export "_start"))), then the same entry with an infinite loop.
	module, err := hex.DecodeString("0061736d0100000001040160000003020100070a01065f737461727400000a040102000b")
	require.NoError(t, err)
	result, err := ExecuteAssetWASM(context.Background(), module, "")
	require.NoError(t, err)
	require.Empty(t, result)
	loop, err := hex.DecodeString("0061736d0100000001040160000003020100070a01065f737461727400000a0901070003400c000b0b")
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = ExecuteAssetWASM(ctx, loop, "")
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestAssetWASIStdoutAndMemoryIsolation(t *testing.T) {
	// Minimal WASI module: one fd_write import, one memory, and "_start".
	section := func(id byte, body []byte) []byte {
		require.Less(t, len(body), 128)
		return append([]byte{id, byte(len(body))}, body...)
	}
	text := func(s string) []byte { return append([]byte{byte(len(s))}, []byte(s)...) }
	module := []byte{0, 97, 115, 109, 1, 0, 0, 0}
	module = append(module, section(1, []byte{2, 0x60, 4, 0x7f, 0x7f, 0x7f, 0x7f, 1, 0x7f, 0x60, 0, 0})...)
	imports := append([]byte{1}, text("wasi_snapshot_preview1")...)
	imports = append(imports, text("fd_write")...)
	imports = append(imports, 0, 0)
	module = append(module, section(2, imports)...)
	module = append(module, section(3, []byte{1, 1})...)
	module = append(module, section(5, []byte{1, 0, 1})...)
	exports := append([]byte{2}, text("memory")...)
	exports = append(exports, 2, 0)
	exports = append(exports, text("_start")...)
	exports = append(exports, 0, 1)
	module = append(module, section(7, exports)...)
	body := []byte{0, 0x41, 1, 0x41, 0, 0x41, 1, 0x41, 20, 0x10, 0, 0x1a, 0x0b}
	module = append(module, section(10, append([]byte{1, byte(len(body))}, body...))...)
	data := []byte{8, 0, 0, 0, 2, 0, 0, 0, 'o', 'k'}
	module = append(module, section(11, append([]byte{1, 0, 0x41, 0, 0x0b, byte(len(data))}, data...))...)
	output, err := ExecuteAssetWASM(context.Background(), module, "private input")
	require.NoError(t, err)
	require.Equal(t, "ok", output)

	tooLarge := []byte{0, 97, 115, 109, 1, 0, 0, 0}
	tooLarge = append(tooLarge, section(5, []byte{1, 0, 0x81, 0x08})...)
	_, err = ExecuteAssetWASM(context.Background(), tooLarge, "")
	require.ErrorIs(t, err, ErrCapabilityAssetPackageInvalid)
}
