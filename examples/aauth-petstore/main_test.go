package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	aauth "github.com/aauth-dev/aauth-go"
	"github.com/aauth-dev/aauth-go/sandbox"
)

const shopSpec = `{"openapi":"3.0.3","info":{"title":"Petstore","version":"1"},
"servers":[{"url":"https://petstore.aauth.test/v2"}],
"paths":{
 "/pet/findByStatus":{"get":{"operationId":"findPetsByStatus",
   "parameters":[{"name":"status","in":"query","style":"form","explode":false,"schema":{"type":"array","items":{"type":"string"}}}],
   "responses":{"200":{"description":"ok"}}}},
 "/store/order":{"post":{"operationId":"placeOrder",
   "requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"object","properties":{"petId":{"type":"integer"}}}}}},
   "responses":{"200":{"description":"ok"}}}}}}`

// petstore runs the sandbox and a small Petstore behind it at
// https://petstore.aauth.test.
func petstore(t *testing.T) *sandbox.Sandbox {
	t.Helper()
	var h http.Handler
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { h.ServeHTTP(w, r) }))
	t.Cleanup(up.Close)
	sb, err := sandbox.New(sandbox.Config{StateDir: t.TempDir(), Upstreams: map[string]string{"petstore": up.URL}},
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
		Resource: "https://petstore.aauth.test", Key: key,
		Verify:      aauth.TokenVerifyOptions{Resolver: aauth.NewJWKSResolver(sb.Client("petstore"))},
		TrustIssuer: aauth.TrustIssuers(sb.Config().URL(sandbox.RolePS)),
	})
	if err != nil {
		t.Fatal(err)
	}
	reply := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if _, err := io.WriteString(w, body); err != nil {
				t.Error(err)
			}
		}
	}
	mux := http.NewServeMux()
	mux.Handle("GET /openapi.json", reply(shopSpec))
	mux.Handle("GET /v2/pet/findByStatus", g.Require(aauth.Access{Level: aauth.AccessAgent}, reply(`[{"id":1,"name":"Fido","status":"available"}]`)))
	mux.Handle("POST /v2/store/order", g.Require(aauth.Access{Level: aauth.AccessAuth, Scope: "write:orders"}, reply(`{"id":1,"petId":1,"status":"placed"}`)))
	mux.Handle("/", g.WellKnown())
	h = mux
	return sb
}

// scriptedModel is an OpenAI-compatible chat API that calls the find tool,
// then the order tool, then answers, recording the tool results it sees.
type scriptedModel struct {
	mu      sync.Mutex
	results []string
}

func (m *scriptedModel) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Messages []struct {
			Role    string `json:"role"`
			Content any    `json:"content"`
		} `json:"messages"`
		Tools []struct {
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		} `json:"tools"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	tool := func(suffix string) string {
		for _, t := range req.Tools {
			if strings.HasSuffix(t.Function.Name, suffix) {
				return t.Function.Name
			}
		}
		names := make([]string, 0, len(req.Tools))
		for _, t := range req.Tools {
			names = append(names, t.Function.Name)
		}
		return "missing-" + suffix + " among " + strings.Join(names, ",")
	}
	m.mu.Lock()
	m.results = m.results[:0]
	for _, msg := range req.Messages {
		if msg.Role == "tool" {
			b, err := json.Marshal(msg.Content)
			if err != nil {
				b = []byte(err.Error())
			}
			m.results = append(m.results, string(b))
		}
	}
	n := len(m.results)
	m.mu.Unlock()

	message := map[string]any{"role": "assistant", "content": "Ordered Fido for you."}
	finish := "stop"
	call := func(name, args string) {
		message = map[string]any{"role": "assistant", "content": nil, "tool_calls": []map[string]any{{
			"id": "call_" + name, "type": "function", "function": map[string]any{"name": name, "arguments": args},
		}}}
		finish = "tool_calls"
	}
	switch n {
	case 0:
		call(tool("findPetsByStatus"), `{"status":["available"]}`)
	case 1:
		call(tool("placeOrder"), `{"body_petId":1}`)
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"id": "chatcmpl-1", "object": "chat.completion", "created": 0, "model": "scripted",
		"choices": []map[string]any{{"index": 0, "message": message, "finish_reason": finish}},
		"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func TestChatOrdersAPetAsAnAAuthAgent(t *testing.T) {
	sb := petstore(t)
	model := &scriptedModel{}
	llm := httptest.NewServer(model)
	defer llm.Close()
	t.Setenv("OMNIAGENT_AGENT_PROVIDER", "openai")
	t.Setenv("OMNIAGENT_AGENT_MODEL", "scripted")
	t.Setenv("OMNIAGENT_AGENT_BASE_URL", llm.URL)
	t.Setenv("OPENAI_API_KEY", "test-key")

	var out, errOut strings.Builder
	o := &options{
		agentProvider: sb.Config().URL(sandbox.RoleAP), enrollPath: sandbox.PathEnroll,
		petstore: "https://petstore.aauth.test", prompts: []string{"Order an available pet for me."},
		base: sb.Client("agent").Transport,
	}
	if err := o.run(context.Background(), nil, &out, &errOut); err != nil {
		t.Fatalf("%v\nstderr:\n%s", err, errOut.String())
	}
	if !strings.Contains(out.String(), "Ordered Fido for you.") {
		t.Errorf("reply:\n%s", out.String())
	}
	if !strings.Contains(errOut.String(), "Enrolled as aauth:") || !strings.Contains(errOut.String(), "Approval needed. Open https://ps.aauth.test/") {
		t.Errorf("stderr:\n%s", errOut.String())
	}
	model.mu.Lock()
	defer model.mu.Unlock()
	if len(model.results) != 2 || !strings.Contains(model.results[0], "Fido") || !strings.Contains(model.results[1], "placed") {
		t.Fatalf("tool results the model saw: %q", model.results)
	}
}
