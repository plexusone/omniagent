package commands

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/agentplexus/envoy/agent"
	"github.com/agentplexus/envoy/gateway"
)

var (
	gatewayAddress string
)

var gatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Gateway management commands",
	Long:  "Commands for managing the envoy WebSocket gateway.",
}

var gatewayRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the gateway server",
	Long: `Start the envoy WebSocket gateway server.

The gateway serves as the control plane for all connected clients,
routing messages between channels and the AI agent.`,
	RunE: runGateway,
}

func init() {
	gatewayRunCmd.Flags().StringVar(&gatewayAddress, "address", "", "gateway listen address (default from config)")

	gatewayCmd.AddCommand(gatewayRunCmd)
}

func runGateway(cmd *cobra.Command, args []string) error {
	cfg := getConfig()
	logger := slog.Default()

	// Override from flag if provided
	address := cfg.Gateway.Address
	if gatewayAddress != "" {
		address = gatewayAddress
	}

	// Create agent if API key is configured
	var agentProcessor gateway.AgentProcessor
	if cfg.Agent.APIKey != "" {
		agentInstance, err := agent.New(agent.Config{
			Provider:     cfg.Agent.Provider,
			Model:        cfg.Agent.Model,
			APIKey:       cfg.Agent.APIKey,
			BaseURL:      cfg.Agent.BaseURL,
			Temperature:  cfg.Agent.Temperature,
			MaxTokens:    cfg.Agent.MaxTokens,
			SystemPrompt: cfg.Agent.SystemPrompt,
			Logger:       logger,
		})
		if err != nil {
			return fmt.Errorf("create agent: %w", err)
		}
		defer agentInstance.Close()
		agentProcessor = agentInstance
		logger.Info("agent initialized", "provider", cfg.Agent.Provider, "model", cfg.Agent.Model)
	} else {
		logger.Warn("no API key configured, agent disabled (messages will be echoed)")
	}

	// Create gateway
	gw, err := gateway.New(gateway.Config{
		Address:      address,
		ReadTimeout:  cfg.Gateway.ReadTimeout,
		WriteTimeout: cfg.Gateway.WriteTimeout,
		PingInterval: cfg.Gateway.PingInterval,
		Agent:        agentProcessor,
		Logger:       logger,
	})
	if err != nil {
		return fmt.Errorf("create gateway: %w", err)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nShutting down gateway...")
		cancel()
	}()

	// Start gateway
	fmt.Printf("Starting gateway on %s\n", address)
	if err := gw.Run(ctx); err != nil && err != context.Canceled {
		return fmt.Errorf("gateway error: %w", err)
	}

	fmt.Println("Gateway stopped")
	return nil
}
