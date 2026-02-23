package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfig_HasCapability(t *testing.T) {
	tests := []struct {
		name         string
		capabilities []Capability
		check        Capability
		want         bool
	}{
		{
			name:         "has capability",
			capabilities: []Capability{CapFSRead, CapNetHTTP},
			check:        CapFSRead,
			want:         true,
		},
		{
			name:         "missing capability",
			capabilities: []Capability{CapFSRead},
			check:        CapFSWrite,
			want:         false,
		},
		{
			name:         "empty capabilities",
			capabilities: []Capability{},
			check:        CapFSRead,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{Capabilities: tt.capabilities}
			if got := cfg.HasCapability(tt.check); got != tt.want {
				t.Errorf("HasCapability() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHostFunctions_FSRead(t *testing.T) {
	// Create temp directory and file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("hello world")
	if err := os.WriteFile(testFile, testContent, 0600); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	t.Run("without capability", func(t *testing.T) {
		h := NewHostFunctions(Config{})
		_, err := h.FSRead(ctx, testFile)
		if err == nil {
			t.Error("expected error without capability")
		}
		execErr, ok := err.(*ExecutionError)
		if !ok || execErr.Kind != "capability" {
			t.Errorf("expected capability error, got %v", err)
		}
	})

	t.Run("with capability", func(t *testing.T) {
		h := NewHostFunctions(Config{
			Capabilities: []Capability{CapFSRead},
			AllowedPaths: []string{tmpDir},
		})
		data, err := h.FSRead(ctx, testFile)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if string(data) != string(testContent) {
			t.Errorf("got %q, want %q", data, testContent)
		}
	})

	t.Run("path outside allowed", func(t *testing.T) {
		h := NewHostFunctions(Config{
			Capabilities: []Capability{CapFSRead},
			AllowedPaths: []string{"/nonexistent"},
		})
		_, err := h.FSRead(ctx, testFile)
		if err == nil {
			t.Error("expected error for path outside allowed")
		}
	})
}

func TestHostFunctions_FSWrite(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "output.txt")
	testContent := []byte("written content")

	ctx := context.Background()

	t.Run("without capability", func(t *testing.T) {
		h := NewHostFunctions(Config{})
		err := h.FSWrite(ctx, testFile, testContent)
		if err == nil {
			t.Error("expected error without capability")
		}
	})

	t.Run("with capability", func(t *testing.T) {
		h := NewHostFunctions(Config{
			Capabilities: []Capability{CapFSWrite},
			AllowedPaths: []string{tmpDir},
		})
		err := h.FSWrite(ctx, testFile, testContent)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Verify file was written
		data, err := os.ReadFile(testFile)
		if err != nil {
			t.Errorf("failed to read written file: %v", err)
		}
		if string(data) != string(testContent) {
			t.Errorf("got %q, want %q", data, testContent)
		}
	})
}

func TestHostFunctions_ExecRun(t *testing.T) {
	ctx := context.Background()

	t.Run("without capability", func(t *testing.T) {
		h := NewHostFunctions(Config{})
		_, _, _, err := h.ExecRun(ctx, "echo", []string{"hello"})
		if err == nil {
			t.Error("expected error without capability")
		}
	})

	t.Run("command not in allowed list", func(t *testing.T) {
		h := NewHostFunctions(Config{
			Capabilities:    []Capability{CapExecRun},
			AllowedCommands: []string{"ls"},
		})
		_, _, _, err := h.ExecRun(ctx, "echo", []string{"hello"})
		if err == nil {
			t.Error("expected error for command not in allowed list")
		}
	})

	t.Run("with capability and allowed command", func(t *testing.T) {
		h := NewHostFunctions(Config{
			Capabilities:    []Capability{CapExecRun},
			AllowedCommands: []string{"echo"},
			MaxOutputBytes:  1024,
		})
		stdout, stderr, exitCode, err := h.ExecRun(ctx, "echo", []string{"hello"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if exitCode != 0 {
			t.Errorf("exit code = %d, want 0", exitCode)
		}
		if string(stdout) != "hello\n" {
			t.Errorf("stdout = %q, want %q", stdout, "hello\n")
		}
		if len(stderr) != 0 {
			t.Errorf("stderr = %q, want empty", stderr)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		h := NewHostFunctions(Config{
			Capabilities:    []Capability{CapExecRun},
			AllowedCommands: []string{"sleep"},
			Timeout:         50 * time.Millisecond,
		})
		timeoutCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		_, _, exitCode, err := h.ExecRun(timeoutCtx, "sleep", []string{"5"})
		// Either we get an error or the process was killed (exit code -1)
		if err == nil && exitCode == 0 {
			t.Error("expected timeout error or non-zero exit code")
		}
	})
}

func TestHostFunctions_HTTPFetch(t *testing.T) {
	ctx := context.Background()

	t.Run("without capability", func(t *testing.T) {
		h := NewHostFunctions(Config{})
		_, _, err := h.HTTPFetch(ctx, "GET", "https://example.com", nil, nil)
		if err == nil {
			t.Error("expected error without capability")
		}
	})

	t.Run("host not in allowed list", func(t *testing.T) {
		h := NewHostFunctions(Config{
			Capabilities: []Capability{CapNetHTTP},
			AllowedHosts: []string{"api.example.com"},
		})
		_, _, err := h.HTTPFetch(ctx, "GET", "https://other.com", nil, nil)
		if err == nil {
			t.Error("expected error for host not in allowed list")
		}
	})
}

func TestRuntime_Basic(t *testing.T) {
	ctx := context.Background()

	cfg := DefaultConfig()
	cfg.MemoryLimitMB = 8

	runtime, err := NewRuntime(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to create runtime: %v", err)
	}
	defer runtime.Close(ctx)

	// Test that runtime can be closed without error
	if err := runtime.Close(ctx); err != nil {
		t.Errorf("close error: %v", err)
	}
}

func TestExecutionError(t *testing.T) {
	err := NewCapabilityError(CapFSRead, "read_file")
	if err.Kind != "capability" {
		t.Errorf("Kind = %q, want %q", err.Kind, "capability")
	}
	if err.Error() == "" {
		t.Error("Error() returned empty string")
	}

	timeoutErr := NewTimeoutError(5 * time.Second)
	if timeoutErr.Kind != "timeout" {
		t.Errorf("Kind = %q, want %q", timeoutErr.Kind, "timeout")
	}
	if timeoutErr.Unwrap() != context.DeadlineExceeded {
		t.Error("Unwrap() should return DeadlineExceeded")
	}
}
