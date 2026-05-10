# Managed-Block Config File Updates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow repo-relative paths inside `~/.zshrc`, `~/.gitconfig`, and `~/.gnupg/gpg-agent.conf` to evolve on subsequent `melvin-config setup` runs (today they're frozen at first-install).

**Architecture:** New `internal/managedblock` package owns a sentinel-fenced read-modify-write primitive (`Upsert`). The three `configgen` functions are refactored into two phases: phase 1 = today's first-install-only writes (now restricted to user-machine values like exports / identity / mkdir+gpg-agent restart), phase 2 = always-run `managedblock.Upsert` for the repo-relative payload that needs to evolve. A per-file regex enables one-shot migration of files that pre-date the marker convention.

**Tech Stack:** Go stdlib (`os`, `regexp`, `bytes`, `path/filepath`), testify (`require`, `assert`, `mock` for existing FakeCmdRunner), `t.TempDir()` for FS-touching tests.

**Spec:** `docs/superpowers/specs/2026-05-14-managed-block-design.md`

**Branch:** `ml/ref/config/go-6` (direct, no per-step branches).

---

## File Structure

| File | Change | Purpose |
|---|---|---|
| `internal/managedblock/managedblock.go` | NEW | `Upsert` + `BeginMarker` / `EndMarker` constants + algorithm. |
| `internal/managedblock/managedblock_test.go` | NEW | 12 unit tests covering fresh / idempotent / splice / migration / malformed / perm-preserve. |
| `internal/configgen/zshrc.go` | MODIFY | Drop full-file first-install guard; gate only user-exports write on file absence; always-run `managedblock.Upsert` for the `source` line. |
| `internal/configgen/zshrc_test.go` | MODIFY | Update `FreshWrite` / `FirstInstallGuard` assertions to expect managed-block markers; add `_RerunUpdatesManagedBlock` and `_MigrationFromPreManagedBlock` tests. |
| `internal/configgen/gitconfig.go` | MODIFY | Drop full-file first-install guard; gate only identity/url/commit write on file absence; always-run `managedblock.Upsert` for the `[include]` block. |
| `internal/configgen/gitconfig_test.go` | MODIFY | Same shape as zshrc test update. |
| `internal/configgen/gpg.go` | MODIFY | Stat-then-branch: phase 1 (file absent) = mkdir, then phase 2 = Upsert, then phase 3 (file was absent) = killall + gpg-agent --daemon. Two-phase becomes three-phase due to the agent-restart ordering. |
| `internal/configgen/gpg_test.go` | MODIFY | Update `FreshWrite` / `FirstInstallGuard` assertions; add `_RerunUpdatesManagedBlockNoRestart` test. |

