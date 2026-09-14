# Deployment

Deploy OmniAgent-based applications to cloud infrastructure.

## Overview

OmniAgent applications can be deployed as:

1. **Standalone binary** - Direct deployment to VMs
2. **Container** - Docker deployment to container services
3. **Managed service** - Using OmniDeploy for automated infrastructure

This guide covers containerized deployment using [OmniDeploy](https://github.com/plexusone/omnideploy)
for a **single-operator** (personal-mode) deployment. For a self-hosted
**team-mode** stack (Caddy + PostgreSQL via Docker Compose on one VM), see
[Team Deployment](team-deployment.md) instead.

## Deployment Checklist

Follow these steps to deploy your OmniAgent application to AWS LightSail.

### Prerequisites

Before starting, ensure you have:

- [ ] Go 1.26+ installed
- [ ] Docker installed and running
- [ ] AWS CLI configured
- [ ] GitHub account with a Personal Access Token (PAT) for GHCR

### Step 1: Prepare Deployment Files

Ensure your project has these files:

| File | Purpose |
|------|---------|
| `Dockerfile` | Multi-stage container build |
| `deploy.yaml` | OmniDeploy configuration |
| `.github/workflows/build.yaml` | CI container build (optional) |

### Step 2: Commit and Push to GitHub

```bash
# Add deployment files
git add Dockerfile deploy.yaml .github/

# Commit
git commit -m "build: add Docker and OmniDeploy deployment configuration"

# Push to trigger CI build
git push origin main
```

### Step 3: Build Container Image

**Option A: Via GitHub Actions (recommended)**

Pushing a version tag (`vX.Y.Z`) triggers `.github/workflows/docker.yaml`
automatically (RMI-OMNIAGENT-003), publishing
`ghcr.io/<owner>/<repo>:vX.Y.Z`, `:X.Y`, and `:latest`. Trigger a one-off
build without a tag via **Actions → Docker Build & Publish → Run
workflow**, which publishes `:latest` plus a `:smoke` tag.

**Option B: Build locally**

```bash
# Start Docker if not running
open -a Docker  # macOS

# Build for linux/amd64 (required for Lightsail)
docker build --platform linux/amd64 -t ghcr.io/<owner>/<repo>:latest .

# Login to GHCR (see GHCR Authentication below)
echo $GITHUB_TOKEN | docker login ghcr.io -u <username> --password-stdin

# Push
docker push ghcr.io/<owner>/<repo>:latest
```

### GHCR Authentication

GitHub Container Registry (GHCR) uses a standard GitHub Personal Access Token (PAT).

**Create a token:**

1. Go to https://github.com/settings/tokens
2. Click **Generate new token (classic)**
3. Select scopes: `write:packages`, `read:packages`, `delete:packages`
4. Copy the token (shown once)

**Login:**

```bash
# Store token in environment (don't commit)
export GITHUB_TOKEN="ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

# Login to GHCR
echo $GITHUB_TOKEN | docker login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin
```

**Make the package public (one-time, after first push):**

1. Go to https://github.com/orgs/<org>/packages (or your user packages page)
2. Find the package and open **Package settings**
3. Under **Danger Zone**, click **Change visibility** → **Public**

Once public, Lightsail can pull the image without registry credentials, which sidesteps the need for `PrivateRegistryAccess` configuration.

### Step 4: Install OmniDeploy

```bash
go install github.com/plexusone/omnideploy/cmd/omnideploy@latest
```

Verify installation:

```bash
omnideploy --version
```

### Step 5: Configure AWS Credentials and the Pulumi Backend

Create a dedicated IAM user with a least-privilege policy rather than
using broad account credentials. The deploy needs:

- `lightsail:*` (Lightsail is a self-contained blast radius)
- `ssm:PutParameter`/`GetParameter`/`GetParameters`/`GetParametersByPath`/
  `DeleteParameter`/`AddTagsToResource` scoped to
  `arn:aws:ssm:<region>:*:parameter/<app-name>/*`
- `kms:Encrypt`/`kms:Decrypt` conditioned on
  `kms:ViaService: ssm.<region>.amazonaws.com`

Store the key as a named profile so it's clear what it deploys:

```bash
aws configure --profile <app-name>-omnideploy
export AWS_PROFILE="<app-name>-omnideploy"
```

!!! tip "Region choice"
    Lightsail's bundled pricing is identical in every supported region, so
    there is no cost reason to pick `us-east-1` — and it carries an
    outsized incident-history blast radius as AWS's oldest and most
    complex region. Prefer `us-west-2` (or `us-east-2`): same price tier
    now *and* for a later ECS/Fargate/RDS migration. Note Lightsail is not
    offered in `us-west-1`.

`omnideploy`'s Pulumi backend also needs a state backend and passphrase.
For a first deploy, local file state with an empty passphrase is fine
(no secrets are stored in stack config — they pass through the
environment):

```bash
export PULUMI_BACKEND_URL="file://$HOME/.pulumi-state-<app-name>"
export PULUMI_CONFIG_PASSPHRASE=""
```

### Step 6: Provide Secrets

Store each secret in SSM Parameter Store as a `SecureString` (the IAM
policy from Step 5 already scopes access to `/<app-name>/*`):

```bash
aws ssm put-parameter \
    --name "/<app-name>/anthropic-api-key" \
    --value "sk-ant-..." \
    --type SecureString
```

Reference them from `deploy.secrets` (RMI-OMNIAGENT-006):

```yaml
deploy:
  secrets:
    - name: ANTHROPIC_API_KEY
      source: ssm:/<app-name>/anthropic-api-key
```

Supported sources are `env:VAR` (deploying shell), `ssm:/path`
(SecureString, decrypted at deploy time), and `secretsmanager:name`.
Every ref must resolve to a non-empty value — a missing secret fails the
deploy up front instead of shipping a container with a silently absent
credential. Resolved values are injected as Pulumi secrets, so they are
**encrypted in Pulumi state**, and the deploying shell needs no secret
env vars at all for `ssm:`/`secretsmanager:` sources.

!!! note "Lightsail ceiling"
    Lightsail containers have no task IAM role and no native secret
    references, so resolved values still become container environment
    variables visible in the Lightsail console to anyone with
    `lightsail:Get*`. Deploy-time resolution centralizes storage,
    rotation (update the parameter, rerun `omnideploy up`), and
    CloudTrail audit — full runtime injection arrives with an ECS
    target.

### Step 7: Preview Deployment

For this repository the tested, annotated config is
`deploy/lightsail/deploy.yaml` — start from it rather than writing one
from scratch; its comments record the sharp edges found deploying for
real.

```bash
omnideploy preview \
    --config deploy/lightsail/deploy.yaml \
    --target lightsail \
    --backend pulumi
```

Review the resources that will be created.

### Step 8: Deploy

```bash
omnideploy up \
    --config deploy/lightsail/deploy.yaml \
    --target lightsail \
    --backend pulumi \
    --yes
```

The deployment outputs the service URL.

### Step 9: Verify Deployment

```bash
# Health check
curl https://<service-url>/health

# Test chat endpoint
curl https://<service-url>/openai/v1/chat/completions \
    -H "Content-Type: application/json" \
    -d '{"model":"default","messages":[{"role":"user","content":"Hello"}]}'
```

### Step 10: Monitor and Maintain

```bash
# View logs (via AWS Console or CLI)
aws lightsail get-container-log \
    --service-name <app-name> \
    --container-name <app-name>

# Update deployment (after pushing new image)
omnideploy up --config deploy.yaml --target lightsail --backend pulumi --yes

# Destroy when done
omnideploy destroy --stack <app-name> --yes
```

## Building a Custom Agent

Before deployment, you typically create a custom agent that bundles compiled skills with OmniAgent.

!!! note "Storage no longer requires a custom binary"
    The `STORAGE_PATH`-driven `RegisterAgentOption(agent.WithStorage(...))`
    step below predates RMI-OMNIAGENT-007: `omniagent gateway run` now
    builds a durable storage backend itself from `storage.type`/
    `storage.path`/`storage.redis.url` config (see
    [Configuration → Storage](../reference/configuration.md#storage)), and
    starts the cron scheduler against it in single-agent mode. A custom
    binary is still the right call for bundling **compiled skills**, but
    if that's all you needed a custom `main.go` for, the stock binary
    plus a `storage:` block in your deployment config now covers it.

### Project Structure

```
my-agent/
├── cmd/
│   └── agent/
│       └── main.go       # Entry point with compiled skills
├── config/
│   └── config.yaml       # Default configuration
├── Dockerfile            # Container build
├── deploy.yaml           # OmniDeploy configuration
├── go.mod
└── go.sum
```

### Entry Point

Create `cmd/agent/main.go`:

```go
package main

import (
    "log"
    "os"

    "github.com/plexusone/omniagent/agent"
    "github.com/plexusone/omniagent/cmd/omniagent/commands"
    "github.com/plexusone/omnistorage/kvs/sqlite"

    // Import your compiled skills
    myskill "github.com/example/my-skill/omniagent/skill"
)

func main() {
    // Configure storage
    storagePath := os.Getenv("STORAGE_PATH")
    if storagePath != "" {
        store, err := sqlite.New(sqlite.Config{Path: storagePath})
        if err != nil {
            log.Fatalf("failed to create storage: %v", err)
        }
        commands.RegisterAgentOption(agent.WithStorage(store))
    }

    // Register compiled skills
    skill, err := myskill.New(myskill.Config{
        APIKey: os.Getenv("MY_SKILL_API_KEY"),
    })
    if err != nil {
        log.Fatalf("failed to create skill: %v", err)
    }
    commands.RegisterAgentOption(agent.WithCompiledSkill(skill))

    // Run the standard OmniAgent CLI
    if err := commands.Execute(); err != nil {
        log.Fatalf("error: %v", err)
    }
}
```

### Configuration

Create `config/config.yaml`:

```yaml
agent:
  name: my-agent
  greeting: "Hello, I'm My Agent. How can I help?"

llm:
  provider: anthropic
  model: claude-sonnet-5

gateway:
  enabled: true
  addr: ":8080"

skills:
  enabled: true
```

## Containerization

### Dockerfile

Create a multi-stage Dockerfile for minimal image size. This follows the
PlexusOne container base-image policy: a digest-pinned
[Chainguard static](https://images.chainguard.dev/directory/image/static/overview)
runtime — no shell, no package manager, no libc — which is what this
repo's own `Dockerfile` uses; copy its exact base-image digest rather
than retyping it.

```dockerfile
# Build stage
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

WORKDIR /app

# Install git for private dependencies
RUN apk add --no-cache git ca-certificates

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build binary. -tags timetzdata embeds the IANA timezone database (the
# agent's Timezone config needs it) — the runtime image below carries no
# OS tzdata.
COPY . .
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build \
    -trimpath \
    -tags timetzdata \
    -ldflags="-s -w" \
    -o /app/my-agent \
    ./cmd/agent

# Writable data directory, owned by the runtime image's built-in nonroot
# user (65532) — no shell there to mkdir/chown.
RUN mkdir -p /out/data && chown -R 65532:65532 /out/data

# Runtime stage — see this repo's own Dockerfile for the current pinned
# digest (`FROM cgr.dev/chainguard/static@sha256:...`).
FROM cgr.dev/chainguard/static@sha256:<copy-from-this-repos-Dockerfile>

WORKDIR /opt/omniagent

COPY --from=builder /app/my-agent /opt/omniagent/my-agent
COPY --from=builder /app/config/config.yaml /opt/omniagent/config.yaml
COPY --from=builder --chown=65532:65532 /out/data /data

USER 65532:65532

EXPOSE 8080

# Exec-form probe via the binary's own healthcheck subcommand — there is
# no wget/curl/shell in this image.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/opt/omniagent/my-agent", "healthcheck"]

ENV OMNIAGENT_GATEWAY_ADDRESS="0.0.0.0:8080" \
    STORAGE_PATH="/data/omniagent.db"

ENTRYPOINT ["/opt/omniagent/my-agent"]
CMD ["gateway", "run"]
```

### Build Locally

```bash
# Build image
docker build -t my-agent:latest .

# Test locally
docker run -p 8080:8080 \
    -e ANTHROPIC_API_KEY="$ANTHROPIC_API_KEY" \
    my-agent:latest
```

## OmniDeploy Configuration

[OmniDeploy](https://github.com/plexusone/omnideploy) provides declarative deployment to multiple cloud targets.

### Installation

```bash
go install github.com/plexusone/omnideploy/cmd/omnideploy@latest
```

### Configuration File

omnideploy detects the config's *runtime adapter* from its shape. A file
with top-level `gateway:`/`agent:` keys selects the **omniagent adapter**
(the schema used by this repo's tested
[`deploy/lightsail/deploy.yaml`](https://github.com/plexusone/omniagent/blob/main/deploy/lightsail/deploy.yaml)):

```yaml
gateway:
  address: "0.0.0.0:8080"   # must match the Dockerfile's EXPOSE/HEALTHCHECK port

agent:
  provider: anthropic
  model: claude-sonnet-5

deploy:
  name: my-agent
  region: us-west-2
  image: ghcr.io/example/my-agent:latest
  replicas: 1               # keep 1 for stateful channels (e.g. Discord WebSocket)
  resources:
    size: micro             # LightSail sizes: nano, micro, small, medium, large, xlarge
  environment:
    OMNIAGENT_GATEWAY_ADDRESS: "0.0.0.0:8080"
    ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY}
```

Two gotchas learned from the first real deploy:

1. The adapter derives the container *port* from `gateway.address` but
   does **not** inject `OMNIAGENT_GATEWAY_ADDRESS` into the container
   environment — without the explicit entry above, the process binds its
   default `127.0.0.1:18789`, the health check probes the wrong port, and
   the deployment never goes healthy.
2. `${VAR}` (and `${VAR:-default}`) values in `environment:` are expanded
   from the deploying shell's environment.

### Secrets Management

Declare secrets under `deploy.secrets` (see Step 6 for sources and the
Lightsail visibility ceiling). On a name collision with an
`environment:` entry the secret wins, so promoting a variable from
plain env to a secret needs no removal of the old key:

```yaml
secrets:
  - name: ANTHROPIC_API_KEY
    source: ssm:/my-agent/anthropic-api-key
  - name: MY_SKILL_API_KEY
    source: ssm:/my-agent/skill-api-key
```

## Deploying to AWS LightSail

### Prerequisites

1. AWS credentials configured
2. Container image pushed to registry
3. OmniDeploy installed

### Deploy

```bash
# Use the named least-privilege profile from Step 5
export AWS_PROFILE="my-agent-omnideploy"
export PULUMI_BACKEND_URL="file://$HOME/.pulumi-state-my-agent"
export PULUMI_CONFIG_PASSPHRASE=""

# Preview deployment
omnideploy preview --config deploy.yaml --target lightsail --backend pulumi

# Deploy
omnideploy up --config deploy.yaml --target lightsail --backend pulumi --yes
```

### Verify

```bash
# Get service URL from deployment output
curl https://my-agent.xxxxx.us-west-2.cs.amazonlightsail.com/health

# Test chat endpoint
curl https://my-agent.xxxxx.us-west-2.cs.amazonlightsail.com/openai/v1/chat/completions \
    -H "Content-Type: application/json" \
    -d '{"model":"my-agent","messages":[{"role":"user","content":"Hello"}]}'
```

### Destroy

```bash
omnideploy destroy --stack my-agent --yes
```

## CI/CD with GitHub Actions

### Container Build Workflow

Create `.github/workflows/build.yaml`:

```yaml
name: Build and Push Container

on:
  push:
    branches: [main]
    tags: ['v*']
  pull_request:
    branches: [main]

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write

    steps:
      - uses: actions/checkout@v4

      - uses: docker/setup-buildx-action@v3

      - name: Log in to Container Registry
        if: github.event_name != 'pull_request'
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Extract metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
          tags: |
            type=ref,event=branch
            type=semver,pattern={{version}}
            type=sha

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: .
          push: ${{ github.event_name != 'pull_request' }}
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

### Deployment Workflow

Create `.github/workflows/deploy.yaml`:

```yaml
name: Deploy to LightSail

on:
  workflow_run:
    workflows: ["Build and Push Container"]
    types: [completed]
    branches: [main]

jobs:
  deploy:
    if: ${{ github.event.workflow_run.conclusion == 'success' }}
    runs-on: ubuntu-latest
    environment: production

    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.26'

      - name: Install OmniDeploy
        run: go install github.com/plexusone/omnideploy/cmd/omnideploy@latest

      - name: Deploy
        env:
          AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
          AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          AWS_REGION: us-west-2
        run: |
          omnideploy up \
            --config deploy.yaml \
            --target lightsail \
            --backend pulumi \
            --yes

      - name: Health Check
        run: |
          for i in {1..30}; do
            if curl -sf "${{ vars.SERVICE_URL }}/health"; then
              echo "Deployment healthy"
              exit 0
            fi
            sleep 10
          done
          echo "Health check failed"
          exit 1
```

### Required Secrets

Configure in **Settings → Secrets and variables → Actions**:

| Secret | Description |
|--------|-------------|
| `AWS_ACCESS_KEY_ID` | AWS access key |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key |

Configure in **Settings → Secrets and variables → Actions → Variables**:

| Variable | Description |
|----------|-------------|
| `SERVICE_URL` | Deployed service URL for health checks |

## Deploy Script

For manual deployments, create `deploy/deploy.sh`:

```bash
#!/bin/bash
# Deploy to AWS LightSail using OmniDeploy

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CONFIG_FILE="$PROJECT_ROOT/deploy.yaml"
STACK_NAME="${STACK_NAME:-my-agent}"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo -e "${GREEN}[deploy]${NC} $1"; }
warn() { echo -e "${YELLOW}[warn]${NC} $1"; }

# Check prerequisites
check_prereqs() {
    command -v omnideploy &> /dev/null || {
        echo "Install omnideploy: go install github.com/plexusone/omnideploy/cmd/omnideploy@latest"
        exit 1
    }

    [ -z "$AWS_ACCESS_KEY_ID" ] && warn "AWS_ACCESS_KEY_ID not set"
    [ ! -f "$CONFIG_FILE" ] && { echo "Config not found: $CONFIG_FILE"; exit 1; }
}

# Build and push image
build() {
    log "Building container image..."
    cd "$PROJECT_ROOT"
    VERSION=$(git describe --tags --always 2>/dev/null || echo "latest")
    IMAGE="ghcr.io/example/my-agent:$VERSION"

    docker build -t "$IMAGE" -t "ghcr.io/example/my-agent:latest" .
    docker push "$IMAGE"
    docker push "ghcr.io/example/my-agent:latest"
    log "Image pushed: $IMAGE"
}

# Deploy
deploy() {
    log "Deploying to LightSail..."
    omnideploy up \
        --config "$CONFIG_FILE" \
        --target lightsail \
        --backend pulumi \
        --stack "$STACK_NAME" \
        --yes
}

# Preview
preview() {
    log "Previewing deployment..."
    omnideploy preview \
        --config "$CONFIG_FILE" \
        --target lightsail \
        --backend pulumi
}

# Destroy
destroy() {
    warn "This will destroy all resources in stack: $STACK_NAME"
    read -p "Are you sure? (y/N) " -n 1 -r
    echo
    [[ $REPLY =~ ^[Yy]$ ]] && omnideploy destroy --stack "$STACK_NAME" --yes
}

check_prereqs

case "${1:-deploy}" in
    build)   build ;;
    preview) preview ;;
    deploy)  deploy ;;
    destroy) destroy ;;
    full)    build && deploy ;;
    *)
        echo "Usage: $0 {build|preview|deploy|destroy|full}"
        exit 1
        ;;
esac
```

## Resource Sizing

### LightSail Container Sizes

| Size | vCPU | Memory | Monthly Cost |
|------|------|--------|--------------|
| nano | 0.25 | 512MB | ~$7 |
| micro | 0.5 | 1GB | ~$10 |
| small | 1 | 2GB | ~$25 |
| medium | 2 | 4GB | ~$50 |
| large | 4 | 8GB | ~$100 |
| xlarge | 8 | 16GB | ~$200 |

### Recommendations

| Use Case | Size | Notes |
|----------|------|-------|
| Development/Testing | nano | Minimal traffic |
| Personal assistant | micro | Light usage |
| Production API | small | Moderate traffic |
| High-traffic API | medium+ | Scale replicas |

## Other Deployment Targets

OmniDeploy supports additional targets (planned):

| Target | Status | Description |
|--------|--------|-------------|
| lightsail | Available | AWS LightSail containers |
| ecs | Planned | AWS ECS/Fargate |
| kubernetes | Planned | Kubernetes clusters |
| digitalocean | Planned | DigitalOcean App Platform |

See [OmniDeploy documentation](https://github.com/plexusone/omnideploy) for details.
