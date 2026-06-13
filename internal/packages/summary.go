package packages

import (
	"fmt"
	"io"
)

// Summary aggregates the outcomes of an Install run. Failures are
// collected rather than aborting the run, so the caller can offer a
// retry of just the failed items. Successful installs are not tracked.
type Summary struct {
	// Skipped lists casks that were already installed and whose app was
	// running, so the upgrade was deferred.
	Skipped []string
	// FailedUpgrades lists packages still outdated after `brew upgrade`
	// exited non-zero. May hold the single UpgradeAll sentinel when the
	// failed subset could not be identified.
	FailedUpgrades []FailedItem
	// FailedFormulae lists formulae whose isolated `brew install` exited
	// non-zero after their group install failed.
	FailedFormulae []FailedItem
	// FailedCasks lists casks whose `brew install --cask` exited non-zero
	// (Reason is the trimmed last non-empty stderr line) or whose
	// InstallCask call itself errored (Reason is the wrapped error).
	FailedCasks []FailedItem
}

// UpgradeAll is the sentinel FailedItem name recorded when a bulk
// `brew upgrade` failed AND `brew outdated` could not identify the
// failed subset. Retrying it re-runs the unscoped `brew upgrade`.
const UpgradeAll = "(all outdated packages)"

// FailedItem pairs a failed package name with the captured failure reason.
type FailedItem struct {
	// Name is the brew package name (formula or cask), or the UpgradeAll
	// sentinel.
	Name string
	// Reason is a short human-readable cause: the trimmed last non-empty
	// stderr line for casks (or the wrapped error when the InstallCask
	// call itself failed), the wrapped exec error for formulae, and for
	// upgrades a "still outdated" / "could not re-check" note — or, on
	// the UpgradeAll sentinel, the wrapped `brew outdated` failure.
	Reason string
}

// PrintSkipped writes the skipped-casks trailer; an empty list prints
// nothing. The failure sections are owned by the InstallWithRetry
// prompt loop (PrintFailures), so callers print only this trailer once
// the loop returns — re-printing the failures would duplicate what the
// prompt just showed.
func (s Summary) PrintSkipped(w io.Writer) {
	if len(s.Skipped) == 0 {
		return
	}
	fmt.Fprint(w, "\nSkipped cask updates because the app is running:\n")
	for _, name := range s.Skipped {
		fmt.Fprintf(w, "\t- %s\n", name)
	}
}

// PrintFailures writes only the failure sections — used by the
// retry prompt loop, which re-lists the failures before each ask
// without repeating the skipped-casks trailer. Empty sections are
// omitted.
func (s Summary) PrintFailures(w io.Writer) {
	printFailedSection(w, "Failed upgrades:", s.FailedUpgrades)
	printFailedSection(w, "Failed formula installs:", s.FailedFormulae)
	printFailedSection(w, "Failed cask installs or upgrades:", s.FailedCasks)
}

// printFailedSection writes one titled failure list; an empty list
// prints nothing so callers can emit sections unconditionally.
func printFailedSection(w io.Writer, title string, items []FailedItem) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(w, "\n%s\n", title)
	for _, it := range items {
		fmt.Fprintf(w, "\t- %s: %s\n", it.Name, it.Reason)
	}
}

// HasFailures reports whether any upgrade, formula, or cask failed.
func (s Summary) HasFailures() bool {
	return len(s.FailedUpgrades)+len(s.FailedFormulae)+len(s.FailedCasks) > 0
}
