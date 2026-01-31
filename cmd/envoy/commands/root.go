// Package commands implements the envoy CLI commands.
package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/agentplexus/envoy/config"
)

var (
	cfgFile string
	cfg     *config.Config
)

// rootCmd is the base command for envoy.
var rootCmd = &cobra.Command{
	Use:   "envoy",
	Short: "Your AI representative across communication channels",
	Long: `Envoy is a personal AI assistant that routes messages across
multiple communication platforms, processes them via an AI agent,
and responds on your behalf.

Start the gateway:
  envoy gateway run

Check channel status:
  envoy channels status

Show configuration:
  envoy config show`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for version command
		if cmd.Name() == "version" {
			return nil
		}

		var err error
		cfg, err = config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		return nil
	},
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default: envoy.yaml)")

	// Add subcommands
	rootCmd.AddCommand(gatewayCmd)
	rootCmd.AddCommand(channelsCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(versionCmd)
}

// getConfig returns the loaded configuration.
func getConfig() *config.Config {
	if cfg == nil {
		cfg = &config.Config{}
		*cfg = config.Default()
	}
	return cfg
}
