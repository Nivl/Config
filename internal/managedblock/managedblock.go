// Package managedblock installs and refreshes a sentinel-fenced
// region in a text config file. It exists so that repo-relative
// paths written into materialized files (~/.zshrc, ~/.gitconfig,
// ~/.gnupg/gpg-agent.conf) can evolve across `melvin-config setup`
// runs without losing the user customizations stored outside the
// managed region.
package managedblock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Nivl/config/internal/dryrun"
)

// BeginMarker and EndMarker delimit the managed region. Both files
// touched by current callers (.zshrc, .gitconfig, gpg-agent.conf)
// accept `#` as a line-comment introducer.
const (
	BeginMarker = "# >>> melvin-config managed >>>"
	EndMarker   = "# <<< melvin-config managed <<<"
)

// noticeLine is prepended to the payload inside every wrapped block,
// telling humans not to edit it by hand.
const noticeLine = "# Do not edit this block by hand — it is rewritten by `melvin-config setup`."

// UpsertOpts carries the dry-run and reporter knobs forwarded from
// the cmd layer.
type UpsertOpts struct {
	// Reporter is called on every chokepoint decision (FileChange for
	// rewrites, FileNoOp for content-equal short-circuits). Required;
	// callers that don't care about the reporting should pass
	// dryrun.NewNullReporter().
	Reporter dryrun.Reporter
	// DryRun, when true, suppresses the writeAtomic call. The Reporter
	// is notified of the would-be change instead.
	DryRun bool
}

