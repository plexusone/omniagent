# Release Notes - v0.1.0

**Release Date:** 2026-02-22

## Highlights

- Initial release with WebSocket gateway, AI agent runtime, and unified messaging via omnichat

## Added

- WebSocket gateway with client management and health checks ([`da77c73`](https://github.com/agentplexus/omniagent/commit/da77c73))
- AI agent runtime with omnillm multi-provider support (OpenAI, Anthropic, Gemini) ([`da77c73`](https://github.com/agentplexus/omniagent/commit/da77c73))
- Unified messaging via omnichat (WhatsApp, Telegram, Discord) ([`ac7b21f`](https://github.com/agentplexus/omniagent/commit/ac7b21f))
- WhatsApp provider with QR code linking ([`ac7b21f`](https://github.com/agentplexus/omniagent/commit/ac7b21f))
- LLM observability integration via omniobserve ([`ac7b21f`](https://github.com/agentplexus/omniagent/commit/ac7b21f))
- Tool execution loop with max 5 iterations ([`26e2cba`](https://github.com/agentplexus/omniagent/commit/26e2cba))
- Web search tool via omniserp ([`26e2cba`](https://github.com/agentplexus/omniagent/commit/26e2cba))
- Browser automation tool using Rod ([`da77c73`](https://github.com/agentplexus/omniagent/commit/da77c73))
- Shell execution tool with allowlist security ([`da77c73`](https://github.com/agentplexus/omniagent/commit/da77c73))
- Cobra-based CLI with gateway, channels, config, and version commands ([`da77c73`](https://github.com/agentplexus/omniagent/commit/da77c73))
- YAML/JSON configuration with environment variable overrides ([`6295af1`](https://github.com/agentplexus/omniagent/commit/6295af1))
- CI/CD pipeline with multi-platform testing ([`da77c73`](https://github.com/agentplexus/omniagent/commit/da77c73))

## Fixed

- Pin fetchup to v0.2.3 for go-rod/rod v0.116.2 compatibility ([`de7f46d`](https://github.com/agentplexus/omniagent/commit/de7f46d))
- Nil interface gotcha for observability hook when disabled ([`ac7b21f`](https://github.com/agentplexus/omniagent/commit/ac7b21f))

## Installation

```bash
go install github.com/agentplexus/omniagent/cmd/omniagent@v0.1.0
```

## Quick Start

```bash
# Set your API key
export OPENAI_API_KEY="sk-..."

# Run with WhatsApp enabled
OMNIAGENT_AGENT_PROVIDER=openai \
OMNIAGENT_AGENT_MODEL=gpt-4o \
WHATSAPP_ENABLED=true \
omniagent gateway run
```

## Full Changelog

See [CHANGELOG.md](CHANGELOG.md) for the complete changelog.
