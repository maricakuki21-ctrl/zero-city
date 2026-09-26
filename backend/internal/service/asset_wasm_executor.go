package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"
)

const assetPluginOutputLimit = 1 << 20

var assetPluginSlots = make(chan struct{}, 2)

// ExecuteAssetWASM runs a WASI command with text stdin/stdout. No filesystem,
// environment variables, network sockets or application credentials are exposed.
func ExecuteAssetWASM(ctx context.Context, module []byte, input string) (string, error) {
	if len(module) == 0 || len(module) > 8<<20 || len(input) > assetPluginOutputLimit {
		return "", fmt.Errorf("%w: plugin input exceeds limits", ErrCapabilityAssetPackageInvalid)
	}
	select {
	case assetPluginSlots <- struct{}{}:
		defer func() { <-assetPluginSlots }()
	default:
		return "", errors.New("plugin capacity reached; retry later")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	runtime := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigInterpreter().
		WithMemoryLimitPages(1024).WithCloseOnContextDone(true).WithDebugInfoEnabled(false))
	defer runtime.Close(context.Background())
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
		return "", fmt.Errorf("initialize plugin runtime: %w", err)
	}
	compiled, err := runtime.CompileModule(ctx, module)
	if err != nil {
		return "", fmt.Errorf("%w: invalid WASI module", ErrCapabilityAssetPackageInvalid)
	}
	defer compiled.Close(context.Background())
	if _, ok := compiled.ExportedFunctions()["_start"]; !ok {
		return "", fmt.Errorf("%w: plugin must export _start", ErrCapabilityAssetPackageInvalid)
	}
	output := &assetPluginOutput{}
	_, err = runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().
		WithStdin(strings.NewReader(input)).WithStdout(output).WithStderr(io.Discard))
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		var exit *sys.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 0 {
			return "", errors.New("plugin execution failed")
		}
	}
	if output.exceeded {
		return "", errors.New("plugin output exceeds 1 MiB")
	}
	if !utf8.Valid(output.Bytes()) {
		return "", errors.New("plugin output must be UTF-8 text")
	}
	return output.String(), nil
}

type assetPluginOutput struct {
	bytes.Buffer
	exceeded bool
}

func (w *assetPluginOutput) Write(p []byte) (int, error) {
	if len(p) > assetPluginOutputLimit-w.Len() {
		w.exceeded = true
		return 0, io.ErrShortWrite
	}
	return w.Buffer.Write(p)
}
