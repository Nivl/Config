package packages

import (
	"context"
	"fmt"
	"io"

	"github.com/Nivl/config/internal/brew"
)

// Opts controls optional behaviors of Install.
type Opts struct {
	// Personal, when true, also installs PersonalCasks. Driven by the
	// PERSONAL_COMPUTER env var in bash; passed via --personal in Go.
	Personal bool
}

// Install runs upgrade → formula install → cask install with
// limp-and-report semantics for casks. Writes progress lines to w.
// Always returns a complete Summary, even on partial failure —
// per-cask failures do not abort the run. Returns a non-nil error
// only for catastrophic situations (context cancelled, formula
// install failure, brew binary missing). Per-cask failures live in
// summary.Failed.
func Install(ctx context.Context, w io.Writer, r brew.Runner, opts Opts) (Summary, error) {
	var summary Summary

	fmt.Fprintln(w, "Upgrading existing brew packages...")
	if err := r.Upgrade(ctx); err != nil {
		return summary, fmt.Errorf("upgrade: %w", err)
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
		if err := r.Install(ctx, group.formulae...); err != nil {
			return summary, fmt.Errorf("install %s: %w", group.label, err)
		}
	}

	caskGroups := [][]string{CommonCasks, BetaCasks}
	if opts.Personal {
		caskGroups = append(caskGroups, PersonalCasks)
	}

	for _, group := range caskGroups {
		for _, cask := range group {
			outcome, err := r.InstallCask(ctx, cask)
			if err != nil {
				return summary, fmt.Errorf("install cask %s: %w", cask, err)
			}
			switch outcome.Status {
			case brew.StatusInstalled:
				// No-op: successful installs aren't tracked (bash didn't either).
			case brew.StatusSkipped:
				summary.Skipped = append(summary.Skipped, cask)
				fmt.Fprintf(w, "Skipping cask update for %s because its app is running\n", cask)
			case brew.StatusFailed:
				summary.Failed = append(summary.Failed, FailedCask{Name: cask, Reason: outcome.Reason})
				fmt.Fprintf(w, "Failed to install or upgrade cask %s: %s\n", cask, outcome.Reason)
			}
		}
	}

	return summary, nil
}
