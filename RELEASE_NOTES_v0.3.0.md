# Release Notes: v0.3.0

**Release Date:** 2026-02-22

## Overview

OmniAgent v0.3.0 introduces a powerful skill system compatible with OpenClaw/ClawHub and layered sandboxing for secure tool execution. This release significantly expands the agent's extensibility while maintaining security through WASM and Docker isolation.

## Highlights

- **Skill System** - Load and use skills from the OpenClaw/ClawHub ecosystem
- **WASM Sandboxing** - Lightweight isolation using wazero runtime
- **Docker Sandboxing** - OS-level container isolation for CLI tools

## New Features

### Skill System

OmniAgent now supports skills compatible with the [OpenClaw](https://github.com/openclaw/openclaw) SKILL.md format, enabling you to extend your agent with domain-specific capabilities.

**Key capabilities:**

- Parse SKILL.md files with YAML frontmatter and metadata
- Discover skills from multiple directories with deduplication
- Check requirements (binaries, environment variables) with install hints
- Inject skill instructions into the system prompt

**CLI commands:**

```bash
omniagent skills list      # List all discovered skills
omniagent skills info NAME # Show skill details and requirements
omniagent skills check     # Validate all skill requirements
```

**Configuration:**

```yaml
skills:
  enabled: true
  paths:
    - ~/.omniagent/skills
    - /opt/skills
  disabled:
    - experimental-skill
  max_injected: 20
```

### WASM Sandbox

A lightweight sandbox using [wazero](https://github.com/tetratelabs/wazero) provides capability-based isolation for tool execution.

**Capabilities:**

| Capability | Description |
|------------|-------------|
| `fs_read` | Read files from allowed paths |
| `fs_write` | Write files to allowed paths |
| `net_http` | Make HTTP requests to allowed hosts |
| `exec_run` | Execute allowed commands |

**Security features:**

- Memory limits (default 16MB, max 4GB)
- Timeout enforcement via context
- Path validation with symlink resolution
- Command allowlist enforcement

### Docker Sandbox

For stronger OS-level isolation, tools can run inside Docker containers with security hardening.

**Default security settings:**

- `NetworkMode: none` - No network access
- `CapDrop: ALL` - Drop all Linux capabilities
- `ReadonlyRootfs: true` - Read-only root filesystem
- `SecurityOpt: no-new-privileges` - Prevent privilege escalation
- Memory and CPU limits

**Volume mounts** allow controlled filesystem access:

```go
Mounts: []sandbox.DockerMount{
    {HostPath: "/data/input", ContainerPath: "/input", ReadOnly: true},
    {HostPath: "/data/output", ContainerPath: "/output", ReadOnly: false},
}
```

## Bug Fixes

- Config tests now clear environment variables to prevent interference from user's shell environment

## Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/tetratelabs/wazero` | v1.11.0 | WASM runtime |
| `github.com/moby/moby/client` | v0.2.2 | Docker SDK |
| `github.com/moby/moby/api` | v1.53.0 | Docker API types |

## Upgrade Guide

### From v0.2.0

This release is backwards compatible. No configuration changes are required.

**To enable skills:**

1. Skills are enabled by default
2. Place SKILL.md files in `~/.omniagent/skills/` or configure custom paths
3. Run `omniagent skills list` to verify discovery

**To use sandboxing:**

Sandboxing is available programmatically for tool developers. See the `sandbox/` package documentation.

## Known Issues

- The `testdata/skills/self-improving-agent` directory is committed as a git submodule reference. Clone the repository separately if needed.
- Docker sandbox tests are skipped when Docker is not available

## What's Next (v0.4.0)

- Sandboxed shell tool integration with agent
- Deno runtime for TypeScript skill hooks
- Skill marketplace integration

## Full Changelog

See [CHANGELOG.md](CHANGELOG.md) for the complete list of changes.

## Contributors

- AgentPlexus Team
- Claude Opus 4.5 (AI pair programmer)
