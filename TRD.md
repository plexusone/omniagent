# Technical Requirements Document: Skill System

## Architecture Overview

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
   │ YAML +      │   │ prompt with │   │  │WASM │ │ Deno │ │
   │ Markdown    │   │ skills      │   │  │(P3) │ │(P2)  │ │
   └─────────────┘   └─────────────┘   └───────────────────┘
```

## Phase 1: Skill Loader

### Data Structures

```go
// skills/skill.go

package skills

import (
    "encoding/json"
    "os"
    "os/exec"
    "path/filepath"
    "strings"

    "gopkg.in/yaml.v3"
)

// Skill represents a loaded SKILL.md
type Skill struct {
    // From YAML frontmatter
    Name        string       `yaml:"name"`
    Description string       `yaml:"description"`
    Homepage    string       `yaml:"homepage,omitempty"`
    Metadata    SkillMeta    `yaml:"metadata"`

    // Parsed from file
    Content     string       `yaml:"-"` // Markdown body
    Path        string       `yaml:"-"` // Directory path
    HasHooks    bool         `yaml:"-"` // Has hooks/ directory
    HasScripts  bool         `yaml:"-"` // Has scripts/ directory
}

// SkillMeta contains platform-specific metadata
type SkillMeta struct {
    OpenClaw *OpenClawMeta `json:"openclaw,omitempty"`
}

// OpenClawMeta is the openclaw-specific metadata block
type OpenClawMeta struct {
    Emoji    string     `json:"emoji,omitempty"`
    Requires *Requires  `json:"requires,omitempty"`
    Install  []Installer `json:"install,omitempty"`
    Always   bool       `json:"always,omitempty"`
}

// Requires specifies skill prerequisites
type Requires struct {
    Bins    []string `json:"bins,omitempty"`    // Required binaries
    AnyBins []string `json:"anyBins,omitempty"` // At least one required
    Env     []string `json:"env,omitempty"`     // Required env vars
}

// Installer specifies how to install dependencies
type Installer struct {
    ID      string   `json:"id"`
    Kind    string   `json:"kind"`              // brew, apt, go, npm, etc.
    Formula string   `json:"formula,omitempty"` // For brew
    Package string   `json:"package,omitempty"` // For apt
    Module  string   `json:"module,omitempty"`  // For go
    Bins    []string `json:"bins,omitempty"`
    Label   string   `json:"label,omitempty"`
}
```

### Skill Discovery

```go
// skills/loader.go

// DefaultSearchPaths returns the default skill directories
func DefaultSearchPaths() []string {
    home, _ := os.UserHomeDir()
    return []string{
        filepath.Join(home, ".omniagent", "skills"),
        "skills",
        ".skills",
    }
}

// Discover finds all skills in the given directories
func Discover(dirs []string) ([]*Skill, error) {
    var skills []*Skill
    seen := make(map[string]bool)

    for _, dir := range dirs {
        entries, err := os.ReadDir(dir)
        if err != nil {
            continue // Skip missing directories
        }

        for _, entry := range entries {
            if !entry.IsDir() {
                continue
            }

            skillPath := filepath.Join(dir, entry.Name())
            skillMD := filepath.Join(skillPath, "SKILL.md")

            if _, err := os.Stat(skillMD); err != nil {
                continue // No SKILL.md
            }

            skill, err := Load(skillPath)
            if err != nil {
                continue // Invalid skill
            }

            // Dedupe by name (first wins)
            if seen[skill.Name] {
                continue
            }
            seen[skill.Name] = true

            skills = append(skills, skill)
        }
    }

    return skills, nil
}

// Load parses a single skill from its directory
func Load(skillDir string) (*Skill, error) {
    content, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
    if err != nil {
        return nil, err
    }

    skill, err := Parse(string(content))
    if err != nil {
        return nil, err
    }

    skill.Path = skillDir

    // Check for hooks/scripts directories
    if _, err := os.Stat(filepath.Join(skillDir, "hooks")); err == nil {
        skill.HasHooks = true
    }
    if _, err := os.Stat(filepath.Join(skillDir, "scripts")); err == nil {
        skill.HasScripts = true
    }

    return skill, nil
}

// Parse extracts skill data from SKILL.md content
func Parse(content string) (*Skill, error) {
    // Split on YAML frontmatter delimiters
    parts := strings.SplitN(content, "---", 3)
    if len(parts) < 3 {
        return nil, fmt.Errorf("invalid SKILL.md: missing frontmatter")
    }

    var skill Skill
    if err := yaml.Unmarshal([]byte(parts[1]), &skill); err != nil {
        return nil, fmt.Errorf("invalid frontmatter: %w", err)
    }

    skill.Content = strings.TrimSpace(parts[2])
    return &skill, nil
}
```

### Requirement Checking

```go
// skills/requirements.go