// Upsert installs payload into path, wrapped in BeginMarker /
// EndMarker plus the notice line. The full algorithm:
//
//   - Missing file: write a fresh file containing the wrapped block
//     with perm 0o644.
//   - Markers present (exactly one BeginMarker and one EndMarker, in
//     order): replace the inter-marker region with the new payload.
//     Adjacent blank lines are normalized to at most one on each
//     side of the wrapped block.
//   - Markers absent and priorLine matches: replace the first match
//     in place with the wrapped block (migration of pre-marker
//     files).
//   - Markers absent and priorLine nil or no match: append the
//     wrapped block at EOF (leading "\n" if the file doesn't already
//     end in one).
//   - Malformed markers (only one of the pair, wrong order,
//     duplicates): return a wrapped error; the file is not touched.
//   - Proposed content byte-identical to existing: return nil
//     without writing (mtime preserved).
//
// Writes are atomic: a tempfile in the same directory is created,
// written, fsynced, and os.Rename'd over the destination.
// Permissions copy from the existing file when present, or 0o644
// for fresh writes.
//
// priorLine may be nil to disable migration (just append the wrapped
// block at EOF if markers absent). When non-nil and markers are
// absent, the first regex match in the file is replaced with the
// wrapped block; subsequent occurrences are left as-is.
//
// When opts.DryRun is true, the writeAtomic call is suppressed and
// Reporter is notified instead (FileChange for rewrites, FileNoOp for
// content-equal short-circuits). Malformed-marker errors propagate
// regardless of DryRun.
func Upsert(path, payload string, priorLine *regexp.Regexp, opts UpsertOpts) error {
	reporter := opts.Reporter
	existing, err := os.ReadFile(path) //nolint:gosec // caller-supplied config-file path, read-by-design
	if errors.Is(err, os.ErrNotExist) {
		next := wrap(payload)
		reporter.FileChange(path, nil, []byte(next), "fresh write")
		if opts.DryRun {
			return nil
		}
		return writeAtomic(path, next, 0o644)
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	next, err := computeNext(string(existing), payload, priorLine, path)
	if err != nil {
		return err
	}
	if next == string(existing) {
		reporter.FileNoOp(path, "managed block in sync")
		return nil
	}

	reporter.FileChange(path, existing, []byte(next), "managed block update")
	if opts.DryRun {
		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	return writeAtomic(path, next, info.Mode().Perm())
}

// ComputeNext returns the content that would be written if Upsert
// were called with the given inputs, without performing any I/O.
// Used by configgen's dry-run composition path to compute proposed
// content in memory.
//
// Error semantics match Upsert: malformed marker states return a
// wrapped error.
func ComputeNext(existing, payload string, priorLine *regexp.Regexp, path string) (string, error) {
	return computeNext(existing, payload, priorLine, path)
}

// computeNext returns the file content that would be written after an
// Upsert call. existing is the current file content as a string;
// payload is the new managed-block body (without markers/notice);
// priorLine optionally enables migration; path is included in errors
// for context.
func computeNext(existing, payload string, priorLine *regexp.Regexp, path string) (string, error) {
	begin := strings.Count(existing, BeginMarker)
	end := strings.Count(existing, EndMarker)

	switch {
	case begin == 0 && end == 0:
		return migrateOrAppend(existing, payload, priorLine), nil
	case begin == 1 && end == 1:
		beginIdx := strings.Index(existing, BeginMarker)
		endIdx := strings.Index(existing, EndMarker)
		if beginIdx > endIdx {
			return "", fmt.Errorf("%s: malformed managed block: EndMarker appears before BeginMarker; resolve manually", path)
		}
		return splice(existing, payload, beginIdx, endIdx), nil
	case begin == 1 && end == 0:
		return "", fmt.Errorf("%s: malformed managed block: found BeginMarker without EndMarker; resolve manually", path)
	case begin == 0 && end == 1:
		return "", fmt.Errorf("%s: malformed managed block: found EndMarker without BeginMarker; resolve manually", path)
	default:
		return "", fmt.Errorf("%s: malformed managed block: BeginMarker appears %d time(s), EndMarker appears %d time(s); resolve manually", path, begin, end)
	}
}

// migrateOrAppend handles the "no markers present" case. If priorLine
// is non-nil and matches, the first match's full line(s) are
// replaced with the wrapped block (using the same blank-line
// normalization as splice). Otherwise the wrapped block is appended
// at EOF (with a leading "\n" if the existing content does not
// already end in one).
func migrateOrAppend(existing, payload string, priorLine *regexp.Regexp) string {
	if priorLine != nil {
		if loc := priorLine.FindStringIndex(existing); loc != nil {
			beginLineStart := lineStart(existing, loc[0])
			endLineEnd := lineEnd(existing, loc[1]-1)
			before := normalizeBefore(existing[:beginLineStart])
			after := normalizeAfter(existing[endLineEnd:])
			return before + wrap(payload) + after
		}
	}
	if existing == "" {
		return wrap(payload)
	}
	leadingNL := ""
	if !strings.HasSuffix(existing, "\n") {
		leadingNL = "\n"
	}
	return existing + leadingNL + wrap(payload)
}

// splice replaces the existing wrapped block (the lines containing
// BeginMarker / EndMarker) with the new wrap(payload). Surrounding
// blank lines are normalized so that at most one blank line remains
// between the wrapped block and surrounding content on each side.
func splice(existing, payload string, beginIdx, endIdx int) string {
	beginLineStart := lineStart(existing, beginIdx)
	endLineEnd := lineEnd(existing, endIdx)
	before := normalizeBefore(existing[:beginLineStart])
	after := normalizeAfter(existing[endLineEnd:])
	return before + wrap(payload) + after
}

// lineStart returns the byte offset of the first character on the
// line that contains idx.
func lineStart(s string, idx int) int {
	if idx <= 0 {
		return 0
	}
	if nl := strings.LastIndexByte(s[:idx], '\n'); nl != -1 {
		return nl + 1
	}
	return 0
}

// lineEnd returns the byte offset just past the newline of the line
// that contains idx, or len(s) if there is no trailing newline.
func lineEnd(s string, idx int) int {
	if idx >= len(s) {
		return len(s)
	}
	if nl := strings.IndexByte(s[idx:], '\n'); nl != -1 {
		return idx + nl + 1
	}
	return len(s)
}

// normalizeBefore trims trailing newlines of the "before" half to at
// most 2 (i.e. one optional blank line between the prior content
// and the BeginMarker). The wrapped block starts with BeginMarker+"\n"
// and carries no leading newline of its own, so two trailing newlines
// on the "before" half produce exactly one visible blank line between
// the prior content and the BeginMarker.
func normalizeBefore(s string) string {
	if s == "" {
		return ""
	}
	trimmed := strings.TrimRight(s, "\n")
	nls := min(len(s)-len(trimmed), 2)
	return trimmed + strings.Repeat("\n", nls)
}

// normalizeAfter trims leading newlines of the "after" half to at
// most 1. The wrapped block ends with EndMarker+"\n" (one trailing
// newline of its own), so one leading newline on the "after" half
// produces exactly one visible blank line between the EndMarker and
// the following content.
func normalizeAfter(s string) string {
	if s == "" {
		return ""
	}
	trimmed := strings.TrimLeft(s, "\n")
	nls := min(len(s)-len(trimmed), 1)
	return strings.Repeat("\n", nls) + trimmed
}

// wrap returns the complete managed-block region (markers + notice +
// payload + trailing newline) for a given payload.
func wrap(payload string) string {
	return BeginMarker + "\n" +
		noticeLine + "\n" +
		payload + "\n" +
		EndMarker + "\n"
}

// writeAtomic writes data to path via a sibling tempfile + os.Rename.
// perm applies to the destination when it doesn't already exist; the
// tempfile carries the same perm.
//
// On any failure between create and rename the tempfile is removed so
// the user doesn't end up with orphan .tmp.XXXX files next to their
// dotfiles. Once the rename succeeds the source path is gone, so the
// cleanup is a no-op.
func writeAtomic(path, data string, perm os.FileMode) (err error) {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp.*")
	if err != nil {
		return fmt.Errorf("create tempfile in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	closed := false
	defer func() {
		if !closed {
			_ = tmp.Close()
		}
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.WriteString(data); err != nil {
		return fmt.Errorf("write tempfile %s: %w", tmpPath, err)
	}
	if err := tmp.Chmod(perm); err != nil {
		return fmt.Errorf("chmod tempfile %s: %w", tmpPath, err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync tempfile %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close tempfile %s: %w", tmpPath, err)
	}
	closed = true
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmpPath, path, err)
	}
	return nil
}
