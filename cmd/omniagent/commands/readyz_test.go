package commands

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/plexusone/omniagent/config"
)

func TestStorageRoundTrip(t *testing.T) {
	backend, err := buildStorageBackend(config.StorageConfig{Type: "memory"})
	if err != nil {
		t.Fatalf("buildStorageBackend: %v", err)
	}
	defer backend.Close()

	if err := storageRoundTrip(context.Background(), backend); err != nil {
		t.Errorf("storageRoundTrip() = %v, want nil", err)
	}

	// The probe key must not leak into the store afterward.
	if _, err := backend.Get(context.Background(), readyzKey); err == nil {
		t.Error("readyz probe key still present after storageRoundTrip; want it deleted")
	}
}

func TestStorageRoundTrip_NilStore(t *testing.T) {
	if err := storageRoundTrip(context.Background(), nil); err == nil {
		t.Error("storageRoundTrip(nil) = nil, want error")
	}
}

func TestNewReadinessHandler_AllHealthy(t *testing.T) {
	backend, err := buildStorageBackend(config.StorageConfig{Type: "memory"})
	if err != nil {
		t.Fatalf("buildStorageBackend: %v", err)
	}
	defer backend.Close()

	cfg := &config.Config{Agent: config.AgentConfig{Provider: "anthropic", APIKey: "sk-ant-test"}} //nolint:gosec // G101: fake literal, not a real credential
	h := newReadinessHandler(cfg, backend, true)

	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Status string `json:"status"`
		Checks []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("status = %s, want ok", body.Status)
	}
	names := make(map[string]bool)
	for _, c := range body.Checks {
		names[c.Name] = true
		if c.Status != "ok" {
			t.Errorf("check %s status = %s, want ok", c.Name, c.Status)
		}
	}
	if !names["storage"] || !names["agent_configured"] {
		t.Errorf("checks = %v, want storage and agent_configured", names)
	}
}

func TestNewReadinessHandler_NoAgentConfigured(t *testing.T) {
	backend, err := buildStorageBackend(config.StorageConfig{Type: "memory"})
	if err != nil {
		t.Fatalf("buildStorageBackend: %v", err)
	}
	defer backend.Close()

	// agentConfigured=false: no agent was built at startup, so the
	// agent_configured checker shouldn't even be registered.
	h := newReadinessHandler(&config.Config{}, backend, false)

	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (storage-only check passes)", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "agent_configured") {
		t.Errorf("body = %s, want no agent_configured check when agentConfigured=false", rec.Body.String())
	}
}

func TestNewReadinessHandler_MissingAPIKey(t *testing.T) {
	backend, err := buildStorageBackend(config.StorageConfig{Type: "memory"})
	if err != nil {
		t.Fatalf("buildStorageBackend: %v", err)
	}
	defer backend.Close()

	cfg := &config.Config{Agent: config.AgentConfig{Provider: "anthropic", APIKey: ""}}
	h := newReadinessHandler(cfg, backend, true)

	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 for a missing API key", rec.Code)
	}
}
