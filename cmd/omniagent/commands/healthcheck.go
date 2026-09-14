package commands

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

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
		// A bind-all address is not dialable; probe loopback instead.
		if host, port, err := net.SplitHostPort(addr); err == nil {
			if host == "0.0.0.0" || host == "::" || host == "" {
				addr = net.JoinHostPort("127.0.0.1", port)
			} else {
				addr = net.JoinHostPort(host, port)
			}
		}

		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get("http://" + addr + "/health")
		if err != nil {
			return fmt.Errorf("healthcheck: %w", err)
		}
		defer func() {
			if cerr := resp.Body.Close(); cerr != nil {
				fmt.Fprintf(os.Stderr, "healthcheck: close body: %v\n", cerr)
			}
		}()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("healthcheck: %s returned %s", strings.TrimPrefix(addr, "127.0.0.1"), resp.Status)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(healthcheckCmd)
}
