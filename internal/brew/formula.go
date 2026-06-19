package brew

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Upgrade runs `brew upgrade` (optionally scoped to the given package
// names) to bring installed packages current. Stdout/stderr are
// inherited from the parent so the user sees brew's live progress.
// A cancelled context propagates as ctx.Err() rather than the
// "signal: killed" wrap from c.Run(), so callers can distinguish
// catastrophic cancellation from real brew failures.
//
// With no package names the upgrade is scoped to --formula. A bare
// `brew upgrade` also upgrades outdated casks and quits their running
// apps by default — that would clobber a running app before the
// per-cask running check in InstallCask ever runs. Casks are upgraded
// individually (and skipped while running) by InstallCask instead.
func (r *runner) Upgrade(ctx context.Context, packages ...string) error {
	args := []string{"upgrade"}
	if len(packages) == 0 {
		args = append(args, "--formula")
	}
	args = append(args, packages...)
	c := exec.CommandContext(ctx, "brew", args...)
	c.Stdout = r.streams.Out
	c.Stderr = r.streams.Err
	err := c.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("brew upgrade: %w", ctxErr)
	}
	if err != nil {
		return fmt.Errorf("brew upgrade: %w", err)
	}
	return nil
}

// Outdated runs `brew outdated --quiet` (optionally scoped, e.g. with
// "--formula") and returns the package names that still have a newer
// version available. Stdout is captured rather than inherited — the
// names are data, not progress. Stderr still flows to the user's
// stream.
func (r *runner) Outdated(ctx context.Context, scopes ...string) ([]string, error) {
	var out bytes.Buffer
	args := append([]string{"outdated", "--quiet"}, scopes...)
	c := exec.CommandContext(ctx, "brew", args...)
	c.Stdout = &out
	c.Stderr = r.streams.Err
	err := c.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, fmt.Errorf("brew outdated: %w", ctxErr)
	}
	if err != nil {
		return nil, fmt.Errorf("brew outdated: %w", err)
	}
	var names []string
	for line := range strings.SplitSeq(out.String(), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}

// Install runs `brew install <formulae...>` as a single call. With no
// formulae the call is skipped entirely — brew install with no arguments
// would error, but we treat empty-list as a no-op. Stdout/stderr are
// inherited from the parent so the user sees brew's live progress.
// A cancelled context propagates as ctx.Err() rather than the
// "signal: killed" wrap from c.Run(), so callers can distinguish
// catastrophic cancellation from real brew failures.
func (r *runner) Install(ctx context.Context, formulae ...string) error {
	if len(formulae) == 0 {
		return nil
	}
	args := append([]string{"install"}, formulae...)
	c := exec.CommandContext(ctx, "brew", args...)
	c.Stdout = r.streams.Out
	c.Stderr = r.streams.Err
	err := c.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("brew install %v: %w", formulae, ctxErr)
	}
	if err != nil {
		return fmt.Errorf("brew install %v: %w", formulae, err)
	}
	return nil
}