No new struct types, so no `.golangci.yml` exhaustruct entries needed. No new test sub-packages (the `managedblock` package needs no fake — it's a pure file operation).

---

## Conventions (project-wide)

These apply to all code in this plan. Don't deviate without noting why.

- **testify** for assertions: `require` for fail-stop (e.g. setup), `assert` for continuation (e.g. multiple property checks). Never gomock.
- **`t.TempDir()`** for all FS tests. Never touch `$HOME` or system paths.
- **Doc comments on every declaration**, including unexported ones. Stricter than stdlib convention.
- **Error wrapping (Uber-style):** every `fmt.Errorf("...: %w", err)` — the `...` describes the *callee's* action, not the caller's. E.g. `fmt.Errorf("read %s: %w", path, err)` (not `"upsert: read failed: %w"`).
- **`fmt.Fprintf` / `fmt.Fprintln` to `io.Writer`:** errcheck-ignored via `_, _ = fmt.Fprintf(...)` per project convention. (Not relevant in this plan — no writer output from `Upsert`.)
- **Run lint via `golangci-lint run ./...`** before each commit. Project has gofumpt+goimports enabled.
- **Run tests via `go test ./...`** before each commit.
- **Commits**: `feat:` for new package, `refactor:` for the three configgen rewrites. Each phase = exactly one commit. Use `git commit -m "$(cat <<'EOF' ... EOF)"` heredoc.

---

## Phase 1 — `internal/managedblock` package

Single commit at the end of Phase 1. All TDD tasks in this phase land together.

### Task 1.1: Package scaffold

**Files:**
- Create: `internal/managedblock/managedblock.go`

- [ ] **Step 1: Create the scaffold**

Create `internal/managedblock/managedblock.go`:

```go
// Package managedblock installs and refreshes a sentinel-fenced
// region in a text config file. It exists so that repo-relative
// paths written into materialized files (~/.zshrc, ~/.gitconfig,
// ~/.gnupg/gpg-agent.conf) can evolve across `melvin-config setup`
// runs without losing the user customizations stored outside the
// managed region.
package managedblock

import "regexp"

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

// Upsert installs payload into path, wrapped in BeginMarker /
// EndMarker plus the notice line. See the package design doc at
// docs/superpowers/specs/2026-05-14-managed-block-design.md §7 for
// the full algorithm.
//
// Returns nil without writing if the resulting file content would be
// byte-identical to the current content.
//
// priorLine may be nil to disable migration (just append the wrapped
// block at EOF if markers absent). When non-nil and markers are
// absent, the first regex match in the file is replaced with the
// wrapped block; subsequent occurrences are left as-is.
func Upsert(path, payload string, priorLine *regexp.Regexp) error {
	_ = path
	_ = payload
	_ = priorLine
	return nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/managedblock/...`
Expected: builds cleanly, no output.

---

### Task 1.2: TDD — `TestUpsert_FreshFile`

**Files:**
- Create: `internal/managedblock/managedblock_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/managedblock/managedblock_test.go`:

```go
package managedblock

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpsert_FreshFile — when the target file does not exist, Upsert
// creates it containing just the wrapped block (markers + notice +
// payload + closing marker + trailing newline), perm 0o644.
func TestUpsert_FreshFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "file")

	err := Upsert(path, "payload-line", nil)
	require.NoError(t, err)

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	want := "# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"payload-line\n" +
		"# <<< melvin-config managed <<<\n"
	assert.Equal(t, want, string(got))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}
```

- [ ] **Step 2: Run the test, confirm it fails**

Run: `go test ./internal/managedblock/ -run TestUpsert_FreshFile -v`
Expected: FAIL — file not created (Upsert is a no-op stub).

- [ ] **Step 3: Implement fresh-file path**

Replace `managedblock.go`'s `Upsert` body with:

```go
func Upsert(path, payload string, priorLine *regexp.Regexp) error {
	existing, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return writeAtomic(path, wrap(payload), 0o644)
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	_ = existing
	_ = priorLine
	return nil
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
func writeAtomic(path string, data string, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp.*")
	if err != nil {
		return fmt.Errorf("create tempfile in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.WriteString(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write tempfile %s: %w", tmpPath, err)
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod tempfile %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close tempfile %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmpPath, path, err)
	}
	return nil
}
```

Add imports to the top of `managedblock.go`:

```go
import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)
```

- [ ] **Step 4: Run the test, confirm it passes**

Run: `go test ./internal/managedblock/ -run TestUpsert_FreshFile -v`
Expected: PASS.

---

### Task 1.3: TDD — `TestUpsert_IdempotentNoOp`

- [ ] **Step 1: Add the failing test**

Append to `managedblock_test.go`:

```go
// TestUpsert_IdempotentNoOp — markers present and payload unchanged,
// Upsert returns nil without writing. Verified by mtime preservation.
func TestUpsert_IdempotentNoOp(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "file")
	require.NoError(t, Upsert(path, "stable-payload", nil))

	before, err := os.Stat(path)
	require.NoError(t, err)
	// Sleep a couple of ms so a stray write would have a detectably
	// different mtime. macOS APFS has nanosecond mtime resolution, so
	// even sub-ms is detectable, but a small margin keeps the test
	// robust on slower filesystems.
	time.Sleep(10 * time.Millisecond)

	require.NoError(t, Upsert(path, "stable-payload", nil))

	after, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, before.ModTime(), after.ModTime(),
		"second identical Upsert must not rewrite the file")
}
```

Add `"time"` to the test file's imports.

- [ ] **Step 2: Run the test, confirm it fails**

Run: `go test ./internal/managedblock/ -run TestUpsert_IdempotentNoOp -v`
Expected: FAIL — second Upsert call returns nil immediately (existing != nil branch is a no-op stub at this point), so the file ends up empty after first call OR mtime updates. Either way, the test exposes the missing logic.

- [ ] **Step 3: Implement marker detection + content-equality short-circuit**

Replace `Upsert`'s body in `managedblock.go`:

```go
func Upsert(path, payload string, priorLine *regexp.Regexp) error {
	existing, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return writeAtomic(path, wrap(payload), 0o644)
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	next, err := computeNext(string(existing), payload, priorLine, path)
	if err != nil {
		return err
	}
	if next == string(existing) {
		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	return writeAtomic(path, next, info.Mode().Perm())
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
		return "", fmt.Errorf("%s: malformed managed block: BeginMarker appears %d times, EndMarker appears %d times; resolve manually", path, begin, end)
	}
}

// migrateOrAppend handles the "no markers present" case. The other
// helpers (splice, etc.) get implemented in later tasks; this stub
// keeps the IdempotentNoOp test from compiling-failing.
func migrateOrAppend(existing, payload string, priorLine *regexp.Regexp) string {
	_ = priorLine
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
// and the BeginMarker). The wrapped block starts with the marker
// directly, no leading newline of its own.
func normalizeBefore(s string) string {
	if s == "" {
		return ""
	}
	trimmed := strings.TrimRight(s, "\n")
	nls := len(s) - len(trimmed)
	if nls > 2 {
		nls = 2
	}
	return trimmed + strings.Repeat("\n", nls)
}

// normalizeAfter trims leading newlines of the "after" half to at
// most 1 (one optional blank line: combined with the wrapped
// block's own trailing "\n" after EndMarker that yields one blank
// line between EndMarker and following content).
func normalizeAfter(s string) string {
	if s == "" {
		return ""
	}
	trimmed := strings.TrimLeft(s, "\n")
	nls := len(s) - len(trimmed)
	if nls > 1 {
		nls = 1
	}
	return strings.Repeat("\n", nls) + trimmed
}
```

Add `"strings"` to the `managedblock.go` import block.

- [ ] **Step 4: Run the test, confirm it passes**

Run: `go test ./internal/managedblock/ -run "TestUpsert_FreshFile|TestUpsert_IdempotentNoOp" -v`
Expected: both PASS.

---

### Task 1.4: TDD — `TestUpsert_RewriteBetweenMarkers`

- [ ] **Step 1: Add the failing test**

Append to `managedblock_test.go`:

```go
// TestUpsert_RewriteBetweenMarkers — when markers are present and
// payload differs, Upsert replaces the inter-marker region while
// preserving surrounding content.
func TestUpsert_RewriteBetweenMarkers(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "file")
	initial := "alpha-prefix\n" +
		"# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"old-payload\n" +
		"# <<< melvin-config managed <<<\n" +
		"omega-suffix\n"
	require.NoError(t, os.WriteFile(path, []byte(initial), 0o644))

	require.NoError(t, Upsert(path, "new-payload", nil))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	want := "alpha-prefix\n" +
		"# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"new-payload\n" +
		"# <<< melvin-config managed <<<\n" +
		"omega-suffix\n"
	assert.Equal(t, want, string(got))
}
```

- [ ] **Step 2: Run the test**

Run: `go test ./internal/managedblock/ -run TestUpsert_RewriteBetweenMarkers -v`
Expected: PASS (the splice helper from Task 1.3 already implements this — this test confirms it works end-to-end).

If the test fails with a blank-line-handling issue, fix `splice` to handle the no-blank-line case correctly. The expected output has no blank lines around the wrapped block because the original `initial` had no blank lines around it.

---

### Task 1.5: TDD — `TestUpsert_MigrationRegexMatches`

- [ ] **Step 1: Add the failing test**

Append to `managedblock_test.go`:

```go
// TestUpsert_MigrationRegexMatches — file has no markers, but
// priorLine matches an existing line; Upsert replaces that line in
// place with the wrapped block.
func TestUpsert_MigrationRegexMatches(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "file")
	initial := "header-line\n" +
		"STALE-LINE\n" +
		"footer-line\n"
	require.NoError(t, os.WriteFile(path, []byte(initial), 0o644))

	priorLine := regexp.MustCompile(`(?m)^STALE-LINE$`)
	require.NoError(t, Upsert(path, "new-payload", priorLine))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	want := "header-line\n" +
		"# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"new-payload\n" +
		"# <<< melvin-config managed <<<\n" +
		"footer-line\n"
	assert.Equal(t, want, string(got))
}
```

Add `"regexp"` to the test file's imports.

- [ ] **Step 2: Run the test, confirm it fails**

Run: `go test ./internal/managedblock/ -run TestUpsert_MigrationRegexMatches -v`
Expected: FAIL — `migrateOrAppend` ignores `priorLine` (stub).

- [ ] **Step 3: Implement the regex-replace path**

Replace `migrateOrAppend` in `managedblock.go`:

```go
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
```

- [ ] **Step 4: Run the test, confirm it passes**

Run: `go test ./internal/managedblock/ -run "TestUpsert_FreshFile|TestUpsert_IdempotentNoOp|TestUpsert_RewriteBetweenMarkers|TestUpsert_MigrationRegexMatches" -v`
Expected: all PASS.

---

### Task 1.6: TDD — `TestUpsert_MigrationRegexNoMatch`

- [ ] **Step 1: Add the failing test**

Append to `managedblock_test.go`:

```go
// TestUpsert_MigrationRegexNoMatch — file has no markers, priorLine
// doesn't match anything; the wrapped block is appended at EOF.
func TestUpsert_MigrationRegexNoMatch(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "file")
	initial := "header-line\n" +
		"footer-line\n"
	require.NoError(t, os.WriteFile(path, []byte(initial), 0o644))

	priorLine := regexp.MustCompile(`(?m)^STALE-LINE$`)
	require.NoError(t, Upsert(path, "appended-payload", priorLine))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	want := "header-line\n" +
		"footer-line\n" +
		"# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"appended-payload\n" +
		"# <<< melvin-config managed <<<\n"
	assert.Equal(t, want, string(got))
}
```

- [ ] **Step 2: Run the test**

Run: `go test ./internal/managedblock/ -run TestUpsert_MigrationRegexNoMatch -v`
Expected: PASS (already covered by the append-at-EOF branch in `migrateOrAppend`).

---

### Task 1.7: TDD — `TestUpsert_MigrationFileNoTrailingNewline`

- [ ] **Step 1: Add the failing test**

Append to `managedblock_test.go`:

```go
// TestUpsert_MigrationFileNoTrailingNewline — when the existing file
// doesn't end in '\n' and there are no markers + no regex match,
// append-at-EOF adds a leading '\n' so the wrapped block starts on a
// new line.
func TestUpsert_MigrationFileNoTrailingNewline(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "file")
	require.NoError(t, os.WriteFile(path, []byte("no-trailing-newline"), 0o644))

	require.NoError(t, Upsert(path, "payload", nil))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	want := "no-trailing-newline\n" +
		"# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"payload\n" +
		"# <<< melvin-config managed <<<\n"
	assert.Equal(t, want, string(got))
}
```

- [ ] **Step 2: Run the test**

Run: `go test ./internal/managedblock/ -run TestUpsert_MigrationFileNoTrailingNewline -v`
Expected: PASS (already covered by the leading-`\n` branch).

---

### Task 1.8: TDD — `TestUpsert_MigrationRegexMatchesMultiple`

- [ ] **Step 1: Add the failing test**

Append to `managedblock_test.go`:

```go
// TestUpsert_MigrationRegexMatchesMultiple — when priorLine matches
// multiple times, only the first match is replaced; subsequent
// occurrences are left intact (preventing creation of duplicate
// managed blocks, which would later trip the malformed-marker check).
func TestUpsert_MigrationRegexMatchesMultiple(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "file")
	initial := "header\n" +
		"STALE-LINE\n" +
		"middle\n" +
		"STALE-LINE\n" +
		"footer\n"
	require.NoError(t, os.WriteFile(path, []byte(initial), 0o644))

	priorLine := regexp.MustCompile(`(?m)^STALE-LINE$`)
	require.NoError(t, Upsert(path, "payload", priorLine))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	want := "header\n" +
		"# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"payload\n" +
		"# <<< melvin-config managed <<<\n" +
		"middle\n" +
		"STALE-LINE\n" +
		"footer\n"
	assert.Equal(t, want, string(got))
}
```

- [ ] **Step 2: Run the test**

Run: `go test ./internal/managedblock/ -run TestUpsert_MigrationRegexMatchesMultiple -v`
Expected: PASS (`FindStringIndex` returns the first match by definition).

---

### Task 1.9: TDD — Malformed marker cases (table-driven)

- [ ] **Step 1: Add the failing tests**

Append to `managedblock_test.go`:

```go
// TestUpsert_MalformedMarkers — every malformed marker arrangement
// returns a wrapped error and does NOT write to the file. The error
// message includes the path and a description of what went wrong.
func TestUpsert_MalformedMarkers(t *testing.T) {
	cases := []struct {
		name      string
		content   string
		errSubstr string
	}{
		{
			name:      "only begin marker",
			content:   "head\n# >>> melvin-config managed >>>\nbody\ntail\n",
			errSubstr: "found BeginMarker without EndMarker",
		},
		{
			name:      "only end marker",
			content:   "head\nbody\n# <<< melvin-config managed <<<\ntail\n",
			errSubstr: "found EndMarker without BeginMarker",
		},
		{
			name:      "wrong order",
			content:   "# <<< melvin-config managed <<<\nbody\n# >>> melvin-config managed >>>\n",
			errSubstr: "EndMarker appears before BeginMarker",
		},
		{
			name: "duplicate begin",
			content: "# >>> melvin-config managed >>>\nA\n" +
				"# <<< melvin-config managed <<<\n" +
				"# >>> melvin-config managed >>>\nB\n" +
				"# <<< melvin-config managed <<<\n",
			errSubstr: "BeginMarker appears 2 times",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			path := filepath.Join(tmp, "file")
			require.NoError(t, os.WriteFile(path, []byte(tc.content), 0o644))

			err := Upsert(path, "irrelevant", nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errSubstr)
			assert.Contains(t, err.Error(), path)

			// File must be untouched.
			got, readErr := os.ReadFile(path)
			require.NoError(t, readErr)
			assert.Equal(t, tc.content, string(got))
		})
	}
}
```

- [ ] **Step 2: Run the test**

Run: `go test ./internal/managedblock/ -run TestUpsert_MalformedMarkers -v`
Expected: PASS (the switch in `computeNext` already handles all four cases).

---

### Task 1.10: TDD — `TestUpsert_PreservesExistingPermissions`

- [ ] **Step 1: Add the failing test**

Append to `managedblock_test.go`:

```go
// TestUpsert_PreservesExistingPermissions — when the file already
// exists with non-default perms, the rewrite preserves them.
func TestUpsert_PreservesExistingPermissions(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "file")
	initial := "# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"old-payload\n" +
		"# <<< melvin-config managed <<<\n"
	require.NoError(t, os.WriteFile(path, []byte(initial), 0o600))

	require.NoError(t, Upsert(path, "new-payload", nil))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
```

- [ ] **Step 2: Run the test**

Run: `go test ./internal/managedblock/ -run TestUpsert_PreservesExistingPermissions -v`
Expected: PASS (Upsert passes `info.Mode().Perm()` to `writeAtomic`, which `Chmod`s the tempfile).

---

### Task 1.11: Run full package test sweep + lint + commit

- [ ] **Step 1: Run all tests in the new package**

Run: `go test ./internal/managedblock/... -v`
Expected: 12 tests passing (4 from the table-driven malformed test + 8 standalone).

- [ ] **Step 2: Run lint**

Run: `golangci-lint run ./internal/managedblock/...`
Expected: `No issues found`.

- [ ] **Step 3: Run the full project test + lint sweep to catch any spillover**

Run: `go test ./... && golangci-lint run ./...`
Expected: all green.

- [ ] **Step 4: Commit**

```bash
git add internal/managedblock/
git commit -m "$(cat <<'EOF'
feat(managedblock): introduce sentinel-fenced config-file upsert primitive

New internal/managedblock package owns the read-modify-write
primitive used by configgen to manage repo-relative path lines
inside materialized config files. Upsert handles: fresh write,
idempotent re-run (mtime preserved on no-op), in-place splice
between markers, regex-driven migration of pre-marker files, and
fail-loud error on malformed marker state.

See docs/superpowers/specs/2026-05-14-managed-block-design.md for
the full design.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 2 — Refactor `SetupZshrc`

Single commit at the end of Phase 2.

### Task 2.1: TDD — Re-run updates managed block

The new test pins the behavior we're about to add. Existing tests will need updated assertions later in this phase.

**Files:**
- Modify: `internal/configgen/zshrc_test.go`

- [ ] **Step 1: Add the failing test**

Append to `internal/configgen/zshrc_test.go`:

```go
// TestSetupZshrc_RerunUpdatesManagedBlock — when the file already
// exists with a managed block whose payload has drifted (e.g. the
// source path was changed by a previous build of the code), a fresh
// SetupZshrc rewrites the inter-marker region. Phase 1 (user
// exports) is skipped because the file exists.
func TestSetupZshrc_RerunUpdatesManagedBlock(t *testing.T) {
	tmp := t.TempDir()
	preExisting := "# user header\n" +
		"export GIT_HOST=\"frozen\"\n" +
		"# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"source \"$HOME/.melvin/config/OLD/base.zshrc\"\n" +
		"# <<< melvin-config managed <<<\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmp, ".zshrc"), []byte(preExisting), 0o644))

	err := SetupZshrc(tmp, "/irrelevant", ZshrcOpts{
		PersonalComputer: "true",
		DevRoot:          "/d",
		GitHost:          "g",
		GitCloneUserName: "u",
	})
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(tmp, ".zshrc"))
	require.NoError(t, err)
	// Phase 1 must NOT run: existing user-edited content stays as-is.
	assert.Contains(t, string(got), "# user header\n")
	assert.Contains(t, string(got), "export GIT_HOST=\"frozen\"")
	assert.NotContains(t, string(got), "export DEV_ROOT=") // would only exist if phase 1 ran
	// Phase 2 must run: the managed block now points at shared_config.
	assert.Contains(t, string(got), "source \"$HOME/.melvin/config/shared_config/base.zshrc\"")
	assert.NotContains(t, string(got), "OLD/base.zshrc")
}
```

- [ ] **Step 2: Run the test, confirm it fails**

Run: `go test ./internal/configgen/ -run TestSetupZshrc_RerunUpdatesManagedBlock -v`
Expected: FAIL — current `SetupZshrc` returns nil early when the file exists; the managed block stays stale.

---

### Task 2.2: Refactor `SetupZshrc` to two-phase

**Files:**
- Modify: `internal/configgen/zshrc.go`

- [ ] **Step 1: Rewrite `zshrc.go`**

Replace the entire body of `internal/configgen/zshrc.go` with:

```go
package configgen

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

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

