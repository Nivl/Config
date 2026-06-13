package packages

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/Nivl/config/internal/brew"
	"github.com/Nivl/config/internal/userinput"
)

// ErrAborted is returned by InstallWithRetry when the user answers the
// failed-packages prompt with "abort".
var ErrAborted = errors.New("aborted: some packages failed to install")

// InstallWithRetry runs Install, then — as long as the summary carries
// failures — lists them and asks the user whether to abort, retry just
// the failed items, or ignore the failures and continue. The choice is
// read from in; the failure list and the question go to promptOut
// (stderr at both call sites, like every sibling prompt) so they reach
// the terminal even when out is redirected, while progress lines keep
// flowing to out. opts.FailureResolution pre-answers the first ask;
// any later ask — reachable only when a pre-resolved retry leaves
// failures behind — prompts interactively. Returns ErrAborted on
// abort; on ignore the returned summary still carries the failures but
// the error is nil, so callers continue with the rest of their
// pipeline.
func InstallWithRetry(ctx context.Context, in io.Reader, out, promptOut io.Writer, r brew.Runner, opts Opts) (Summary, error) {
	summary, err := Install(ctx, out, r, opts)
	if err != nil {
		return summary, err
	}
	// One reader for every prompt iteration: a fresh bufio per ask would
	// drop bytes the previous one had buffered past its consumed line
	// (bufio.NewReader returns an existing *bufio.Reader unchanged).
	br := bufio.NewReader(in)
	prearg := opts.FailureResolution
	for summary.HasFailures() {
		summary.PrintFailures(promptOut)
		choice, err := userinput.InstallRetryChoice(prearg, br, promptOut)
		if err != nil {
			return summary, fmt.Errorf("failed-packages prompt: %w", err)
		}
		// A pre-resolved answer applies to the first ask only — keeping
		// "retry" for every ask would loop forever on a package that
		// never recovers.
		prearg = ""
		switch choice {
		case userinput.InstallRetryAbort:
			return summary, ErrAborted
		case userinput.InstallRetryIgnore:
			fmt.Fprintln(promptOut, "Ignoring the failed packages and continuing.")
			return summary, nil
		case userinput.InstallRetryAgain:
			summary, err = retryFailed(ctx, out, r, summary)
			if err != nil {
				return summary, err
			}
		}
	}
	return summary, nil
}

// retryFailed re-attempts only the failed items from prev and returns
// a fresh Summary: Skipped carries over (those casks were deliberately
// deferred and are not retried), each Failed* list is rebuilt from this
// pass's outcomes.
func retryFailed(ctx context.Context, w io.Writer, r brew.Runner, prev Summary) (Summary, error) {
	next := Summary{Skipped: prev.Skipped}

	if len(prev.FailedUpgrades) > 0 {
		names := failedNames(prev.FailedUpgrades)
		if len(names) == 1 && names[0] == UpgradeAll {
			// The failed subset was unknown — re-run the full upgrade.
			names = nil
		}
		fmt.Fprintln(w, "Retrying brew upgrade...")
		if err := r.Upgrade(ctx, names...); err != nil {
			if cerr := catastrophic(ctx, err); cerr != nil {
				return next, fmt.Errorf("retry upgrade: %w", cerr)
			}
			failed, oerr := outdatedAfterFailedUpgrade(ctx, w, r, names)
			if oerr != nil {
				return next, oerr
			}
			next.FailedUpgrades = failed
		}
	}

	if len(prev.FailedFormulae) > 0 {
		fmt.Fprintf(w, "Retrying %d formula install(s)...\n", len(prev.FailedFormulae))
		failed, cerr := installEachFormula(ctx, w, r, failedNames(prev.FailedFormulae))
		if cerr != nil {
			return next, fmt.Errorf("retry formulae: %w", cerr)
		}
		next.FailedFormulae = failed
	}

	if len(prev.FailedCasks) > 0 {
		fmt.Fprintf(w, "Retrying %d cask install(s)...\n", len(prev.FailedCasks))
	}
	for _, item := range prev.FailedCasks {
		outcome, err := r.InstallCask(ctx, item.Name)
		if err != nil {
			if cerr := catastrophic(ctx, err); cerr != nil {
				return next, fmt.Errorf("retry cask %s: %w", item.Name, cerr)
			}
			next.FailedCasks = append(next.FailedCasks, FailedItem{Name: item.Name, Reason: err.Error()})
			fmt.Fprintf(w, "Failed to install or upgrade cask %s: %s\n", item.Name, err.Error())
			continue
		}
		recordCaskOutcome(w, &next, item.Name, outcome)
	}

	return next, nil
}

// failedNames extracts the package names from a failure list.
func failedNames(items []FailedItem) []string {
	names := make([]string, 0, len(items))
	for _, it := range items {
		names = append(names, it.Name)
	}
	return names
}
