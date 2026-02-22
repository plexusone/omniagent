// Package main demonstrates basic omniagent usage.
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/mdp/qrterminal/v3"

	"github.com/agentplexus/omniagent/agent"
	"github.com/agentplexus/omniagent/config"
	"github.com/agentplexus/omniagent/gateway"
	"github.com/agentplexus/omnichat/provider"
	"github.com/agentplexus/omnichat/providers/telegram"
	"github.com/agentplexus/omnichat/providers/whatsapp"
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

	// Create provider router
	router := provider.NewRouter(logger)

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

	// Add WhatsApp channel if configured
	if cfg.Channels.WhatsApp.Enabled {
		dbPath := cfg.Channels.WhatsApp.DBPath
		if dbPath == "" {
			dbPath = "whatsapp.db"
		}
		wa, err := whatsapp.New(whatsapp.Config{
			DBPath: dbPath,
			Logger: logger,
			QRCallback: func(qr string) {
				fmt.Println("\n📱 Scan this QR code with WhatsApp:")
				fmt.Println("   (Settings → Linked Devices → Link a Device)")
				fmt.Println()
				qrterminal.GenerateWithConfig(qr, qrterminal.Config{
					Level:     qrterminal.L,
					Writer:    os.Stdout,
					BlackChar: qrterminal.WHITE,
					WhiteChar: qrterminal.BLACK,
					QuietZone: 1,
				})
				fmt.Println()
			},
		})
		if err != nil {
			log.Fatalf("Failed to create WhatsApp adapter: %v", err)
		}
		router.Register(wa)
	}

	// Set the agent on the router and use the built-in processor
	router.SetAgent(agentInstance)
	router.OnMessage(provider.All(), router.ProcessWithAgent())

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
	fmt.Printf("OmniAgent starting on %s\n", cfg.Gateway.Address)
	fmt.Println("Press Ctrl+C to stop")

	if err := gw.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("Gateway error: %v", err)
	}

	// Disconnect channels
	if err := router.DisconnectAll(context.Background()); err != nil {
		log.Printf("Warning: disconnect error: %v", err)
	}
	fmt.Println("OmniAgent stopped")
}
