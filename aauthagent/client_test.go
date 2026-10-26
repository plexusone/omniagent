package aauthagent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	aauth "github.com/aauth-dev/aauth-go"
	"github.com/aauth-dev/aauth-go/sandbox"

	"github.com/plexusone/omniagent/skills/remote/openapi"
	skilltypes "github.com/plexusone/omniskill/skill"
)

const shopURL = "https://shop.aauth.test"

// shopSpec is a small API: a catalog for any agent and orders that need
// the person's consent.
const shopSpec = `openapi: 3.0.3
info: {title: Shop, version: "1"}
servers: [{url: "https://shop.aauth.test"}]
paths:
  /items:
    get:
      operationId: listItems
      responses: {"200": {description: ok}}
  /orders:
    post:
      operationId: placeOrder
      requestBody:
        required: true
        content:
          application/json:
            schema: {type: object, properties: {item: {type: string}}}
      responses: {"200": {description: ok}}
`

// sandboxWorld runs the aauth-go sandbox with manual consent and a shop
// behind it at https://shop.aauth.test, guarded by aauth.ResourceGuard.
func sandboxWorld(t *testing.T) *sandbox.Sandbox {
	t.Helper()
	var h http.Handler
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { h.ServeHTTP(w, r) }))
	t.Cleanup(up.Close)
	sb, err := sandbox.New(sandbox.Config{StateDir: t.TempDir(), Consent: sandbox.ConsentManual, Upstreams: map[string]string{"shop": up.URL}},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	sb.SetAddr(ln.Addr().String())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- sb.Serve(ctx, ln) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})

	key, err := aauth.GenerateKey(aauth.AlgEd25519)
	if err != nil {
		t.Fatal(err)
	}
	g, err := aauth.NewResourceGuard(aauth.ResourceGuardConfig{
		Resource: shopURL, Key: key,
		Verify:      aauth.TokenVerifyOptions{Resolver: aauth.NewJWKSResolver(sb.Client("shop"))},
		TrustIssuer: aauth.TrustIssuers(sb.Config().URL(sandbox.RolePS)),
	})
	if err != nil {
		t.Fatal(err)
	}
	reply := func(body string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if _, err := io.WriteString(w, body); err != nil {
				t.Error(err)
			}
		})
	}
	mux := http.NewServeMux()
	mux.Handle("GET /openapi.yaml", reply(shopSpec))
	mux.Handle("GET /items", g.Require(aauth.Access{Level: aauth.AccessAgent}, reply(`["kibble","leash"]`)))
	mux.Handle("POST /orders", g.Require(aauth.Access{Level: aauth.AccessAuth, Scope: "orders:write"}, reply(`{"status":"placed"}`)))
	mux.Handle("/", g.WellKnown())
	h = mux
	return sb
}

func TestOpenAPISkillOverAAuth(t *testing.T) {
	sb := sandboxWorld(t)
	ctx := context.Background()
	cfg := sb.Config()

	var mu sync.Mutex
	var prompts []Interaction
	client, err := NewClient(ctx, Config{
		AgentProvider: cfg.URL(sandbox.RoleAP),
		EnrollPath:    sandbox.PathEnroll,
		Base:          sb.Client("agent").Transport,
		OnInteraction: func(i Interaction) {
			mu.Lock()
			prompts = append(prompts, i)
			mu.Unlock()
			// The person approves, as the sandbox's headless endpoint lets a test.
			res, err := sb.Client("person").Post(cfg.URL(sandbox.RolePS)+sandbox.PathApprove+"?code="+url.QueryEscape(i.Code), "", nil)
			if err != nil {
				t.Error(err)
				return
			}
			if err := res.Body.Close(); err != nil {
				t.Error(err)
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(client.AgentID(), "@ap.aauth.test") || client.PersonServer() != cfg.URL(sandbox.RolePS) {
		t.Fatalf("agent %s, PS %s", client.AgentID(), client.PersonServer())
	}

	skill := openapi.NewSkill(openapi.Config{Name: "shop", SpecURL: shopURL + "/openapi.yaml", HTTPClient: client.HTTPClient()})
	if err := skill.Init(ctx); err != nil {
		t.Fatal(err)
	}
	tools := map[string]skilltypes.Tool{}
	for _, tool := range skill.Tools() {
		tools[tool.Name()] = tool
	}

	items, err := tools["listItems"].Call(ctx, nil)
	if err != nil || fmt.Sprint(items) != "[kibble leash]" {
		t.Fatalf("listItems = %v, %v", items, err)
	}
	order, err := tools["placeOrder"].Call(ctx, map[string]any{"body_item": "kibble"})
	if err != nil || fmt.Sprint(order) != "map[status:placed]" {
		t.Fatalf("placeOrder = %v, %v", order, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(prompts) != 1 || !strings.HasPrefix(prompts[0].Link(), cfg.URL(sandbox.RolePS)+"/") || !strings.Contains(prompts[0].Link(), "code=") {
		t.Fatalf("consent prompts = %+v", prompts)
	}
}

func TestInteractionLink(t *testing.T) {
	for _, tc := range []struct {
		in   Interaction
		want string
	}{
		{Interaction{URL: "https://ps.example/interaction", Code: "AB12-CD34"}, "https://ps.example/interaction?code=AB12-CD34"},
		{Interaction{URL: "https://ps.example/i?lang=en", Code: "X"}, "https://ps.example/i?code=X&lang=en"},
		{Interaction{URL: "https://ps.example/i"}, "https://ps.example/i"},
	} {
		if got := tc.in.Link(); got != tc.want {
			t.Errorf("Link(%+v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNewClientErrors(t *testing.T) {
	ctx := context.Background()
	if _, err := NewClient(ctx, Config{}); err == nil {
		t.Error("a client without an agent provider was created")
	}
	if _, err := NewClient(ctx, Config{AgentProvider: "https://ap.example", RootCAsPEM: []byte("x")}); err == nil || !strings.Contains(err.Error(), "no certificates") {
		t.Errorf("bad RootCAsPEM: %v", err)
	}
	down := httptest.NewServer(http.NotFoundHandler())
	down.Close()
	if _, err := NewClient(ctx, Config{AgentProvider: down.URL}); err == nil || !strings.Contains(err.Error(), "enrolling") {
		t.Errorf("unreachable provider: %v", err)
	}
}
