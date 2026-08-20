package sync

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Nivl/config/internal/claude/sync/files"
	"github.com/Nivl/config/internal/claude/sync/prompt"
	"github.com/Nivl/config/internal/claude/sync/settings"
	"github.com/Nivl/config/internal/claude/sync/state"
	"github.com/Nivl/config/internal/dryrun"
	"github.com/Nivl/config/internal/symlinkfs"
)

// dirExists reports whether path is a directory. Used by the dry-run
// branches to detect mkdir no-ops without bringing in a helper package.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// fileExists reports whether path exists as a regular file. Used by
// the dry-run branches to detect already-seeded files.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// Mode selects between the copy-merge and symlink sync paths.
type Mode int

const (
	// ModeCopy is the 3-way merge path (PERSONAL_COMPUTER != "true").
	// Runs settings, top-files, the per-entry symlinks for linkedDirs,
	// and the mergedDirs tree merges in that order, then advances
	// last-sync-commit on a clean run.
	ModeCopy Mode = iota
	// ModeSymlink is the symlink-each-curated-item path
	// (PERSONAL_COMPUTER == "true"). Installs absolute-target symlinks
	// for the six curated items into ~/.claude/.
	ModeSymlink
)

// Options is the input to Sync.
type Options struct {
	Out      io.Writer
	Prompter prompt.Prompter
	// NewGit is injectable for tests. Defaults to state.NewGit when nil.
	NewGit func(state.Paths) state.Git
	// Reporter is invoked at each chokepoint decision. Required;
	// callers that don't care about the reporting should pass
	// dryrun.NewNullReporter().
	Reporter dryrun.Reporter
	Mode     Mode
	// DryRun, when true, propagates through all sub-chokepoints
	// (settings.Merge, files.MergeFile/MergeDir, LinkDirEntries,
	// state.Advance, prompt.Remember, InstallPrecommitHook) to
	// suppress writes.
	DryRun bool
}

