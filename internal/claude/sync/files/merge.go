package files

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Nivl/config/internal/claude/sync/prompt"
	"github.com/Nivl/config/internal/claude/sync/state"
	"github.com/Nivl/config/internal/dryrun"
)

// FileResult is what MergeFile returns to its caller. It captures the
// resolved Decision (post-prompt for the conflict path), whether the
// user chose Skip on a conflict, and whether the on-disk local file
// was mutated.
type FileResult struct {
	// Decision is the truth-table outcome BEFORE the conflict resolver
	// fires. For ActionConflict, the caller can inspect ConflictType to
	// learn what kind of conflict was prompted.
	Decision Decision
	// HadSkip is true if the user (or env override / cache) resolved
	// a conflict by choosing Skip. Drives the orchestrator's HadSkips
	// aggregation.
	HadSkip bool
	// Wrote is true if MergeFile mutated $HOME/.claude/<rel>: either
	// wrote bytes (copy-r-to-l) or removed the file (rm-l).
	Wrote bool
}

// Options controls MergeFile/MergeDir side effects.
type Options struct {
	// Prompter resolves conflicts. Required when any conflict can fire.
	Prompter prompt.Prompter
	// Reporter is invoked at each write chokepoint. Required; callers
	// that don't care about the reporting should pass
	// dryrun.NewNullReporter().
	Reporter dryrun.Reporter
	// DryRun, when true, suppresses all writes. reporter.FileChange is
	// called unconditionally so dry-run output is complete.
	DryRun bool
}

// MergeFile resolves the merge state of a single file and applies the
// chosen action. `rel` is the relative path within .claude/ (e.g.
// "CLAUDE.md" or "skills/foo.md").
//
// Backup-and-mutate sequence:
//   - ActionNoop:               no I/O.
//   - ActionCopyRemoteToLocal:  if L existed, state.BackupFile(L);
//     os.MkdirAll(filepath.Dir(L)); WriteFile(L, R).
//   - ActionRemoveLocal:        state.BackupFile(L); os.Remove(L).
//   - ActionConflict:           invoke opts.Prompter.Resolve via
//     fileConflictRenderer; dispatch on
//     Choice. keep-local: no backup, no
//     mutation. take-remote: same as
//     ActionCopyRemoteToLocal when has_R==1,
//     same as ActionRemoveLocal when has_R==0
//     (delete-modify resolved toward delete).
//     skip: no I/O, HadSkip=true.
func MergeFile(ctx context.Context, paths state.Paths, git state.Git,
	rel string, opts Options) (FileResult, error) {
	localPath := filepath.Join(paths.HomeDir, rel)
	remotePath := filepath.Join(paths.RepoDir, rel)

	localBytes, hasLocal, err := readOptional(localPath)
	if err != nil {
		return FileResult{}, fmt.Errorf("read local %s: %w", rel, err)
	}
	remoteBytes, hasRemote, err := readOptional(remotePath)
	if err != nil {
		return FileResult{}, fmt.Errorf("read remote %s: %w", rel, err)
	}
	baseBytes, hasBase, err := readBase(ctx, git, rel)
	if err != nil {
		return FileResult{}, fmt.Errorf("read base %s: %w", rel, err)
	}

	p := Presence{HasLocal: hasLocal, HasRemote: hasRemote, HasBase: hasBase}
	e := Equality{
		LocalEqBase:   hasLocal && hasBase && bytes.Equal(localBytes, baseBytes),
		RemoteEqBase:  hasRemote && hasBase && bytes.Equal(remoteBytes, baseBytes),
		LocalEqRemote: hasLocal && hasRemote && bytes.Equal(localBytes, remoteBytes),
	}
	d := Decide(p, e)

	reporter := opts.Reporter
	switch d.Action {
	case ActionNoop:
		return FileResult{Decision: d}, nil
	case ActionCopyRemoteToLocal:
		wrote, err := copyRemoteToLocal(localPath, remotePath, remoteBytes, localBytes, hasLocal, opts.DryRun, reporter)
		if err != nil {
			return FileResult{Decision: d}, fmt.Errorf("copy remote to local: %w", err)
		}
		return FileResult{Decision: d, Wrote: wrote}, nil
	case ActionRemoveLocal:
		wrote, err := removeLocal(localPath, localBytes, opts.DryRun, reporter)
		if err != nil {
			return FileResult{Decision: d}, fmt.Errorf("remove local: %w", err)
		}
		return FileResult{Decision: d, Wrote: wrote}, nil
	case ActionConflict:
		return resolveConflict(ctx, paths, rel, opts, d, p, hasRemote, localPath, remotePath, remoteBytes, localBytes, baseBytes, reporter)
	default:
		return FileResult{Decision: d}, fmt.Errorf("unknown action %d", d.Action)
	}
}