// SetupZshrc materializes ~/.zshrc in two phases:
//
//   - Phase 1 (first-install only): when the file is absent, write
//     seven export lines (GIT_HOST / GIT_CLONE_USER_NAME /
//     PERSONAL_COMPUTER / DEV_ROOT / WORKTREES_ROOT / REPOS_ROOT /
//     SDKS_ROOT) followed by a single newline. These reflect values
//     resolved once at install time; subsequent runs don't re-prompt
//     and don't overwrite the user's edits.
//
//   - Phase 2 (always): upsert the managed-block region containing
//     the `source` line for shared_config/base.zshrc. See
//     internal/managedblock for upsert semantics.
//
// configDir is unused but retained in the signature for symmetry
// with the other configgen functions.
func SetupZshrc(homeDir, _ string, opts ZshrcOpts) error {
	target := filepath.Join(homeDir, ".zshrc")

	if err := writeZshrcUserExportsIfAbsent(target, opts); err != nil {
		return err
	}
	if err := managedblock.Upsert(target, zshrcSourceLine, zshrcPriorLine); err != nil {
		return fmt.Errorf("upsert managed block in %s: %w", target, err)
	}
	return nil
}

// writeZshrcUserExportsIfAbsent writes the seven export lines when
// target is absent. It returns nil silently if target already
// exists (or any directory entry exists at target). Anything other
// than fs.ErrNotExist propagates.
func writeZshrcUserExportsIfAbsent(target string, opts ZshrcOpts) error {
	if _, err := os.Stat(target); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", target, err)
	}

	worktreesRoot := opts.DevRoot + "/worktrees"
	reposRoot := opts.DevRoot + "/repos"
	sdksRoot := opts.DevRoot + "/sdks"

	content := fmt.Sprintf(
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

	if err := os.WriteFile(target, []byte(content), 0o644); err != nil { //nolint:gosec // 0644 is the conventional default umask + write
		return fmt.Errorf("write %s: %w", target, err)
	}
	return nil
}
```

- [ ] **Step 2: Run the new test, confirm it now passes**

Run: `go test ./internal/configgen/ -run TestSetupZshrc_RerunUpdatesManagedBlock -v`
Expected: PASS.

---

### Task 2.3: Update existing `zshrc_test.go` assertions

The three existing tests reference the old fresh-write shape (source line outside any managed block). They need to be updated to reflect the new two-phase reality.

**Files:**
- Modify: `internal/configgen/zshrc_test.go`

- [ ] **Step 1: Update `TestSetupZshrc_FreshWrite` assertions**

Replace the `expected` block + assertion in `TestSetupZshrc_FreshWrite`:

```go
	expected := "\nexport GIT_HOST=\"git@github.com\"" +
		"\nexport GIT_CLONE_USER_NAME=\"Nivl\"" +
		"\nexport PERSONAL_COMPUTER=\"true\"" +
		"\nexport DEV_ROOT=\"/home/user/dev\"" +
		"\nexport WORKTREES_ROOT=\"/home/user/dev/worktrees\"" +
		"\nexport REPOS_ROOT=\"/home/user/dev/repos\"" +
		"\nexport SDKS_ROOT=\"/home/user/dev/sdks\"\n" +
		"# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"source \"$HOME/.melvin/config/shared_config/base.zshrc\"\n" +
		"# <<< melvin-config managed <<<\n"
	assert.Equal(t, expected, string(got))
