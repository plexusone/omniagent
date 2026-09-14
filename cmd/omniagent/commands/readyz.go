package commands

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/grokify/mogo/net/http/healthz"
	"github.com/plexusone/omnistorage-core/kvs"

	"github.com/plexusone/omniagent/config"
)

// readyzKey is a reserved, self-expiring key used only to prove the
// configured storage backend actually accepts a write — never read by
// application code.
const readyzKey = "__omniagent_readyz_probe__"

// newReadinessHandler builds /readyz (RMI-OMNIAGENT-036): distinguishes
// "process up" (liveness, /health — see gateway.handleHealth) from
// "actually able to serve." Deliberately conservative about what counts
// as a checker here:
//
//   - storage: a real Set/Get/Delete round-trip against the configured
//     backend — the one check in this set that reflects live runtime
//     state (a backend that was fine at startup can still fail later:
//     a corrupted SQLite file, a dropped Redis connection).
//   - agent_configured: a static check that an agent was constructed at
//     startup with a non-empty API key. This does NOT make a live LLM
//     API call — readiness is typically polled every few seconds, and a
//     billed request per poll would be wasteful for a signal that's
//     effectively constant once the process is up.
//
// Channel-connection state (Discord/Telegram/etc. actually connected
// right now) is intentionally not a checker here: omnichat's Provider
// interface exposes Connect/Disconnect but no live connected-state query,
// so a "channel connected" check would have to be faked from
// ConnectAll's one-time startup result. Adding Provider.Connected() bool
// to omnichat is the honest way to close this gap — a natural follow-up,
// not something to paper over here.
func newReadinessHandler(cfg *config.Config, storageBackend kvs.Store, agentConfigured bool) http.HandlerFunc {
	checks := []healthz.Checker{
		healthz.CheckerFunc{CheckerName: "storage", Fn: func(ctx context.Context) error {
			return storageRoundTrip(ctx, storageBackend)
		}},
	}
	if agentConfigured {
		checks = append(checks, healthz.CheckerFunc{CheckerName: "agent_configured", Fn: func(context.Context) error {
			if cfg.Agent.APIKey == "" {
				return fmt.Errorf("no API key configured for provider %q", cfg.Agent.Provider)
			}
			return nil
		}})
	}
	return healthz.ReadinessHandler(checks...)
}

// storageRoundTrip proves the backend accepts writes, not just reads: Set
// a throwaway key with a short self-expiring TTL, read it back, then
// delete it immediately rather than waiting on the TTL.
func storageRoundTrip(ctx context.Context, store kvs.Store) error {
	if store == nil {
		return fmt.Errorf("no storage backend configured")
	}
	const val = "ok"
	if err := store.Set(ctx, readyzKey, []byte(val), 10*time.Second); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	got, err := store.Get(ctx, readyzKey)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	if string(got) != val {
		return fmt.Errorf("read back %q, want %q", got, val)
	}
	if err := store.Delete(ctx, readyzKey); err != nil {
		return fmt.Errorf("cleanup: %w", err)
	}
	return nil
}
