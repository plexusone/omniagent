// Command aauth-petstore is a terminal chat with omniagent acting as an
// AAuth agent for the Petstore: the Petstore's OpenAPI operations are its
// tools, and every call is signed as an agent enrolled at an agent
// provider. When an operation needs the person's approval, the chat prints
// the approval link and the call waits until the person approves.
//
// Run it against the AAuth sandbox and the Petstore (see README.md):
//
//	ANTHROPIC_API_KEY=… aauth-petstore --ca-file ~/.aauth-sandbox/ca.pem
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/plexusone/omnistorage-core/kvs/backend/memory"

	"github.com/plexusone/omniagent/aauthagent"
	"github.com/plexusone/omniagent/agent"
	"github.com/plexusone/omniagent/config"
	"github.com/plexusone/omniagent/sessions"
	"github.com/plexusone/omniagent/skills/remote/openapi"
)

const systemPrompt = `You are a shopping assistant at the Petstore, acting for the person you are
chatting with. Use the petstore tools to browse pets, place and check orders,
and manage the catalog. Each call is made as an AAuth agent: browsing needs
only your identity, while placing or cancelling an order, or changing the
catalog, needs the person's approval. When a tool call needs approval, the
person sees a link to approve it and the call waits; tell them what you are
about to do and why before you call such a tool. Keep answers short.`

type options struct {
	agentProvider string
	enrollPath    string
	petstore      string
	caFile        string
	configFile    string
	prompts       []string

	base http.RoundTripper // tests: reach the sandbox by name
}

func main() {
	if err := newRoot(os.Stdin, os.Stdout, os.Stderr).Execute(); err != nil {
		os.Exit(1)
	}
}

func newRoot(in io.Reader, out, errOut io.Writer) *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "aauth-petstore",
		Short: "Chat with omniagent as an AAuth agent for the Petstore",
		Long: `Chat with omniagent as an AAuth agent for the Petstore.

The model is configured as for omniagent: OMNIAGENT_AGENT_PROVIDER,
OMNIAGENT_AGENT_MODEL, and the provider's API key (ANTHROPIC_API_KEY,
OPENAI_API_KEY or GEMINI_API_KEY), or a config file.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return o.run(ctx, in, out, errOut)
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.agentProvider, "agent-provider", "https://ap.aauth.test", "agent provider to enroll at")
	f.StringVar(&o.enrollPath, "enroll-path", "/enroll", "the agent provider's token endpoint path")
	f.StringVar(&o.petstore, "petstore", "https://petstore.aauth.test", "the Petstore's URL")
	f.StringVar(&o.caFile, "ca-file", os.Getenv("AAUTH_CA_FILE"), "PEM file of an extra root CA to trust, such as the sandbox CA (default $AAUTH_CA_FILE)")
	f.StringVar(&o.configFile, "config", "", "omniagent config file for the model settings")
	f.StringArrayVar(&o.prompts, "prompt", nil, "send this prompt instead of reading the terminal (repeatable, in order)")
	return cmd
}

func (o *options) run(ctx context.Context, in io.Reader, out, errOut io.Writer) error {
	cfg, err := config.Load(o.configFile)
	if err != nil {
		return fmt.Errorf("loading the model settings: %w", err)
	}
	var caPEM []byte
	if o.caFile != "" {
		if caPEM, err = os.ReadFile(o.caFile); err != nil {
			return fmt.Errorf("--ca-file: %w", err)
		}
	}
	client, err := aauthagent.NewClient(ctx, aauthagent.Config{
		AgentProvider: o.agentProvider,
		EnrollPath:    o.enrollPath,
		RootCAsPEM:    caPEM,
		Base:          o.base,
		OnInteraction: func(i aauthagent.Interaction) {
			printf(errOut, "\n  Approval needed. Open %s\n  (code %s). Waiting for the approval...\n\n", i.Link(), i.Code)
		},
	})
	if err != nil {
		return err
	}
	printf(errOut, "Enrolled as %s; person server %s\n", client.AgentID(), client.PersonServer())

	logger := slog.New(slog.NewTextHandler(errOut, &slog.HandlerOptions{Level: slog.LevelWarn}))
	a, err := agent.New(agent.Config{
		Provider: cfg.Agent.Provider, Model: cfg.Agent.Model, APIKey: cfg.Agent.APIKey, BaseURL: cfg.Agent.BaseURL,
		MaxTokens: cfg.Agent.MaxTokens, SystemPrompt: systemPrompt, Logger: logger,
	},
		agent.WithSessionStore(sessions.NewStore(sessions.StoreConfig{Backend: memory.New()})),
		agent.WithOpenAPISkill(openapi.Config{
			Name:        "petstore",
			Description: "The Petstore: browse pets, place and cancel orders, manage the catalog.",
			SpecURL:     strings.TrimSuffix(o.petstore, "/") + "/openapi.json",
			HTTPClient:  client.HTTPClient(),
		}),
	)
	if err != nil {
		return err
	}
	defer func() {
		if err := a.Close(); err != nil {
			logger.Error("closing the agent", "error", err)
		}
	}()
	// Load the Petstore's spec, through the AAuth client, and offer its
	// operations to the model as tools.
	if err := a.InitCompiledSkills(ctx); err != nil {
		return err
	}

	ask := func(prompt string) error {
		reply, err := a.ProcessWithSession(ctx, "aauth-petstore", prompt)
		if err != nil {
			return err
		}
		printf(out, "%s\n\n", reply)
		return nil
	}
	if len(o.prompts) > 0 {
		for _, p := range o.prompts {
			printf(out, "> %s\n", p)
			if err := ask(p); err != nil {
				return err
			}
		}
		return nil
	}
	printf(errOut, "Ask about pets, for example: what dogs are available? (Ctrl-D to quit)\n")
	sc := bufio.NewScanner(in)
	for {
		printf(out, "> ")
		if !sc.Scan() {
			printf(out, "\n")
			return sc.Err()
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if err := ask(line); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			printf(errOut, "error: %v\n", err)
		}
	}
}

// printf writes to the terminal; a failed write there has nowhere better
// to be reported.
func printf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...) //nolint:errcheck // see above
}
