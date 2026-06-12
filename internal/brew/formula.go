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
func (r *runner) Upgrade(ctx context.Context, packages ...string) error {
	args := append([]string{"upgrade"}, packages...)
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

// Outdated runs `brew outdated --quiet` and returns the package names
// (formulae and casks) that still have a newer version available.
// Stdout is captured rather than inherited — the names are data, not
// progress. Stderr still flows to the user's stream.
func (r *runner) Outdated(ctx context.Context) ([]string, error) {
	var out bytes.Buffer
	c := exec.CommandContext(ctx, "brew", "outdated", "--quiet")
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
