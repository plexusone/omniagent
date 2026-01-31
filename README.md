# Envoy

[![Build Status][build-status-svg]][build-status-url]
[![Lint Status][lint-status-svg]][lint-status-url]
[![Go Report Card][goreport-svg]][goreport-url]
[![Docs][docs-godoc-svg]][docs-godoc-url]
[![License][license-svg]][license-url]

Your AI representative across communication channels.

Envoy is a personal AI assistant that routes messages across multiple communication platforms, processes them via an AI agent, and responds on your behalf.

## Features

- **Multi-Channel Support** - Telegram, Discord, Slack, WhatsApp, and more
- **AI-Powered Responses** - Powered by omnillm (Claude, GPT, Gemini, etc.)
- **Browser Automation** - Built-in browser control via Rod
- **WebSocket Gateway** - Real-time control plane for device connections
- **Observability** - Integrated tracing via omniobserve

## Installation

```bash
go install github.com/agentplexus/envoy/cmd/envoy@latest
```

## Quick Start

1. Create a configuration file:

```yaml
# envoy.yaml
gateway:
  address: "127.0.0.1:18789"

agent:
  provider: anthropic
  model: claude-sonnet-4-20250514
  api_key: ${ANTHROPIC_API_KEY}

channels:
  telegram:
    enabled: true
    token: ${TELEGRAM_BOT_TOKEN}
```

2. Start the gateway:

```bash
envoy gateway run --config envoy.yaml
```

## CLI Commands

```bash
envoy gateway run      # Start the gateway server
envoy channels list    # List registered channels
envoy channels status  # Show channel connection status
envoy config show      # Display current configuration
envoy version          # Show version information
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Messaging Channels                      │
│     Telegram  │  Discord  │  Slack  │  WhatsApp  │  ...     │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────┐
│              Gateway (WebSocket Control Plane)              │
│              ws://127.0.0.1:18789                           │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────┐
│                      Agent Runtime                          │
│  • omnillm (LLM providers)                                  │
│  • omniobserve (tracing)                                    │
│  • Tools (browser, shell, http)                             │
└─────────────────────────────────────────────────────────────┘
```

## Configuration

Envoy can be configured via:

- YAML/JSON configuration file
- Environment variables
- CLI flags

See [Configuration Reference](docs/configuration.md) for details.

## Dependencies

| Package | Purpose |
|---------|---------|
| [omnillm](https://github.com/agentplexus/omnillm) | Multi-provider LLM abstraction |
| [omniobserve](https://github.com/agentplexus/omniobserve) | LLM observability |
| [Rod](https://github.com/go-rod/rod) | Browser automation |
| [gorilla/websocket](https://github.com/gorilla/websocket) | WebSocket server |

## Related Projects

- [omnichat](https://github.com/agentplexus/omnichat) - Channel abstraction (planned)
- [omnibrowser](https://github.com/agentplexus/omnibrowser) - Browser abstraction (planned)
- [omnivoice](https://github.com/agentplexus/omnivoice) - Voice interactions

## License

MIT License - see [LICENSE](LICENSE) for details.

 [build-status-svg]: https://github.com/agentplexus/envoy/actions/workflows/ci.yaml/badge.svg?branch=main
 [build-status-url]: https://github.com/agentplexus/envoy/actions/workflows/ci.yaml
 [lint-status-svg]: https://github.com/agentplexus/envoy/actions/workflows/lint.yaml/badge.svg?branch=main
 [lint-status-url]: https://github.com/agentplexus/envoy/actions/workflows/lint.yaml
 [goreport-svg]: https://goreportcard.com/badge/github.com/agentplexus/envoy
 [goreport-url]: https://goreportcard.com/report/github.com/agentplexus/envoy
 [docs-godoc-svg]: https://pkg.go.dev/badge/github.com/agentplexus/envoy
 [docs-godoc-url]: https://pkg.go.dev/github.com/agentplexus/envoy
 [license-svg]: https://img.shields.io/badge/license-MIT-blue.svg
 [license-url]: https://github.com/agentplexus/envoy/blob/master/LICENSE
 [used-by-svg]: https://sourcegraph.com/github.com/agentplexus/envoy/-/badge.svg
 [used-by-url]: https://sourcegraph.com/github.com/agentplexus/envoy?badge