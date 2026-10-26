# ROADMAP — AAuth agent for OpenAPI skills

**Initiative:** `INIT-OMNIAGENT-007`
**Repository:** `github.com/plexusone/omniagent`

Commits implementing an item carry `Refs: RMI-OMNIAGENT-<NNN>`.

## Phase 1 — AAuth Client

**Theme:** OpenAPI skills that call AAuth resources

- [ ] `RMI-OMNIAGENT-405` Package `aauthagent`: enroll at an agent provider and sign, authorize and wait for consent through aauth-go's transport
- [ ] `RMI-OMNIAGENT-406` Consent prompts: interaction requirements reach a callback with the approval URL and code
  - Depends on: `RMI-OMNIAGENT-405`
- [ ] `RMI-OMNIAGENT-407` An OpenAPI skill over the AAuth client, tested against aauth-go's sandbox
  - Depends on: `RMI-OMNIAGENT-405`

## Phase 2 — Petstore Demo

**Theme:** The same Petstore story as the Postman collection

- [ ] `RMI-OMNIAGENT-408` `examples/aauth-petstore`: a terminal chat with the Petstore skill, and its container
  - Depends on: `RMI-OMNIAGENT-407`
- [ ] `RMI-OMNIAGENT-409` Guide: AAuth for OpenAPI skills, and running the Petstore demo
  - Depends on: `RMI-OMNIAGENT-408`

## Phase 3 — Inbound AAuth

**Theme:** omniagent's own API as an AAuth resource

- [ ] `RMI-OMNIAGENT-410` Replace the bearer-token AAuth verifier in `api/openai/auth` with draft-11 signed-request verification