```

The phase-1 exports now end with `\n` (no trailing source line), and the wrapped block is appended at EOF (because the regex doesn't match — phase 1 wrote no source line). The trailing newline on the exports + the leading `# >>>` line means there's exactly one newline between them, which is what `migrateOrAppend` produces.

- [ ] **Step 2: Update `TestSetupZshrc_FirstInstallGuard` assertions**

Replace the assertion block in `TestSetupZshrc_FirstInstallGuard`:

```go
	got, err := os.ReadFile(filepath.Join(tmp, ".zshrc"))
	require.NoError(t, err)
	// Phase 1 must be guarded: no export lines added.
	assert.NotContains(t, string(got), "export DEV_ROOT=")
	// Phase 2 must still run: managed block appended at EOF (regex
	// doesn't match the pre-existing comment, so append-at-EOF).
	expected := preExisting +
		"# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"source \"$HOME/.melvin/config/shared_config/base.zshrc\"\n" +
		"# <<< melvin-config managed <<<\n"
	assert.Equal(t, expected, string(got))
```

Update the test's doc comment to reflect the new behavior:

```go
// TestSetupZshrc_FirstInstallGuard — when the file pre-exists, the
// seven exports are NOT added (phase 1 guard), but the managed block
// IS appended at EOF (phase 2 always runs).
```

