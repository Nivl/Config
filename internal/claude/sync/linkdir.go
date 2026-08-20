package sync

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Nivl/config/internal/claude/sync/state"
	"github.com/Nivl/config/internal/symlinkfs"
)

// LinkDirEntries makes every top-level entry of <RepoDir>/<dirName> a
// symlink at <HomeDir>/<dirName>/<entry>, then prunes home links whose
// repo target is gone. dirName is relative to .claude/ (e.g. "skills").
//
// Per-entry rather than whole-dir because ~/.claude/<dirName> also
// holds entries this repo doesn't own. Those are locally authored
// ones and hand-made links into other repos. They are left exactly as
// they are, matched by name. Only names the repo carries get replaced.
//
// A colliding entry is deleted rather than backed up. Claude Code
// reads every subdirectory of skills/ as a skill, so a
// "<name>.<timestamp>.bkp" sibling would come back as a duplicate.
// The bytes being dropped are the repo's own content at an older
// commit, recoverable from git.
//
// out receives a "Linked <target>" line per entry that actually
// changed; dry-run output is the reporter's job.
//
// Every entry installs with Replace set, whatever installOpts carried
// in. Deleting the collision is the point of linking per entry, so it
// is not the caller's choice to make.
func LinkDirEntries(paths state.Paths, dirName string, out io.Writer,
	installOpts symlinkfs.InstallOpts) error {
	localDir := filepath.Join(paths.HomeDir, dirName)
	remoteDir := filepath.Join(paths.RepoDir, dirName)

	// MkdirAll is a no-op on an existing dir; report it as one so the
	// dry-run preview doesn't count an already-present dir as a change.
	if installOpts.DryRun {
		if dirExists(localDir) {
			installOpts.Reporter.FileNoOp(localDir, "dir already exists")
		} else {
			installOpts.Reporter.FileChange(localDir, nil, nil, "would mkdir 0o755")
		}
	} else {
		if err := os.MkdirAll(localDir, 0o755); err != nil { //nolint:gosec // 0755 conventional mkdir -p default
			return fmt.Errorf("mkdir local %s: %w", localDir, err)
		}
	}

	entries, err := os.ReadDir(remoteDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read remote %s: %w", remoteDir, err)
	}

	now := time.Now()
	for _, entry := range entries {
		// Dot entries are bookkeeping (.gitkeep) or OS litter
		// (.DS_Store), never a member of the curated set.
		if entry.Name()[0] == '.' {
			continue
		}
		source := filepath.Join(remoteDir, entry.Name())
		target := filepath.Join(localDir, entry.Name())

		linked, err := isLinkTo(target, source)
		if err != nil {
			return fmt.Errorf("inspect target %s: %w", target, err)
		}
		opts := installOpts
		opts.Replace = true // see the doc comment. Not the caller's choice.
		if err := symlinkfs.Install(source, target, now, opts); err != nil {
			return fmt.Errorf("install symlink %s: %w", entry.Name(), err)
		}
		if !installOpts.DryRun && !linked {
			_, _ = fmt.Fprintf(out, "Linked %s -> %s\n", target, source)
		}
	}

	if err := pruneDanglingLinks(localDir, remoteDir, installOpts); err != nil {
		return fmt.Errorf("prune %s: %w", localDir, err)
	}
	return nil
}

// isLinkTo reports whether path is already a symlink pointing at
// source. A missing path, or anything that isn't a symlink, is false.
func isLinkTo(path, source string) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("lstat %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return false, nil
	}
	link, err := os.Readlink(path)
	if err != nil {
		return false, fmt.Errorf("readlink %s: %w", path, err)
	}
	return link == source, nil
}

// pruneDanglingLinks removes top-level symlinks in localDir that point
// inside remoteDir but whose target no longer resolves. That is an
// entry deleted from the repo since the last sync. Pruning is scoped
// to links this function could have created. A broken link the user
// made to somewhere else is theirs to keep.
func pruneDanglingLinks(localDir, remoteDir string, installOpts symlinkfs.InstallOpts) error {
	entries, err := os.ReadDir(localDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read local %s: %w", localDir, err)
	}
	prefix := remoteDir + string(filepath.Separator)
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink == 0 {
			continue
		}
		target := filepath.Join(localDir, entry.Name())
		link, err := os.Readlink(target)
		if err != nil {
			return fmt.Errorf("readlink %s: %w", target, err)
		}
		if !strings.HasPrefix(link, prefix) {
			continue
		}
		if _, err := os.Stat(link); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("stat link target %s: %w", link, err)
		}
		if installOpts.DryRun {
			installOpts.Reporter.FileChange(target, nil, nil, "would remove dangling symlink")
			continue
		}
		if err := os.Remove(target); err != nil {
			return fmt.Errorf("remove dangling symlink %s: %w", target, err)
		}
	}
	return nil
}
