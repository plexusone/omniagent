# Try AAuth

Run omniagent as an AAuth agent against the AAuth sandbox and the Petstore,
and see every message in the sandbox's trace. The same setup is the one to use
for interop tests.

The servers come from [aauth-go](https://github.com/aauth-dev/aauth-go). Its
[sandbox guide](https://aauth-dev.github.io/aauth-go/guides/sandbox/) is the
reference for the sandbox and its options, and
[aauth-postman](https://github.com/aistandardsio/aauth-postman) has the Compose
file that runs the sandbox and the Petstore together. This page covers only
what is specific to omniagent.

## With Docker Compose

You need Docker, and four repositories checked out as `<org>/<repo>` under one
parent, for example `~/go/src/github.com`: `aistandardsio/aauth-postman`,
`aauth-dev/aauth-go`, `grokify/petstore` and `plexusone/omniagent`. The Compose
file builds the images from them until they are published. Set `AAUTH_GO_DIR`,
`PETSTORE_DIR` or `OMNIAGENT_DIR` if yours are elsewhere.

```bash
cd aauth-postman
docker compose up -d --build --wait                  # sandbox and Petstore
ANTHROPIC_API_KEY=... docker compose --profile omniagent run --rm omniagent
```

At the prompt, ask for something that needs only identity, then something that
needs approval:

```text
> What pets are available?
> Place an order for pet 2, quantity 1.
```

For the order you see:

```text
Approval needed. Open https://ps.aauth.test/interaction?code=XXXX-XXXX
(code XXXX-XXXX). Waiting for the approval...
```

The sandbox approves on its own, so the order then goes through. To approve
yourself, start the sandbox with `--consent manual`, trust its CA in the
browser, and open the link.

If a host port is taken, change it: `SIGNER_PORT=18099
SANDBOX_HTTPS_PORT=18443 docker compose up -d --wait`. The omniagent
container reaches the sandbox over the Compose network, so the host ports do
not matter to it.

## Read the Trace

```bash
docker compose exec sandbox aauth-sandbox trace --state /state
```

The trace lists every request and response between the agent, the person
server and the Petstore: the enrollment, the 401 requirements, the 202 and
the final 200. [How AAuth works](how-it-works.md) names each step.

## Without Docker

Run the sandbox and the Petstore on your machine (see the
[sandbox guide](https://aauth-dev.github.io/aauth-go/guides/sandbox/#run-it)),
add the `aauth-sandbox hosts` line to `/etc/hosts`, and run:

```bash
ANTHROPIC_API_KEY=... go run ./examples/aauth-petstore --ca-file ~/.aauth-sandbox/ca.pem
```

| Flag | Default |
|------|---------|
| `--agent-provider` | `https://ap.aauth.test` |
| `--enroll-path` | `/enroll` |
| `--petstore` | `https://petstore.aauth.test` |
| `--ca-file` | `$AAUTH_CA_FILE` |
| `--prompt` | read prompts from the terminal |

Use `--prompt "..."` to send one message and exit, which suits scripts and
agents.

## For Interop Tests

- **Your resource.** Front it with `--upstream` (see the sandbox guide's
  [Front your own server](https://aauth-dev.github.io/aauth-go/guides/sandbox/#front-your-own-server)),
  then point an OpenAPI skill at its spec with the client from
  `aauthagent.NewClient`.
- **Your person server or agent provider.** Start the sandbox with `--remote`
  ([Test another implementation](https://aauth-dev.github.io/aauth-go/guides/sandbox/#test-another-implementation))
  and run the same prompts.
- **What to check.** Judge success from the trace, not from the model's
  words: the order shows a 202 then a 200 at the person server, and a 200 at
  the resource. The model may ask for confirmation, or search a tag that
  matches nothing; name the tool and arguments in the prompt.
- **Other agents.** The aauth-postman
  [interop runbook](https://github.com/aistandardsio/aauth-postman/blob/main/docs/interop.md)
  covers the Postman side and a table of common failures.

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `connection refused` to `ap.aauth.test:443` | The sandbox command lacks `--listen-any-interface`; use aauth-postman's current `compose.yaml` |
| `x509: certificate signed by unknown authority` | Pass `--ca-file`, or set `AAUTH_CA_FILE`, to the sandbox CA |
| `421 Misdirected Request` from the signing helper | Add `--signer-host <service>` to the sandbox; this affects Newman, not omniagent |
| The order fails on a repeat run | The pet was sold; restart the Petstore with `docker compose up -d --force-recreate petstore` |
