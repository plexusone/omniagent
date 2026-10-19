# PRD — AAuth agent for OpenAPI skills

**Initiative:** `INIT-OMNIAGENT-007`
**Repository:** `github.com/plexusone/omniagent`

## Problem

omniagent's OpenAPI skills call APIs with an API key, a bearer token or
basic credentials that the operator configures. An agent acting for a person
needs something else: the API should know which agent is calling, which
person it acts for, and that the person approved what it is about to do.

AAuth (draft-hardt-oauth-aauth-protocol) gives agents their own identity
from an agent provider and gets the person's consent through the person's
person server, one scope at a time, when an API asks for it. aauth-go
implements the agent side as an HTTP transport.

## Goals

- An OpenAPI skill can call an AAuth resource: omniagent enrolls a key at an
  agent provider and signs every request as that agent.
- When an operation needs the person or their consent, omniagent obtains the
  person token or auth token itself, and tells the person where to approve.
- A demo: omniagent as the agent for the AAuth Petstore, behind the same
  agent provider and person server as the Postman collection, so both demos
  produce the same messages.

## Non-Goals

- Verifying AAuth requests to omniagent's own API. The existing verifier in
  `api/openai/auth` predates draft-11 and is tracked separately.
- Consent relayed through chat channels (Telegram, Slack). The demo prints
  the approval link; channel delivery comes later.

## Requirements

1. **Identity.** `aauth.NewClient` generates or loads a key, enrolls it at
   the agent provider, and keeps the agent token fresh.
2. **Authorization.** The client answers each AAuth requirement: agent token,
   person token, auth token, and deferred consent.
3. **Consent prompts.** An interaction requirement reaches a callback with
   the approval URL and code.
4. **Skills.** `openapi.Config.HTTPClient` takes the client; no other skill
   change is needed.
