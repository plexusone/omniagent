# aauth-petstore: omniagent as an AAuth agent for the Petstore.
#
# Build from the repository root:
#
#   docker build -f examples/aauth-petstore/Dockerfile -t aauth-petstore .

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
WORKDIR /src/plexusone/omniagent
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build \
    -trimpath -ldflags="-s -w" -o /out/aauth-petstore ./examples/aauth-petstore

FROM cgr.dev/chainguard/static@sha256:bf639cba19ba56329e6907ac26a7afcdde57a80b6aa66d5100da6883196e6b82
COPY --from=builder /out/aauth-petstore /opt/aauth-petstore/aauth-petstore
USER 65532:65532
ENTRYPOINT ["/opt/aauth-petstore/aauth-petstore"]
