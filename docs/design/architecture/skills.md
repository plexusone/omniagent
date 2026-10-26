# Skills System Architecture

A skill gives the agent knowledge or tools. OmniAgent supports four kinds, which
differ in how they are defined and in what they give the model.

| Kind | Defined by | Gives the model | Registered with |
|------|------------|-----------------|-----------------|
| Markdown | A `SKILL.md` file ([OpenClaw](https://github.com/openclaw/openclaw) and [ClawHub](https://github.com/clawhub) compatible) | Instructions added to the system prompt | `skills.Manager`, configured with packs and directories |
| Compiled | A Go type that implements `compiled.Skill` | Tools | `agent.WithCompiledSkill` |
| MCP | An MCP server run as a subprocess | The server's tools | `agent.WithMCPSkill` |
| OpenAPI | An OpenAPI 3.x document | One tool per operation | `agent.WithOpenAPISkill` |

Markdown skills shape what the model knows. The other three give it things it
can call. For how to write and configure skills, see the
[Skills guide](../../guides/skills.md).

## Overview

```
                        +-------------------------+
 SKILL.md files   --->  | skills.Manager          |  ---> system prompt
 (packs, dirs)          | discover, filter, check |
                        +-------------------------+

 compiled.Skill   --->  +-------------------------+
 MCP server       --->  | agent.RegisterCompiled  |  ---> tool list
 OpenAPI spec     --->  | Skill / InitCompiled    |       (one tool wrapper
                        | Skills                  |        per skill tool)
                        +-------------------------+
                                    |
                          secrets, storage, agent
                          injected by capability
```

## Markdown Skills

A markdown skill is a directory with a `SKILL.md`: YAML frontmatter followed by
a Markdown body. The loader parses it, and the agent injects the body into the
system prompt.

### Data structures

```go
type Skill struct {
    // From YAML frontmatter
    Name        string
    Description string
    Homepage    string
    Metadata    SkillMeta

    // Parsed from the directory
    Content    string // Markdown body
    Path       string // Directory path
    HasHooks   bool   // Has a hooks/ directory
    HasScripts bool   // Has a scripts/ directory
}

type SkillMeta struct {
    Emoji    string
    Requires *Requires
    Install  []Installer
    Always   bool
}

type Requires struct {
    Bins    []string            // Required binaries
    AnyBins []string            // At least one required
    Env     []string            // Required env vars
    Secrets []SecretRequirement // Declared secrets, set per agent
}
```

### Discovery

`skills.Manager` collects skills from three sources and applies the filters:

1. **Packs** - skill sets embedded in the binary (`skills.packs`)
2. **Directories** - `~/.omniagent/skills`, `skills` and `.skills`, plus
   `skills.paths`
3. **Includes and excludes** - `skills.includes` limits the set;
   `skills.excludes` removes names after that

A skill found in a directory overrides an embedded skill with the same name.
Within the directories, the first skill with a given name wins.

### Requirements

Before a skill is offered, its requirements are checked: every binary in
`bins`, at least one of `anyBins`, and every variable in `env`. A skill with an
unmet requirement is skipped.

Secrets are handled separately. A skill declares the secrets it needs in
`requires.secrets`, and the owner of the agent supplies the values. A skill
whose required secret has no value is left out of that agent's prompt instead
of failing later. Secret values are injected into the skill as environment
variables and are never part of the prompt.

### Prompt injection

`InjectIntoPrompt` appends the available skills to the system prompt, each
under a heading with its emoji, up to `skills.max_injected`:

```go
func InjectIntoPrompt(systemPrompt string, skills []*Skill, cfg InjectConfig) string
```

### Hooks and scripts

The loader records whether a skill has a `hooks/` or `scripts/` directory and
`omniagent skills info` shows it. OmniAgent does not run those hooks. A skill's
tool use goes through the compiled, MCP or OpenAPI kinds instead.

### Configuration

```yaml
skills:
  enabled: true
  packs: [omniagent-skills]
  paths:
    - ~/.omniagent/skills
    - /opt/shared-skills
  includes: []
  excludes:
    - experimental-skill
  max_injected: 20
  config:
    github:
      secrets:
        GITHUB_TOKEN: "op://Private/GitHub/token"
```

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | bool | Enable skill loading |
| `packs` | []string | Embedded skill packs to load |
| `paths` | []string | Additional skill directories |
| `includes` | []string | Load only these skills |
| `excludes` | []string | Skip these skills (`disabled` is a deprecated alias) |
| `max_injected` | int | Maximum skills in the prompt |
| `config.<name>.secrets` | map | Bind a skill's declared secret names to values or vault URIs |

## Compiled Skills

A compiled skill is Go code. It implements a small interface and exposes its
tools:

```go
type Skill interface {
    Name() string
    Description() string
    Tools() []skill.Tool
    Init(ctx context.Context) error
    Close() error
}
```

The agent registers each tool through a wrapper that adapts it to the agent's
tool interface and records where the tool came from, so a tool call can be
traced to its skill. `Init` runs from `InitCompiledSkills`, and the tools a
skill discovers during `Init` are registered then.

A skill opts into what the agent can give it by implementing optional
interfaces. The agent checks for each with a type assertion:

| Interface | The agent calls | Purpose |
|-----------|-----------------|---------|
| `StorageAware` | `SetStorage(kvs.Store)` | A key-value store for the skill's state |
| `AgentAware` | `SetAgent(agent)` | The agent that owns the skill |
| `SecretsAware` | `SetSecrets(map[string]string)` | The agent's resolved secrets as env values |
| `SecretRequirer` | reads `RequiredSecrets()` | Names the skill cannot run without; the skill is skipped when one is missing |
| `SourceType() string` | reads it | Labels the tool source, such as `mcp` |

## MCP Skills

An MCP skill runs a Model Context Protocol server as a subprocess and offers
the server's tools. It implements `SecretsAware` and `SecretRequirer`:
injected secrets are merged into the subprocess environment over any static
`Env`, and a server's `RequiredEnv` names gate it. The skill builds a fresh
environment per agent instance, so one agent's secrets never leak into
another's server.

## OpenAPI Skills

An OpenAPI skill loads an OpenAPI 3.x document and offers one tool per
operation. Parameters, arrays and request bodies are serialized as the spec
describes, and the response comes back as the tool result.

- **Spec source.** A file path, or an `http` or `https` URL. A URL spec is
  fetched, together with the documents it references, with the skill's own HTTP
  client. The fetch follows no redirect and refuses a document over 10 MiB. A
  fetched spec may reference only `http` and `https` URLs, never local files.
- **Authentication.** A static API key, bearer token or basic credentials, taken
  from the skill's config or injected through its secret environment variables
  (`APIKeyEnv`, `TokenEnv`, `PasswordEnv`).
- **Identity.** Give the skill an `HTTPClient` from `aauthagent` and every call
  is signed as an AAuth agent, with the person approving what the API asks them
  to approve.

### AAuth

`aauthagent.NewClient` enrolls a key at an agent provider and returns an HTTP
client. Its transport signs each request, and when a resource answers with a
requirement the client obtains person and auth tokens from the person server
named in the agent token, then retries. When the person must act, the approval
URL and code reach `Config.OnInteraction` while the request waits. There is no
client-level timeout, because a request may wait for the person; bound it with
the request context. See the [AAuth guide](../../guides/aauth/index.md).

## Tool Source Identity

Every tool carries its source kind (`skill` by default, `mcp` for MCP skills),
the skill name and the tool's own name. Hooks and observability use this to
attribute a tool call to the skill that provided it.

## CLI

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
