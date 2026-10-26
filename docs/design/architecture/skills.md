# Skills System Architecture

The skills system enables OmniAgent to load and use skills compatible with [OpenClaw](https://github.com/openclaw/openclaw) and [ClawHub](https://github.com/clawhub).

## Overview

```
┌─────────────────────────────────────────────────────────────┐
│  OMNIAGENT (Go)                                             │
│                                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ Skill Loader │  │ Agent        │  │ Tool Dispatcher  │   │
│  │ (Phase 1)    │  │              │  │                  │   │
│  └──────┬───────┘  └──────┬───────┘  └────────┬─────────┘   │
│         │                 │                   │             │
└─────────┼─────────────────┼───────────────────┼─────────────┘
          │                 │                   │
          ▼                 ▼                   ▼
   ┌─────────────┐   ┌─────────────┐   ┌───────────────────┐
   │ Parse       │   │ LLM API     │   │ Execution Layer   │
   │ SKILL.md    │   │             │   │                   │
   │             │   │ System      │   │  ┌─────┐ ┌──────┐ │
   │ YAML +      │   │ prompt with │   │  │WASM │ │Docker│ │
   │ Markdown    │   │ skills      │   │  │     │ │      │ │
   └─────────────┘   └─────────────┘   └───────────────────┘
```

## Implementation Phases

### Phase 1: Skill Loader (Current)

Parses SKILL.md files and injects content into the LLM system prompt.

**Capabilities:**

- Parse YAML frontmatter + Markdown body
- Discover skills from multiple directories
- Check binary and environment requirements
- Inject into system prompt with emoji support

### Phase 2: Hook Runner (Planned)

Execute TypeScript/shell hooks via Deno runtime.

**Capabilities:**

- Run TypeScript hooks with restricted permissions
- OpenClaw hook API compatibility
- Shell script execution

### Phase 3: Tool Sandbox (Current)

Secure execution environment for tools.

**Capabilities:**

- WASM runtime via wazero
- Docker container isolation
- Capability-based permissions

## Data Structures

### Skill

```go
type Skill struct {
    // From YAML frontmatter
    Name        string
    Description string
    Homepage    string
    Metadata    SkillMeta

    // Parsed from file
    Content     string // Markdown body
    Path        string // Directory path
    HasHooks    bool   // Has hooks/ directory
    HasScripts  bool   // Has scripts/ directory
}
```

### SkillMeta

```go
type SkillMeta struct {
    Emoji    string
    Requires *Requires
    Install  []Installer
    Always   bool
}

type Requires struct {
    Bins    []string // Required binaries
    AnyBins []string // At least one required
    Env     []string // Required env vars
}
```

## Skill Discovery

Skills are discovered from configured directories:

```go
func DefaultSearchPaths() []string {
    home, _ := os.UserHomeDir()
    return []string{
        filepath.Join(home, ".omniagent", "skills"),
        "skills",
        ".skills",
    }
}
```

Discovery process:

1. Scan each directory for subdirectories
2. Check for `SKILL.md` file in each subdirectory
3. Parse and validate the skill
4. Deduplicate by name (first wins)

## Requirement Checking

Before loading a skill, requirements are validated:

```go
func (s *Skill) CheckRequirements() []error {
    // Check required binaries
    for _, bin := range req.Bins {
        if _, err := exec.LookPath(bin); err != nil {
            // Missing binary
        }
    }

    // Check anyBins (at least one must exist)
    // Check required env vars
}
```

Skills with missing requirements are skipped by default.

## Prompt Injection

Skills are injected into the system prompt:

```go
func InjectIntoPrompt(systemPrompt string, skills []*Skill, cfg InjectConfig) string {
    var sb strings.Builder
    sb.WriteString(systemPrompt)
    sb.WriteString("\n\n# Available Skills\n\n")

    for _, skill := range skills {
        sb.WriteString("## ")
        sb.WriteString(skill.Metadata.Emoji)
        sb.WriteString(" ")
        sb.WriteString(skill.Name)
        sb.WriteString("\n\n")
        sb.WriteString(skill.Content)
        sb.WriteString("\n\n---\n\n")
    }

    return sb.String()
}
```

## Configuration

```yaml
skills:
  enabled: true
  paths:
    - ~/.omniagent/skills
    - /opt/shared-skills
  disabled:
    - experimental-skill
  max_injected: 20
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable skill loading |
| `paths` | []string | `[]` | Additional skill directories |
| `disabled` | []string | `[]` | Skills to skip |
| `max_injected` | int | `20` | Max skills in prompt |

## CLI Commands

```bash
# List all discovered skills
omniagent skills list

# Show details for a specific skill
omniagent skills info sonoscli

# Check requirements for all skills
omniagent skills check
```

## File Structure

```
~/.omniagent/skills/
├── sonoscli/
│   └── SKILL.md
├── github/
│   └── SKILL.md
└── weather/
    └── SKILL.md
```
