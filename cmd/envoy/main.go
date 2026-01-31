// Package main is the entry point for the envoy CLI.
package main

import (
	"os"

	"github.com/agentplexus/envoy/cmd/envoy/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		os.Exit(1)
	}
}
