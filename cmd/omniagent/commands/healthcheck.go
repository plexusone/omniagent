package commands

import (
	"context"
	"os"
	"time"

	"github.com/grokify/mogo/net/http/healthz"
	"github.com/spf13/cobra"
)

// healthcheckCmd probes the running gateway's /health endpoint and exits
// 0/1. It exists for container HEALTHCHECK exec-form on shell-less
// runtime images (Chainguard static / distroless), where wget/curl are
// unavailable (RMI-OMNIAGENT-035). It deliberately loads no config and
// resolves no credentials — see the name check in root.go's
// PersistentPreRunE.
var healthcheckCmd = &cobra.Command{
	Use:   "healthcheck",
	Short: "Probe the running gateway's /health endpoint (for container health checks)",
	Args:  cobra.NoArgs,
	// A failing probe is an operational signal, not a usage error — keep
	// container logs free of help text.
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		addr := os.Getenv("OMNIAGENT_GATEWAY_ADDRESS")
		if addr == "" {
			addr = "127.0.0.1:18789"
		}
		url := "http://" + healthz.NormalizeLoopback(addr) + "/health"
		return healthz.Probe(context.Background(), url, 3*time.Second)
	},
}

func init() {
	rootCmd.AddCommand(healthcheckCmd)
}
