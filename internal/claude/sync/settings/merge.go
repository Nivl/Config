package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Nivl/config/internal/claude/sync/prompt"
	"github.com/Nivl/config/internal/claude/sync/state"
	"github.com/Nivl/config/internal/dryrun"
	"github.com/Nivl/config/internal/errutil"
)

// Result is what Merge returns to its caller.
type Result struct {
	// Decisions is the full plan that was computed.
	Decisions []Decision
	// HadSkips is true if any conflict was resolved as Skip. The
	// orchestrator MUST NOT advance last-sync-commit when this is
	// true.
	HadSkips bool
	// Wrote indicates whether local on disk was modified.
	Wrote bool
}

// Options controls Merge's side effects.
type Options struct {
	// Prompter resolves conflicts. Required.
	Prompter prompt.Prompter
	// Out is where progress lines (set-merge summaries, warnings) are
	// written. Required (non-nil) — callers wire it from the cmd layer.
	Out io.Writer
	// Reporter is invoked at each write chokepoint. Required; callers
	// that don't care about the reporting should pass
	// dryrun.NewNullReporter().
	Reporter dryrun.Reporter
	// DryRun, when true, suppresses all writes. reporter.FileChange is
	// called unconditionally so dry-run output is complete.
	DryRun bool
}

// Merge is the orchestrator: reads base via git, local from disk,
// remote from the repo working copy; calls Decide; prompts for any
// conflicts via opts.Prompter; backs up local; writes the merged
// result. Idempotent — a run with no divergence touches no files.
func Merge(ctx context.Context, paths state.Paths, git state.Git, opts Options) (Result, error) {
	out := opts.Out
	reporter := opts.Reporter
	remotePath := filepath.Join(paths.RepoDir, "settings.json")
	localPath := filepath.Join(paths.HomeDir, "settings.json")

	// Missing remote → early return clean.
	remoteBytes, err := os.ReadFile(remotePath) //nolint:gosec // sync-managed local path
	if errors.Is(err, os.ErrNotExist) {
		return Result{}, nil
	}
	if err != nil {
		return Result{}, fmt.Errorf("read remote settings.json: %w", err)
	}

	// Malformed remote → warn + clean.
	if !validJSON(remoteBytes) {
		fmt.Fprintf(out, "claude_merge_settings: %s is not valid JSON, skipping\n", remotePath)
		return Result{}, nil
	}

	// Missing local -> fast-path copy.
	localBytes, err := os.ReadFile(localPath) //nolint:gosec // sync-managed local path
	if errors.Is(err, os.ErrNotExist) {
		reporter.FileChange(localPath, nil, remoteBytes, "copy remote settings.json (no local copy)")
		if !opts.DryRun {
			if writeErr := os.WriteFile(localPath, remoteBytes, 0o644); writeErr != nil { //nolint:gosec // 0644 default umask
				return Result{}, fmt.Errorf("copy remote to local: %w", writeErr)
			}
		}
		return Result{Wrote: !opts.DryRun}, nil
	}
	if err != nil {
		return Result{}, fmt.Errorf("read local settings.json: %w", err)
	}

	// Malformed local → warn + clean WITHOUT touching the file or
	// creating a backup.
	if !validJSON(localBytes) {
		fmt.Fprintf(out,
			"claude_merge_settings: %s is not valid JSON, refusing to merge "+
				"(fix or remove the file and re-run)\n", localPath)
		return Result{}, nil
	}

	// Base resolution with fallback.
	baseBytes, baseWarning := readBase(ctx, paths, git)
	if baseWarning != "" {
		fmt.Fprint(out, baseWarning)
	}
	// Distinguish catastrophic context cancellation from soft fallback:
	// if ctx is cancelled, propagate the error.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return Result{}, fmt.Errorf("merge cancelled: %w", ctxErr)
	}

	decisions, err := Decide(baseBytes, localBytes, remoteBytes)
	if err != nil {
		return Result{}, fmt.Errorf("decide: %w", err)
	}

	// Resolve conflicts and build the apply-list.
	var hadSkips bool
	apply := make([]Decision, 0, len(decisions))
	for _, d := range decisions {
		switch d.Action {
		case ActionNoop, ActionKeepLocal:
			// no apply
		case ActionTakeRemote, ActionRemoteDelete:
			apply = append(apply, d)
			if d.Kind == KindSet && d.Action == ActionTakeRemote {
				fmt.Fprintf(out,
					"settings.json: %s merged: +%d -%d (now %d items)\n",
					renderPath(d.Path), d.Adds, d.Removes, d.Total)
			}
		case ActionConflict:
			req := prompt.Request{
				Kind:     prompt.KindSettings,
				Key:      keyForSettings(d.Path),
				Header:   "Conflict in settings.json at " + renderPath(d.Path),
				Renderer: newConflictRenderer(d, baseShortSHA(paths)),
			}
			choice, resolveErr := opts.Prompter.Resolve(ctx, req)
			if resolveErr != nil {
				return Result{Decisions: decisions, HadSkips: hadSkips},
					fmt.Errorf("resolve conflict at %v: %w", d.Path, resolveErr)
			}
			switch choice {
			case prompt.ChoiceKeepLocal:
				// no apply
			case prompt.ChoiceTakeRemote:
				resolved := d
				if d.RemoteValue != nil {
					resolved.Action = ActionTakeRemote
					resolved.Value = *d.RemoteValue
				} else {
					resolved.Action = ActionRemoteDelete
				}
				apply = append(apply, resolved)
			case prompt.ChoiceSkip:
				hadSkips = true
			}
		}
	}

	if len(apply) == 0 {
		return Result{Decisions: decisions, HadSkips: hadSkips}, nil
	}

	newBytes, err := Apply(localBytes, apply)
	if err != nil {
		return Result{}, fmt.Errorf("apply: %w", err)
	}

	reporter.FileChange(localPath, localBytes, newBytes, "merge settings.json")
	if opts.DryRun {
		return Result{Decisions: decisions, HadSkips: hadSkips}, nil
	}

	// Backup before destructive change.
	if err := state.BackupFile(localPath); err != nil {
		return Result{}, fmt.Errorf("backup local: %w", err)
	}

	// Atomic write via tempfile + rename.
	if err := atomicWrite(localPath, newBytes); err != nil {
		return Result{}, fmt.Errorf("atomic write: %w", err)
	}

	return Result{Decisions: decisions, HadSkips: hadSkips, Wrote: true}, nil
}

