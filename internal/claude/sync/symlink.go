package sync

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Nivl/config/internal/claude/sync/state"
	"github.com/Nivl/config/internal/symlinkfs"
)

// InstallSymlinks installs absolute-target symlinks from
// $HOME/.claude/<item> to <RepoDir>/<item> for each item in
// curatedItems() (the set shared with copy mode).
//
// Behavior:
//   - Source missing -> silent skip.
//   - Otherwise delegated to symlinkfs.Install: an already-correct
//     symlink is a no-op, anything else (regular file, directory,
//     wrong/broken symlink) gets renamed to
//     <target>.<YYYYMMDDHHmmSS>.bkp before relinking.
//
// Does NOT advance last-sync-commit, does NOT touch decisions.json --
// the caller doesn't call EnsureStateDir or AdvanceLastSyncCommit in
// symlink mode. out receives a "Linked <target>" progress line for
// each item when not in dry-run mode; dry-run output is handled by
// the symlinkfs.Install reporter.
func InstallSymlinks(_ context.Context, paths state.Paths, out io.Writer, installOpts symlinkfs.InstallOpts) error {
	now := time.Now()
	for _, item := range curatedItems() {
		source := filepath.Join(paths.RepoDir, item)
		target := filepath.Join(paths.HomeDir, item)

		// Source missing -> silent skip.
		if _, err := os.Lstat(source); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return fmt.Errorf("lstat source %s: %w", item, err)
		}

		if err := symlinkfs.Install(source, target, now, installOpts); err != nil {
			return fmt.Errorf("install symlink %s: %w", item, err)
		}
		if !installOpts.DryRun {
			_, _ = fmt.Fprintf(out, "Linked %s -> %s\n", target, source)
		}
	}
	return nil
}