// readOptional reads path; on os.ErrNotExist returns (nil, false, nil).
// Other errors propagate.
func readOptional(path string) (data []byte, exists bool, err error) {
	data, err = os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read file %s: %w", path, err)
	}
	return data, true, nil
}

// readBase materializes the base bytes via git.ShowBase. A missing base
// — no anchor SHA (ErrNoBase) or an anchor whose commit lacks the file
// (ErrBaseUnreadable) — collapses to (nil, false, nil), so the caller
// treats has_B=0. The settings merge surfaces the stale-anchor warning;
// the file merges only need the base-absent signal.
func readBase(ctx context.Context, git state.Git, rel string) (data []byte, exists bool, err error) {
	data, err = git.ShowBase(ctx, rel)
	if errors.Is(err, state.ErrNoBase) || errors.Is(err, state.ErrBaseUnreadable) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read base %s: %w", rel, err)
	}
	return data, true, nil
}

// copyRemoteToLocal implements the copy-remote-to-local action:
// backup local if present, mkdir-p parent, write remote bytes with
// the source mode (preserves mode like cp -p).
//
// Returns (true, nil) when the write succeeds. Returns (false, nil)
// when dryRun is true (write suppressed). reporter.FileChange is
// called unconditionally so dry-run output is complete.
func copyRemoteToLocal(localPath, remotePath string, remoteBytes, localBytes []byte,
	hasLocal, dryRun bool, reporter dryrun.Reporter) (bool, error) {
	reporter.FileChange(localPath, localBytes, remoteBytes, "copy remote to local")
	if dryRun {
		return false, nil
	}
	if hasLocal {
		if err := state.BackupFile(localPath); err != nil {
			return false, fmt.Errorf("backup before copy: %w", err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return false, fmt.Errorf("mkdir parent: %w", err)
	}
	info, err := os.Stat(remotePath)
	if err != nil {
		return false, fmt.Errorf("stat remote: %w", err)
	}
	mode := info.Mode().Perm()
	if err := os.WriteFile(localPath, remoteBytes, mode); err != nil {
		return false, fmt.Errorf("write local: %w", err)
	}
	// os.WriteFile's mode arg only applies when creating a new file;
	// when localPath already existed (the backup-then-overwrite path),
	// the existing mode bits would survive. Chmod unconditionally so
	// the source mode propagates on both fresh-create and overwrite.
	if err := os.Chmod(localPath, mode); err != nil {
		return false, fmt.Errorf("chmod local: %w", err)
	}
	// Forward the source's mtime to local (we use it for both atime
	// and mtime since we only have one timestamp to work with).
	if err := os.Chtimes(localPath, info.ModTime(), info.ModTime()); err != nil {
		return false, fmt.Errorf("chtimes local: %w", err)
	}
	return true, nil
}

// removeLocal implements the rm-l action: backup local then remove.
//
// Returns (true, nil) when the remove succeeds. Returns (false, nil)
// when dryRun is true (remove suppressed). reporter.FileChange is
// called unconditionally with nil after-bytes to signal deletion.
func removeLocal(localPath string, localBytes []byte, dryRun bool, reporter dryrun.Reporter) (bool, error) {
	reporter.FileChange(localPath, localBytes, nil, "remove local (deleted in remote)")
	if dryRun {
		return false, nil
	}
	if err := state.BackupFile(localPath); err != nil {
		return false, fmt.Errorf("backup before remove: %w", err)
	}
	if err := os.Remove(localPath); err != nil {
		return false, fmt.Errorf("remove local: %w", err)
	}
	return true, nil
}

// resolveConflict invokes the 3-tier resolver and dispatches on the
// returned Choice.
func resolveConflict(ctx context.Context, paths state.Paths,
	rel string, opts Options, d Decision, p Presence,
	hasRemote bool, localPath, remotePath string, remoteBytes, localBytes, baseBytes []byte,
	reporter dryrun.Reporter) (FileResult, error) {
	header := fmt.Sprintf("Conflict in %s (%s)", rel, d.ConflictType)
	renderer := newFileConflictRenderer(ctx, rel, baseBytes, p, d.ConflictType, paths)
	choice, err := opts.Prompter.Resolve(ctx, prompt.Request{
		Kind:     prompt.KindFiles,
		Key:      rel,
		Header:   header,
		Renderer: renderer,
	})
	if err != nil {
		return FileResult{Decision: d}, fmt.Errorf("resolve conflict for %s: %w", rel, err)
	}
	switch choice {
	case prompt.ChoiceKeepLocal:
		// keep-local is a no-op AND no backup is created.
		return FileResult{Decision: d}, nil
	case prompt.ChoiceTakeRemote:
		if hasRemote {
			wrote, err := copyRemoteToLocal(localPath, remotePath, remoteBytes, localBytes, p.HasLocal, opts.DryRun, reporter)
			if err != nil {
				return FileResult{Decision: d}, fmt.Errorf("copy remote to local: %w", err)
			}
			return FileResult{Decision: d, Wrote: wrote}, nil
		}
		// delete-modify resolved toward delete.
		wrote, err := removeLocal(localPath, localBytes, opts.DryRun, reporter)
		if err != nil {
			return FileResult{Decision: d}, fmt.Errorf("remove local: %w", err)
		}
		return FileResult{Decision: d, Wrote: wrote}, nil
	case prompt.ChoiceSkip:
		return FileResult{Decision: d, HadSkip: true}, nil
	default:
		return FileResult{Decision: d}, fmt.Errorf("unknown choice %d", choice)
	}
}

// fileConflictRenderer is the prompt.Renderer for file-level
// conflicts. Summary emits one line per side (present/absent + byte
// count) plus the conflict type. Diff shells to /usr/bin/diff -u,
// 4-space indented.
//
//nolint:govet // fieldalignment: logical grouping takes priority
type fileConflictRenderer struct {
	ctx          context.Context //nolint:containedctx // renderer is constructed per-Resolve call and called only by the Prompter on the same call's ctx
	rel          string
	baseBytes    []byte
	presence     Presence
	conflictType ConflictType
	paths        state.Paths
}

// newFileConflictRenderer constructs a renderer for one MergeFile
// conflict call. baseBytes are the already-fetched base file contents
// (nil when HasBase is false).
func newFileConflictRenderer(ctx context.Context, rel string, baseBytes []byte,
	p Presence, ct ConflictType, paths state.Paths) *fileConflictRenderer {
	return &fileConflictRenderer{
		ctx: ctx, rel: rel, baseBytes: baseBytes,
		presence: p, conflictType: ct, paths: paths,
	}
}

// Summary implements prompt.Renderer. Emits four header lines.
func (r *fileConflictRenderer) Summary(w io.Writer) {
	sha := baseShortSHA(r.paths)
	if r.presence.HasBase {
		fmt.Fprintf(w, "  base   (sha %s): present (%d bytes)\n", sha, len(r.baseBytes))
	} else {
		fmt.Fprintf(w, "  base   (sha %s): absent\n", sha)
	}
	if r.presence.HasLocal {
		sz := fileByteCount(filepath.Join(r.paths.HomeDir, r.rel))
		fmt.Fprintf(w, "  local                : present (%d bytes)\n", sz)
	} else {
		fmt.Fprintln(w, "  local                : absent")
	}
	if r.presence.HasRemote {
		sz := fileByteCount(filepath.Join(r.paths.RepoDir, r.rel))
		fmt.Fprintf(w, "  remote               : present (%d bytes)\n", sz)
	} else {
		fmt.Fprintln(w, "  remote               : absent")
	}
	fmt.Fprintf(w, "  conflict type        : %s\n", r.conflictType)
}

// Diff implements prompt.Renderer. Emits the unified diff between
// local and remote via /usr/bin/diff -u, 4-space indented. Absent
// sides are represented by /dev/null.
func (r *fileConflictRenderer) Diff(w io.Writer) {
	lhs := "/dev/null"
	rhs := "/dev/null"
	if r.presence.HasLocal {
		lhs = filepath.Join(r.paths.HomeDir, r.rel)
	}
	if r.presence.HasRemote {
		rhs = filepath.Join(r.paths.RepoDir, r.rel)
	}
	c := exec.CommandContext(r.ctx, "/usr/bin/diff", "-u", lhs, rhs)
	raw, _ := c.Output()
	if ctxErr := r.ctx.Err(); ctxErr != nil {
		fmt.Fprintf(w, "    (diff cancelled: %v)\n", ctxErr)
		return
	}
	// `diff -u` exits 1 when files differ — that's the success path
	// here; we don't care about the exit code. Stderr is discarded
	// by design.
	indented := indentLines(raw, "    ")
	if len(indented) == 0 {
		fmt.Fprintln(w, "    (no diff produced)")
		return
	}
	if _, err := w.Write(indented); err != nil {
		return
	}
	if !bytes.HasSuffix(indented, []byte{'\n'}) {
		fmt.Fprintln(w)
	}
}

// indentLines prepends prefix to every line in raw, including the last
// even if raw doesn't end in a newline.
func indentLines(raw []byte, prefix string) []byte {
	if len(raw) == 0 {
		return nil
	}
	var buf bytes.Buffer
	for _, line := range bytes.SplitAfter(raw, []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		buf.WriteString(prefix)
		buf.Write(line)
	}
	return buf.Bytes()
}

// fileByteCount returns the byte size of path, or 0 on error.
func fileByteCount(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// baseShortSHA reads LastSyncFile and returns the first 7 chars, or
// "none" if absent/empty. Mirrors the same helper in
// internal/claude/sync/settings/merge.go but copied here so files/
// stays independent of settings/.
func baseShortSHA(p state.Paths) string {
	sha, err := state.ReadLastSyncSHA(p)
	if err != nil || sha == "" {
		return "none"
	}
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