// readBase returns the base settings.json bytes and an optional
// warning string to emit. Fallback is empty-object on missing /
// unreachable / malformed base.
//
// A genuine first sync (ErrNoBase: no anchor SHA yet) is silent. A
// set-but-unusable anchor (ErrBaseUnreadable: the commit predates the
// file's current path, or is unreachable) warns with a recovery hint —
// an empty base makes diverged keys look like conflicts, which can stop
// last-sync-commit from advancing.
func readBase(ctx context.Context, paths state.Paths, git state.Git) (baseBytes []byte, warning string) {
	out, err := git.ShowBase(ctx, "settings.json")
	switch {
	case errors.Is(err, state.ErrNoBase):
		// Genuine first sync (no anchor SHA) — silent fallback.
		return []byte("{}"), ""
	case errors.Is(err, state.ErrBaseUnreadable):
		// Anchor SHA is set but settings.json couldn't be read at it.
		// Warn with the short SHA and a recovery hint so a stale anchor
		// gets noticed instead of silently wedging the advance.
		return []byte("{}"),
			fmt.Sprintf("claude_merge_settings: settings.json not found at last-sync-commit %s "+
				"(the anchor predates the file's current path, or is unreachable); treating base as "+
				"empty. Diverged keys will look like conflicts and last-sync-commit may not advance — "+
				"reset it to HEAD to recover.\n", baseShortSHA(paths))
	case err != nil:
		// Some other git failure (e.g. git binary problems) → warn + fallback.
		return []byte("{}"),
			"claude_merge_settings: cannot read settings.json at last-sync-commit, treating base as empty\n"
	case len(out) == 0, !validJSON(out):
		return []byte("{}"),
			"claude_merge_settings: cannot read settings.json at last-sync-commit, treating base as empty\n"
	}
	return out, ""
}

// validJSON returns true if data parses as JSON (any value).
func validJSON(data []byte) bool {
	var v any
	return json.Unmarshal(data, &v) == nil
}

