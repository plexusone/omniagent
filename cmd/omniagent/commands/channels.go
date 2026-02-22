package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var channelsCmd = &cobra.Command{
	Use:   "channels",
	Short: "Channel management commands",
	Long:  "Commands for managing messaging channels.",
}

var channelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available channels",
	Long:  "List all available messaging channels and their configuration status.",
	RunE:  listChannels,
}

var channelsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show channel connection status",
	Long:  "Show the connection status of all configured channels.",
	RunE:  statusChannels,
}

func init() {
	channelsCmd.AddCommand(channelsListCmd)
	channelsCmd.AddCommand(channelsStatusCmd)
}

func listChannels(cmd *cobra.Command, args []string) error {
	cfg := getConfig()

	fmt.Println("Available Channels:")
	fmt.Println()

	// Telegram
	telegramStatus := "disabled"
	if cfg.Channels.Telegram.Enabled {
		telegramStatus = "enabled"
	}
	fmt.Printf("  telegram    %s\n", telegramStatus)

	// Discord
	discordStatus := "disabled"
	if cfg.Channels.Discord.Enabled {
		discordStatus = "enabled"
	}
	fmt.Printf("  discord     %s\n", discordStatus)

	fmt.Println()
	fmt.Println("Use 'envoy channels status' to check connection status.")

	return nil
}

func statusChannels(cmd *cobra.Command, args []string) error {
	cfg := getConfig()

	fmt.Println("Channel Status:")
	fmt.Println()

	// Telegram
	if cfg.Channels.Telegram.Enabled {
		tokenSet := "token not set"
		if cfg.Channels.Telegram.Token != "" {
			tokenSet = "token configured"
		}
		fmt.Printf("  telegram    enabled     %s\n", tokenSet)
	} else {
		fmt.Println("  telegram    disabled")
	}

	// Discord
	if cfg.Channels.Discord.Enabled {
		tokenSet := "token not set"
		if cfg.Channels.Discord.Token != "" {
			tokenSet = "token configured"
		}
		fmt.Printf("  discord     enabled     %s\n", tokenSet)
	} else {
		fmt.Println("  discord     disabled")
	}

	return nil
}
