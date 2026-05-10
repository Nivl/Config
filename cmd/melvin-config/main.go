// Package main is the entrypoint for the melvin-config CLI. The cobra
// subcommand graph lives in internal/cmd; this file is the thin binary
// shell around it (signal handling, stdio, error sink).
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Nivl/config/internal/cmd"
	"github.com/Nivl/config/internal/iox"
)

// version is set at build time via -ldflags "-X main.version=...".
// Passed through to internal/cmd's `version` subcommand via NewRoot.
var version = "dev"

// main is the binary entrypoint. It resolves the current working directory,
// installs a SIGINT/SIGTERM-cancellable context, constructs the root cobra
// command with stdio writers, and runs it. Errors are printed by exitError
// and surface as exit code 1.
func main() {
	cwd, err := os.Getwd()
	if err != nil {
		exitError(err)
	}
	// Wire signal-driven cancellation so subcommand contexts (and the
	// brew/git subprocesses they spawn via exec.CommandContext) get a
	// chance to clean up on Ctrl-C instead of dying mid-write.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	root := cmd.NewRoot(version, cwd, iox.System())
	if err := root.ExecuteContext(ctx); err != nil {
		exitError(err)
	}
}

// exitError prints err to stderr and exits with code 1. Used by main as
// the single error sink so error formatting is uniform across the binary.
func exitError(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
