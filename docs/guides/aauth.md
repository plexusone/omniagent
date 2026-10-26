# AAuth for OpenAPI Skills

An OpenAPI skill normally calls an API with a key or token you configure.
With [AAuth](https://github.com/aauth-dev/aauth-go), omniagent calls the API
as an agent with its own identity, acting for a person:

- An **agent provider** issues omniagent an agent token for a key it holds.
  Every request is signed with that key (HTTP Message Signatures).
- When an operation needs to know the person, omniagent gets a **person
  token** from the person's **person server**.
- When an operation needs the person's consent for a scope, omniagent gets an
  **auth token** from the person server, which asks the person to approve.
  The tool call waits, and omniagent tells the person where to approve.

The API decides what each operation needs and says so in its responses, so
the skill needs no per-operation configuration.

## Use It

Package `aauthagent` enrolls at an agent provider and returns an HTTP client
that does all of the above. Give it to an OpenAPI skill:

```go
client, err := aauthagent.NewClient(ctx, aauthagent.Config{
	AgentProvider: "https://ap.aauth.test",
	EnrollPath:    "/enroll", // the provider's token endpoint path
	OnInteraction: func(i aauthagent.Interaction) {
		fmt.Println("Approve at", i.Link())
	},
})
if err != nil {
	return err
}
a, err := agent.New(cfg, agent.WithOpenAPISkill(openapi.Config{
	Name:       "petstore",
	SpecURL:    "https://petstore.aauth.test/openapi.json",
	HTTPClient: client.HTTPClient(),
}))
if err != nil {
	return err
}
// Load the spec, through the client, and offer its operations as tools.
if err := a.InitCompiledSkills(ctx); err != nil {
	return err
}
```

| `aauthagent.Config` | Meaning |
|---|---|
| `AgentProvider` | the agent provider to enroll at (required) |
| `EnrollPath` | its token endpoint path |
| `Key` | the agent's key; without one a new Ed25519 key is generated, so the agent is new each run |
| `RootCAsPEM` | extra root certificates to trust, such as a development CA |
| `OnInteraction` | called with the approval URL and code when the person must act |

`OnInteraction` runs while the request waits. A terminal app prints the link;
a chat channel would send it to the person.

## Try It: the Petstore Demo

`examples/aauth-petstore` is a terminal chat with the Petstore as an AAuth
resource. Browsing needs only the agent's identity; placing an order or
changing the catalog asks the person to approve. See
[its README](https://github.com/plexusone/omniagent/tree/main/examples/aauth-petstore)
to run it with Docker Compose next to the AAuth sandbox, the same setup the
AAuth Postman collection uses.
