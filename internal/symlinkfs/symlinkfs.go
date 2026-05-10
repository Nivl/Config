// Package symlinkfs has the "install a symlink, handle collisions
// safely" primitive shared by the subsystems that drop symlinks into
// $HOME (dotfiles, claude sync). The logic: if the target already
// exists, check whether it's already the right symlink; if so, do
// nothing. Otherwise, rename whatever is there to a timestamped
// backup and create the symlink. This is unconditional and never
// prompts — historical "skip existing" behaviors are subsumed by the
// idempotency check.
package symlinkfs

import (
	"errors"
	"fmt"
	"os"
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
	// "no-op" / "would-create" / "would-back-up-then-create").
	// Required; callers that don't care about the reporting should
	// pass dryrun.NewNullReporter().
	Reporter dryrun.Reporter
	// DryRun, when true, suppresses the os.Symlink / os.Rename
	// calls. The Reporter is notified of the would-be decision
	// instead.
	DryRun bool
}

// Install creates a symlink at target pointing at source. Collision
// behavior:
//   - target doesn't exist: os.Symlink(source, target).
//   - target is a symlink whose readlink == source: no-op (idempotent
//     re-run; no backup written).
//   - target is anything else (regular file, dir, symlink-to-elsewhere,
//     broken symlink): os.Rename target to
//     "<target>.<now.Format(BackupTimestampFormat)>.bkp", then
//     os.Symlink(source, target).
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
	reporter.Symlink(target, source, "would-back-up-then-create")
	if opts.DryRun {
		return nil
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
