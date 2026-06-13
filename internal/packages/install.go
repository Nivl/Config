package packages

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"

	"github.com/Nivl/config/internal/brew"
)

// Opts controls optional behaviors of Install and InstallWithRetry.
type Opts struct {
	// FailureResolution pre-answers the failed-packages prompt with
	// "abort", "retry", or "ignore"; empty means "ask interactively."
	// Resolved from --install-failure-resolution /
	// INSTALL_FAILURE_RESOLUTION by the cmd layer. Consumed only by
	// InstallWithRetry — Install itself never prompts.
	FailureResolution string
	// Personal, when true, also installs PersonalCasks. Driven by the
	// PERSONAL_COMPUTER env var in bash; passed via --personal in Go.
	Personal bool
}

// Install runs upgrade → formula install → cask install with
// limp-and-report semantics across all three steps. Writes progress
// lines to w. Always returns a complete Summary, even on partial
// failure — per-package failures do not abort the run; they land in
// the summary's Failed* lists so the caller can offer a retry (see
// InstallWithRetry). Returns a non-nil error only for catastrophic
// situations (context cancelled, brew binary missing or unrunnable).
func Install(ctx context.Context, w io.Writer, r brew.Runner, opts Opts) (Summary, error) {
	var summary Summary

	fmt.Fprintln(w, "Upgrading existing brew packages...")
	if err := r.Upgrade(ctx); err != nil {
		if cerr := catastrophic(ctx, err); cerr != nil {
			return summary, fmt.Errorf("upgrade: %w", cerr)
		}
		failed, oerr := outdatedAfterFailedUpgrade(ctx, w, r, nil)
		if oerr != nil {
			return summary, oerr
		}
		summary.FailedUpgrades = failed
	}

	for _, group := range []struct {
		label    string
		formulae []string
	}{
		{"core utilities", Formulae},
		{"fonts", Fonts},
		{"dev tools", DevTools},
		{"AI", AI},
	} {
		fmt.Fprintf(w, "Installing %s...\n", group.label)
		err := r.Install(ctx, group.formulae...)
		if err == nil {
			continue
		}
		if cerr := catastrophic(ctx, err); cerr != nil {
			return summary, fmt.Errorf("install %s: %w", group.label, cerr)
		}
		// The group call doesn't say which formula broke it. Re-run each
		// one alone to isolate the failures — already-installed formulae
		// are cheap no-ops for brew.
		fmt.Fprintf(w, "Installing %s failed; retrying each formula alone to isolate the failures...\n", group.label)
		failed, cerr := installEachFormula(ctx, w, r, group.formulae)
		if cerr != nil {
			return summary, fmt.Errorf("install %s: %w", group.label, cerr)
		}
		// Mirror the upgrade path's parity note so a transient group
		// failure doesn't resolve without a trace.
		if len(failed) == 0 {
			fmt.Fprintf(w, "Installing %s failed as a group but every formula installed alone; continuing\n", group.label)
		}
		summary.FailedFormulae = append(summary.FailedFormulae, failed...)
	}

	caskGroups := [][]string{CommonCasks, BetaCasks}
	if opts.Personal {
		caskGroups = append(caskGroups, PersonalCasks)
	}

	for _, group := range caskGroups {
		for _, cask := range group {
			outcome, err := r.InstallCask(ctx, cask)
			if err != nil {
				if cerr := catastrophic(ctx, err); cerr != nil {
					return summary, fmt.Errorf("install cask %s: %w", cask, cerr)
				}
				summary.FailedCasks = append(summary.FailedCasks, FailedItem{Name: cask, Reason: err.Error()})
				fmt.Fprintf(w, "Failed to install or upgrade cask %s: %s\n", cask, err.Error())
				continue
			}
			recordCaskOutcome(w, &summary, cask, outcome)
		}
	}

	return summary, nil
}

// recordCaskOutcome folds one InstallCask outcome into the summary and
// writes the matching progress line. Shared by the initial pass and
// the retry pass so the two can't drift.
func recordCaskOutcome(w io.Writer, summary *Summary, cask string, outcome brew.CaskOutcome) {
	switch outcome.Status {
	case brew.StatusInstalled:
		// No-op: successful installs aren't tracked.
	case brew.StatusSkipped:
		summary.Skipped = append(summary.Skipped, cask)
		fmt.Fprintf(w, "Skipping cask update for %s because its app is running\n", cask)
	case brew.StatusFailed:
		summary.FailedCasks = append(summary.FailedCasks, FailedItem{Name: cask, Reason: outcome.Reason})
		fmt.Fprintf(w, "Failed to install or upgrade cask %s: %s\n", cask, outcome.Reason)
	}
}

