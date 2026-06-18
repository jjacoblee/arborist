// Command arb is the command-line interface for Arborist, a guided workflow
// tool for managing Git worktrees across multiple repositories.
package main

import (
	"os"

	"github.com/jjacoblee/arborist/internal/cli"
)

// version is the Arborist version. It defaults to "dev" and can be overridden
// at build time:
//
//	go build -ldflags "-X main.version=0.1.0" ./cmd/arb
var version = "dev"

func main() {
	if err := cli.Execute(version); err != nil {
		// Cobra already prints the error to stderr; here we only set the
		// process exit code so scripts can detect failure.
		os.Exit(1)
	}
}
