package commands

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthcheckCmd_Healthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("probed path = %s, want /health", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	addr := srv.Listener.Addr().String()
	t.Setenv("OMNIAGENT_GATEWAY_ADDRESS", addr)

	if err := healthcheckCmd.RunE(healthcheckCmd, nil); err != nil {
		t.Errorf("healthcheck RunE() = %v, want nil", err)
	}
}

func TestHealthcheckCmd_Unhealthy(t *testing.T) {
	// Nothing listening on this address.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	t.Setenv("OMNIAGENT_GATEWAY_ADDRESS", addr)

	if err := healthcheckCmd.RunE(healthcheckCmd, nil); err == nil {
		t.Error("healthcheck RunE() = nil, want error when gateway is unreachable")
	}
}

func TestHealthcheckCmd_DefaultAddress(t *testing.T) {
	// No OMNIAGENT_GATEWAY_ADDRESS set: falls back to 127.0.0.1:18789,
	// which nothing is listening on in this test — just confirms the
	// default-address path doesn't panic and fails cleanly.
	t.Setenv("OMNIAGENT_GATEWAY_ADDRESS", "")

	if err := healthcheckCmd.RunE(healthcheckCmd, nil); err == nil {
		t.Error("healthcheck RunE() = nil, want error against the default address with nothing listening")
	}
}
