# Tasks: Skill System Implementation

## Phase 1: Skill Loader (Markdown-Only)

### Setup

- [x] **TASK-001**: Create `skills/` package directory structure
  - Create `skills/skill.go` with data structures
  - Create `skills/loader.go` stub
  - Create `skills/requirements.go` stub
  - Create `skills/inject.go` stub

- [x] **TASK-002**: Copy test fixtures
  - Copy `sonoscli` skill from OpenClaw to `testdata/skills/sonoscli/`
  - Copy `github` skill from OpenClaw to `testdata/skills/github/`
  - Copy `self-improving-agent` skill to `testdata/skills/self-improving-agent/`

### Core Implementation

- [x] **TASK-003**: Implement SKILL.md parser
  - Parse YAML frontmatter (between `---` delimiters)
  - Handle nested JSON in `metadata` field
  - Extract Markdown body
  - Write unit tests

- [x] **TASK-004**: Implement skill discovery
  - Scan directories for `SKILL.md` files
  - Support multiple search paths
  - Deduplicate by skill name (first wins)
  - Write unit tests

- [x] **TASK-005**: Implement requirement checking
  - Check for required binaries via `exec.LookPath`
  - Check for required env vars via `os.Getenv`
  - Support `anyBins` (at least one)
  - Return structured errors with install instructions
  - Write unit tests

- [x] **TASK-006**: Implement prompt injection
  - Append skill content to system prompt
  - Include emoji if present
  - Skip skills with missing requirements
  - Configurable max skills limit
  - Write unit tests

### CLI Commands

- [x] **TASK-007**: Add `omniagent skills list` command
  - Show all discovered skills
  - Indicate status (available/unavailable)
  - Show emoji and description

- [x] **TASK-008**: Add `omniagent skills info <name>` command
  - Show full skill details
  - Show requirements and status
  - Show install instructions if missing

- [x] **TASK-009**: Add `omniagent skills check` command
  - Validate all skills
  - Report missing requirements
  - Suggest install commands

### Agent Integration

- [x] **TASK-010**: Integrate skill loader into Agent
  - Load skills on agent startup
  - Inject into system prompt
  - Log loaded/skipped skills

- [x] **TASK-011**: Add skill configuration
  - `skills.paths` - Additional search directories
  - `skills.disabled` - Skills to skip (TODO: implement filtering)
  - `skills.maxInjected` - Limit skills in prompt

### Testing

- [x] **TASK-012**: Write integration tests
  - Load real OpenClaw skills
  - Verify parsing correctness
  - Test with agent

- [ ] **TASK-013**: End-to-end test with LLM
  - Inject skills into prompt
  - Verify LLM can use skill instructions
  - Test CLI tool invocation

---

## Phase 2: Tool Sandbox (WASM) - PRIORITIZED

> Prioritized over Deno hooks because WASM secures existing tool execution
> and most skills don't require TypeScript hooks.

### Setup

- [x] **TASK-030**: Add wazero dependency
  - Add `github.com/tetratelabs/wazero` to go.mod
  - Verify version compatibility (v1.11.0)

- [x] **TASK-031**: Create `sandbox/` package
  - Create `sandbox/sandbox.go` with core types
  - Create `sandbox/runtime.go` for WASM runtime
  - Create `sandbox/host.go` for host functions

### Implementation

- [x] **TASK-032**: Implement WASM runtime wrapper
  - Initialize wazero runtime with resource limits
  - Support module compilation and caching
  - Handle stdin/stdout for tool I/O

- [x] **TASK-033**: Implement capability-based permissions
  - Define capabilities: `fs_read`, `fs_write`, `net_http`, `exec_run`
  - Capability checking before host function calls
  - Per-tool capability configuration

- [x] **TASK-034**: Implement resource limits
  - Memory limit (default 16MB, max 4GB)
  - Output size limit (default 1MB)
  - Timeout enforcement via context

