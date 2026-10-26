# TRD — AAuth agent for OpenAPI skills

**Initiative:** `INIT-OMNIAGENT-007`
**Repository:** `github.com/plexusone/omniagent`

## Package `aauthagent`

```go
client, err := aauthagent.NewClient(ctx, aauthagent.Config{
    AgentProvider: "https://ap.aauth.test",
    EnrollPath:    "/enroll",
    RootCAsPEM:    caPEM,                         // optional, e.g. a sandbox CA
    OnInteraction: func(i aauthagent.Interaction) { … }, // approval URL and code
})
skill := openapi.NewSkill(openapi.Config{
    Name: "petstore", SpecURL: "https://petstore.aauth.test/openapi.json",
    HTTPClient: client.HTTPClient(),
})
```

`NewClient` enrolls an Ed25519 key with aauth-go's
`agentprovider.Client.Enroll` (hwk scheme), builds an `aauth.PSClient` for
the person server the agent token names, and wraps an `http.Transport` that
trusts the system roots plus `RootCAsPEM` in `aauth.Transport`. The
transport declares the `interaction` capability, so resources and the
person server may ask the person to approve; `OnRequirement` passes each
interaction to `OnInteraction`.

`Config.Key` supplies a key instead of generating one, so an install keeps
its agent identity across restarts.

## Demo

`examples/aauth-petstore` is a terminal chat: it builds an agent with the
Petstore OpenAPI skill over the AAuth client, reads prompts from standard
input, and prints approval links as they arrive. It runs as a container on
the demo's Compose network, where every role is reachable by name and the
sandbox CA is on a shared volume.
