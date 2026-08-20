// Package symlinkfs has the "install a symlink, handle collisions
// safely" primitive shared by the subsystems that drop symlinks into
// $HOME (dotfiles, claude sync). The logic: if the target already
// exists, check whether it's already the right symlink; if so, do
// nothing. Otherwise, rename whatever is there to a timestamped
// backup and create the symlink, or delete it outright when the caller
// opts in with InstallOpts.Replace. Either way it never prompts.
// Historical "skip existing" behaviors are subsumed by the idempotency
// check.
package symlinkfs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Nivl/config/internal/dryrun"
)

// BackupTimestampFormat is the Go reference layout for the
// "<YYYYMMDDHHmmSS>" suffix used by Install. Exported so tests and
// related backup helpers can reuse it.
const BackupTimestampFormat = "20060102150405"

// InstallOpts carries the dry-run and reporter knobs forwarded from
// the cmd layer. Reporter is required (see the field doc); callers
// that don't care about the reporting should construct
// InstallOpts{Reporter: dryrun.NewNullReporter()}.
type InstallOpts struct {
	// Reporter is called on every chokepoint decision (one of
	// "no-op" / "would-create" / "would-back-up-then-create" /
	// "would-replace"). Required; callers that don't care about the
	// reporting should pass dryrun.NewNullReporter().
	Reporter dryrun.Reporter
	// DryRun, when true, suppresses the os.Symlink / os.Rename
	// calls. The Reporter is notified of the would-be decision
	// instead.
	DryRun bool
	// Replace makes a colliding target get deleted rather than
	// renamed to a "<target>.<timestamp>.bkp" sibling. Set it when the
	// target sits inside a directory something else scans, where a
	// leftover backup would be picked up as a real entry.
	Replace bool
}

// Install creates a symlink at target pointing at source. Collision
// behavior:
//   - target doesn't exist: os.Symlink(source, target).
//   - target is a symlink whose readlink == source: no-op (idempotent
//     re-run; no backup written).
//   - target is anything else (regular file, dir, symlink-to-elsewhere,
//     broken symlink): os.Rename target to
//     "<target>.<now.Format(BackupTimestampFormat)>.bkp", then
//     os.Symlink(source, target). With opts.Replace the collision is
//     deleted instead of renamed, so no backup sibling survives a
//     successful call. See replace for what a failed one leaves.
//
// now is the timestamp used when a backup is written; pass
// time.Now() in production. Tests can pass a fixed time to assert
// the exact backup filename.
//
// When opts.DryRun is true, the os.Symlink / os.Rename calls are
// suppressed. The Reporter is notified of the decision
// unconditionally in both modes.
func Install(source, target string, now time.Time, opts InstallOpts) error {
	reporter := opts.Reporter
	info, err := os.Lstat(target)
	if errors.Is(err, os.ErrNotExist) {
		reporter.Symlink(target, source, "would-create")
		if opts.DryRun {
			return nil
		}
		if err := os.Symlink(source, target); err != nil {
			return fmt.Errorf("symlink %s -> %s: %w", target, source, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("lstat %s: %w", target, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		existing, err := os.Readlink(target)
		if err != nil {
			return fmt.Errorf("readlink %s: %w", target, err)
		}
		if existing == source {
			reporter.Symlink(target, source, "no-op")
			return nil
		}
	}
	if opts.Replace {
		reporter.Symlink(target, source, "would-replace")
	} else {
		reporter.Symlink(target, source, "would-back-up-then-create")
	}
	if opts.DryRun {
		return nil
	}
	if opts.Replace {
		return replace(source, target, now)
	}
	backup := target + "." + now.Format(BackupTimestampFormat) + ".bkp"
	if err := os.Rename(target, backup); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", target, backup, err)
	}
	if err := os.Symlink(source, target); err != nil {
		return fmt.Errorf("symlink %s -> %s: %w", target, source, err)
	}
	return nil
}

// replace swaps target for a symlink to source and deletes whatever
// was there. The old content moves aside first so target is never
// empty without the symlink taking its place. A failed symlink is
// rolled back by moving the content home again.
//
// A failed delete returns an error, which aborts whatever the caller
// was doing. The symlink is already correct by then, so the staging
// entry is the only residue. Nothing sweeps it later, because the next
// Install for that target matches the idempotency check and returns
// before reaching here. The error names the staging path so it can be
// removed by hand.
//
// The staging path is a sibling because os.Rename cannot cross a
// filesystem boundary. Its name is dot-prefixed so a leftover is
// never picked up as a real entry by anything scanning the directory,
// which is the whole reason a caller asks for Replace. The timestamp
// keeps it clear of a backup from the same run. Two replaces of one
// target within the same second would still collide, so this is not a
// concurrency guarantee.
func replace(source, target string, now time.Time) error {
	dir, base := filepath.Split(target)
	staged := filepath.Join(dir, "."+base+"."+now.Format(BackupTimestampFormat)+".replacing")
	if err := os.Rename(target, staged); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", target, staged, err)
	}
	// Untested on purpose. Reaching either branch means making
	// os.Symlink fail against a path the rename just vacated, which
	// needs an injection seam in this package for a path only a
	// concurrent mutation of the parent directory can hit.
	if err := os.Symlink(source, target); err != nil {
		if rollbackErr := os.Rename(staged, target); rollbackErr != nil {
			return fmt.Errorf("symlink %s -> %s: %w (content left at %s: %w)",
				target, source, err, staged, rollbackErr)
		}
		return fmt.Errorf("symlink %s -> %s: %w", target, source, err)
	}
	if err := os.RemoveAll(staged); err != nil {
		return fmt.Errorf("remove %s: %w", staged, err)
	}
	return nil
}
