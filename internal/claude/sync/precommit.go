package sync

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Nivl/config/internal/dryrun"
)

// hookRelativeTarget is the symlink content for
// .git/hooks/pre-commit. The target is RELATIVE so the symlink keeps
// working if the repo is moved to a different filesystem location.
const hookRelativeTarget = "../../.githooks/pre-commit"

// InstallPrecommitHook installs .git/hooks/pre-commit as a symlink
// to ../../.githooks/pre-commit.
//
// Behavior:
//   - If configDir is not a git repo, print a warning to out and
//     return nil.
//   - Normalize the hooks dir via `git rev-parse --git-path hooks`
//     (relative for normal repos, absolute for linked worktrees /
//     bare).
//   - If .githooks/pre-commit is a regular file but not executable,
//     chmod +x defensively (not suppressed by dryRun — it's not a
//     write to a tracked file).
//   - Target priority: already-correct-symlink → silent return;
//     foreign symlink → warn-leave-alone; regular file →
//     warn-leave-alone; otherwise `ln -s ../../.githooks/pre-commit`.
//   - Link target is RELATIVE.
//   - Does NOT touch core.hooksPath.
//   - When dryRun is true, reporter.Symlink is called but os.Symlink
//     is suppressed (warn-leave-alone paths remain unchanged).
func InstallPrecommitHook(ctx context.Context, configDir string, out io.Writer,
	dryRun bool, reporter dryrun.Reporter,
) error {
	if !isGitRepo(ctx, configDir) {
		_, _ = fmt.Fprintf(out, "install_claude_precommit_hook: %s is not a git repo, skipping\n", configDir)
		return nil
	}

	hooksDir, err := resolveHooksDir(ctx, configDir)
	if err != nil {
		return fmt.Errorf("resolve hooks dir: %w", err)
	}
	if dryRun {
		if info, err := os.Stat(hooksDir); err == nil && info.IsDir() {
			reporter.FileNoOp(hooksDir, "dir already exists")
		} else {
			reporter.FileChange(hooksDir, nil, nil, "would mkdir 0o755")
		}
	} else {
		if err := os.MkdirAll(hooksDir, 0o755); err != nil { //nolint:gosec // 0755 conventional mkdir -p default
			return fmt.Errorf("mkdir hooks dir: %w", err)
		}
	}

	// Defensive chmod +x of the versioned hook source.
	hookSource := filepath.Join(configDir, ".githooks", "pre-commit")
	if err := ensureExecutable(hookSource); err != nil {
		return fmt.Errorf("ensure executable: %w", err)
	}

	target := filepath.Join(hooksDir, "pre-commit")
	return installHookSymlink(target, out, dryRun, reporter)
}

// isGitRepo reports whether configDir is a git repo by running
// `git rev-parse --git-dir`. Errors and non-zero exit both count as
// "not a git repo".
func isGitRepo(ctx context.Context, configDir string) bool {
	c := exec.CommandContext(ctx, "git", "-C", configDir, "rev-parse", "--git-dir") //nolint:gosec // configDir is a sync-managed local path
	c.Stdout = io.Discard
	c.Stderr = io.Discard
	return c.Run() == nil
}

// resolveHooksDir runs `git rev-parse --git-path hooks` and
// normalizes the output: absolute paths are returned as-is; relative
// paths are prepended with configDir.
func resolveHooksDir(ctx context.Context, configDir string) (string, error) {
	c := exec.CommandContext(ctx, "git", "-C", configDir, "rev-parse", "--git-path", "hooks") //nolint:gosec // configDir is a sync-managed local path
	c.Stderr = io.Discard
	out, err := c.Output()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", fmt.Errorf("git rev-parse --git-path hooks: %w", ctxErr)
	}
	if err != nil {
		return "", fmt.Errorf("git rev-parse --git-path hooks: %w", err)
	}
	raw := strings.TrimRight(string(out), "\n")
	if filepath.IsAbs(raw) {
		return raw, nil
	}
	return filepath.Join(configDir, raw), nil
}

// ensureExecutable chmods .githooks/pre-commit to add the exec bit
// when it exists as a regular file without it. Missing source or
// non-regular source is silently tolerated — the symlink will still
// install, and a missing source is the user's problem at commit time.
func ensureExecutable(hookSource string) error {
	info, err := os.Stat(hookSource)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat hook source: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	if info.Mode().Perm()&0o111 != 0 {
		return nil // already executable
	}
	if err := os.Chmod(hookSource, info.Mode().Perm()|0o111); err != nil {
		return fmt.Errorf("chmod +x hook source: %w", err)
	}
	return nil
}

// installHookSymlink implements the 4-priority dispatch.
//
//	priority 1: existing correct symlink → silent return.
//	priority 2: existing foreign symlink → warn-leave-alone.
//	priority 3: existing regular file (or anything else) → warn-leave-alone.
//	priority 4: target absent → ln -s (suppressed when dryRun is true).
//
// reporter.Symlink is called unconditionally on priority 4 so dry-run
// output is complete regardless of the dryRun flag.
func installHookSymlink(target string, out io.Writer, dryRun bool, reporter dryrun.Reporter) error {
	// Use Lstat so we can distinguish a symlink from its dereference target.
	info, err := os.Lstat(target)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			// Symlink. Compare readlink to the expected relative target.
			existing, readErr := os.Readlink(target)
			if readErr == nil && existing == hookRelativeTarget {
				reporter.Symlink(target, hookRelativeTarget, "no-op")
				return nil // priority 1: already correct
			}
			// priority 2: foreign symlink
			_, _ = fmt.Fprintf(out, "install_claude_precommit_hook: existing pre-commit symlink points elsewhere (%s), leaving alone\n", existing)
			return nil
		}
		// priority 3: regular file or other non-symlink
		_, _ = fmt.Fprintf(out, "install_claude_precommit_hook: existing pre-commit hook found at %s, leaving alone\n", target)
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("lstat hook target: %w", err)
	}
	// priority 4: target absent → ln -s
	reporter.Symlink(target, hookRelativeTarget, "would-create")
	if dryRun {
		return nil
	}
	if err := os.Symlink(hookRelativeTarget, target); err != nil {
		return fmt.Errorf("symlink hook: %w", err)
	}
	return nil
}
