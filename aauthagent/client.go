// Package aauthagent makes omniagent an AAuth agent
// (draft-hardt-oauth-aauth-protocol): it enrolls a key at an agent
// provider and returns an HTTP client that signs every request as that
// agent and answers the requirements a resource raises, obtaining person
// and auth tokens from the person's person server and waiting while the
// person approves.
//
// Give the client to an OpenAPI skill to call an AAuth resource:
//
//	client, err := aauthagent.NewClient(ctx, aauthagent.Config{
//		AgentProvider: "https://ap.aauth.test",
//		EnrollPath:    "/enroll",
//		OnInteraction: func(i aauthagent.Interaction) { fmt.Println("Approve at", i.Link()) },
//	})
//	skill := openapi.NewSkill(openapi.Config{
//		Name:       "petstore",
//		SpecURL:    "https://petstore.aauth.test/openapi.json",
//		HTTPClient: client.HTTPClient(),
//	})
package aauthagent

import (
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	aauth "github.com/aauth-dev/aauth-go"
	"github.com/aauth-dev/aauth-go/agentprovider"
)

// Config configures NewClient.
type Config struct {
	// AgentProvider is the agent provider's issuer URL. Required.
	AgentProvider string
	// EnrollPath is the provider's token endpoint path (default
	// agentprovider.DefaultPaths().Token). The aauth-go sandbox uses
	// /enroll.
	EnrollPath string
	// Enrollment is sent with the enrollment request: the provider's own
	// enrollment fields. Nil sends {}.
	Enrollment any
	// Key is the agent's signing key. Nil generates an Ed25519 key, so the
	// agent is new each time; pass a stored key to keep its identity.
	Key crypto.Signer
	// RootCAsPEM holds certificates to trust as well as the system roots,
	// such as the aauth-go sandbox CA.
	RootCAsPEM []byte
	// Base is the transport requests are sent on (default: one trusting
	// the system roots and RootCAsPEM).
	Base http.RoundTripper
	// OnInteraction is called when the person must act, such as approving
	// access at their person server. The request waits meanwhile.
	OnInteraction func(Interaction)
}

// Interaction asks the person to act at URL, entering or confirming Code.
type Interaction struct {
	URL  string
	Code string
}

// Link returns URL with the code as its code query parameter, which an
// interaction page accepts in place of typing it.
func (i Interaction) Link() string {
	u, err := url.Parse(i.URL)
	if err != nil || i.Code == "" {
		return i.URL
	}
	q := u.Query()
	q.Set("code", i.Code)
	u.RawQuery = q.Encode()
	return u.String()
}

// Client is an enrolled AAuth agent.
type Client struct {
	agent *aauth.Agent
	hc    *http.Client
}

// NewClient enrolls the agent's key at the agent provider and returns a
// client acting as that agent.
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.AgentProvider == "" {
		return nil, errors.New("aauthagent: Config.AgentProvider is required")
	}
	base := cfg.Base
	if base == nil {
		var err error
		if base, err = transportTrusting(cfg.RootCAsPEM); err != nil {
			return nil, err
		}
	}
	plain := &http.Client{Transport: base, Timeout: 30 * time.Second}
	key := cfg.Key
	if key == nil {
		var err error
		if key, err = aauth.GenerateKey(aauth.AlgEd25519); err != nil {
			return nil, err
		}
	}
	paths := agentprovider.DefaultPaths()
	if cfg.EnrollPath != "" {
		paths.Token = cfg.EnrollPath
	}
	ap := &agentprovider.Client{BaseURL: cfg.AgentProvider, Paths: paths, HTTPClient: plain}
	agent, err := ap.Enroll(ctx, key, cfg.Enrollment)
	if err != nil {
		return nil, fmt.Errorf("aauthagent: enrolling at %s: %w", cfg.AgentProvider, err)
	}

	relay := func(r aauth.Requirement) {
		if r.Requirement == aauth.RequirementInteraction && cfg.OnInteraction != nil {
			cfg.OnInteraction(Interaction{URL: r.URL, Code: r.Code})
		}
	}
	var ps *aauth.PSClient
	if agent.PS != "" {
		ps = aauth.NewPSClient(agent.PS, agent)
		ps.HTTPClient = plain
		ps.OnRequirement = relay
	}
	tr := aauth.NewTransport(agent, ps)
	tr.Base = base
	tr.ResourceVerify = aauth.TokenVerifyOptions{Resolver: aauth.NewJWKSResolver(plain)}
	tr.Capabilities = []string{aauth.CapabilityInteraction}
	tr.OnRequirement = relay
	// No client timeout: a request may wait for the person; bound it with
	// the request's context instead.
	return &Client{agent: agent, hc: &http.Client{Transport: tr}}, nil
}

// HTTPClient returns the client that signs and authorizes requests.
func (c *Client) HTTPClient() *http.Client { return c.hc }

// AgentID returns the agent's identifier, such as
// aauth:assistant@ap.aauth.test.
func (c *Client) AgentID() string { return c.agent.ID.String() }

// PersonServer returns the person server the agent token names.
func (c *Client) PersonServer() string { return c.agent.PS }

func transportTrusting(extraPEM []byte) (http.RoundTripper, error) {
	dt, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("aauthagent: http.DefaultTransport is not an *http.Transport")
	}
	tr := dt.Clone()
	if len(extraPEM) > 0 {
		roots, err := x509.SystemCertPool()
		if err != nil || roots == nil {
			roots = x509.NewCertPool() // minimal containers have no system pool
		}
		if !roots.AppendCertsFromPEM(extraPEM) {
			return nil, errors.New("aauthagent: no certificates in RootCAsPEM")
		}
		tr.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	}
	return tr, nil
}
