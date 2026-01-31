package config

import "time"

// Default returns a Config with sensible defaults.
func Default() Config {
	return Config{
		Gateway: GatewayConfig{
			Address:      "127.0.0.1:18789",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			PingInterval: 30 * time.Second,
		},
		Agent: AgentConfig{
			Provider:     "anthropic",
			Model:        "claude-sonnet-4-20250514",
			Temperature:  0.7,
			MaxTokens:    4096,
			SystemPrompt: "You are Envoy, a helpful AI assistant. You represent the user across communication channels, responding on their behalf with care and precision.",
		},
		Channels: ChannelsConfig{
			Telegram: TelegramConfig{
				Enabled: false,
			},
			Discord: DiscordConfig{
				Enabled: false,
			},
		},
		Tools: ToolsConfig{
			Browser: BrowserToolConfig{
				Enabled:  true,
				Headless: true,
			},
			Shell: ShellToolConfig{
				Enabled: false, // Disabled by default for security
			},
		},
		Observability: ObservabilityConfig{
			Enabled: false,
		},
	}
}