// catastrophic returns a non-nil error when err is unrecoverable for
// the whole run — the context was cancelled or the brew binary cannot
// run at all — and nil for ordinary per-package failures worth limping
// past.
func catastrophic(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return err
	}
	if errors.Is(err, exec.ErrNotFound) {
		return err
	}
	// A brew that exists but cannot be run (e.g. clobbered permission
	// bits) is as dead as a missing one. The lookup/exec stage fails
	// with *exec.Error — never *exec.ExitError, which means brew ran
	// and exited non-zero, the limp-worthy case.
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return err
	}
	return nil
}

// outdatedAfterFailedUpgrade identifies which packages a failed `brew
// upgrade` left behind. brew limps through every package on its own,
// so whatever is still outdated afterwards is treated as the failed
// set. The list is a proxy, not ground truth: it can also include names
// brew deliberately skips (pinned formulae) or casks the later cask
// phase will fix — accepted, because a retry re-checks with a scope
// and self-corrects. A non-empty scope (the names a retry just
// attempted) filters out packages that became outdated mid-run; when
// the re-check itself fails ordinarily, the scope is kept as the
// failed set so the next retry stays scoped. Only with no scope does
// an unreadable outdated list degrade to the UpgradeAll sentinel (a
// retry then re-runs the full upgrade). A catastrophic failure
// (context cancelled, brew binary missing or unrunnable) is returned
// instead, so the run aborts rather than looping back to a prompt
// that can no longer be served.
func outdatedAfterFailedUpgrade(ctx context.Context, w io.Writer, r brew.Runner, scope []string) ([]FailedItem, error) {
	names, err := r.Outdated(ctx)
	if err != nil {
		if cerr := catastrophic(ctx, err); cerr != nil {
			return nil, cerr
		}
		if len(scope) > 0 {
			items := make([]FailedItem, 0, len(scope))
			for _, n := range scope {
				items = append(items, FailedItem{Name: n, Reason: "could not re-check after a failed retry: " + err.Error()})
			}
			return items, nil
		}
		return []FailedItem{{
			Name:   UpgradeAll,
			Reason: "brew upgrade failed and the failed subset could not be identified: " + err.Error(),
		}}, nil
	}
	if len(scope) > 0 {
		in := make(map[string]bool, len(scope))
		for _, s := range scope {
			in[s] = true
		}
		kept := names[:0:0]
		for _, n := range names {
			if in[n] {
				kept = append(kept, n)
			}
		}
		names = kept
	}
	if len(names) == 0 {
		// With a scope, out-of-scope packages can still be outdated —
		// claim only what was checked.
		if len(scope) > 0 {
			fmt.Fprintln(w, "brew upgrade exited non-zero but none of the retried packages is still outdated; continuing")
		} else {
			fmt.Fprintln(w, "brew upgrade exited non-zero but nothing is left outdated; continuing")
		}
		return nil, nil
	}
	items := make([]FailedItem, 0, len(names))
	for _, n := range names {
		items = append(items, FailedItem{Name: n, Reason: "still outdated after brew upgrade (see brew output above)"})
		fmt.Fprintf(w, "Failed to upgrade %s\n", n)
	}
	return items, nil
}

// installEachFormula installs each formula in its own `brew install`
// call, collecting failures instead of stopping at the first one.
// Returns a catastrophic error only for context cancellation or a
// brew binary that cannot run at all.
func installEachFormula(ctx context.Context, w io.Writer, r brew.Runner, formulae []string) ([]FailedItem, error) {
	var failed []FailedItem
	for _, f := range formulae {
		err := r.Install(ctx, f)
		if err == nil {
			continue
		}
		if cerr := catastrophic(ctx, err); cerr != nil {
			return nil, cerr
		}
		failed = append(failed, FailedItem{Name: f, Reason: err.Error()})
		fmt.Fprintf(w, "Failed to install formula %s: %s\n", f, err.Error())
	}
	return failed, nil
}
