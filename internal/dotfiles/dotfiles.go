// Package dotfiles installs the curated dotfiles from the config repo
// into the user's home directory via symlinks. Collisions are handled
// by the shared symlinkfs.Install helper: an already-correct symlink
// is a no-op, anything else gets renamed to <name>.<timestamp>.bkp
// and replaced.
package dotfiles

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Nivl/config/internal/dryrun"
	"github.com/Nivl/config/internal/symlinkfs"
)

// sharedConfigSubdir is the in-repo subdirectory that holds the
// canonical copies of files we symlink into $HOME. The Go code reads
// from <configDir>/shared_config/<item>; the matching home-side
// symlink keeps its leaf name (e.g. ~/.oh-my-zsh).
const sharedConfigSubdir = "shared_config"

// dotfileItems is the ordered list of items that get symlinked from
// <configDir>/shared_config/<item> to <homeDir>/<item>.
var dotfileItems = []string{ //nolint:gochecknoglobals // ordered constant
	".emacs.d",
	".golangci.yml",
}

// CopyConfigFiles symlinks the curated dotfile items from
// <configDir>/shared_config/ into homeDir, then ensures
// <homeDir>/.emacs-saves exists. Each item is delegated to
// symlinkfs.Install: re-runs are idempotent when the existing target
// is already the right symlink, and any other collision (regular
// file, directory, wrong-symlink, broken-symlink) is moved to a
// timestamped .bkp before relinking. Never prompts the user.
//
// Under dryRun, every os.Symlink / os.Rename / os.MkdirAll is
// suppressed; the Reporter is notified of the would-be decisions
// instead. The "Linked …" progress lines are also suppressed in
// dry-run mode — the Reporter output supersedes them.
//
// The unused first parameter exists so the signature can grow a
// context if a future change needs cancellation; out receives the
// "Linked <target> -> <source>" progress lines in real mode.
func CopyConfigFiles(_ context.Context, configDir, homeDir string,
	out io.Writer, dryRun bool, reporter dryrun.Reporter,
) error {
	installOpts := symlinkfs.InstallOpts{DryRun: dryRun, Reporter: reporter}
	now := time.Now()
	for _, item := range dotfileItems {
		source := filepath.Join(configDir, sharedConfigSubdir, item)
		target := filepath.Join(homeDir, item)
		if err := symlinkfs.Install(source, target, now, installOpts); err != nil {
			return fmt.Errorf("install symlink %s: %w", item, err)
		}
		if !dryRun {
			_, _ = fmt.Fprintf(out, "Linked %s -> %s\n", target, source)
		}
	}

	emacsSaves := filepath.Join(homeDir, ".emacs-saves")
	if dryRun {
		// Report a no-op when the dir already exists (matches live-path
		// MkdirAll, which short-circuits) instead of inflating the
		// "would change" count with already-present dirs.
		if info, err := os.Stat(emacsSaves); err == nil && info.IsDir() {
			reporter.FileNoOp(emacsSaves, "dir already exists")
		} else {
			reporter.FileChange(emacsSaves, nil, nil, "would mkdir 0o755")
		}
	} else {
		if err := os.MkdirAll(emacsSaves, 0o755); err != nil { //nolint:gosec // 0755 is the conventional mkdir -p default
			return fmt.Errorf("mkdir %s: %w", emacsSaves, err)
		}
	}
	return nil
}
