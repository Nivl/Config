package brew

import (
	"context"
	"fmt"
	"os/exec"
)

// Upgrade runs `brew upgrade` to bring all installed packages current.
// Stdout/stderr are inherited from the parent so the user sees brew's
// live progress.
// A cancelled context propagates as ctx.Err() rather than the
// "signal: killed" wrap from c.Run(), so callers can distinguish
// catastrophic cancellation from real brew failures.
func (r *runner) Upgrade(ctx context.Context) error {
	c := exec.CommandContext(ctx, "brew", "upgrade")
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