- [ ] **Step 3: Add the migration test**

Append to `zshrc_test.go`:

```go
// TestSetupZshrc_MigrationFromPreManagedBlock — when the file has
// the pre-2026-05-14 shape (7 exports + bare source line, no
// markers), the bare source line is replaced in place with the
// wrapped block; the seven exports above stay intact.
func TestSetupZshrc_MigrationFromPreManagedBlock(t *testing.T) {
	tmp := t.TempDir()
	preExisting := "\nexport GIT_HOST=\"g\"" +
		"\nexport GIT_CLONE_USER_NAME=\"u\"" +
		"\nexport PERSONAL_COMPUTER=\"true\"" +
		"\nexport DEV_ROOT=\"/d\"" +
		"\nexport WORKTREES_ROOT=\"/d/worktrees\"" +
		"\nexport REPOS_ROOT=\"/d/repos\"" +
		"\nexport SDKS_ROOT=\"/d/sdks\"" +
		"\nsource \"$HOME/.melvin/config/shared_config/base.zshrc\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmp, ".zshrc"), []byte(preExisting), 0o644))

	err := SetupZshrc(tmp, "/irrelevant", ZshrcOpts{})
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(tmp, ".zshrc"))
	require.NoError(t, err)
	// Migration: the lone source line is now wrapped.
	assert.Contains(t, string(got), "# >>> melvin-config managed >>>")
	assert.Contains(t, string(got), "# <<< melvin-config managed <<<")
	// Seven exports remain.
	assert.Contains(t, string(got), "export DEV_ROOT=\"/d\"")
	assert.Contains(t, string(got), "export SDKS_ROOT=\"/d/sdks\"")
	// Only one occurrence of the source line (no duplicate from a
	// bad migration that would append a second copy).
	assert.Equal(t, 1, strings.Count(string(got),
		"source \"$HOME/.melvin/config/shared_config/base.zshrc\""))
}
```

Add `"strings"` to the test file's imports if not already present.

- [ ] **Step 4: Run all configgen tests**

Run: `go test ./internal/configgen/ -v`
Expected: all PASS (existing + new).

---

### Task 2.4: Lint + commit Phase 2

- [ ] **Step 1: Lint**

Run: `golangci-lint run ./...`
Expected: `No issues found`.

- [ ] **Step 2: Full test sweep**

Run: `go test ./...`
Expected: all green.

- [ ] **Step 3: Commit**

```bash
git add internal/configgen/zshrc.go internal/configgen/zshrc_test.go
git commit -m "$(cat <<'EOF'
refactor(configgen): split SetupZshrc into two phases

Phase 1 (file absent) writes only the seven user-machine export
lines. Phase 2 (always) upserts the managed block containing the
`source` line for shared_config/base.zshrc via
managedblock.Upsert. The shared_config path now updates on every
setup run instead of being frozen at first install.

Existing files written before the marker convention migrate in
place: the bare source line is replaced with the wrapped block;
the seven exports above it stay intact.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 3 — Refactor `SetupGitconfig`

Single commit at the end of Phase 3. Same shape as Phase 2; condensed for brevity but every step still has full code.

### Task 3.1: TDD — Re-run updates managed block

**Files:**
- Modify: `internal/configgen/gitconfig_test.go`

- [ ] **Step 1: Add the failing test**

Append to `gitconfig_test.go`:

```go
// TestSetupGitconfig_RerunUpdatesManagedBlock — when the file
// already has a managed block whose [include] path has drifted, the
// fresh SetupGitconfig rewrites the inter-marker region while
// leaving the [user] / [commit] blocks untouched (phase 1 is
// skipped because the file exists).
func TestSetupGitconfig_RerunUpdatesManagedBlock(t *testing.T) {
	tmp := t.TempDir()
	preExisting := "[user]\n\temail = pinned@example.com\n\n" +
		"# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"[include]\n\tpath = \"" + tmp + "/.melvin/config/OLD/.gitconfig\"\n" +
		"# <<< melvin-config managed <<<\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmp, ".gitconfig"), []byte(preExisting), 0o644))

	err := SetupGitconfig(tmp, "/irrelevant", true)
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(tmp, ".gitconfig"))
	require.NoError(t, err)
	// Phase 1 guarded.
	assert.Contains(t, string(got), "email = pinned@example.com")
	assert.NotContains(t, string(got), "noreply@melvin.la")
	// Phase 2 ran: managed block now points at shared_config.
	assert.Contains(t, string(got),
		"path = \""+tmp+"/.melvin/config/shared_config/.gitconfig\"")
	assert.NotContains(t, string(got), "OLD/.gitconfig")
}
```

- [ ] **Step 2: Run the test, confirm it fails**

Run: `go test ./internal/configgen/ -run TestSetupGitconfig_RerunUpdatesManagedBlock -v`
Expected: FAIL.

---

### Task 3.2: Refactor `SetupGitconfig` to two-phase

**Files:**
- Modify: `internal/configgen/gitconfig.go`

- [ ] **Step 1: Rewrite `gitconfig.go`**

Replace the entire body of `internal/configgen/gitconfig.go` with:

```go
package configgen

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Nivl/config/internal/managedblock"
)

// gitconfigPriorLine matches the two-line [include] block as
// previously written by SetupGitconfig (outside any managed block).
// Used by managedblock.Upsert for one-shot migration of pre-marker
// files. Matches both lines as a pair to avoid leaving an orphan
// [include] header behind.
var gitconfigPriorLine = regexp.MustCompile( //nolint:gochecknoglobals // immutable regex
	`(?m)^\[include\]\s*\n\s*path\s*=\s*"[^"]*\.melvin/config/.*"\s*$`,
)

