package packages

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"

	"github.com/Nivl/config/internal/brew"
)

// Opts controls optional behaviors of Install.
type Opts struct {
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
// situations (context cancelled, brew binary missing).
func Install(ctx context.Context, w io.Writer, r brew.Runner, opts Opts) (Summary, error) {
	var summary Summary

	fmt.Fprintln(w, "Upgrading existing brew packages...")
	if err := r.Upgrade(ctx); err != nil {
		if cerr := catastrophic(ctx, err); cerr != nil {
			return summary, fmt.Errorf("upgrade: %w", cerr)
		}
		summary.FailedUpgrades = outdatedAfterFailedUpgrade(ctx, w, r, nil)
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
		// No-op: successful installs aren't tracked (bash didn't either).
	case brew.StatusSkipped:
		summary.Skipped = append(summary.Skipped, cask)
		fmt.Fprintf(w, "Skipping cask update for %s because its app is running\n", cask)
	case brew.StatusFailed:
		summary.FailedCasks = append(summary.FailedCasks, FailedItem{Name: cask, Reason: outcome.Reason})
		fmt.Fprintf(w, "Failed to install or upgrade cask %s: %s\n", cask, outcome.Reason)
	}
}

// catastrophic returns a non-nil error when err is unrecoverable for
// the whole run — the context was cancelled or the brew binary is
// missing — and nil for ordinary per-package failures worth limping
// past.
func catastrophic(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return err
	}
	if errors.Is(err, exec.ErrNotFound) {
		return err
	}
	return nil
}

// outdatedAfterFailedUpgrade identifies which packages a failed `brew
// upgrade` left behind. brew limps through every package on its own,
// so whatever is still outdated afterwards is exactly the failed set.
// A non-empty scope (the names a retry just attempted) filters out
// packages that became outdated mid-run. When `brew outdated` itself
// fails the set is unknown — the UpgradeAll sentinel is recorded so a
// retry re-runs the full upgrade.
func outdatedAfterFailedUpgrade(ctx context.Context, w io.Writer, r brew.Runner, scope []string) []FailedItem {
	names, err := r.Outdated(ctx)
	if err != nil {
		return []FailedItem{{
			Name:   UpgradeAll,
			Reason: "brew upgrade failed and the failed subset could not be identified: " + err.Error(),
		}}
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
		fmt.Fprintln(w, "brew upgrade exited non-zero but nothing is left outdated; continuing")
		return nil
	}
	items := make([]FailedItem, 0, len(names))
	for _, n := range names {
		items = append(items, FailedItem{Name: n, Reason: "still outdated after brew upgrade (see brew output above)"})
		fmt.Fprintf(w, "Failed to upgrade %s\n", n)
	}
	return items
}

// installEachFormula installs each formula in its own `brew install`
// call, collecting failures instead of stopping at the first one.
// Returns a catastrophic error only for context cancellation or a
// missing brew binary.
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
