# Multi-stage Dockerfile for omniagent
# Builds standalone with no local replace directives.
#
# Build:
#   docker build --platform linux/amd64 -t ghcr.io/plexusone/omniagent:smoke .
#
# Run (requires env vars for LLM + Discord):
#   docker run -p 8080:8080 \
#     -e OMNIAGENT_GATEWAY_ADDRESS=0.0.0.0:8080 \
#     -e OMNIAGENT_AGENT_PROVIDER=anthropic \
#     -e OMNIAGENT_AGENT_MODEL=claude-sonnet-5 \
#     -e ANTHROPIC_API_KEY=... \
#     -e DISCORD_BOT_TOKEN=... \
#     ghcr.io/plexusone/omniagent:smoke
#
# IMPORTANT: Single-replica deployment required for Discord
# Discord uses a WebSocket gateway connection that is stateful. Running
# multiple replicas will cause each to receive and process messages,
# resulting in duplicate (double) answers. Set OMNIAGENT_REPLICAS=1 or
# ensure only one container instance runs when Discord is enabled.

# ---------------------------------------------------------------------------
# Build stage
# ---------------------------------------------------------------------------
# --platform=$BUILDPLATFORM keeps the Go toolchain native on the build
# host and cross-compiles via GOARCH, so a multi-arch build doesn't run
# the compiler under QEMU emulation (RMI-OMNIAGENT-032).
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

WORKDIR /build

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# -tags timetzdata embeds the IANA timezone database in the binary (the
# agent's Timezone config needs it) — the shell-less runtime image below
# carries no OS tzdata.
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build \
    -trimpath \
    -tags timetzdata \
    -ldflags="-s -w" \
    -o omniagent \
    ./cmd/omniagent

# Writable data directory for the runtime stage, owned by the static
# image's built-in nonroot user (65532) — no shell there to mkdir/chown.
RUN mkdir -p /out/data && chown -R 65532:65532 /out/data

# ---------------------------------------------------------------------------
# Runtime stage — Chainguard static (PlexusOne container base-image
# policy): no shell, no package manager, no libc; CA certs included;
# runs as the built-in nonroot user. Pinned by digest so a release never
# changes because :latest moved — refresh the digest deliberately.
# ---------------------------------------------------------------------------
FROM cgr.dev/chainguard/static@sha256:bf639cba19ba56329e6907ac26a7afcdde57a80b6aa66d5100da6883196e6b82

WORKDIR /opt/omniagent

COPY --from=builder /build/omniagent /opt/omniagent/omniagent
COPY --from=builder --chown=65532:65532 /out/data /data

USER 65532:65532

EXPOSE 8080

# Exec-form probe via the binary's own healthcheck subcommand — there is
# no wget/curl/shell in this image.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/opt/omniagent/omniagent", "healthcheck"]

ENTRYPOINT ["/opt/omniagent/omniagent"]
CMD ["gateway", "run"]
