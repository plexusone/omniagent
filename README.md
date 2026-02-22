# OmniAgent

[![Build Status][build-status-svg]][build-status-url]
[![Lint Status][lint-status-svg]][lint-status-url]
[![Go Report Card][goreport-svg]][goreport-url]
[![Docs][docs-godoc-svg]][docs-godoc-url]
[![License][license-svg]][license-url]

Your AI representative across communication channels.

OmniAgent is a personal AI assistant that routes messages across multiple communication platforms, processes them via an AI agent, and responds on your behalf.

## Features

- **Multi-Channel Support** - Telegram, Discord, Slack, WhatsApp, and more
- **AI-Powered Responses** - Powered by omnillm (Claude, GPT, Gemini, etc.)
- **Voice Notes** - Transcribe incoming voice, respond with synthesized speech via OmniVoice
- **Browser Automation** - Built-in browser control via Rod
- **WebSocket Gateway** - Real-time control plane for device connections
- **Observability** - Integrated tracing via omniobserve

## Installation

```bash
go install github.com/agentplexus/omniagent/cmd/omniagent@latest
```

## Quick Start

### WhatsApp + OpenAI

The fastest way to get started is with WhatsApp and OpenAI:

```bash
# Set your OpenAI API key
export OPENAI_API_KEY="sk-..."

# Run with WhatsApp enabled
OMNIAGENT_AGENT_PROVIDER=openai \
OMNIAGENT_AGENT_MODEL=gpt-4o \
WHATSAPP_ENABLED=true \
omniagent gateway run
```

A QR code will appear in your terminal. Scan it with WhatsApp (Settings -> Linked Devices -> Link a Device) to connect.

### Configuration File

For more control, create a configuration file:

```yaml
# omniagent.yaml
gateway:
  address: "127.0.0.1:18789"

agent:
  provider: openai          # or: anthropic, gemini
  model: gpt-4o             # or: claude-sonnet-4-20250514, gemini-2.0-flash
  api_key: ${OPENAI_API_KEY}
  system_prompt: "You are OmniAgent, responding on behalf of the user."

channels:
  whatsapp:
    enabled: true
    db_path: "whatsapp.db"  # Session storage

  telegram:
    enabled: false
    token: ${TELEGRAM_BOT_TOKEN}

  discord:
    enabled: false
    token: ${DISCORD_BOT_TOKEN}

voice:
  enabled: true
  response_mode: auto        # auto, always, never
  stt:
    provider: deepgram
    model: nova-2
  tts:
    provider: deepgram
    model: aura-asteria-en
    voice_id: aura-asteria-en
```

Run with the config file:

```bash
omniagent gateway run --config omniagent.yaml
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `OPENAI_API_KEY` | OpenAI API key |
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `GEMINI_API_KEY` | Google Gemini API key |
| `OMNIAGENT_AGENT_PROVIDER` | LLM provider: `openai`, `anthropic`, `gemini` |
| `OMNIAGENT_AGENT_MODEL` | Model name (e.g., `gpt-4o`, `claude-sonnet-4-20250514`) |
| `WHATSAPP_ENABLED` | Set to `true` to enable WhatsApp |
| `WHATSAPP_DB_PATH` | WhatsApp session storage path |
| `TELEGRAM_BOT_TOKEN` | Telegram bot token (auto-enables Telegram) |
| `DISCORD_BOT_TOKEN` | Discord bot token (auto-enables Discord) |
| `DEEPGRAM_API_KEY` | Deepgram API key for voice STT/TTS |
| `OMNIAGENT_VOICE_ENABLED` | Set to `true` to enable voice processing |
| `OMNIAGENT_VOICE_RESPONSE_MODE` | Voice response mode: `auto`, `always`, `never` |

## CLI Commands

```bash
omniagent gateway run      # Start the gateway server
omniagent channels list    # List registered channels
omniagent channels status  # Show channel connection status
omniagent config show      # Display current configuration
omniagent version          # Show version information
```

## Architecture

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
|  - omnillm (LLM providers)                                  |
|  - omniobserve (tracing)                                    |
|  - Tools (browser, shell, http)                             |
+-------------------------------------------------------------+
```

## Configuration

OmniAgent can be configured via:

- YAML/JSON configuration file
- Environment variables
- CLI flags

See [Configuration Reference](docs/configuration.md) for details.

## Dependencies

| Package | Purpose |
|---------|---------|
| [omnichat](https://github.com/agentplexus/omnichat) | Unified messaging (WhatsApp, Telegram, Discord) |
| [omnillm](https://github.com/agentplexus/omnillm) | Multi-provider LLM abstraction |
| [omnivoice](https://github.com/agentplexus/omnivoice) | Voice STT/TTS interfaces |
| [omnivoice-deepgram](https://github.com/agentplexus/omnivoice-deepgram) | Deepgram voice provider |
| [omniobserve](https://github.com/agentplexus/omniobserve) | LLM observability |
| [Rod](https://github.com/go-rod/rod) | Browser automation |
| [gorilla/websocket](https://github.com/gorilla/websocket) | WebSocket server |

## Related Projects

- [omnichat](https://github.com/agentplexus/omnichat) - Unified messaging provider interface (WhatsApp, Telegram, Discord)
- [omnibrowser](https://github.com/agentplexus/omnibrowser) - Browser abstraction (planned)
- [omnivoice](https://github.com/agentplexus/omnivoice) - Voice interactions

## License

MIT License - see [LICENSE](LICENSE) for details.

 [build-status-svg]: https://github.com/agentplexus/omniagent/actions/workflows/ci.yaml/badge.svg?branch=main
 [build-status-url]: https://github.com/agentplexus/omniagent/actions/workflows/ci.yaml
 [lint-status-svg]: https://github.com/agentplexus/omniagent/actions/workflows/lint.yaml/badge.svg?branch=main
 [lint-status-url]: https://github.com/agentplexus/omniagent/actions/workflows/lint.yaml
 [goreport-svg]: https://goreportcard.com/badge/github.com/agentplexus/omniagent
 [goreport-url]: https://goreportcard.com/report/github.com/agentplexus/omniagent
 [docs-godoc-svg]: https://pkg.go.dev/badge/github.com/agentplexus/omniagent
 [docs-godoc-url]: https://pkg.go.dev/github.com/agentplexus/omniagent
 [license-svg]: https://img.shields.io/badge/license-MIT-blue.svg
 [license-url]: https://github.com/agentplexus/omniagent/blob/master/LICENSE
 [used-by-svg]: https://sourcegraph.com/github.com/agentplexus/omniagent/-/badge.svg
 [used-by-url]: https://sourcegraph.com/github.com/agentplexus/omniagent?badge
