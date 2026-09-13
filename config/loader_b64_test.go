package config

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const b64TestYAML = `
gateway:
  address: "0.0.0.0:9999"
web:
  enabled: true
team:
  database:
    app_dsn: "file:/data/b64-test.db"
`

// TestLoad_ConfigB64 covers RMI-OMNIAGENT-031: a full config document
// injected through one env var, reaching nested fields (team.database)
// that individual OMNIAGENT_* env vars cannot express.
func TestLoad_ConfigB64(t *testing.T) {
	t.Setenv("OMNIAGENT_CONFIG_B64", base64.StdEncoding.EncodeToString([]byte(b64TestYAML)))

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Gateway.Address != "0.0.0.0:9999" {
		t.Errorf("Gateway.Address = %q, want value from B64 config", cfg.Gateway.Address)
	}
	if !cfg.Web.Enabled {
		t.Error("Web.Enabled = false, want true from B64 config")
	}
	if cfg.Team.Database.AppDSN != "file:/data/b64-test.db" {
		t.Errorf("Team.Database.AppDSN = %q, want nested value from B64 config", cfg.Team.Database.AppDSN)
	}
}

func TestLoad_ConfigB64_AllAlphabets(t *testing.T) {
	encodings := map[string]*base64.Encoding{
		"std":    base64.StdEncoding,
		"rawstd": base64.RawStdEncoding,
		"url":    base64.URLEncoding,
		"rawurl": base64.RawURLEncoding,
	}
	for name, enc := range encodings {
		t.Run(name, func(t *testing.T) {
			t.Setenv("OMNIAGENT_CONFIG_B64", enc.EncodeToString([]byte(b64TestYAML)))
			cfg, err := Load("")
			if err != nil {
				t.Fatalf("Load with %s alphabet: %v", name, err)
			}
			if cfg.Gateway.Address != "0.0.0.0:9999" {
				t.Errorf("Gateway.Address = %q, want B64 value", cfg.Gateway.Address)
			}
		})
	}
}

func TestLoad_ConfigB64_JSONPayload(t *testing.T) {
	doc := `{"gateway": {"address": "0.0.0.0:7777"}}`
	t.Setenv("OMNIAGENT_CONFIG_B64", base64.StdEncoding.EncodeToString([]byte(doc)))
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Gateway.Address != "0.0.0.0:7777" {
		t.Errorf("Gateway.Address = %q, want JSON B64 value", cfg.Gateway.Address)
	}
}

func TestLoad_ConfigB64_ExplicitPathWins(t *testing.T) {
	t.Setenv("OMNIAGENT_CONFIG_B64", base64.StdEncoding.EncodeToString([]byte(b64TestYAML)))

	dir := t.TempDir()
	path := filepath.Join(dir, "explicit.yaml")
	if err := os.WriteFile(path, []byte("gateway:\n  address: \"127.0.0.1:1111\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Gateway.Address != "127.0.0.1:1111" {
		t.Errorf("Gateway.Address = %q, want explicit --config file to win over OMNIAGENT_CONFIG_B64", cfg.Gateway.Address)
	}
	if cfg.Web.Enabled {
		t.Error("Web.Enabled = true — B64 config must be ignored entirely when a path is given, not merged")
	}
}

func TestLoad_ConfigB64_EnvOverridesOnTop(t *testing.T) {
	t.Setenv("OMNIAGENT_CONFIG_B64", base64.StdEncoding.EncodeToString([]byte(b64TestYAML)))
	t.Setenv("OMNIAGENT_GATEWAY_ADDRESS", "0.0.0.0:8080")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Gateway.Address != "0.0.0.0:8080" {
		t.Errorf("Gateway.Address = %q, want individual env var to override the B64 document", cfg.Gateway.Address)
	}
	if !cfg.Web.Enabled {
		t.Error("Web.Enabled = false, want B64 value preserved where no env override exists")
	}
}

func TestLoad_ConfigB64_Errors(t *testing.T) {
	t.Run("invalid base64", func(t *testing.T) {
		t.Setenv("OMNIAGENT_CONFIG_B64", "!!!not-base64!!!")
		if _, err := Load(""); err == nil || !strings.Contains(err.Error(), "OMNIAGENT_CONFIG_B64") {
			t.Errorf("Load = %v, want OMNIAGENT_CONFIG_B64 decode error", err)
		}
	})
	t.Run("valid base64, unparseable payload", func(t *testing.T) {
		t.Setenv("OMNIAGENT_CONFIG_B64", base64.StdEncoding.EncodeToString([]byte("{{{ not yaml or json")))
		if _, err := Load(""); err == nil || !strings.Contains(err.Error(), "OMNIAGENT_CONFIG_B64") {
			t.Errorf("Load = %v, want parse error", err)
		}
	})
}
