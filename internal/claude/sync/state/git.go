package state

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// ErrNoBase signals "no anchor SHA available" — a genuine first sync.
// Caller treats base as missing/empty. Returned by ShowBase and BaseHas
// when LastSyncFile is empty or absent.
var ErrNoBase = errors.New("no anchor SHA")

// ErrBaseUnreadable signals "an anchor SHA is set, but the base file
// could not be read at it" — git show exited non-zero because the path
// did not exist at that commit (e.g. the anchor predates a path move) or
// the commit itself is unreachable. Like ErrNoBase the caller treats the
// base as empty, but unlike ErrNoBase this is worth warning about: a
// stale anchor makes diverged keys look like conflicts and can stop
// last-sync-commit from ever advancing.
var ErrBaseUnreadable = errors.New("base unreadable at anchor SHA")

// Git wraps the four git subprocess calls the sync engine needs:
// "show me <repo>/.claude/<rel> at SHA X", "did .claude/<rel> exist at
// SHA X", "list the files under .claude/<dir> at SHA X", and "what is
// the current HEAD SHA".
type Git interface {
	// ShowBase returns the bytes of .claude/<rel> at the SHA in
	// LastSyncFile. Returns (nil, ErrNoBase) when there is no anchor SHA
	// (genuine first sync: empty/missing LastSyncFile). Returns
	// (nil, ErrBaseUnreadable) when an anchor SHA is set but git show
	// exits non-zero (the path did not exist at that commit, or the SHA
	// is unreachable). Returns the bytes and nil on success. Other errors
	// propagate (e.g. git binary missing).
	ShowBase(ctx context.Context, rel string) ([]byte, error)
	// BaseHas reports whether .claude/<rel> existed at the anchor SHA.
	// Returns (false, nil) when there's no anchor SHA — no error for
	// that case.
	BaseHas(ctx context.Context, rel string) (bool, error)
	// ListTree returns relative paths under .claude/<dir> at the
	// anchor SHA, with ".claude/<dir>/" stripped from each entry.
	// Returns (nil, nil) on empty/missing anchor. Stderr from git is
	// silenced via io.Discard. Other errors propagate.
	ListTree(ctx context.Context, dir string) ([]string, error)
	// HeadSHA returns the trimmed 40-char HEAD SHA via
	// `git -C <configDir> rev-parse HEAD`. Used by
	// AdvanceLastSyncCommit to overwrite last-sync-commit. ctx.Err()
	// propagates as a returned error; git stderr is silenced via
	// io.Discard.
	HeadSHA(ctx context.Context) (string, error)
}

// NewGit returns the production Git that shells to
// `git -C <configDir> show <sha>:.claude/<rel>` and
// `git -C <configDir> cat-file -e <sha>:.claude/<rel>`. The paths arg
// supplies LastSyncFile; configDir is the dotfiles repo root.
func NewGit(p Paths, configDir string) Git {
	return &realGit{paths: p, configDir: configDir}
}

// realGit is the production Git. Unexported so consumers always go
// through the Git interface — keeps call sites trivially mockable.
type realGit struct {
	paths     Paths
	configDir string
}

// ShowBase implements Git.
func (g *realGit) ShowBase(ctx context.Context, rel string) ([]byte, error) {
	sha, err := ReadLastSyncSHA(g.paths)
	if err != nil {
		return nil, fmt.Errorf("read last-sync sha: %w", err)
	}
	if sha == "" {
		return nil, ErrNoBase
	}
	target := sha + ":" + RepoSubdir + "/" + rel
	c := exec.CommandContext(ctx, "git", "-C", g.configDir, "show", target) //nolint:gosec // configDir/rel are sync-managed local paths
	out, err := c.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// git show exited non-zero: the file didn't exist at that SHA
			// (e.g. the anchor predates a path move) or the SHA itself is
			// unreachable. Both mean "no usable base" but, unlike a true
			// first sync, an anchor IS set — return ErrBaseUnreadable so the
			// caller can warn instead of silently treating base as empty.
			return nil, ErrBaseUnreadable
		}
		return nil, fmt.Errorf("git show %s: %w", target, err)
	}
	return out, nil
}

// BaseHas implements Git.
func (g *realGit) BaseHas(ctx context.Context, rel string) (bool, error) {
	sha, err := ReadLastSyncSHA(g.paths)
	if err != nil {
		return false, fmt.Errorf("read last-sync sha: %w", err)
	}
	if sha == "" {
		return false, nil
	}
	target := sha + ":" + RepoSubdir + "/" + rel
	c := exec.CommandContext(ctx, "git", "-C", g.configDir, "cat-file", "-e", target) //nolint:gosec // configDir/rel are sync-managed local paths
	if err := c.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil
		}
		return false, fmt.Errorf("git cat-file -e %s: %w", target, err)
	}
	return true, nil
}

// ListTree implements Git.
func (g *realGit) ListTree(ctx context.Context, dir string) ([]string, error) {
	sha, err := ReadLastSyncSHA(g.paths)
	if err != nil {
		return nil, fmt.Errorf("read last-sync sha: %w", err)
	}
	if sha == "" {
		return nil, nil
	}
	pathInRepo := RepoSubdir + "/" + dir
	prefix := pathInRepo + "/"
	c := exec.CommandContext(ctx, "git", "-C", g.configDir, "ls-tree", "-r", "--name-only", sha, "--", pathInRepo) //nolint:gosec // configDir/dir are sync-managed local paths
	c.Stderr = io.Discard
	out, err := c.Output()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, fmt.Errorf("git ls-tree %s: %w", pathInRepo, ctxErr)
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// Subtree not in this SHA (legitimately empty or never existed).
			return nil, nil
		}
		return nil, fmt.Errorf("git ls-tree %s: %w", pathInRepo, err)
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	var result []string
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Strip the .claude/<dir>/ prefix. Lines without the prefix
		// (shouldn't happen given the path constraint above) are
		// kept verbatim so a surprise survives to test diagnostics.
		result = append(result, strings.TrimPrefix(line, prefix))
	}
	return result, nil
}

// HeadSHA implements Git.
func (g *realGit) HeadSHA(ctx context.Context) (string, error) {
	c := exec.CommandContext(ctx, "git", "-C", g.configDir, "rev-parse", "HEAD") //nolint:gosec // configDir is a sync-managed local path
	c.Stderr = io.Discard
	out, err := c.Output()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", ctxErr)
	}
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}
