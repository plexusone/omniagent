# Architecture Overview

OmniAgent is designed as a modular AI agent framework with clear separation between messaging channels, agent runtime, and tool execution.

## High-Level Architecture

```
+-------------------------------------------------------------+
|                     Messaging Channels                      |
|     Telegram  |  Discord  |  Slack  |  WhatsApp  |  ...     |
+---------------------------+---------------------------------+
                            |
+---------------------------v---------------------------------+
|              Gateway (WebSocket Control Plane)              |
|              ws://127.0.0.1:18789                           |
+---------------------------+---------------------------------+
                            |
+---------------------------v---------------------------------+
|                      Agent Runtime                          |
|  +------------------+  +------------------+                 |
|  |    Skills        |  |    Sandbox       |                 |
|  |  (SKILL.md)      |  |  (WASM/Docker)   |                 |
|  +------------------+  +------------------+                 |
|  - omnillm (LLM providers)                                  |
|  - omnivoice (STT/TTS)                                      |
|  - omniobserve (tracing)                                    |
|  - Tools (browser, shell, http)                             |
+-------------------------------------------------------------+
```

## Components

### Gateway

The WebSocket gateway serves as the control plane for OmniAgent:

- Manages client connections
- Routes messages to/from channels
- Provides health check endpoints
- Handles authentication (planned)

```go
gateway, _ := gateway.New(gateway.Config{
    Address:      "127.0.0.1:18789",
    ReadTimeout:  30 * time.Second,
    WriteTimeout: 30 * time.Second,
})
gateway.Run(ctx)
```

### Agent Runtime

The agent processes messages using LLM providers:

```go
agent, _ := agent.New(agent.Config{
    Provider:     "anthropic",
    Model:        "claude-sonnet-4-20250514",
    SystemPrompt: "You are OmniAgent...",
})

response, _ := agent.Process(ctx, message)
```

### Channels

Channels connect to external messaging platforms via [omnichat](https://github.com/plexusone/omnichat):

- **WhatsApp** - Via WhatsApp Web protocol
- **Telegram** - Via Bot API
- **Discord** - Via Bot API

### Skills System

Skills extend agent capabilities via SKILL.md files:

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ Skill       │───▶│ Skill       │───▶│ System      │
│ Discovery   │    │ Loader      │    │ Prompt      │
└─────────────┘    └─────────────┘    └─────────────┘
```

See [Skills System](skills.md) for details.

### Sandbox

Tools execute in isolated environments:

- **App-Level** - Capability-based permissions
- **WASM** - Lightweight isolation via wazero
- **Docker** - Full container isolation

## Data Flow

### Message Processing

```
1. Channel receives message
         │
         ▼
2. Gateway routes to agent
         │
         ▼
3. Agent loads skills into prompt
         │
         ▼
4. LLM processes message
         │
         ▼
5. Tools execute in sandbox
         │
         ▼
6. Response sent via channel
```

### Voice Processing

```
1. Voice message received
         │
         ▼
2. STT transcription (omnivoice)
         │
         ▼
3. Agent processes text
         │
         ▼
4. TTS synthesis (omnivoice)
         │
         ▼
5. Voice response sent
```

## Dependencies

### Core Libraries

| Package | Purpose |
|---------|---------|
| [omnillm](https://github.com/plexusone/omnillm) | Multi-provider LLM abstraction |
| [omnichat](https://github.com/plexusone/omnichat) | Unified messaging interface |
| [omnivoice](https://github.com/plexusone/omnivoice) | Voice STT/TTS |
| [omniobserve](https://github.com/plexusone/omniobserve) | LLM observability |
| [omniserp](https://github.com/plexusone/omniserp) | Web search |

### Infrastructure

| Package | Purpose |
|---------|---------|
| [wazero](https://github.com/tetratelabs/wazero) | WASM runtime |
| [moby](https://github.com/moby/moby) | Docker SDK |
| [Rod](https://github.com/go-rod/rod) | Browser automation |
| [gorilla/websocket](https://github.com/gorilla/websocket) | WebSocket server |

## Configuration

OmniAgent uses a layered configuration system:

1. **Defaults** - Built-in sensible defaults
2. **Config File** - YAML/JSON configuration
3. **Environment Variables** - Override specific values

```yaml
# omniagent.yaml
gateway:
  address: "127.0.0.1:18789"

agent:
  provider: ${OMNIAGENT_AGENT_PROVIDER:-anthropic}
  model: ${OMNIAGENT_AGENT_MODEL:-claude-sonnet-4-20250514}
```
