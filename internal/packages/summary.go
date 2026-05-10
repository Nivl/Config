package packages

import (
	"fmt"
	"io"
)

// Summary aggregates the outcomes of an Install run. Skipped and
// Failed feed the "Skipped cask updates" and "Failed cask installs"
// trailers; successful installs are not tracked. Formulae are not
// tracked individually — a formula failure aborts the run before
// Summary is returned.
type Summary struct {
	// Skipped lists casks that were already installed and whose app was
	// running, so the upgrade was deferred.
	Skipped []string
	// Failed lists casks whose `brew install --cask` exited non-zero, with
	// the trimmed last non-empty line of stderr captured as Reason.
	Failed []FailedCask
}

// FailedCask pairs a failed cask name with the captured failure reason.
type FailedCask struct {
	// Name is the brew cask name (e.g. "raycast").
	Name string
	// Reason is the trimmed last non-empty line of `brew install --cask`
	// stderr, produced by brew.extractCaskFailureReason.
	Reason string
}

// Print writes the human-readable "Skipped cask updates" and "Failed
// cask installs or upgrades" trailers. Empty sections are omitted.
func (s Summary) Print(w io.Writer) {
	if len(s.Skipped) > 0 {
		fmt.Fprint(w, "\nSkipped cask updates because the app is running:\n")
		for _, name := range s.Skipped {
			fmt.Fprintf(w, "\t- %s\n", name)
		}
	}
	if len(s.Failed) > 0 {
		fmt.Fprint(w, "\nFailed cask installs or upgrades:\n")
		for _, fc := range s.Failed {
			fmt.Fprintf(w, "\t- %s: %s\n", fc.Name, fc.Reason)
		}
	}
}

// HasFailures reports whether any cask failed. Drives the exit code in the
// cobra layer (Summary.HasFailures() → exit 1).
func (s Summary) HasFailures() bool {
	return len(s.Failed) > 0
}
