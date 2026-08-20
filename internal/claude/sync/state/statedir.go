package state

import (
	"fmt"
	"os"
	"path/filepath"
)

// readmeContent is the first-run documentation written to
// StateDir/README.md.
const readmeContent = `This directory is gitignored and managed by melvin-config.

- last-sync-commit : repo SHA at last successful Claude config sync. Used as
  the base for 3-way merges of settings.json and the agents/commands
  directories. skills/ is symlinked per entry, not merged, so it never
  consults this SHA.
- decisions.json   : remembered "always keep local" / "always take remote"
  choices per conflict path. Delete this file to clear all remembered choices.

Wiping this directory forces a re-prompt on every divergent key/file the
next time melvin-config setup runs.
`

// initialDecisions is the first-run content for decisions.json.
const initialDecisions = `{"version":1,"settings":{},"files":{}}` + "\n"

// EnsureStateDir creates StateDir if missing, seeds README.md on first
// run, and initializes decisions.json on first run. Idempotent: an
// existing decisions.json or README.md is left alone.
func EnsureStateDir(p Paths) error {
	if err := os.MkdirAll(p.StateDir, 0o755); err != nil { //nolint:gosec // 0755 conventional mkdir -p default
		return fmt.Errorf("create state dir: %w", err)
	}
	readmePath := filepath.Join(p.StateDir, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		if err := os.WriteFile(readmePath, []byte(readmeContent), 0o644); err != nil { //nolint:gosec // 0644 seeded file permissions
			return fmt.Errorf("seed README.md: %w", err)
		}
	}
	if _, err := os.Stat(p.DecisionsFile); os.IsNotExist(err) {
		if err := os.WriteFile(p.DecisionsFile, []byte(initialDecisions), 0o644); err != nil { //nolint:gosec // 0644 seeded file permissions
			return fmt.Errorf("seed decisions.json: %w", err)
		}
	}
	return nil
}
