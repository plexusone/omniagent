# Architecture Overview

OmniAgent is designed as a modular AI agent framework with clear separation between messaging channels, agent runtime, and tool execution.

## High-Level Architecture

```
+-------------------------------------------------------------------------+
|  Clients:  messaging channels | browser (web UI) | OpenAI-API clients   |
|            Telegram, Discord, WhatsApp, Twilio, ...                      |
+------------------------------+------------------------------------------+
                               |
+------------------------------v------------------------------------------+
|  Gateway process (one HTTP server)                                       |
|  WebSocket control plane | OpenAI-compatible API | embedded web UI       |
|  /health  /readyz  | team APIs: auth, chats, agents, admin (team mode)   |
+------------------------------+------------------------------------------+
                               |
+------------------------------v------------------------------------------+
|  Agent runtime                                                           |
|  sessions | context + compaction | memory | cron | hooks | profiles      |
|  +--------------------------------------------------------------+       |
|  | Skills: markdown (SKILL.md) | compiled (Go) | MCP | OpenAPI    |       |
|  +--------------------------------------------------------------+       |
|  omnillm (LLM) | omnivoice (STT/TTS) | omniobserve | sandbox             |
+------------------------------+------------------------------------------+
                               |
+------------------------------v------------------------------------------+
|  Storage and secrets                                                     |
|  kvs store (sqlite | redis | memory) | team database (Ent: SQLite or      |
|  PostgreSQL + RLS) | OmniVault (op, bw, file, env) + redaction           |
+-------------------------------------------------------------------------+
```

## Deployment Modes

OmniAgent runs in one of two shapes from the same binary and packages.

| | Personal mode (default) | Team mode (`team.enabled`) |
|---|---|---|
| Users | One implicit user | Many; magic link, email and password, Google and GitHub sign-in |
| Database | None required; sessions and cron in the kvs store | Ent over SQLite or PostgreSQL with row-level security |
| Agents | One agent, or several named agents from config | Virtual agents with roles, a catalog and per-agent secrets |
| Surfaces | Channels, WebSocket, OpenAI API, web UI | The same, plus chats (DMs and groups), an admin UI and team APIs |

See [Team Mode](../../guides/team-mode.md) and [Virtual Agents](../../guides/agents.md).

## Components

### Gateway

`gateway` is the control plane. It owns the HTTP server and mounts every
surface on it:

- WebSocket connections for clients and tool RPC, with authentication and
  origin validation
- The OpenAI-compatible API (`api/openai`, `openai`) and the embedded web UI
  (`web`), a single capability-driven SPA with no build step
- Team APIs for authentication, chats, virtual agents and administration
- `/health` for liveness and `/readyz` for readiness

The chat path calls `ProcessWithSession` when the agent keeps durable session
history and `Process` otherwise. The gateway detects that with a type
assertion rather than requiring every agent to implement it.

Liveness and readiness are separate on purpose. `/health` returns 200 while
the process serves HTTP and is what a container restart decision uses.
`/readyz` returns 200 only when a storage write, read and delete round-trip
succeeds and an agent was built with an API key; it never makes a billed LLM
call, and it does not report channel connectivity because the channel library
exposes no connected-state query. See the [deployment guide](../../guides/deployment.md).

### Agent Runtime

`agent.New` builds one agent instance. Options configure the LLM provider and
model, skills, sessions, storage, memory, hooks, profiles, roles and the cron
scheduler:

```go
a, _ := agent.New(agent.Config{
    Provider:     "anthropic",
    Model:        "claude-sonnet-5",
    SystemPrompt: "You are OmniAgent...",
}, agent.WithStorage(store), agent.WithSessionStore(sessions))

response, _ := a.Process(ctx, message)
```

- **Sessions** (`sessions`) keep per-conversation history in the kvs store and
  roll a session over when its context fills.
- **Context** (`context`) counts tokens and applies a window strategy, including
  LLM-based summarization of older messages.
