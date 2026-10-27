# aauth-petstore

A terminal chat with omniagent acting as an [AAuth](https://github.com/aauth-dev/aauth-go)
agent for the Petstore. The Petstore's OpenAPI operations are omniagent's
tools. Each operation asks for what it needs:

| You ask | The Petstore needs | What happens |
|---|---|---|
| "What dogs are available?" | the agent's identity | the call is signed as the agent |
| "Order Fido for me." | your approval for `write:orders` | the chat prints an approval link and waits |
| "Add a pet named Nemo." | your approval for `write:pets` | the same, for `write:pets` |

## Run It with Docker Compose

The demo runs next to the AAuth sandbox (agent provider, person server and
access server) and the Petstore, from the
[aauth-postman](https://github.com/aistandardsio/aauth-postman) Compose file: the
same servers the Postman collection calls, so the two demos send the same
messages.

```bash
cd aauth-postman
docker compose --profile petstore up -d --build --wait   # sandbox and Petstore
ANTHROPIC_API_KEY=... docker compose --profile petstore run --rm omniagent
```

The sandbox approves automatically, so the approval link is shown and the
order goes through. To approve yourself, start the sandbox with
`--consent manual`, open the link in a browser that trusts the sandbox CA, and
press Approve.

Every message between the agent, the person server and the Petstore is in
the sandbox's trace:

```bash
docker compose exec sandbox aauth-sandbox trace --state /state
```

## Run It Locally

With the sandbox and the Petstore running on this machine (see the aauth-go
sandbox guide) and the hosts line from `aauth-sandbox hosts` in `/etc/hosts`:

```bash
ANTHROPIC_API_KEY=... go run ./examples/aauth-petstore --ca-file ~/.aauth-sandbox/ca.pem
```

| Flag | Default |
|---|---|
| `--agent-provider` | `https://ap.aauth.test` |
| `--enroll-path` | `/enroll` |
| `--petstore` | `https://petstore.aauth.test` |
| `--ca-file` | `$AAUTH_CA_FILE` |
| `--prompt` | read prompts from the terminal |

The model is configured as for omniagent: `OMNIAGENT_AGENT_PROVIDER`,
`OMNIAGENT_AGENT_MODEL` and the provider's API key.
