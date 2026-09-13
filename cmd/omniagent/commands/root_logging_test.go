package commands

import (
	"log"
	"log/slog"
	"testing"
	"time"
)

// TestRootLoggerWiringDoesNotDeadlock guards against the slog/log bridge
// cycle fixed under RMI-OMNIAGENT-204: wrapping slog.Default().Handler()
// (the stdlib bridge into the legacy log package) while slog.SetDefault
// rewires that same log package back into the wrapper self-deadlocks on
// log.Logger's non-reentrant mutex at the first emitted record. The
// deadlock froze every CLI run silently before its first log line, which
// is exactly why no unit test caught it — so this test runs the real
// PersistentPreRunE wiring and requires that both slog and legacy log
// emission complete within a timeout instead of hanging forever.
func TestRootLoggerWiringDoesNotDeadlock(t *testing.T) {
	prevLogger := slog.Default()
	prevCfg, prevCfgFile := cfg, cfgFile
	t.Cleanup(func() {
		slog.SetDefault(prevLogger)
		cfg, cfgFile = prevCfg, prevCfgFile
	})
	cfgFile = "" // load pure defaults; no config file needed

	if err := rootCmd.PersistentPreRunE(rootCmd, nil); err != nil {
		t.Fatalf("PersistentPreRunE: %v", err)
	}

	done := make(chan struct{})
	go func() {
		// Both paths must terminate: the slog default (through the redact
		// wrapper) and the legacy log package (which slog.SetDefault
		// rewired into the same handler — the edge that formed the cycle).
		slog.Info("deadlock guard: slog emission")
		log.Print("deadlock guard: legacy log emission")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("logging deadlocked: slog/log bridge cycle is back (see RMI-OMNIAGENT-204 fix in root.go)")
	}
}