// SetupGitconfig materializes ~/.gitconfig in two phases:
//
//   - Phase 1 (first-install only): when the file is absent, write a
//     per-machine [user] block (email/name plus a signingkey on
//     PERSONAL_COMPUTER=true, or a commented stub otherwise), a
//     commented [url] template, and a [commit] block with the
//     appropriate gpgsign value.
//
//   - Phase 2 (always): upsert the managed-block region containing
//     the [include] two-liner that pulls in
//     shared_config/.gitconfig.
//
// configDir is unused but retained in the signature for symmetry
// with the other configgen functions.
func SetupGitconfig(homeDir, _ string, personalComputer bool) error {
	target := filepath.Join(homeDir, ".gitconfig")

	if err := writeGitconfigIdentityIfAbsent(target, homeDir, personalComputer); err != nil {
		return err
	}

	payload := fmt.Sprintf("[include]\n\tpath = \"%s/.melvin/config/shared_config/.gitconfig\"", homeDir)
	if err := managedblock.Upsert(target, payload, gitconfigPriorLine); err != nil {
		return fmt.Errorf("upsert managed block in %s: %w", target, err)
	}
	return nil
}

// writeGitconfigIdentityIfAbsent writes the per-machine identity +
// [url] stub + [commit] block when target is absent. Existing files
// short-circuit with nil. The [include] section is no longer
// written here — it lives inside the managed block now.
func writeGitconfigIdentityIfAbsent(target, homeDir string, personalComputer bool) error {
	_ = homeDir // homeDir is consumed by the managed-block payload, not here.

	if _, err := os.Stat(target); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", target, err)
	}

	var user, commit string
	if personalComputer {
		user = "[user]\n\temail = noreply@melvin.la" +
			"\n\tname = Melvin" +
			"\n\tsigningkey = 2C307E0D0413344B"
		commit = "[commit]" +
			"\n\tgpgsign = true"
	} else {
		user = "[user]\n\temail = melvin@domain.tld" +
			"\n\tname = Melvin" +
			"\n\t# signingkey = <key>"
		commit = "[commit]" +
			"\n\tgpgsign = false"
	}

	urlComment := "# [url \"ssh://git@github.com/\"]" +
		"\n\t# insteadOf = https://github.com/"

	content := user + "\n\n" + urlComment + "\n\n" + commit + "\n"

	if err := os.WriteFile(target, []byte(content), 0o644); err != nil { //nolint:gosec // 0644 conventional umask write
		return fmt.Errorf("write %s: %w", target, err)
	}
	return nil
}
```

Note the changes vs. the original:
- The `[include]` section is removed from phase 1 (it's now the managed-block payload).
- The body ends with `\n` so phase 2's `migrateOrAppend` appends with exactly one blank-line separator (the `\n` already there plus phase 2's `wrap` payload starting with `BeginMarker`).

- [ ] **Step 2: Run the new test, confirm it now passes**

Run: `go test ./internal/configgen/ -run TestSetupGitconfig_RerunUpdatesManagedBlock -v`
Expected: PASS.

---

### Task 3.3: Update existing `gitconfig_test.go` assertions

**Files:**
- Modify: `internal/configgen/gitconfig_test.go`

- [ ] **Step 1: Update `TestSetupGitconfig_PersonalContent` and `TestSetupGitconfig_WorkContent` assertions**

These tests assert the fresh-write file content byte-by-byte. The shape changes: phase 1 now writes `[user] + [url] + [commit]` first, then phase 2 appends the wrapped block containing `[include]`.

Replace `TestSetupGitconfig_PersonalContent`'s `expected` block:

```go
	expected := "[user]\n\temail = noreply@melvin.la" +
		"\n\tname = Melvin" +
		"\n\tsigningkey = 2C307E0D0413344B" +
		"\n\n# [url \"ssh://git@github.com/\"]" +
		"\n\t# insteadOf = https://github.com/" +
		"\n\n[commit]" +
		"\n\tgpgsign = true\n" +
		"# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"[include]\n\tpath = \"" + tmp + "/.melvin/config/shared_config/.gitconfig\"\n" +
		"# <<< melvin-config managed <<<\n"
	assert.Equal(t, expected, string(got))
```

Replace `TestSetupGitconfig_WorkContent`'s `expected` block:

```go
	expected := "[user]\n\temail = melvin@domain.tld" +
		"\n\tname = Melvin" +
		"\n\t# signingkey = <key>" +
		"\n\n# [url \"ssh://git@github.com/\"]" +
		"\n\t# insteadOf = https://github.com/" +
		"\n\n[commit]" +
		"\n\tgpgsign = false\n" +
		"# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"[include]\n\tpath = \"" + tmp + "/.melvin/config/shared_config/.gitconfig\"\n" +
		"# <<< melvin-config managed <<<\n"
	assert.Equal(t, expected, string(got))
```

- [ ] **Step 2: Update `TestSetupGitconfig_FirstInstallGuard`**

Replace the assertion block:

```go
	got, err := os.ReadFile(filepath.Join(tmp, ".gitconfig"))
	require.NoError(t, err)
	// Phase 1 guarded: no [user] / [commit] added.
	assert.NotContains(t, string(got), "[user]")
	assert.NotContains(t, string(got), "[commit]")
	// Phase 2 always runs: managed block appended at EOF.
	expected := preExisting +
		"# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"[include]\n\tpath = \"" + tmp + "/.melvin/config/shared_config/.gitconfig\"\n" +
		"# <<< melvin-config managed <<<\n"
	assert.Equal(t, expected, string(got))
```

Update the doc comment:

```go
// TestSetupGitconfig_FirstInstallGuard — when the file pre-exists,
// phase 1 (identity / url / commit) is skipped, but phase 2 always
// runs and appends the managed [include] block at EOF.
```

- [ ] **Step 3: Add the migration test**

Append to `gitconfig_test.go`:

```go
// TestSetupGitconfig_MigrationFromPreManagedBlock — when the file
// has the pre-2026-05-14 shape ([include] at top followed by
// [user]/[url]/[commit]), the [include] two-liner is replaced in
// place with the wrapped block; the rest stays intact.
func TestSetupGitconfig_MigrationFromPreManagedBlock(t *testing.T) {
	tmp := t.TempDir()
	preExisting := "[include]\n\tpath = \"" + tmp + "/.melvin/config/shared_config/.gitconfig\"" +
		"\n\n[user]\n\temail = pinned@example.com" +
		"\n\tname = Melvin" +
		"\n\n[commit]\n\tgpgsign = true"
	require.NoError(t, os.WriteFile(filepath.Join(tmp, ".gitconfig"), []byte(preExisting), 0o644))

	err := SetupGitconfig(tmp, "/irrelevant", true)
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(tmp, ".gitconfig"))
	require.NoError(t, err)
	// Migration: the [include] two-liner is now wrapped.
	assert.Contains(t, string(got), "# >>> melvin-config managed >>>")
	assert.Contains(t, string(got), "# <<< melvin-config managed <<<")
	// [user] / [commit] preserved.
	assert.Contains(t, string(got), "email = pinned@example.com")
	assert.Contains(t, string(got), "gpgsign = true")
	// No orphan [include] header.
	assert.Equal(t, 1, strings.Count(string(got), "[include]"))
}
```

Add `"strings"` to the test file's imports if not already present.

- [ ] **Step 4: Run all configgen tests**

Run: `go test ./internal/configgen/ -v`
Expected: all PASS.

---

### Task 3.4: Lint + commit Phase 3

- [ ] **Step 1: Lint + full sweep + commit**

Run: `go test ./... && golangci-lint run ./...`
Expected: all green.

```bash
git add internal/configgen/gitconfig.go internal/configgen/gitconfig_test.go
git commit -m "$(cat <<'EOF'
refactor(configgen): split SetupGitconfig into two phases

