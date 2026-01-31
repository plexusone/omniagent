// Package main demonstrates basic envoy usage.
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/agentplexus/envoy/agent"
	"github.com/agentplexus/envoy/channels"
	"github.com/agentplexus/envoy/channels/adapters/telegram"
	"github.com/agentplexus/envoy/config"
	"github.com/agentplexus/envoy/gateway"
)

func main() {
	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create logger
	logger := slog.Default()

	// Create agent
	agentInstance, err := agent.New(agent.Config{
		Provider:     cfg.Agent.Provider,
		Model:        cfg.Agent.Model,
		APIKey:       cfg.Agent.APIKey,
		Temperature:  cfg.Agent.Temperature,
		MaxTokens:    cfg.Agent.MaxTokens,
		SystemPrompt: cfg.Agent.SystemPrompt,
		Logger:       logger,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}
	defer agentInstance.Close()

	// Create channel router
	router := channels.NewRouter(logger)

	// Add Telegram channel if configured
	if cfg.Channels.Telegram.Enabled {
		tg, err := telegram.New(telegram.Config{
			Token:  cfg.Channels.Telegram.Token,
			Logger: logger,
		})
		if err != nil {
			log.Fatalf("Failed to create Telegram adapter: %v", err)
		}
		router.Register(tg)
	}

	// Set the agent on the router and use the built-in processor
	router.SetAgent(agentInstance)
	router.OnMessage(channels.All(), router.ProcessWithAgent())

	// Connect channels
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := router.ConnectAll(ctx); err != nil {
		log.Fatalf("Failed to connect channels: %v", err)
	}

	// Create and start gateway
	gw, err := gateway.New(gateway.Config{
		Address: cfg.Gateway.Address,
		Logger:  logger,
	})
	if err != nil {
		log.Fatalf("Failed to create gateway: %v", err)
	}

	// Handle shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		cancel()
	}()

	// Start gateway
	fmt.Printf("Envoy starting on %s\n", cfg.Gateway.Address)
	fmt.Println("Press Ctrl+C to stop")

	if err := gw.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("Gateway error: %v", err)
	}

	// Disconnect channels
	if err := router.DisconnectAll(context.Background()); err != nil {
		log.Printf("Warning: disconnect error: %v", err)
	}
	fmt.Println("Envoy stopped")
}
