// Package sandbox provides a WASM-based sandbox for secure code execution.
package sandbox

import (
	"context"
	"fmt"
	"slices"
	"time"
)

// Capability represents a permission that can be granted to sandboxed code.
type Capability string

const (
	// CapFSRead allows reading files from the filesystem.
	CapFSRead Capability = "fs_read"
	// CapFSWrite allows writing files to the filesystem.
	CapFSWrite Capability = "fs_write"
	// CapNetHTTP allows making HTTP requests.
	CapNetHTTP Capability = "net_http"
	// CapExecRun allows executing shell commands.
	CapExecRun Capability = "exec_run"
)

// Config configures a sandbox instance.
type Config struct {
	// Capabilities granted to the sandboxed code.
	Capabilities []Capability

	// MemoryLimitMB is the maximum memory in megabytes (default: 16).
	MemoryLimitMB int

	// FuelLimit is the maximum number of instructions (0 = unlimited).
	FuelLimit uint64

	// Timeout is the maximum execution time.
	Timeout time.Duration

	// WorkingDir is the working directory for file operations.
	WorkingDir string

	// AllowedPaths restricts file access to these paths (empty = WorkingDir only).
	AllowedPaths []string

	// AllowedHosts restricts HTTP access to these hosts (empty = all allowed).
	AllowedHosts []string

	// AllowedCommands restricts exec to these commands (empty = none allowed).
	AllowedCommands []string

	// MaxOutputBytes limits the output size (default: 1MB).
	MaxOutputBytes int
}

// DefaultConfig returns a restrictive default configuration.
func DefaultConfig() Config {
	return Config{
		Capabilities:   []Capability{},
		MemoryLimitMB:  16,
		FuelLimit:      0, // Unlimited by default, use timeout instead
		Timeout:        30 * time.Second,
		MaxOutputBytes: 1024 * 1024, // 1MB
	}
}

// HasCapability checks if a capability is granted.
func (c *Config) HasCapability(cap Capability) bool {
	return slices.Contains(c.Capabilities, cap)
}

// Result represents the result of a sandboxed execution.
type Result struct {
	// Output is the stdout from the execution.
	Output []byte

	// Error is the stderr from the execution.
	Error []byte

	// ExitCode is the exit code (0 = success).
	ExitCode int

	// Duration is how long the execution took.
	Duration time.Duration

	// MemoryUsed is the peak memory usage in bytes.
	MemoryUsed uint64

	// FuelConsumed is the number of instructions executed.
	FuelConsumed uint64
}

// ExecutionError represents an error during sandboxed execution.
type ExecutionError struct {
	Kind    string // "timeout", "memory", "capability", "runtime"
	Message string
	Cause   error
}

func (e *ExecutionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Kind, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}

func (e *ExecutionError) Unwrap() error {
	return e.Cause
}

// NewCapabilityError creates a capability violation error.
func NewCapabilityError(cap Capability, operation string) *ExecutionError {
	return &ExecutionError{
		Kind:    "capability",
		Message: fmt.Sprintf("operation %q requires capability %q", operation, cap),
	}
}

// NewTimeoutError creates a timeout error.
func NewTimeoutError(timeout time.Duration) *ExecutionError {
	return &ExecutionError{
		Kind:    "timeout",
		Message: fmt.Sprintf("execution exceeded timeout of %v", timeout),
		Cause:   context.DeadlineExceeded,
	}
}

// NewMemoryError creates a memory limit error.
func NewMemoryError(limit, used uint64) *ExecutionError {
	return &ExecutionError{
		Kind:    "memory",
		Message: fmt.Sprintf("memory limit exceeded: %d bytes used, %d bytes allowed", used, limit),
	}
}