// RequirementError describes a missing requirement
type RequirementError struct {
    Type     string // "bin" or "env"
    Name     string
    Skill    string
    Install  []Installer // How to fix
}

func (e *RequirementError) Error() string {
    return fmt.Sprintf("skill %q requires %s %q", e.Skill, e.Type, e.Name)
}

// CheckRequirements verifies all skill prerequisites
func (s *Skill) CheckRequirements() []error {
    var errs []error

    if s.Metadata.OpenClaw == nil || s.Metadata.OpenClaw.Requires == nil {
        return nil
    }

    req := s.Metadata.OpenClaw.Requires

    // Check required binaries
    for _, bin := range req.Bins {
        if _, err := exec.LookPath(bin); err != nil {
            errs = append(errs, &RequirementError{
                Type:    "binary",
                Name:    bin,
                Skill:   s.Name,
                Install: s.Metadata.OpenClaw.Install,
            })
        }
    }

    // Check anyBins (at least one must exist)
    if len(req.AnyBins) > 0 {
        found := false
        for _, bin := range req.AnyBins {
            if _, err := exec.LookPath(bin); err == nil {
                found = true
                break
            }
        }
        if !found {
            errs = append(errs, &RequirementError{
                Type:    "binary (any of)",
                Name:    strings.Join(req.AnyBins, ", "),
                Skill:   s.Name,
                Install: s.Metadata.OpenClaw.Install,
            })
        }
    }

    // Check required env vars
    for _, env := range req.Env {
        if os.Getenv(env) == "" {
            errs = append(errs, &RequirementError{
                Type:  "environment variable",
                Name:  env,
                Skill: s.Name,
            })
        }
    }

    return errs
}
```

### Prompt Injection

```go
// skills/inject.go

// InjectConfig controls how skills are injected
type InjectConfig struct {
    MaxSkills       int  // Maximum skills to inject (0 = unlimited)
    IncludeDisabled bool // Include skills with missing requirements
    Separator       string
}

// DefaultInjectConfig returns sensible defaults
func DefaultInjectConfig() InjectConfig {
    return InjectConfig{
        MaxSkills: 20,
        Separator: "\n\n---\n\n",
    }
}

// InjectIntoPrompt appends skill content to the system prompt
func InjectIntoPrompt(systemPrompt string, skills []*Skill, cfg InjectConfig) string {
    if len(skills) == 0 {
        return systemPrompt
    }

    var sb strings.Builder
    sb.WriteString(systemPrompt)
    sb.WriteString("\n\n# Available Skills\n\n")

    count := 0
    for _, skill := range skills {
        if cfg.MaxSkills > 0 && count >= cfg.MaxSkills {
            break
        }

        // Skip skills with missing requirements unless configured otherwise
        if !cfg.IncludeDisabled && len(skill.CheckRequirements()) > 0 {
            continue
        }

        sb.WriteString("## ")
        if skill.Metadata.OpenClaw != nil && skill.Metadata.OpenClaw.Emoji != "" {
            sb.WriteString(skill.Metadata.OpenClaw.Emoji)
            sb.WriteString(" ")
        }
        sb.WriteString(skill.Name)
        sb.WriteString("\n\n")
        sb.WriteString(skill.Content)
        sb.WriteString(cfg.Separator)

        count++
    }

    return sb.String()
}
```

### CLI Commands

```go
// cmd/omniagent/commands/skills.go

var skillsCmd = &cobra.Command{
    Use:   "skills",
    Short: "Manage skills",
}

var skillsListCmd = &cobra.Command{
    Use:   "list",
    Short: "List available skills",
    RunE: func(cmd *cobra.Command, args []string) error {
        discovered, err := skills.Discover(skills.DefaultSearchPaths())
        if err != nil {
            return err
        }

        for _, skill := range discovered {
            status := "✓"
            if errs := skill.CheckRequirements(); len(errs) > 0 {
                status = "✗"
            }

            emoji := ""
            if skill.Metadata.OpenClaw != nil {
                emoji = skill.Metadata.OpenClaw.Emoji
            }

            fmt.Printf("%s %s %s - %s\n", status, emoji, skill.Name, skill.Description)
        }
        return nil
    },
}

var skillsInfoCmd = &cobra.Command{
    Use:   "info <name>",
    Short: "Show skill details",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        // Find and display skill details
        // Show requirements, install instructions, etc.
    },
}
```

### Integration with Agent

```go
// In agent/agent.go or gateway.go

func (a *Agent) loadSkills() error {
    discovered, err := skills.Discover(skills.DefaultSearchPaths())
    if err != nil {
        return err
    }

    // Filter to available skills
    var available []*skills.Skill
    for _, skill := range discovered {
        if errs := skill.CheckRequirements(); len(errs) == 0 {
            available = append(available, skill)
            a.logger.Info("skill loaded", "name", skill.Name)
        } else {
            a.logger.Warn("skill unavailable", "name", skill.Name, "errors", errs)
        }
    }

    a.skills = available
    return nil
}