- [x] **TASK-035**: Implement host functions
  - `FSRead(path) -> bytes` - Read file (if fs_read capability)
  - `FSWrite(path, bytes)` - Write file (if fs_write capability)
  - `HTTPFetch(url, method, body) -> response` - HTTP request (if net_http capability)
  - `ExecRun(cmd, args) -> output` - Run command (if exec_run capability)

- [ ] **TASK-036**: Create sandboxed shell tool
  - Wrap existing shell tool with sandbox host functions
  - Restrict to allowed commands
  - Integrate with agent tool registry

### Testing

- [x] **TASK-037**: Unit tests for sandbox
  - Test capability enforcement
  - Test path validation
  - Test timeout handling
  - Test command allowlist

- [ ] **TASK-038**: Integration tests
  - Execute simple WASM module
  - Test host function calls from WASM
  - Verify sandbox isolation

- [ ] **TASK-039**: Benchmark WASM vs native
  - Compare execution time
  - Measure memory overhead

### Docker Sandbox

- [x] **TASK-040**: Add Docker sandbox support
  - Add moby/moby SDK dependency (client v0.2.2, api v1.53.0)
  - Create `sandbox/docker.go` with DockerSandbox implementation
  - Support volume mounts for filesystem access
  - Integrate with app-level permission checks
  - Configure security options (CapDrop, ReadonlyRootfs, etc.)
  - Add unit tests in `sandbox/docker_test.go`

---

## Test Skills Reference

### Phase 1 Test Skills (Markdown-Only)

| Skill | Source | Requirements | Notes |
|-------|--------|--------------|-------|
| `sonoscli` | OpenClaw | `sonos` binary | CLI tool skill |
| `github` | OpenClaw | `gh` binary | CLI tool skill |
| `weather` | OpenClaw | TBD | CLI tool skill |
| `slack` | OpenClaw | None | API-based skill |

### Phase 2 Test Skills (With Hooks)

| Skill | Source | Requirements | Notes |
|-------|--------|--------------|-------|
| `self-improving-agent` | ClawHub | None | Has TS hooks + shell scripts |

### Skill Locations

```
OpenClaw skills:
~/go/src/github.com/openclaw/openclaw/skills/

ClawHub skill (cloned):
~/go/src/github.com/peterskoett/self-improving-agent/
```

---

## Current Progress

**Started**: 2026-02-22
**Phase**: 2 - WASM + Docker Sandbox
**Status**: Core sandbox implemented (7/10 tasks done)

### Phase 1 Complete (Skills)

- Created `skills/` package with full implementation
- SKILL.md parser with YAML frontmatter and metadata support
- Skill discovery from multiple directories
- Requirement checking (bins, env vars) with install hints
- Prompt injection with emoji support
- CLI commands: `skills list`, `skills info`, `skills check`
- Agent integration with skill loading on startup
- Configuration: `skills.enabled`, `skills.paths`, `skills.maxInjected`

### Phase 2 In Progress (WASM + Docker Sandbox)

- Added wazero v1.11.0 dependency
- Created `sandbox/` package with:
  - `sandbox.go` - Config, Capability, Result types
  - `runtime.go` - WASM runtime wrapper with memory limits
  - `host.go` - Host functions (FSRead, FSWrite, HTTPFetch, ExecRun)
  - `sandbox_test.go` - Unit tests for all capabilities
- Capability-based permission model
- Path validation with symlink resolution
- Command allowlist enforcement
- Output size limiting
- **Docker sandbox** (TASK-040 complete):
  - Added moby/moby SDK (client v0.2.2, api v1.53.0)
  - `docker.go` - DockerSandbox with container isolation
  - Volume mounts for filesystem access
  - App-level + Docker-level security (layered)
  - Security hardening (CapDrop ALL, ReadonlyRootfs, no-new-privileges)
  - `docker_test.go` - Unit and integration tests

### Next Actions

1. Create sandboxed shell tool (TASK-036)
2. Integration tests with WASM modules (TASK-038)
3. Benchmark WASM vs native (TASK-039)
