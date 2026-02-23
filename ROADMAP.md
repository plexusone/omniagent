# Roadmap

This document outlines planned features and improvements for OmniAgent.

## v0.2.0 - Authentication & Security

- [ ] Implement proper WebSocket authentication (`gateway/handlers.go`)
- [ ] Add origin checking for WebSocket connections (`gateway/gateway.go`)
- [ ] Add API key validation for gateway access
- [ ] Add rate limiting for message processing

## v0.3.0 - Channel Improvements

- [ ] Handle reply_to for Telegram messages (`channels/adapters/telegram/telegram.go`)
- [ ] Add Slack adapter
- [ ] Add WhatsApp adapter (via WhatsApp Business API)
- [ ] Add channel-specific message formatting

## v0.4.0 - Agent Enhancements

- [ ] Implement memory-aware processing using omnillm memory features (`agent/agent.go`)
- [ ] Add conversation summarization for long sessions
- [ ] Add persistent session storage (SQLite/PostgreSQL)
- [ ] Add tool result caching

## v0.5.0 - Observability & Monitoring

- [ ] Integrate omniobserve for LLM tracing
- [ ] Add Prometheus metrics endpoint
- [ ] Add structured logging with log levels
- [ ] Add health check endpoints with detailed status

## v0.3.0 - Skill System (Phase 1)

- [ ] Implement skill loader (SKILL.md parsing)
- [ ] Skill discovery from configurable directories
- [ ] Requirement checking (bins, env vars)
- [ ] Prompt injection for loaded skills
- [ ] CLI commands: `skills list`, `skills info`, `skills check`
- [ ] ClawHub/OpenClaw skill compatibility

## v0.4.0 - Hook Runner (Phase 2)

- [ ] Deno-based TypeScript hook execution
- [ ] Shell script hook execution
- [ ] OpenClaw hook API compatibility layer
- [ ] Permission restrictions per skill

## v0.5.0 - Tool Sandbox (Phase 3)

- [ ] WASM runtime via wazero
- [ ] Capability-based permissions
- [ ] Memory and CPU limits
- [ ] Built-in tools in WASM sandbox

## Future

- [ ] Multi-tenant support
- [ ] Web UI for configuration and monitoring
- [ ] Voice channel support via omnivoice
- [ ] Integration with omnichat for unified channel abstraction
- [ ] Integration with omnibrowser for enhanced browser automation

## Related Projects

| Project | Status | Purpose |
|---------|--------|---------|
| [omnillm](https://github.com/agentplexus/omnillm) | Active | Multi-provider LLM abstraction |
| [omniobserve](https://github.com/agentplexus/omniobserve) | Active | LLM observability |
| [omnichat](https://github.com/agentplexus/omnichat) | Planned | Channel abstraction |
| [omnibrowser](https://github.com/agentplexus/omnibrowser) | Planned | Browser abstraction |
| [omnivoice](https://github.com/agentplexus/omnivoice) | Active | Voice interactions |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for how to propose features or submit pull requests.
