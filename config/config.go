// Package config provides configuration types and loading for omniagent.
package config

import "time"

// Config is the root configuration for omniagent.
type Config struct {
	Gateway       GatewayConfig       `json:"gateway" yaml:"gateway"`
	Agent         AgentConfig         `json:"agent" yaml:"agent"`
	Channels      ChannelsConfig      `json:"channels" yaml:"channels"`
	Tools         ToolsConfig         `json:"tools" yaml:"tools"`
	Observability ObservabilityConfig `json:"observability" yaml:"observability"`
}

// GatewayConfig configures the WebSocket gateway.
type GatewayConfig struct {
	Address      string        `json:"address" yaml:"address"`
	ReadTimeout  time.Duration `json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout"`
	PingInterval time.Duration `json:"ping_interval" yaml:"ping_interval"`
}

// AgentConfig configures the AI agent.
type AgentConfig struct {
	Provider     string  `json:"provider" yaml:"provider"`
	Model        string  `json:"model" yaml:"model"`
	APIKey       string  `json:"api_key" yaml:"api_key"` //nolint:gosec // G117: APIKey loaded from config file
	BaseURL      string  `json:"base_url" yaml:"base_url"`
	Temperature  float64 `json:"temperature" yaml:"temperature"`
	MaxTokens    int     `json:"max_tokens" yaml:"max_tokens"`
	SystemPrompt string  `json:"system_prompt" yaml:"system_prompt"`
}

// ChannelsConfig configures messaging channels.
type ChannelsConfig struct {
	Telegram TelegramConfig `json:"telegram" yaml:"telegram"`
	Discord  DiscordConfig  `json:"discord" yaml:"discord"`
	WhatsApp WhatsAppConfig `json:"whatsapp" yaml:"whatsapp"`
}

// WhatsAppConfig configures the WhatsApp channel.
type WhatsAppConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	DBPath  string `json:"db_path" yaml:"db_path"`
}

// TelegramConfig configures the Telegram channel.
type TelegramConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Token   string `json:"token" yaml:"token"`
}

// DiscordConfig configures the Discord channel.
type DiscordConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Token   string `json:"token" yaml:"token"`
	GuildID string `json:"guild_id" yaml:"guild_id"`
}

// ToolsConfig configures available tools.
type ToolsConfig struct {
	Browser BrowserToolConfig `json:"browser" yaml:"browser"`
	Shell   ShellToolConfig   `json:"shell" yaml:"shell"`
}

// BrowserToolConfig configures the browser automation tool.
type BrowserToolConfig struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	Headless bool   `json:"headless" yaml:"headless"`
	UserData string `json:"user_data" yaml:"user_data"`
}

// ShellToolConfig configures the shell execution tool.
type ShellToolConfig struct {
	Enabled    bool     `json:"enabled" yaml:"enabled"`
	WorkingDir string   `json:"working_dir" yaml:"working_dir"`
	Allowlist  []string `json:"allowlist" yaml:"allowlist"`
}

// ObservabilityConfig configures observability features.
type ObservabilityConfig struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	Provider string `json:"provider" yaml:"provider"`
	Endpoint string `json:"endpoint" yaml:"endpoint"`
	APIKey   string `json:"api_key" yaml:"api_key"` //nolint:gosec // G117: APIKey loaded from config file
}