- **Memory** is scoped by tenant and subject (`memscope`), so a team chat
  remembers under its own scope.
- **Cron** (`cron`) runs scheduled jobs. Only a single-agent process starts the
  scheduler, because several schedulers on one shared job store would fire a job
  twice.
- **Hooks** (`hooks`) deliver events such as session creation and update to
  handlers, which is how observability integrations attach.
- **Profiles and roles** (`agent/profiles`, `agent/roles`) set bootstrap
  behavior and agent personas, and `agent/registry` manages several agents.

### Channels

Channels connect to messaging platforms through
[omnichat](https://github.com/plexusone/omnichat): Telegram, Discord, WhatsApp
and Twilio. `channels/policy` checks channel conformance and applies
per-sender tool and channel policy. `autoreply` decides when a message gets a
response.

### Skills

Skills give the agent tools and instructions. There are four kinds, all
registered on the agent; see [Skills System](skills.md).

### Team Mode

Team mode adds a multi-user domain under `team/`, written as a library with the
HTTP handlers kept thin:

- **Two-layer authorization.** The service layer is the primary gate
  (`agents.Service.Can`, membership checks). PostgreSQL row-level security is a
  backstop. On SQLite there are no policies, so the service checks stand alone.
- **System and user context.** Authorization facts are read as the system so a
  decision does not depend on what the user may see; data operations run as the
  user.
- **Per-agent runtime.** `team/agentruntime` builds an agent instance lazily on
  its first turn, from the agent's persona, enabled skills and secrets, and
  keeps instances in a bounded LRU cache.
- **Agent-scoped secrets.** `team/secrets` namespaces an OmniVault store per
  agent, so one agent cannot address another's secrets.

### Secrets

Credentials resolve from `op://`, `bw://`, `file://` and `env://` references
through OmniVault. Every resolved value is registered with `internal/redact`,
which masks it in all log output for the life of the process. Secrets reach a
skill as environment values injected into that skill alone; a skill that
declares a required secret it does not have is left out instead of failing
later. A secret file that other users can read is refused.

### AAuth

`aauthagent` makes OmniAgent an [AAuth](https://github.com/aauth-dev/aauth-go)
agent. `NewClient` enrolls a key at an agent provider and returns an HTTP
client that signs each request as that agent. When a resource asks for more,
the client obtains person and auth tokens from the person server and waits
while the person approves, passing the approval link to the caller. An OpenAPI
skill given that client calls an AAuth-protected API with the agent's own
identity. See the [AAuth guide](../../guides/aauth/index.md).

### Sandbox

The `sandbox` package runs untrusted code in isolation, for tool developers:

- **App-level** - capability-based permissions
- **WASM** - lightweight isolation through wazero
- **Docker** - full container isolation

## Data Flow

### Message Processing

```
1. Channel, WebSocket, web UI or API request arrives
         │
         ▼
2. Gateway authenticates it and routes it to an agent
   (team mode: to the chat's agent instance)
         │
         ▼
3. Agent loads the session and builds the prompt:
   persona, markdown skills, recalled memory
         │
         ▼
4. LLM responds; tool calls run through skills
   (compiled, MCP, OpenAPI) and return results
         │
         ▼
5. Session is saved; hooks receive the events
         │
         ▼
6. Response goes back on the same surface
```

### Voice Processing

OmniAgent supports two voice processing architectures:

**Traditional Pipeline (STT → LLM → TTS)**

```
1. Voice message received
         │
         ▼
2. STT transcription (Deepgram, Whisper)
         │
         ▼
3. Agent processes text (Claude, GPT)
         │
         ▼
4. TTS synthesis (ElevenLabs, Deepgram)
         │
         ▼
5. Voice response sent
```

- Latency: 500-1500ms
- Use case: Custom voice selection, specialized STT

**Native Voice-to-Voice**

```
1. Audio stream received
         │
         ▼
2. Realtime API (OpenAI Realtime / Gemini Live)
         │
         ▼
3. Audio response sent
```

- Latency: 100-200ms
- Use case: Low-latency conversations, natural barge-in

See [Voice Integration Guide](../../guides/voice.md) for configuration details.

**Meetings by Phone**

```
1. Meeting invitation parsed (omnimeet-core invite)
         │
         ▼
2. Dial-in number called; meeting ID and passcode sent as keypad digits
   (omnimeet-core dialin, over an audio-mode Twilio or Telnyx gateway)
         │
         ▼
3. Announcement spoken, meeting audio streamed to STT (meeting.Attendant)
         │
         ▼
4. Transcript summarized by the LLM into meeting notes
```

- The agent listens; it speaks only to announce itself
- Use case: attending Zoom, Google Meet, and Teams meetings without a
  per-platform app

See [Attending Meetings by Phone](../../guides/meetings.md).

## Omni* Library Ecosystem

OmniAgent is built on a modular ecosystem of `omni*` libraries. Each library follows a core/provider split pattern where core modules contain interfaces and generic functionality, while provider modules contain SDK-specific implementations.

```
                         Omni* Library Architecture

┌─────────────────────────────────────────────────────────────────────────────┐
│                              OmniAgent                                      │
│                          (Agent Runtime)                                    │
└───┬────────┬────────┬────────┬────────┬────────┬────────┬────────┬─────────┘
    │        │        │        │        │        │        │        │
    ▼        ▼        ▼        ▼        ▼        ▼        ▼        ▼
┌────────┐┌────────┐┌────────┐┌────────┐┌────────┐┌────────┐┌──────────────────┐
│omnichat││omnillm ││omnivoice│omniobserve│omniserp││omniretrieve│  omnistorage   │
│        ││        ││        ││        ││        ││        ││                  │
│Messaging││  LLM   ││  Voice ││Observability│ Search ││  RAG   ││  File + KVS    │
└────────┘└───┬────┘└───┬────┘└────────┘└────────┘└────────┘└────────┬─────────┘
              │         │                                            │
    ┌─────────┴─────────┴────────────────────────────────────────────┴─────────┐
    │                           Core Modules                                   │
    │  ┌─────────────┐  ┌─────────────┐  ┌────────────────────────────────┐    │
    │  │ omnillm-core│  │omnivoice-core│  │       omnistorage-core         │    │
    │  │             │  │             │  │  ┌─────────┐  ┌─────────┐      │    │
    │  │ - Interfaces│  │ - Interfaces│  │  │ /object │  │  /kvs   │      │    │
    │  │ - Request/  │  │ - STT/TTS   │  │  │         │  │         │      │    │
    │  │   Response  │  │   types     │  │  │ Files   │  │ Sessions│      │    │
    │  │ - Tool      │  │ - Streaming │  │  │ Blobs   │  │ Cache   │      │    │
    │  │   schemas   │  │             │  │  │ Objects │  │ State   │      │    │
    │  └─────────────┘  └─────────────┘  │  └─────────┘  └─────────┘      │    │
    │                                    └────────────────────────────────┘    │
    └──────────────────────────────────────────────────────────────────────────┘
              │                   │                         │
    ┌─────────┴───────────────────┴─────────────────────────┴─────────┐
    │                      Provider Modules                            │
    │                                                                  │
    │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
    │  │    omni-aws     │  │   omni-google   │  │   omni-github   │  │
    │  │                 │  │                 │  │                 │  │
    │  │ /omnillm        │  │ /omnillm        │  │ /omnistorage    │  │
    │  │   (Bedrock)     │  │   (Gemini)      │  │   (GitHub API)  │  │
    │  │                 │  │                 │  │                 │  │
    │  │ /omnistorage    │  │ /omnistorage    │  └─────────────────┘  │
    │  │   (S3)          │  │   (GCS)         │                       │
    │  │                 │  │                 │                       │
    │  │ /omnivoice      │  │ /omnivoice      │                       │
    │  │   (Polly TTS)   │  │   (Gemini Live) │                       │
    │  └─────────────────┘  └─────────────────┘                       │
    │                                                                  │
    │  ┌─────────────────┐                                            │
    │  │   omni-openai   │                                            │
    │  │                 │                                            │
    │  │ /omnillm        │                                            │
    │  │   (GPT)         │                                            │
    │  │                 │                                            │
    │  │ /omnivoice      │                                            │
    │  │   (Realtime)    │                                            │
    │  └─────────────────┘                                            │
    └─────────────────────────────────────────────────────────────────┘
```

### Module Organization

| Pattern | Description | Example |
|---------|-------------|---------|
| `omni*` | Aggregator module, re-exports core + providers | `omnillm`, `omnistorage` |
| `omni*-core` | Interfaces and generic implementations | `omnillm-core`, `omnistorage-core` |
| `omni-<provider>/*` | Provider-specific implementations | `omni-aws/omnillm`, `omni-google/omnistorage` |

### Storage Architecture

`omnistorage-core` provides two storage subsystems:

| Subsystem | Import Path | Purpose | Backends |
|-----------|-------------|---------|----------|
| Object Storage | `omnistorage-core/object` | Files, blobs, documents | Local FS, S3, GCS, GitHub |
| Key-Value Storage | `omnistorage-core/kvs` | Sessions, cache, state | Memory, SQLite |

## Dependencies

### Core Libraries

| Package | Purpose |
|---------|---------|
| [omnillm](https://github.com/plexusone/omnillm) | Multi-provider LLM abstraction |
| [omnichat](https://github.com/plexusone/omnichat) | Unified messaging interface |
| [omnivoice](https://github.com/plexusone/omnivoice) | Voice STT/TTS |
| [omniobserve](https://github.com/plexusone/omniobserve) | LLM observability |
| [omniserp](https://github.com/plexusone/omniserp) | Web search |
| [omnistorage-core](https://github.com/plexusone/omnistorage-core) | Object and key-value storage |
| [omnivault](https://github.com/plexusone/omnivault) | Secret and credential vaults |
| [omnimemory](https://github.com/plexusone/omnimemory) | Scoped agent memory |
| [omniskill](https://github.com/plexusone/omniskill) | Skill and tool interfaces |
| [omnimeet-core](https://github.com/plexusone/omnimeet-core) | Meeting invitations and phone dial-in |

### Infrastructure

| Package | Purpose |
|---------|---------|
| [wazero](https://github.com/tetratelabs/wazero) | WASM runtime |
| [moby](https://github.com/moby/moby) | Docker SDK |
| [Rod](https://github.com/go-rod/rod) | Browser automation |
| [gorilla/websocket](https://github.com/gorilla/websocket) | WebSocket server |
| [Ent](https://entgo.io) with pgx | Team database access (SQLite or PostgreSQL) |
| [aauth-go](https://github.com/aauth-dev/aauth-go) | AAuth agent identity and signing |
| [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) | MCP skills |
| [kin-openapi](https://github.com/getkin/kin-openapi) | OpenAPI skills |

## Configuration

OmniAgent uses a layered configuration system:

1. **Defaults** - Built-in sensible defaults
2. **Config source** - A YAML/JSON file, or, with no `--config` path, a whole
   document in the `OMNIAGENT_CONFIG_B64` environment variable (base64), for
   platforms with no volume mounts
3. **Environment variables** - `OMNIAGENT_*` values override the config source

Credential values written as `op://`, `bw://`, `file://` or `env://` references
are then resolved through OmniVault.

```yaml
# omniagent.yaml
gateway:
  address: "127.0.0.1:18789"

agent:
  provider: ${OMNIAGENT_AGENT_PROVIDER:-anthropic}
  model: ${OMNIAGENT_AGENT_MODEL:-claude-sonnet-5}
```
