# PLAN — AAuth agent for OpenAPI skills

**Initiative:** `INIT-OMNIAGENT-007`
**Repository:** `github.com/plexusone/omniagent`

1. Add package `aauth` over aauth-go's transport and agent provider client.
2. Test it with an OpenAPI skill against aauth-go's sandbox and a resource
   guarded by `aauth.ResourceGuard`, with no LLM.
3. Add the Petstore chat example and its container, and a guide.

omniagent depends on aauth-go v0.3.0. Until it is tagged, `go.mod` points at
a sibling checkout with a relative `replace`, removed before pushing.
