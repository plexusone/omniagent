package gateway

import (
	"context"
	"time"
)

// DefaultMessageHandler provides a basic message handler implementation.
type DefaultMessageHandler struct {
	gateway *Gateway
}

// NewDefaultMessageHandler creates a new default message handler.
func NewDefaultMessageHandler(gw *Gateway) *DefaultMessageHandler {
	return &DefaultMessageHandler{gateway: gw}
}

// Handle processes incoming messages.
func (h *DefaultMessageHandler) Handle(ctx context.Context, client *Client, msg *Message) (*Message, error) {
	switch msg.Type {
	case MessageTypePing:
		return h.handlePing(ctx, client, msg)
	case MessageTypeChat:
		return h.handleChat(ctx, client, msg)
	case MessageTypeAuth:
		return h.handleAuth(ctx, client, msg)
	case MessageTypeSubscribe:
		return h.handleSubscribe(ctx, client, msg)
	default:
		return NewErrorMessage(msg.ID, "unknown message type"), nil
	}
}

// handlePing handles ping messages.
func (h *DefaultMessageHandler) handlePing(_ context.Context, _ *Client, msg *Message) (*Message, error) {
	return &Message{
		ID:        msg.ID,
		Type:      MessageTypePong,
		Timestamp: time.Now(),
	}, nil
}

// handleChat handles chat messages.
func (h *DefaultMessageHandler) handleChat(ctx context.Context, client *Client, msg *Message) (*Message, error) {
	// If no agent configured, echo the message
	if h.gateway.agent == nil {
		return &Message{
			ID:        msg.ID,
			Type:      MessageTypeResponse,
			Content:   "Message received: " + msg.Content,
			Timestamp: time.Now(),
		}, nil
	}

	// Process through agent
	// Use client ID as session ID for conversation continuity
	response, err := h.gateway.agent.Process(ctx, client.ID, msg.Content)
	if err != nil {
		return NewErrorMessage(msg.ID, err.Error()), nil
	}

	return &Message{
		ID:        msg.ID,
		Type:      MessageTypeResponse,
		Content:   response,
		Channel:   msg.Channel,
		Timestamp: time.Now(),
	}, nil
}

// handleAuth handles authentication messages.
func (h *DefaultMessageHandler) handleAuth(_ context.Context, client *Client, msg *Message) (*Message, error) {
	// TODO: Implement proper authentication
	// For now, accept all auth requests
	client.SetMetadata("authenticated", true)

	return &Message{
		ID:   msg.ID,
		Type: MessageTypeResponse,
		Data: map[string]interface{}{
			"authenticated": true,
			"client_id":     client.ID,
		},
		Timestamp: time.Now(),
	}, nil
}

// handleSubscribe handles channel subscription messages.
func (h *DefaultMessageHandler) handleSubscribe(_ context.Context, client *Client, msg *Message) (*Message, error) {
	channel := msg.Channel
	if channel == "" {
		return NewErrorMessage(msg.ID, "channel required"), nil
	}

	// Store subscription in client metadata
	subs, _ := client.GetMetadata("subscriptions")
	subscriptions, ok := subs.([]string)
	if !ok {
		subscriptions = []string{}
	}
	subscriptions = append(subscriptions, channel)
	client.SetMetadata("subscriptions", subscriptions)

	return &Message{
		ID:      msg.ID,
		Type:    MessageTypeResponse,
		Channel: channel,
		Data: map[string]interface{}{
			"subscribed": true,
		},
		Timestamp: time.Now(),
	}, nil
}