// Sync runs the configured mode's full pipeline:
//   - Both modes: activation gates → MkdirAll(HomeDir) →
//     InstallPrecommitHook (unconditional before the mode branch).
//   - ModeSymlink: InstallSymlinks. No EnsureStateDir, no advance.
//   - ModeCopy: EnsureStateDir → settings.Merge → MergeFile×2 →
//     LinkDirEntries×1 → MergeDir×2 → AdvanceLastSyncCommit (gated on
//     HadSkips).
//
// Module activation gates:
//   - If `git` is not on PATH, print warning and return empty Summary.
//   - If paths.RepoDir does not exist as a directory, print warning
//     and return empty Summary.
//   - Both gates result in a clean return; the orchestrator continues
//     with subsequent setup steps.
func Sync(ctx context.Context, paths state.Paths, opts Options) (Summary, error) {
	// opts.Out must be non-nil; the cmd layer wires it from
	// cobra. We don't silently fall back to os.Stderr — callers that
	// forget would rather see a nil-writer panic than a stream sneaking
	// through the package boundary.
	out := opts.Out
	reporter := opts.Reporter
	// Gate 1: git binary present.
	if _, err := exec.LookPath("git"); err != nil {
		fmt.Fprintln(out, "claude_setup: git not found, skipping") //nolint:errcheck // progress output to io.Writer
		return Summary{}, nil                                      //nolint:nilerr // missing git is an activation gate, not a propagated error
	}

	// Gate 2: .claude/ exists in the repo.
	info, err := os.Stat(paths.RepoDir)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(out, "claude_setup: %s does not exist, skipping\n", paths.RepoDir) //nolint:errcheck // progress output to io.Writer
		return Summary{}, nil                                                          //nolint:nilerr // missing RepoDir is an activation gate, not a propagated error
	}

	// mkdir -p ~/.claude before any mode-specific work. Under dryRun
	// we report a no-op when the dir already exists (matches the live
	// path's behaviour — os.MkdirAll is a no-op on existing dirs) and
	// a would-change otherwise, so the preview doesn't inflate the
	// "files would change" count with pre-existing dirs.
	if opts.DryRun {
		if info, err := os.Stat(paths.HomeDir); err == nil && info.IsDir() {
			reporter.FileNoOp(paths.HomeDir, "dir already exists")
		} else {
			reporter.FileChange(paths.HomeDir, nil, nil, "would mkdir 0o755")
		}
	} else {
		if err := os.MkdirAll(paths.HomeDir, 0o755); err != nil { //nolint:gosec // 0755 conventional mkdir -p default
			return Summary{}, fmt.Errorf("ensure home dir: %w", err)
		}
	}

	// Install the precommit hook unconditionally before the mode
	// branch — the hook is present in both PERSONAL_COMPUTER=true and
	// =false deployments.
	if err := InstallPrecommitHook(ctx, paths.ConfigDir, out, opts.DryRun, reporter); err != nil {
		return Summary{}, fmt.Errorf("install precommit hook: %w", err)
	}

	// Symlink mode: install absolute-target symlinks then return.
	// No state dir, no advance, no decisions.json access.
	if opts.Mode == ModeSymlink {
		if err := InstallSymlinks(ctx, paths, out, symlinkfs.InstallOpts{
			DryRun:   opts.DryRun,
			Reporter: reporter,
		}); err != nil {
			return Summary{}, fmt.Errorf("install symlinks: %w", err)
		}
		return Summary{}, nil
	}

	// Copy mode: ensure state dir, run the merge pipeline, then
	// advance. EnsureStateDir seeds .sync-state/ with README.md and
	// decisions.json — both are real writes that must be suppressed
	// under dry-run. Mirror EnsureStateDir's per-file idempotency in
	// the preview: if the dir and both seed files already exist, the
	// live path is a no-op, so report accordingly instead of inflating
	// the "would change" tally.
	if opts.DryRun {
		readme := filepath.Join(paths.StateDir, "README.md")
		if dirExists(paths.StateDir) && fileExists(readme) && fileExists(paths.DecisionsFile) {
			reporter.FileNoOp(paths.StateDir, ".sync-state already seeded")
		} else {
			reporter.FileChange(paths.StateDir, nil, nil, "would seed .sync-state dir (README + decisions.json)")
		}
	} else {
		if err := state.EnsureStateDir(paths); err != nil {
			return Summary{}, fmt.Errorf("ensure state dir: %w", err)
		}
	}

	newGit := opts.NewGit
	if newGit == nil {
		newGit = func(p state.Paths) state.Git {
			return state.NewGit(p, p.ConfigDir)
		}
	}
	git := newGit(paths)

	// 1. settings.json merge.
	settingsResult, err := settings.Merge(ctx, paths, git, settings.Options{
		Prompter: opts.Prompter,
		Out:      out,
		DryRun:   opts.DryRun,
		Reporter: reporter,
	})
	if err != nil {
		return Summary{}, fmt.Errorf("merge settings: %w", err)
	}
	hadSkips := settingsResult.HadSkips

	// 2. Top-level files, in topFiles order (stderr progress
	// determinism).
	fileOpts := files.Options{
		Prompter: opts.Prompter,
		DryRun:   opts.DryRun,
		Reporter: reporter,
	}
	for _, top := range topFiles {
		r, err := files.MergeFile(ctx, paths, git, top, fileOpts)
		if err != nil {
			return Summary{}, fmt.Errorf("merge file %s: %w", top, err)
		}
		hadSkips = hadSkips || r.HadSkip
	}

	// 3. Per-entry symlinked directories, in linkedDirs order. Nothing
	// here can conflict, so hadSkips is untouched.
	for _, dirName := range linkedDirs {
		if err := LinkDirEntries(paths, dirName, out, symlinkfs.InstallOpts{
			DryRun:   opts.DryRun,
			Reporter: reporter,
		}); err != nil {
			return Summary{}, fmt.Errorf("link dir %s: %w", dirName, err)
		}
	}

	// 4. Merged directories, in mergedDirs order.
	for _, dirName := range mergedDirs {
		r, err := files.MergeDir(ctx, paths, git, dirName, fileOpts)
		if err != nil {
			return Summary{}, fmt.Errorf("merge dir %s: %w", dirName, err)
		}
		hadSkips = hadSkips || r.HadSkips
	}

	// 5. Advance last-sync-commit. If any step skipped, leave the
	// pointer alone and emit a stderr warning. Otherwise overwrite
	// with the HEAD SHA.
	if hadSkips {
		fmt.Fprint(out, "claude_setup: leaving last-sync-commit unchanged due to skipped conflicts\n") //nolint:errcheck // progress output to io.Writer
	} else {
		if err := AdvanceLastSyncCommit(ctx, paths, git, opts.DryRun, reporter); err != nil {
			return Summary{}, fmt.Errorf("advance last-sync commit: %w", err)
		}
	}

	return Summary{HadSkips: hadSkips}, nil
}

// settingsFile, topFiles, linkedDirs, and mergedDirs are the single
// source of truth for the curated set synced from
// shared_config/.claude/. Order matters for stderr progress
// determinism. Copy mode handles each category differently.
// settings.json gets a JSON 3-way merge, topFiles a per-file merge,
// linkedDirs per-entry symlinks, mergedDirs a tree merge. Symlink
// mode instead links the flat set from curatedItems(). Both modes
// draw from these same names, so the two paths can't drift.
const settingsFile = "settings.json"

var (
	topFiles = []string{"CLAUDE.md", "AGENTS.md"} //nolint:gochecknoglobals // ordered constant
	// linkedDirs hold self-contained units the repo owns wholesale, so
	// copy mode links each entry instead of merging its files. A skill
	// is edited in the repo and read from ~/.claude; a 3-way merge of
	// its individual files only ever recreated repo content locally.
	linkedDirs = []string{"skills"}             //nolint:gochecknoglobals // ordered constant
	mergedDirs = []string{"agents", "commands"} //nolint:gochecknoglobals // ordered constant
)

// curatedItems returns every synced member in canonical order:
// settings.json, then the top-level files, then the curated
// directories. Symlink mode links these directly; copy mode walks the
// same names by category. Deriving it from the shared constants is
// what keeps the two sync modes in lockstep.
func curatedItems() []string {
	items := make([]string, 0, 1+len(topFiles)+len(linkedDirs)+len(mergedDirs))
	items = append(items, settingsFile)
	items = append(items, topFiles...)
	items = append(items, linkedDirs...)
	items = append(items, mergedDirs...)
	return items
}
