// Package agent provides the AI agent runtime for envoy.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/agentplexus/omnillm"
	"github.com/agentplexus/omnillm/provider"
)

// Agent is the AI agent that processes messages.
type Agent struct {
	client *omnillm.ChatClient
	tools  *ToolRegistry
	config Config
	logger *slog.Logger
}

// Config configures the agent.
type Config struct {
	Provider          string
	Model             string
	APIKey            string //nolint:gosec // G117: APIKey is intentionally stored for provider authentication
	BaseURL           string
	Temperature       float64
	MaxTokens         int
	SystemPrompt      string
	Logger            *slog.Logger
	ObservabilityHook omnillm.ObservabilityHook
}

// New creates a new agent.
func New(config Config) (*Agent, error) {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	// Build provider configuration
	providerConfig := omnillm.ProviderConfig{
		Provider: omnillm.ProviderName(config.Provider),
		APIKey:   config.APIKey,
	}
	if config.BaseURL != "" {
		providerConfig.BaseURL = config.BaseURL
	}

	// Create omnillm client
	client, err := omnillm.NewClient(omnillm.ClientConfig{
		Providers:         []omnillm.ProviderConfig{providerConfig},
		Logger:            config.Logger,
		ObservabilityHook: config.ObservabilityHook,
	})
	if err != nil {
		return nil, fmt.Errorf("create llm client: %w", err)
	}

	return &Agent{
		client: client,
		tools:  NewToolRegistry(),
		config: config,
		logger: config.Logger,
	}, nil
}

// Process processes a message and returns a response.
func (a *Agent) Process(ctx context.Context, sessionID, content string) (string, error) {
	a.logger.Info("processing message", "model", a.config.Model, "provider", a.config.Provider)
	messages := []provider.Message{
		{
			Role:    provider.RoleUser,
			Content: content,
		},
	}

	// Add system prompt if configured
	if a.config.SystemPrompt != "" {
		a.logger.Info("using system prompt", "length", len(a.config.SystemPrompt))
		messages = append([]provider.Message{
			{
				Role:    provider.RoleSystem,
				Content: a.config.SystemPrompt,
			},
		}, messages...)
	}

	// Add tools if available
	tools := a.tools.GetTools()
	a.logger.Info("tools available for request", "count", len(tools))
	for _, t := range tools {
		paramsJSON, _ := json.Marshal(t.Function.Parameters)
		a.logger.Info("tool in request", "name", t.Function.Name, "type", t.Type, "params", string(paramsJSON))
	}

	// Process with potential tool calls (max 5 iterations to prevent infinite loops)
	for i := 0; i < 5; i++ {
		req := &provider.ChatCompletionRequest{
			Model:    a.config.Model,
			Messages: messages,
		}

		if a.config.Temperature > 0 {
			req.Temperature = &a.config.Temperature
		}
		if a.config.MaxTokens > 0 {
			req.MaxTokens = &a.config.MaxTokens
		}

		if len(tools) > 0 {
			req.Tools = tools
		}

		resp, err := a.client.CreateChatCompletion(ctx, req)
		if err != nil {
			return "", fmt.Errorf("chat completion: %w", err)
		}

		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("no response choices")
		}

		choice := resp.Choices[0]

		a.logger.Info("LLM response",
			"content_length", len(choice.Message.Content),
			"tool_calls", len(choice.Message.ToolCalls),
			"finish_reason", choice.FinishReason)

		// Check if the model wants to call tools
		if len(choice.Message.ToolCalls) == 0 {
			// No tool calls, return the response
			return choice.Message.Content, nil
		}

		// Execute tool calls
		a.logger.Info("executing tool calls", "count", len(choice.Message.ToolCalls))

		// Add assistant message with tool calls to conversation
		messages = append(messages, provider.Message{
			Role:      provider.RoleAssistant,
			ToolCalls: choice.Message.ToolCalls,
		})

		// Execute each tool and add results
		for _, toolCall := range choice.Message.ToolCalls {
			a.logger.Info("calling tool", "name", toolCall.Function.Name)

			result, err := a.tools.Execute(ctx, toolCall.Function.Name, []byte(toolCall.Function.Arguments))
			if err != nil {
				a.logger.Error("tool execution failed", "name", toolCall.Function.Name, "error", err)
				result = fmt.Sprintf("Error: %v", err)
			}

			// Add tool result to conversation
			toolCallID := toolCall.ID
			messages = append(messages, provider.Message{
				Role:       provider.RoleTool,
				Content:    result,
				ToolCallID: &toolCallID,
			})
		}
	}

	return "", fmt.Errorf("exceeded maximum tool call iterations")
}

// ProcessWithMemory processes a message using conversation memory.
func (a *Agent) ProcessWithMemory(ctx context.Context, sessionID, content string) (string, error) {
	// TODO: Implement memory-aware processing using omnillm memory features
	return a.Process(ctx, sessionID, content)
}

// RegisterTool registers a tool with the agent.
func (a *Agent) RegisterTool(tool Tool) {
	a.tools.Register(tool)
}

// Close closes the agent and releases resources.
func (a *Agent) Close() error {
	return a.client.Close()
}
