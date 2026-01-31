package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	configFormat string
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration commands",
	Long:  "Commands for viewing and managing configuration.",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  "Display the current configuration (with sensitive values redacted).",
	RunE:  showConfig,
}

func init() {
	configShowCmd.Flags().StringVar(&configFormat, "format", "yaml", "output format (yaml, json)")

	configCmd.AddCommand(configShowCmd)
}

func showConfig(cmd *cobra.Command, args []string) error {
	cfg := getConfig()

	// Redact sensitive values
	redacted := *cfg
	if redacted.Agent.APIKey != "" {
		redacted.Agent.APIKey = "***REDACTED***"
	}
	if redacted.Channels.Telegram.Token != "" {
		redacted.Channels.Telegram.Token = "***REDACTED***"
	}
	if redacted.Channels.Discord.Token != "" {
		redacted.Channels.Discord.Token = "***REDACTED***"
	}
	if redacted.Observability.APIKey != "" {
		redacted.Observability.APIKey = "***REDACTED***"
	}

	var output []byte
	var err error

	switch configFormat {
	case "json":
		output, err = json.MarshalIndent(redacted, "", "  ")
	case "yaml":
		output, err = yaml.Marshal(redacted)
	default:
		return fmt.Errorf("unknown format: %s (use yaml or json)", configFormat)
	}

	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	fmt.Println(string(output))
	return nil
}
