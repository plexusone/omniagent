package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// Runtime manages WASM module execution with sandboxing.
type Runtime struct {
	runtime wazero.Runtime
	config  Config
	mu      sync.Mutex

	// Compiled module cache
	modules map[string]wazero.CompiledModule
}

// NewRuntime creates a new sandbox runtime.
func NewRuntime(ctx context.Context, config Config) (*Runtime, error) {
	// Build runtime configuration
	runtimeConfig := wazero.NewRuntimeConfig()

	// Set memory limits (pages are 64KB each)
	if config.MemoryLimitMB > 0 && config.MemoryLimitMB <= 4096 { // Max 4GB
		pages := uint32(config.MemoryLimitMB) * 16 //nolint:gosec // G115: Bounded by check above
		runtimeConfig = runtimeConfig.WithMemoryLimitPages(pages)
	}

	// Enable close on context done for timeout support
	runtimeConfig = runtimeConfig.WithCloseOnContextDone(true)

	// Create the runtime
	r := wazero.NewRuntimeWithConfig(ctx, runtimeConfig)

	// Instantiate WASI for standard I/O support
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		r.Close(ctx)
		return nil, fmt.Errorf("instantiate WASI: %w", err)
	}

	return &Runtime{
		runtime: r,
		config:  config,
		modules: make(map[string]wazero.CompiledModule),
	}, nil
}

// Close releases all resources.
func (r *Runtime) Close(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.runtime.Close(ctx)
}

// Compile compiles a WASM module and caches it.
func (r *Runtime) Compile(ctx context.Context, name string, wasm []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	compiled, err := r.runtime.CompileModule(ctx, wasm)
	if err != nil {
		return fmt.Errorf("compile module: %w", err)
	}

	r.modules[name] = compiled
	return nil
}

// Execute runs a compiled WASM module with the given input.
func (r *Runtime) Execute(ctx context.Context, name string, stdin []byte) (*Result, error) {
	r.mu.Lock()
	compiled, ok := r.modules[name]
	r.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("module not found: %s", name)
	}

	return r.executeModule(ctx, compiled, stdin)
}

// ExecuteBytes compiles and runs WASM bytes directly (not cached).
func (r *Runtime) ExecuteBytes(ctx context.Context, wasm, stdin []byte) (*Result, error) {
	compiled, err := r.runtime.CompileModule(ctx, wasm)
	if err != nil {
		return nil, fmt.Errorf("compile module: %w", err)
	}
	defer compiled.Close(ctx)

	return r.executeModule(ctx, compiled, stdin)
}

func (r *Runtime) executeModule(ctx context.Context, compiled wazero.CompiledModule, stdin []byte) (*Result, error) {
	start := time.Now()

	// Apply timeout
	if r.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.config.Timeout)
		defer cancel()
	}

	// Setup I/O buffers
	stdinBuf := bytes.NewReader(stdin)
	stdoutBuf := &limitedBuffer{max: r.config.MaxOutputBytes}
	stderrBuf := &limitedBuffer{max: r.config.MaxOutputBytes}

	// Configure the module
	moduleConfig := wazero.NewModuleConfig().
		WithStdin(stdinBuf).
		WithStdout(stdoutBuf).
		WithStderr(stderrBuf).
		WithStartFunctions("_start")

	// Instantiate and run
	mod, err := r.runtime.InstantiateModule(ctx, compiled, moduleConfig)
	if err != nil {
		// Check if it was a timeout
		if ctx.Err() == context.DeadlineExceeded {
			return nil, NewTimeoutError(r.config.Timeout)
		}
		return nil, &ExecutionError{
			Kind:    "runtime",
			Message: "module execution failed",
			Cause:   err,
		}
	}
	defer mod.Close(ctx)

	duration := time.Since(start)

	// Get memory stats if available
	var memUsed uint64
	if mem := mod.Memory(); mem != nil {
		memUsed = uint64(mem.Size())
	}

	return &Result{
		Output:       stdoutBuf.Bytes(),
		Error:        stderrBuf.Bytes(),
		ExitCode:     0,
		Duration:     duration,
		MemoryUsed:   memUsed,
		FuelConsumed: 0, // Would need fuel metering enabled
	}, nil
}

// RegisterHostModule registers a host module with functions that WASM modules can call.
func (r *Runtime) RegisterHostModule(ctx context.Context, moduleName string, builder func(wazero.HostModuleBuilder) wazero.HostModuleBuilder) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	hostBuilder := r.runtime.NewHostModuleBuilder(moduleName)
	hostBuilder = builder(hostBuilder)
	_, err := hostBuilder.Instantiate(ctx)
	return err
}

// limitedBuffer is a bytes.Buffer with a maximum size.
type limitedBuffer struct {
	buf bytes.Buffer
	max int
}

func (b *limitedBuffer) Write(p []byte) (n int, err error) {
	if b.max > 0 && b.buf.Len()+len(p) > b.max {
		// Truncate to fit
		remaining := b.max - b.buf.Len()
		if remaining > 0 {
			return b.buf.Write(p[:remaining])
		}
		return 0, nil // Silently discard
	}
	return b.buf.Write(p)
}

func (b *limitedBuffer) Bytes() []byte {
	return b.buf.Bytes()
}

func (b *limitedBuffer) String() string {
	return b.buf.String()
}