func (a *Agent) buildSystemPrompt() string {
    base := a.config.SystemPrompt
    return skills.InjectIntoPrompt(base, a.skills, skills.DefaultInjectConfig())
}
```

## Phase 2: Hook Runner (Deno)

### Architecture

```
┌─────────────────────────────────────────────┐
│  OmniAgent (Go)                             │
│                                             │
│  HookRunner.Run(hookPath, event)            │
│         │                                   │
└─────────┼───────────────────────────────────┘
          │
          ▼
┌─────────────────────────────────────────────┐
│  Deno Process                               │
│                                             │
│  deno run                                   │
│    --allow-read=/workspace                  │
│    --allow-env=CLAUDE_TOOL_OUTPUT           │
│    --import-map=omniagent-compat.json       │
│    --no-prompt                              │
│    hooks/openclaw/handler.ts                │
│                                             │
│  stdin: HookEvent JSON                      │
│  stdout: HookResult JSON                    │
└─────────────────────────────────────────────┘
```

### OpenClaw Compatibility Layer

```typescript
// omniagent-hooks-compat.ts
// Provides openclaw/hooks API compatibility

export interface HookEvent {
  type: 'agent' | 'tool' | 'message';
  action: string;
  sessionKey: string;
  context: HookContext;
}

export interface HookContext {
  bootstrapFiles: BootstrapFile[];
  workspaceDir?: string;
}

export interface BootstrapFile {
  path: string;
  content: string;
  virtual?: boolean;
}

export type HookHandler = (event: HookEvent) => Promise<void>;

// Wrapper that reads event from stdin, calls handler, writes result to stdout
export async function runHook(handler: HookHandler): Promise<void> {
  const input = await Deno.readAll(Deno.stdin);
  const event: HookEvent = JSON.parse(new TextDecoder().decode(input));

  await handler(event);

  // Write modified context back
  console.log(JSON.stringify(event.context));
}
```

### Import Map

```json
{
  "imports": {
    "openclaw/hooks": "./omniagent-hooks-compat.ts"
  }
}
```

## Phase 3: Tool Sandbox (WASM)

### wazero Integration

```go
// sandbox/wasm.go

package sandbox

import (
    "context"

    "github.com/tetratelabs/wazero"
    "github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type WASMSandbox struct {
    runtime wazero.Runtime
    config  WASMConfig
}

type WASMConfig struct {
    MemoryLimitMB int
    FuelLimit     uint64
    Timeout       time.Duration
}

func NewWASMSandbox(cfg WASMConfig) (*WASMSandbox, error) {
    ctx := context.Background()

    runtimeCfg := wazero.NewRuntimeConfig().
        WithMemoryLimitPages(uint32(cfg.MemoryLimitMB * 16)). // 64KB per page
        WithCloseOnContextDone(true)

    if cfg.FuelLimit > 0 {
        runtimeCfg = runtimeCfg.WithCostMeasurement()
    }

    r := wazero.NewRuntimeWithConfig(ctx, runtimeCfg)
    wasi_snapshot_preview1.MustInstantiate(ctx, r)

    return &WASMSandbox{runtime: r, config: cfg}, nil
}
```

## File Structure

```
omniagent/
├── skills/
│   ├── skill.go          # Data structures
│   ├── loader.go         # Discovery and parsing
│   ├── requirements.go   # Requirement checking
│   ├── inject.go         # Prompt injection
│   └── skill_test.go     # Tests
├── hooks/
│   ├── runner.go         # Deno hook execution
│   ├── compat.ts         # OpenClaw compatibility
│   ├── import-map.json   # Import mapping
│   └── runner_test.go    # Tests
├── sandbox/
│   ├── wasm.go           # WASM sandbox
│   ├── capabilities.go   # Permission model
│   └── wasm_test.go      # Tests
└── cmd/omniagent/commands/
    └── skills.go         # CLI commands
```

## Testing Strategy

### Phase 1 Tests

1. **Unit Tests**
   - Parse valid SKILL.md
   - Parse invalid SKILL.md (missing frontmatter)
   - Check requirements (binary exists/missing)
   - Check requirements (env var set/missing)
   - Prompt injection formatting

2. **Integration Tests**
   - Load sonoscli skill from OpenClaw
   - Load github skill from OpenClaw
   - Load self-improving-agent from ClawHub
   - Discover skills from multiple directories

3. **End-to-End Tests**
   - Agent with injected skills responds correctly
   - Skills CLI commands work

### Test Fixtures

Copy these skills for testing:

- `~/go/src/github.com/openclaw/openclaw/skills/sonoscli/`
- `~/go/src/github.com/openclaw/openclaw/skills/github/`
- `~/go/src/github.com/peterskoett/self-improving-agent/`