Phase 1 (file absent) writes the per-machine [user] block, [url]
template, and [commit] block. Phase 2 (always) upserts the managed
block containing the [include] two-liner that pulls in
shared_config/.gitconfig.

Existing files written before the marker convention migrate in
place: the [include] block is replaced with the wrapped version,
the [user] / [commit] sections stay intact, no orphan [include]
header is left behind.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 4 — Refactor `SetupGpg`

Single commit at the end of Phase 4. More involved than the prior two phases because of the mkdir + Upsert + restart ordering.

### Task 4.1: TDD — Re-run updates managed block, no restart

**Files:**
- Modify: `internal/configgen/gpg_test.go`

- [ ] **Step 1: Add the failing test**

Append to `gpg_test.go`:

```go
// TestSetupGpg_RerunUpdatesManagedBlockNoRestart — when the file
// already exists with a managed block whose payload has drifted,
// SetupGpg rewrites the inter-marker region and does NOT call
// killall / gpg-agent --daemon (the agent re-reads pinentry-program
// on its next pinentry invocation, no restart needed).
func TestSetupGpg_RerunUpdatesManagedBlockNoRestart(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, ".gnupg"), 0o700))
	preExisting := "# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"pinentry-program /old/prefix/bin/pinentry-mac\n" +
		"# <<< melvin-config managed <<<\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmp, ".gnupg", "gpg-agent.conf"),
		[]byte(preExisting), 0o644))
	runner := configgentest.NewFakeCmdRunner() // no expectations: no Run calls allowed

	err := SetupGpg(context.Background(), tmp, "/opt/homebrew", runner)
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(tmp, ".gnupg", "gpg-agent.conf"))
	require.NoError(t, err)
	assert.Contains(t, string(got), "pinentry-program /opt/homebrew/bin/pinentry-mac")
	assert.NotContains(t, string(got), "/old/prefix/")
	runner.AssertNotCalled(t, "Run", mock.Anything, "killall", mock.Anything)
	runner.AssertNotCalled(t, "Run", mock.Anything, "gpg-agent", mock.Anything)
}
```

- [ ] **Step 2: Run the test, confirm it fails**

Run: `go test ./internal/configgen/ -run TestSetupGpg_RerunUpdatesManagedBlockNoRestart -v`
Expected: FAIL.

---

### Task 4.2: Refactor `SetupGpg` to three-phase

**Files:**
- Modify: `internal/configgen/gpg.go`

- [ ] **Step 1: Rewrite `gpg.go`**

Replace the entire body of `internal/configgen/gpg.go` with:

```go
package configgen

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/Nivl/config/internal/managedblock"
)

// gpgAgentConfPriorLine matches any line beginning with
// `pinentry-program ` so SetupGpg can migrate files written before
// the marker convention regardless of the prefix recorded at that
// time.
var gpgAgentConfPriorLine = regexp.MustCompile( //nolint:gochecknoglobals // immutable regex
	`(?m)^pinentry-program\s+.*$`,
)

// SetupGpg materializes ~/.gnupg/gpg-agent.conf in three phases:
//
//   - Phase 1 (file absent): mkdir ~/.gnupg with 0o700.
//   - Phase 2 (always): upsert the managed-block region containing
//     the pinentry-program line.
//   - Phase 3 (file was absent): killall gpg-agent + gpg-agent
//     --daemon. Required only on first install so the freshly
//     written conf is picked up by a fresh agent process. On
//     subsequent runs the existing agent re-reads pinentry-program
//     on its next pinentry invocation, so no restart is needed.
//
// killall exit code 1 means "no such process" — expected on first
// install. Any other error (including non-ExitError) propagates.
//
// runner allows tests to inject a FakeCmdRunner. Production callers
// pass NewCmdRunner().
func SetupGpg(ctx context.Context, homeDir, brewPrefix string, runner CmdRunner) error {
	conf := filepath.Join(homeDir, ".gnupg", "gpg-agent.conf")

	_, statErr := os.Stat(conf)
	firstInstall := errors.Is(statErr, fs.ErrNotExist)
	if statErr != nil && !firstInstall {
		return fmt.Errorf("stat %s: %w", conf, statErr)
	}

	if firstInstall {
		if err := os.MkdirAll(filepath.Dir(conf), 0o700); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(conf), err)
		}
	}

	payload := fmt.Sprintf("pinentry-program %s/bin/pinentry-mac", brewPrefix)
	if err := managedblock.Upsert(conf, payload, gpgAgentConfPriorLine); err != nil {
		return fmt.Errorf("upsert managed block in %s: %w", conf, err)
	}

	if !firstInstall {
		return nil
	}

	if err := runner.Run(ctx, "killall", "gpg-agent"); err != nil {
		// killall exit code 1 means "no such process" — expected on
		// first install where no agent is running yet.
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			return fmt.Errorf("killall gpg-agent: %w", err)
		}
	}

	if err := runner.Run(ctx, "gpg-agent", "--daemon"); err != nil {
		return fmt.Errorf("gpg-agent --daemon: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Run the new test, confirm it now passes**

Run: `go test ./internal/configgen/ -run TestSetupGpg_RerunUpdatesManagedBlockNoRestart -v`
Expected: PASS.

---

### Task 4.3: Update existing `gpg_test.go` assertions

**Files:**
- Modify: `internal/configgen/gpg_test.go`

- [ ] **Step 1: Update `TestSetupGpg_FreshWrite` assertions**

Replace the conf-content assertion in `TestSetupGpg_FreshWrite`:

```go
	got, err := os.ReadFile(filepath.Join(tmp, ".gnupg", "gpg-agent.conf"))
	require.NoError(t, err)
	expected := "# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"pinentry-program /opt/homebrew/bin/pinentry-mac\n" +
		"# <<< melvin-config managed <<<\n"
	assert.Equal(t, expected, string(got))
