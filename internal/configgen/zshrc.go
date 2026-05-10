package configgen

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Nivl/config/internal/dryrun"
	"github.com/Nivl/config/internal/managedblock"
)

// ZshrcOpts bundles the values written into the materialized ~/.zshrc.
// PersonalComputer is a verbatim string (not bool) so that a pre-set
// PERSONAL_COMPUTER="garbage" is exported as-is — no validation runs
// on values set via env or flag.
type ZshrcOpts struct {
	PersonalComputer string
	DevRoot          string
	GitHost          string
	GitCloneUserName string
}

// zshrcSourceLine is the repo-relative `source` line installed inside
// the managed block. The same string is emitted on every run; only
// the surrounding file content can drift.
const zshrcSourceLine = `source "$HOME/.melvin/config/shared_config/base.zshrc"`

// zshrcPriorLine matches the exact line shape SetupZshrc previously
// wrote outside any managed block. Used by managedblock.Upsert for
// one-shot migration of pre-marker files.
var zshrcPriorLine = regexp.MustCompile( //nolint:gochecknoglobals // immutable regex
	`(?m)^source\s+"\$HOME/\.melvin/config/shared_config/base\.zshrc"\s*$`,
)

// SetupZshrc materializes ~/.zshrc:
//
//   - First-install only (file absent): write seven export lines
//     (GIT_HOST / GIT_CLONE_USER_NAME / PERSONAL_COMPUTER / DEV_ROOT
//     / WORKTREES_ROOT / REPOS_ROOT / SDKS_ROOT) followed by a single
//     newline. These reflect values resolved once at install time;
//     subsequent runs don't re-prompt and don't overwrite the user's
//     edits.
//
//   - Every run: upsert the managed-block region containing the
//     `source` line for shared_config/base.zshrc via
//     managedblock.Upsert.
//
// When dryRun is true, no disk writes occur. Instead, setupZshrcDryRun
// composes the full would-be file content in memory and reports a
// single FileChange (or FileNoOp) via reporter.
//
// configDir is unused but retained in the signature for symmetry
// with the other configgen functions.
func SetupZshrc(homeDir, _ string, opts ZshrcOpts, dryRun bool, reporter dryrun.Reporter) error {
	target := filepath.Join(homeDir, ".zshrc")

	if dryRun {
		return setupZshrcDryRun(target, opts, reporter)
	}

	if err := writeZshrcUserExportsIfAbsent(target, opts); err != nil {
		return err
	}
	if err := managedblock.Upsert(target, zshrcSourceLine, zshrcPriorLine, managedblock.UpsertOpts{Reporter: reporter}); err != nil {
		return fmt.Errorf("upsert managed block in %s: %w", target, err)
	}
	return nil
}

// setupZshrcDryRun computes the proposed end state of ~/.zshrc by
// composing phase-1 user exports (when the file is absent) with
// phase-2's managed-block region. Reports a single FileChange (or
// FileNoOp) against the real on-disk content.
//
// Phase-1 is gated on the file's *existence* (same as
// writeZshrcUserExportsIfAbsent), not on len > 0 — an existing empty
// ~/.zshrc means the user pre-created it intentionally and the live
// path will not write the exports.
func setupZshrcDryRun(target string, opts ZshrcOpts, reporter dryrun.Reporter) error {
	_, statErr := os.Stat(target)
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", target, statErr)
	}
	freshInstall := errors.Is(statErr, os.ErrNotExist)

	realExisting, _ := os.ReadFile(target) //nolint:gosec // caller-supplied home-relative config-file path, read-by-design
	virtualExisting := realExisting
	if freshInstall {
		virtualExisting = []byte(buildZshrcExports(opts))
	}
	proposed, err := managedblock.ComputeNext(
		string(virtualExisting), zshrcSourceLine, zshrcPriorLine, target,
	)
	if err != nil {
		return err
	}
	if proposed == string(realExisting) {
		reporter.FileNoOp(target, "zshrc in sync")
		return nil
	}
	summary := "would update zshrc"
	if freshInstall {
		summary = "would create zshrc (fresh install)"
	}
	reporter.FileChange(target, realExisting, []byte(proposed), summary)
	return nil
}

// buildZshrcExports returns the seven user-machine export lines
// previously written inline by writeZshrcUserExportsIfAbsent. It is
// pure so the dry-run composition path can invoke it without
// touching disk.
func buildZshrcExports(opts ZshrcOpts) string {
	worktreesRoot := opts.DevRoot + "/worktrees"
	reposRoot := opts.DevRoot + "/repos"
	sdksRoot := opts.DevRoot + "/sdks"
	return fmt.Sprintf(
		"\nexport GIT_HOST=\"%s\""+
			"\nexport GIT_CLONE_USER_NAME=\"%s\""+
			"\nexport PERSONAL_COMPUTER=\"%s\""+
			"\nexport DEV_ROOT=\"%s\""+
			"\nexport WORKTREES_ROOT=\"%s\""+
			"\nexport REPOS_ROOT=\"%s\""+
			"\nexport SDKS_ROOT=\"%s\"\n",
		opts.GitHost,
		opts.GitCloneUserName,
		opts.PersonalComputer,
		opts.DevRoot,
		worktreesRoot,
		reposRoot,
		sdksRoot,
	)
}

// writeZshrcUserExportsIfAbsent writes the seven export lines when
// target is absent. It returns nil silently if target already
// exists (or any directory entry exists at target). Anything other
// than os.ErrNotExist propagates.
func writeZshrcUserExportsIfAbsent(target string, opts ZshrcOpts) error {
	if _, err := os.Stat(target); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", target, err)
	}

	content := buildZshrcExports(opts)
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil { //nolint:gosec // 0644 is the conventional default umask + write
		return fmt.Errorf("write %s: %w", target, err)
	}
	return nil
}
