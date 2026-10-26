# How AAuth Works

AAuth lets an agent call an API in another trust domain without a shared
secret and without the API knowing the agent in advance. The agent holds its
own signing key and signs every request. A token binds that key to an
identity, so the API can verify who is calling and, when it needs more, ask
the person behind the agent.

This page covers what omniagent does at each step. For the protocol itself,
see aauth-go's
[How AAuth works](https://aauth-dev.github.io/aauth-go/guides/how-aauth-works/)
and its [flows](https://aauth-dev.github.io/aauth-go/guides/flows/).

## The Roles

| Role | What it does | In omniagent |
|------|--------------|--------------|
| **Agent** | Signs each request and carries the tokens it is given | the HTTP client from `aauthagent.NewClient` |
| **Agent provider** | Issues the agent token that binds an identity to the agent's key | `Config.AgentProvider`; the agent enrolls there once per run |
| **Resource** | The API; says what each call needs | the OpenAPI skill's target |
| **Person server** | Issues the person token and, with the person's approval, the auth token | named in the agent token; omniagent finds it |
| **Access server** | Decides access for a resource that has its own | used by resources that have their own; omniagent does not call it directly |

## One Order, Step by Step

The Petstore demo's "place an order" call shows every step. Browsing pets
needs only the first three.

1. **Enroll.** At startup the client sends its public key to the agent
   provider's `EnrollPath` and receives an **agent token**. The key stays in
   memory unless you pass `Config.Key`.
2. **Sign.** Every request carries an HTTP Message Signature (RFC 9421)
   and a `Signature-Key` header with the agent token.
3. **Browse.** `GET /v2/pet/findByStatus` needs only the agent's identity, so
   the resource answers 200.
4. **Person token.** `POST /v2/store/order` is refused with 401 and
   `AAuth-Requirement: requirement=person-token`. The client asks the person
   server named in the agent token for a **person token**, and retries.
5. **Auth token.** The resource now refuses with `requirement=auth-token` and
   a **resource token** naming the scope, `write:orders`. The client posts it
   to the person server's token endpoint.
6. **Approval.** The person server answers 202 with an interaction code while
   the person decides. The client calls `Config.OnInteraction` with the
   approval URL and code, then polls. The tool call waits.
7. **Retry.** When the person approves, the person server returns an **auth
   token**. The client repeats the order with it and the resource answers 200.

If the person declines, the call returns an error to the model. If nobody
answers, the request waits until its context ends, which is why the client
has no timeout of its own.

## What the Skill Does Not Do

The OpenAPI skill is unaware of all this. It builds the request from the
spec, sends it through the client, and returns the response. Which operations
need which scopes comes from the API's responses, not from skill
configuration.

## Next

[Try it](try-it.md) runs these steps against a sandbox and shows them in its
trace.
