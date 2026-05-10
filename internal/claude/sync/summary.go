// Package sync orchestrates the Claude Code config sync. It
// detects the mode, runs the settings + file + directory merges (Copy
// mode) or installs the curated symlinks (Symlink mode), advances
// last-sync-commit when appropriate, and installs the shared
// pre-commit hook.
package sync

// Summary is what Sync returns. HadSkips is true if any sub-merge
// skipped a conflict — the orchestrator MUST NOT advance
// last-sync-commit when this is true.
type Summary struct {
	HadSkips bool
}
