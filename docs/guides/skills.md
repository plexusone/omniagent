# Skills Development

OmniAgent supports skills compatible with the [OpenClaw](https://github.com/openclaw/openclaw) SKILL.md format. Skills extend the agent's capabilities by injecting domain-specific instructions into the system prompt.

## Overview

Skills are Markdown files with YAML frontmatter that teach your agent how to perform specific tasks. They're injected into the LLM's system prompt at runtime.

## Skill Format

Skills are defined in `SKILL.md` files:

```markdown
---
name: weather
description: Get weather forecasts
metadata:
  emoji: "🌤️"
  requires:
    bins: ["curl"]
  install:
    - name: curl
      brew: curl
      apt: curl
---

# Weather Skill

You can check the weather using the `curl` command:

## Get Current Weather

```bash
curl "wttr.in/London?format=3"
```

## Get Detailed Forecast

```bash
curl "wttr.in/London"
```
```

## Skill Discovery

Skills are discovered from:

1. Built-in skills directory
2. `~/.omniagent/skills/`
3. Custom paths via `skills.paths` config

```yaml
skills:
  enabled: true
  paths:
    - ~/.omniagent/skills
    - /opt/shared-skills
  max_injected: 20
```

## Managing Skills

### List Skills

```bash
omniagent skills list
```

Output:
```
✓ 🎵 sonoscli - Control Sonos speakers via CLI
✓ 🐙 github - GitHub CLI operations
✗ ☀️ weather - Weather forecasts (missing: weather binary)
```

### Show Skill Details

```bash
omniagent skills info sonoscli
```

### Check Requirements

```bash
omniagent skills check
```

## Requirements

Skills can declare requirements that must be met:

### Binary Requirements

```yaml
metadata:
  requires:
    bins: ["gh", "jq"]  # All required
    anyBins: ["curl", "wget"]  # At least one required
```

### Environment Variables

```yaml
metadata:
  requires:
    env: ["GITHUB_TOKEN", "OPENAI_API_KEY"]
```

### Install Hints

```yaml
metadata:
  install:
    - name: gh
      brew: gh
      apt: gh
    - name: jq
      brew: jq
      apt: jq
```

## Creating a Skill

### 1. Create Directory

```bash
mkdir -p ~/.omniagent/skills/myskill
```

### 2. Create SKILL.md

```markdown
---
name: myskill
description: My custom skill
metadata:
  emoji: "🔧"
---

# My Skill

Instructions for the AI agent on how to use this skill...

## Available Commands

- `mytool list` - List all items
- `mytool add <name>` - Add a new item
```

### 3. Verify

```bash
omniagent skills list
omniagent skills info myskill
```

## Best Practices

### Keep Instructions Clear

Write instructions as if explaining to a human who knows nothing about your tool.

### Include Examples

Show concrete examples of commands and expected output.

### Declare Requirements

Always declare binary and environment requirements so users know what's needed.

### Use Emojis Sparingly

One emoji in the metadata helps identify skills visually.

## ClawHub Compatibility

OmniAgent is compatible with skills from [ClawHub](https://github.com/clawhub). Install skills using:

```bash
# Coming soon
bunx clawhub install sonoscli
```

Or manually clone to your skills directory:

```bash
git clone https://github.com/user/skill ~/.omniagent/skills/skill
```