// baseShortSHA reads the anchor SHA and returns its first 7 chars,
// or "none" if no anchor.
func baseShortSHA(paths state.Paths) string {
	sha, _ := state.ReadLastSyncSHA(paths)
	if sha == "" {
		return "none"
	}
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// keyForSettings returns the compact-JSON encoding of the path array.
// Uses state.MarshalJq so the key bytes stay byte-stable across
// versions (no HTML escaping).
func keyForSettings(path []string) string {
	b, _ := state.MarshalJq(path) // path is always a marshalable string slice
	return string(b)
}

// atomicWrite writes data to path via tempfile + rename in the same
// directory. Cleanup of the tempfile uses errutil.RunAndSetError so any
// write error is not silently lost.
func atomicWrite(path string, data []byte) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp.*")
	if err != nil {
		return fmt.Errorf("create tempfile: %w", err)
	}
	defer errutil.RunAndSetError(
		func() error { return state.CleanupTempFile(tmp) },
		&err, "cleanup tempfile",
	)
	if _, writeErr := tmp.Write(data); writeErr != nil {
		return fmt.Errorf("write merged: %w", writeErr)
	}
	// os.CreateTemp produces 0o600; we want 0o644 (default umask) to
	// match the explicit 0o644 of the fast-path write earlier in
	// this file.
	if chmodErr := tmp.Chmod(0o644); chmodErr != nil {
		return fmt.Errorf("chmod tempfile: %w", chmodErr)
	}
	if closeErr := tmp.Close(); closeErr != nil {
		return fmt.Errorf("close tempfile: %w", closeErr)
	}
	if renameErr := os.Rename(tmp.Name(), path); renameErr != nil {
		return fmt.Errorf("rename merged: %w", renameErr)
	}
	return nil
}

// conflictRenderer is the prompt.Renderer for settings conflicts. It
// includes the short base SHA in Summary.
//
//nolint:govet // fieldalignment: logical grouping (decision first, then sha) takes priority
type conflictRenderer struct {
	d         Decision
	baseShort string
}

// newConflictRenderer returns a conflictRenderer for the given Decision
// and short base SHA.
func newConflictRenderer(d Decision, baseShort string) *conflictRenderer {
	return &conflictRenderer{d: d, baseShort: baseShort}
}

// Summary implements prompt.Renderer. Emits a three-line diff header
// with the short base SHA.
func (r *conflictRenderer) Summary(w io.Writer) {
	fmt.Fprintf(w, "  base   (sha %s): %s\n", r.baseShort, valueRepr(r.d.BaseValue))
	fmt.Fprintf(w, "  local                : %s\n", valueRepr(r.d.LocalValue))
	fmt.Fprintf(w, "  remote               : %s\n", valueRepr(r.d.RemoteValue))
}

// Diff implements prompt.Renderer. Emits a compact one-line diff
// showing each side's transformation relative to base. Each side
// renders as one of:
//
//	(unchanged)     — side matches base
//	X -> Y          — modify (both base and side present, differ)
//	X -> (deleted)  — side has no value, base had X
//	(added) Y       — base had no value, side has Y
func (r *conflictRenderer) Diff(w io.Writer) {
	fmt.Fprintf(w, "  (diff) local: %s  remote: %s\n",
		sideRepr(r.d.BaseValue, r.d.LocalValue),
		sideRepr(r.d.BaseValue, r.d.RemoteValue))
}

// sideRepr renders the transformation from base to side for Diff.
// Equality is computed on the compact JSON encoding so that JSON
// values that marshal to the same bytes compare equal.
func sideRepr(base, side *any) string {
	switch {
	case base == nil && side == nil:
		return "(unchanged)"
	case base == nil:
		return "(added) " + valueRepr(side)
	case side == nil:
		return valueRepr(base) + " -> (deleted)"
	case valueRepr(base) == valueRepr(side):
		return "(unchanged)"
	default:
		return valueRepr(base) + " -> " + valueRepr(side)
	}
}

// valueRepr renders a *any as compact JSON or "<absent>" if nil.
// Uses state.MarshalJq (no HTML escaping) so e.g.
// `Bash(diff a b > /tmp/c)` displays raw in conflict prompts.
func valueRepr(v *any) string {
	if v == nil {
		return "<absent>"
	}
	b, err := state.MarshalJq(*v)
	if err != nil {
		return fmt.Sprintf("<unmarshalable: %v>", err)
	}
	return string(b)
}