```

- [ ] **Step 2: Update `TestSetupGpg_FirstInstallGuard`**

This test currently asserts the pre-existing single-line content is preserved. After the refactor, phase 2 always runs — the regex matches the existing `pinentry-program /old/path` line and replaces it with the wrapped block. The "guard" now means "no killall/daemon/mkdir," not "no file change."

Rewrite the test fully (replace the function body):

```go
// TestSetupGpg_FirstInstallGuard — when the conf file pre-exists,
// no mkdir + no killall/daemon shellouts. The managed block IS
// upserted (replacing the existing pinentry-program line in place).
func TestSetupGpg_FirstInstallGuard(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, ".gnupg"), 0o700))
	preExisting := "pinentry-program /old/path\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmp, ".gnupg", "gpg-agent.conf"), []byte(preExisting), 0o644))
	runner := configgentest.NewFakeCmdRunner() // no expectations: no Run calls allowed

	err := SetupGpg(context.Background(), tmp, "/opt/homebrew", runner)
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(tmp, ".gnupg", "gpg-agent.conf"))
	require.NoError(t, err)
	expected := "# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"pinentry-program /opt/homebrew/bin/pinentry-mac\n" +
		"# <<< melvin-config managed <<<\n"
	assert.Equal(t, expected, string(got))

	runner.AssertNotCalled(t, "Run", mock.Anything, "killall", mock.Anything)
	runner.AssertNotCalled(t, "Run", mock.Anything, "gpg-agent", mock.Anything)
}
```

- [ ] **Step 3: Verify remaining tests still pass**

The other gpg tests (`KillallExitCode1Ignored`, `KillallExitCodeOtherPropagates`, `KillallNonExitErrorPropagates`, `DaemonLaunchErrorPropagates`) all set up a fresh tempdir (no pre-existing conf), so they exercise the fresh-install path including killall/daemon. They should still pass without modification.

Run: `go test ./internal/configgen/ -v`
Expected: all PASS (existing + new).

---

### Task 4.4: Lint + commit Phase 4

- [ ] **Step 1: Lint + full sweep + commit**

Run: `go test ./... && golangci-lint run ./...`
Expected: all green.

```bash
git add internal/configgen/gpg.go internal/configgen/gpg_test.go
git commit -m "$(cat <<'EOF'
refactor(configgen): split SetupGpg into three phases

Phase 1 (file absent) creates ~/.gnupg with 0o700. Phase 2 (always)
upserts the managed block containing the pinentry-program line.
Phase 3 (file was absent) runs killall + gpg-agent --daemon to
spawn a fresh agent against the freshly-written conf.

The split means subsequent setup runs no longer kill a running
gpg-agent: the agent re-reads pinentry-program on its next pinentry
invocation, so a restart is unnecessary when only the conf path
changed.

Existing files written before the marker convention migrate in
place: the bare pinentry-program line is replaced with the wrapped
block.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 5 — Final verification

### Task 5.1: End-to-end sanity check

- [ ] **Step 1: Confirm full test suite passes**

Run: `go test ./...`
Expected: `ok` for every package; no FAIL lines. Tally should be the prior 333 tests + ~16 new (12 in managedblock + 4 across configgen rerun/migration tests).

- [ ] **Step 2: Confirm lint passes for both build tags**

Run: `golangci-lint run ./... && GOOS=darwin golangci-lint run ./...`
Expected: `No issues found` for both.

- [ ] **Step 3: Confirm `git log` shows the four phase commits**

Run: `git log --oneline -5`
Expected: top four entries (newest first) are the Phase 4, 3, 2, 1 commits with the messages above. The fifth entry is the spec commit (`spec: managed-block updates for configgen-materialized files`).

- [ ] **Step 4: Update AGENTS.md to reflect the new managedblock package**

**Files:**
- Modify: `AGENTS.md`

In the `## Layout` table, add a new row alphabetically between `internal/iox/` and `internal/packages/`:

```markdown
| `internal/managedblock/` | `Upsert(path, payload, priorLine)` — sentinel-fenced read-modify-write primitive for materialized config files. Used by `configgen` so repo-relative paths inside `~/.zshrc` / `~/.gitconfig` / `~/.gnupg/gpg-agent.conf` can evolve across setup runs without losing user customizations outside the managed block. |
```

In the `## Go packages` table (second occurrence), add an equivalent row:

```markdown
| `internal/managedblock` | `Upsert(path, payload, priorLine *regexp.Regexp) error` — installs / refreshes / migrates a sentinel-fenced managed block inside an existing text file. Idempotent (mtime preserved on no-op), atomic (tempfile + rename), fails loud on malformed marker state. Consumed only by `internal/configgen`. |
```

In the `## Bootstrap flow` section under the `melvin-config setup` bullets, update the **Dotfile copy + config generation** bullet to mention the managed-block behavior:

```markdown
- **Dotfile copy + config generation** (`internal/dotfiles` + `internal/configgen` + `internal/managedblock`) — symlinks 2 curated dotfiles (via `internal/symlinkfs.Install`; existing targets that aren't already the right symlink get auto-backed-up as `<name>.<YYYYMMDDHHMMSS>.bkp` then replaced) + creates `~/.emacs-saves`; materializes `~/.zshrc`, `~/.gitconfig`, and `~/.gnupg/gpg-agent.conf`. The repo-relative `source` / `[include]` / `pinentry-program` lines inside those materialized files live in a sentinel-fenced managed block that gets re-checked and rewritten on every `setup` run — user customizations outside the block are preserved.
```

- [ ] **Step 5: Commit the AGENTS.md update**

```bash
git add AGENTS.md
git commit -m "$(cat <<'EOF'
docs(agents): document internal/managedblock + two-phase configgen

New row in the layout / Go-packages tables for internal/managedblock,
and the bootstrap-flow bullet for "Dotfile copy + config generation"
now reflects that the materialized config files re-check their
managed-block region on every setup run.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 6: Final tally**

Run: `git log --oneline -7`
Expected: 5 commits land on top of the spec commit (Phase 1 + 2 + 3 + 4 + AGENTS update).

---

## Out of scope

- Migrating files that point at paths pre-dating `shared_config/` (e.g., users whose `~/.zshrc` has `source "$HOME/.melvin/config/base.zshrc"` without the `shared_config/` segment). The regex won't match → wrapped block is appended at EOF, leaving the stale line behind for manual cleanup. Acceptable: catching every historical path shape isn't tractable.

- Re-resolving phase-1 user-machine values (DEV_ROOT, identity email, etc.) on subsequent runs. Those remain set-once-at-first-install by design (spec §3 non-goals).

- Symlink-following inside `Upsert`. Current callers operate on regular files; if a future caller needs symlink-target updates, it should `filepath.EvalSymlinks` before passing the path in (spec §12).
