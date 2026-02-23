# Product Requirements Document: Skill System

## Overview

OmniAgent needs to support loading and executing skills compatible with OpenClaw/ClawHub to enable extensible AI agent capabilities without skill fragmentation across the ecosystem.

## Problem Statement

Currently, OmniAgent has:

- A static tool registration system
- No dynamic skill/plugin loading
- No way to leverage the growing ClawHub skill ecosystem

Users want to:

- Install skills from ClawHub (`bunx clawhub install sonoscli`)
- Use the same skills across OpenClaw, NanoClaw, IronClaw, and OmniAgent
- Avoid maintaining separate skill versions for each platform

## Goals

1. **Full ClawHub Compatibility**: Load and use ClawHub skills without modification
2. **Security**: Execute skill code in sandboxed environments (Deno for TypeScript/shell, WASM for tools)
3. **Minimal Overhead**: Markdown-only skills should load with zero runtime cost
4. **Incremental Adoption**: Users can start with simple skills and adopt hooks/scripts later

## Non-Goals

- Creating a competing skill format
- Building a skill marketplace (use ClawHub)
- Docker-based sandboxing (use Deno + WASM instead)

## User Stories

### US-1: Load Markdown-Only Skills

As a user, I want to install a ClawHub skill and have OmniAgent automatically include its instructions in my agent's context.

**Acceptance Criteria:**

- Skills are discovered from `~/.omniagent/skills/` and `./skills/`
- SKILL.md files are parsed (YAML frontmatter + Markdown body)
- Skill content is injected into the LLM system prompt
- Requirements (bins, env vars) are checked before loading

### US-2: Use CLI-Based Skills

As a user, I want to use skills like `sonoscli` or `github` that teach my agent how to use CLI tools.

**Acceptance Criteria:**

- Skill requirements are validated (e.g., `gh` binary exists)
- Missing requirements show clear error messages with install instructions
- Agent can execute CLI commands via the shell tool

### US-3: Run Skill Hooks (Phase 2)

As a user, I want skills with hooks (like `self-improving-agent`) to execute their TypeScript/shell hooks.

**Acceptance Criteria:**

- TypeScript hooks run via Deno with restricted permissions
- Shell hooks run via Deno with `--allow-run` restrictions
- OpenClaw hook API is compatible via import map

### US-4: Secure Tool Execution (Phase 3)

As a user, I want tool execution to be sandboxed for security.

**Acceptance Criteria:**

- Built-in tools run in WASM sandbox (wazero)
- Capability-based permissions for file/network access
- Memory and CPU limits enforced

## Feature Phases

### Phase 1: Skill Loader (Markdown-Only)

- Parse SKILL.md format
- Discover skills from configured directories
- Check requirements (bins, env)
- Inject into LLM system prompt
- CLI command: `omniagent skills list`, `omniagent skills info <name>`

### Phase 2: Hook Runner (Deno)

- Run TypeScript hooks via Deno
- Run shell hooks via Deno
- OpenClaw hook API compatibility layer
- Permission restrictions per skill

### Phase 3: Tool Sandbox (WASM)

- WASM runtime via wazero
- Capability-based permissions
- Built-in tools (file, http) in WASM
- Memory/CPU limits

## Test Skills

### Phase 1 Test Cases

| Skill | Type | Requirements | Source |
|-------|------|--------------|--------|
| `sonoscli` | CLI | `sonos` binary | OpenClaw bundled |
| `github` | CLI | `gh` binary | OpenClaw bundled |
| `weather` | CLI | `weather` binary | OpenClaw bundled |
| `slack` | CLI | None (API-based) | OpenClaw bundled |

### Phase 2 Test Cases

| Skill | Type | Requirements | Source |
|-------|------|--------------|--------|
| `self-improving-agent` | Hooks + Scripts | None | ClawHub |

## Success Metrics

- 100% of ClawHub Markdown-only skills load without modification
- <100ms skill loading time for 50 skills
- Zero security incidents from sandboxed execution

## Dependencies

- Deno runtime (for Phase 2)
- wazero library (for Phase 3)

## Timeline

| Phase | Description | Target |
|-------|-------------|--------|
| 1 | Skill Loader | v0.3.0 |
| 2 | Hook Runner | v0.4.0 |
| 3 | Tool Sandbox | v0.5.0 |
