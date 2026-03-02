// Package main is the entry point for the omniagent CLI.
package main

import (
	"os"

	"github.com/plexusone/omniagent/cmd/omniagent/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		os.Exit(1)
	}
}
