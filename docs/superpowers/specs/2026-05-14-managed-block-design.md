# Managed-Block Config File Updates — Design

**Status:** Approved
**Date:** 2026-05-14
**Scope:** `internal/configgen` (`SetupZshrc`, `SetupGitconfig`, `SetupGpg`) + new `internal/managedblock` package.

## 1. Problem

`SetupZshrc`, `SetupGitconfig`, `SetupGpg` each have a first-install guard: they `os.Stat` the target file, return `nil` silently if it exists, and only write content on first install. The repo paths they write inside those files are therefore frozen at install time.

Consequence: renaming or moving in-repo paths (e.g. `shared_config/base.zshrc` → some new location) leaves every existing user with a stale `source` line in `~/.zshrc`, a stale `[include] path` in `~/.gitconfig`, or a stale `pinentry-program` line in `~/.gnupg/gpg-agent.conf`. There is no current mechanism to evolve these paths post-install.

## 2. Goals

- Allow repo-relative paths written into materialized config files to evolve safely on subsequent `melvin-config setup` runs.
- Preserve user customizations made outside the auto-managed region (aliases in `.zshrc`, identity in `.gitconfig`, etc.).
- Migrate existing installs (whose files don't have markers yet) without destructive overwrites.
- Idempotent: re-running `setup` with no changes is a true no-op (no mtime bump, no progress output).
- Fail loud on malformed state; never silently auto-repair.

## 3. Non-goals

- Managing arbitrary user-machine values (DEV_ROOT, identity email, etc.) across runs. Those remain set-once-on-first-install.
- Surfacing or merging user edits inside the managed block. The block is owned exclusively by `setup`; anything between the markers is replaced wholesale.
- Migrating from paths older than the current `shared_config/`-rooted layout. If a user's file points at an even older path, the migration regex won't match and they'll see the new managed block appended at EOF, with their stale line left for manual cleanup.

## 4. Architecture

A new `internal/managedblock` package owns the read-modify-write primitive. Each `configgen` function is split into two phases:

1. **First-install phase** (today's logic, file-existence-gated): write user-machine values that are resolved once and never re-checked — env-var exports for `.zshrc`, `[user]`/`[commit]`/`[url]` blocks for `.gitconfig`, the `~/.gnupg` directory creation + `gpg-agent` restart for the pinentry case.
2. **Always-run phase** (new): call `managedblock.Upsert` to install / refresh / migrate the managed block.

The split means `SetupZshrc` etc. no longer return early when the file exists — they fall through to phase 2.

### 4.1 Why a shared package

All three configgen functions need the same primitive (find-or-create a fenced region in a text file, splice, atomic-write). Inlining it three times would invite drift. A small, well-tested package is the right unit.

## 5. Public API

```go
package managedblock

import "regexp"

// BeginMarker / EndMarker delimit the managed region. Both files we
// touch (.zshrc, .gitconfig, .gnupg/gpg-agent.conf) accept `#` as a
// line-comment introducer.
const (
    BeginMarker = "# >>> melvin-config managed >>>"
    EndMarker   = "# <<< melvin-config managed <<<"
)

// Upsert installs payload into path, wrapped in BeginMarker /
// EndMarker plus a descriptive "do not edit" comment line. Atomic via
// tempfile + os.Rename. Returns nil if the file's new content would
// be identical to its current content (no write, mtime preserved).
//
// Behavior by file state:
//
//   - File missing: write a new file containing just the wrapped block.
//   - Markers present (exactly one BeginMarker and exactly one EndMarker,
//     in order): replace the region between them with the new payload.
//   - Markers absent + priorLine non-nil + priorLine matches: replace the
//     first matched line(s) with the wrapped block (migration path).
//   - Markers absent + no migration match: append the wrapped block at
//     EOF, with a leading newline if the file doesn't end in '\n'.
//   - Markers malformed (only one of the pair, wrong order, duplicates):
//     return error; do not write.
//
// priorLine may be nil to disable migration ("just append if markers
// absent"). When non-nil, only the first regex match is replaced;
// subsequent occurrences are left intact.
func Upsert(path, payload string, priorLine *regexp.Regexp) error
```

The descriptive comment line (`# Do not edit this block by hand — it is rewritten by 'melvin-config setup'.`) is hardcoded inside `Upsert` and prepended to `payload` automatically. Callers pass payload lines only.

## 6. Wrapped block shape

For a given `payload`, the wrapped block emitted by `Upsert` is:

```
# >>> melvin-config managed >>>
# Do not edit this block by hand — it is rewritten by `melvin-config setup`.
<payload>
# <<< melvin-config managed <<<
```

Trailing newline included. When splicing into an existing file, adjacent blank lines on each side of the splice are trimmed so that at most one blank line remains between the wrapped block and surrounding content. This prevents blank-line accumulation across runs.

## 7. Algorithm

`Upsert(path, payload, priorLine)`:

1. **Read** the file. If `os.IsNotExist`: emit content = the wrapped block, write atomically with perm `0o644`, return.
2. **Find markers.** Count occurrences of `BeginMarker` and `EndMarker` (substring match on whole lines).
   - Both = 1 and `Begin` appears before `End` → splice path: keep `content[:beginLineStart]` + wrapped block + `content[endLineEnd:]`. Trim adjacent blank lines around the splice to one each.
   - Both = 0 → migration path: if `priorLine != nil` and matches, replace the first match's full line(s) with the wrapped block; else append the wrapped block at EOF (leading `\n` if file doesn't end in one).
   - Any other count combination (1+0, 0+1, 2+1, 1+2, 2+2, etc.) OR wrong-order (Begin after End) → return error: `"<path>: malformed managed block: <reason>; resolve manually"`.
3. **Compare** the proposed new content byte-for-byte against the existing content. If equal → return `nil` without writing.
4. **Atomic write**: create `<path>.tmp.<pid>.<rand>` in the same directory, write payload, fsync, `os.Rename` over the destination. Permission inherits from the existing file's mode bits; for fresh files, `0o644`.

### 7.1 Cleanup on error

If write fails after tempfile creation, the tempfile remains on disk for diagnosis. `Upsert` returns the wrapped error; the tempfile name is included in the error message so the user can locate it. We do **not** auto-delete tempfiles on error — they're evidence.

## 8. Per-file payloads and migration regexes

### 8.1 `~/.zshrc`

```go
// Phase 1 (first-install only): existing user-exports write — unchanged.
// Phase 2 (always-run):
payload := `source "$HOME/.melvin/config/shared_config/base.zshrc"`
priorLine := regexp.MustCompile(`(?m)^source\s+"\$HOME/\.melvin/config/shared_config/base\.zshrc"\s*$`)
return managedblock.Upsert(filepath.Join(homeDir, ".zshrc"), payload, priorLine)
```

The migration regex matches the exact line shape `SetupZshrc` previously wrote. On hit, the line is replaced with the wrapped block; the 7 export lines above it remain untouched.

### 8.2 `~/.gitconfig`

```go
// Phase 1 (first-install only): identity + [url] stub + [commit] block.
// Phase 2 (always-run):
includePath := filepath.Join(homeDir, ".melvin/config/shared_config/.gitconfig")
payload := fmt.Sprintf("[include]\n\tpath = \"%s\"", includePath)
priorLine := regexp.MustCompile(`(?m)^\[include\]\s*\n\s*path\s*=\s*"[^"]*\.melvin/config/.*"\s*$`)
return managedblock.Upsert(filepath.Join(homeDir, ".gitconfig"), payload, priorLine)
```

The migration regex matches both lines of the existing `[include]` block as a pair, avoiding an orphan `[include]` header in the migrated file.

### 8.3 `~/.gnupg/gpg-agent.conf`

This case has more orchestration than zshrc/gitconfig because `gpg-agent.conf` lives inside `~/.gnupg/` (which may not exist yet) and a first-install needs to start the agent. `SetupGpg` therefore stats the conf file up front, then branches:

```go
conf := filepath.Join(homeDir, ".gnupg", "gpg-agent.conf")
_, statErr := os.Stat(conf)
firstInstall := errors.Is(statErr, os.ErrNotExist)
if statErr != nil && !firstInstall {
    return fmt.Errorf("stat %s: %w", conf, statErr)
}
if firstInstall {
    if err := os.MkdirAll(filepath.Dir(conf), 0o700); err != nil {
        return fmt.Errorf("mkdir %s: %w", filepath.Dir(conf), err)
    }
}

payload := fmt.Sprintf("pinentry-program %s/bin/pinentry-mac", brewPrefix)
priorLine := regexp.MustCompile(`(?m)^pinentry-program\s+.*$`)
if err := managedblock.Upsert(conf, payload, priorLine); err != nil {
    return err
}

if firstInstall {
    // killall + gpg-agent --daemon (today's existing logic).
}
return nil
```

Loose regex match (any `pinentry-program …` line). `brewPrefix` may have changed since the last install — the regex isn't anchored to a specific prefix, so it still matches.

**gpg-agent restart trigger:** runs only on first install. Changing the `pinentry-program` path in-place does not require restarting `gpg-agent`; the agent re-reads `pinentry-program` on the next pinentry invocation. Avoiding the restart on subsequent runs keeps `setup` idempotent (no killing of a running agent when nothing materially changed).

## 9. Failure modes

| Situation | Behavior |
|---|---|
| File missing | Create fresh with just the wrapped block; perm `0o644` |
| Both markers present, well-formed, content unchanged | No-op (no write, no mtime bump) |
| Both markers present, well-formed, content changed | Atomic rewrite between markers |
| Markers absent + regex match | Replace first match with wrapped block |
| Markers absent + no regex match | Append wrapped block at EOF |
| Only `BeginMarker` present | Error: `"<path>: malformed managed block: found BeginMarker without EndMarker; resolve manually"` |
| Only `EndMarker` present | Error: `"<path>: malformed managed block: found EndMarker without BeginMarker; resolve manually"` |
| Duplicate markers (either side) | Error: `"<path>: malformed managed block: <BeginMarker\|EndMarker> appears N times; resolve manually"` |
| Markers in wrong order (`End` before `Begin`) | Error: `"<path>: malformed managed block: EndMarker appears before BeginMarker; resolve manually"` |
| Tempfile creation fails | Wrapped error, no destination change |
| Rename fails | Wrapped error including tempfile path; destination unchanged; tempfile left for diagnosis |

All errors propagate up through the configgen function and surface as a `setup` command failure.

## 10. Package layout

```
internal/managedblock/
├── managedblock.go        # Upsert + marker constants + algorithm
└── managedblock_test.go   # ~12 unit tests (see §11)
```

Single-file package; no sub-packages, no fake/mock helpers (no interface to mock — `Upsert` is a pure file operation).

Edits to existing files:

| File | Change |
|---|---|
| `internal/configgen/zshrc.go` | Drop `os.Stat` early-return wrapping the whole function; gate only the user-exports write on file absence; always-run `managedblock.Upsert` for the source line. |
| `internal/configgen/gitconfig.go` | Same shape: gate the identity/url/commit write on file absence; always-run `managedblock.Upsert` for the `[include]` block. |
| `internal/configgen/gpg.go` | Gate `os.MkdirAll` + `killall` + `gpg-agent --daemon` on file absence; always-run `managedblock.Upsert` for the pinentry line. |

## 11. Testing

### 11.1 `internal/managedblock/managedblock_test.go` — 12 unit tests

1. `TestUpsert_FreshFile` — file missing → creates file containing just the wrapped block; perm `0o644`.
2. `TestUpsert_IdempotentNoOp` — markers present, content unchanged → no write (assert mtime preserved via `os.Stat` before/after).
3. `TestUpsert_RewriteBetweenMarkers` — markers present, payload changed → splice replaces the inter-marker region; surrounding content intact.
4. `TestUpsert_MigrationRegexMatches` — no markers, `priorLine` matches → match replaced with wrapped block.
5. `TestUpsert_MigrationRegexNoMatch` — no markers, `priorLine` doesn't match → block appended at EOF.
6. `TestUpsert_MigrationFileNoTrailingNewline` — append-at-EOF adds a leading `\n` if the file doesn't already end in one.
7. `TestUpsert_MigrationRegexMatchesMultiple` — only first match replaced; later matches preserved verbatim.
8. `TestUpsert_MalformedOnlyBeginMarker` — error contains "found BeginMarker without EndMarker".
9. `TestUpsert_MalformedOnlyEndMarker` — error contains "found EndMarker without BeginMarker".
10. `TestUpsert_MalformedMarkersWrongOrder` — error contains "EndMarker appears before BeginMarker".
11. `TestUpsert_MalformedDuplicateMarkers` — error contains "appears N times".
12. `TestUpsert_PreservesExistingPermissions` — file with `0o600` stays `0o600` after rewrite.

### 11.2 Existing configgen tests

Each of `zshrc_test.go`, `gitconfig_test.go`, `gpg_test.go` gets one new test covering "re-run updates the managed block":

- Set up the file with the pre-managed-block content (matches the current write shape).
- Call `SetupX` once. Assert markers now present + content correct.
- Call `SetupX` again with a different payload (e.g., mock the in-repo path constant). Assert the inter-marker region updates while phase-1 content (exports / identity / mkdir) is untouched.

Existing tests for first-install behavior remain valid (they assert the post-write file contents, which now also include the managed block — the assertions need a small update to expect the marker lines).

## 12. Out-of-scope considerations

- **Concurrent runs** of `melvin-config setup` are not a real concern — single user, single machine. `os.Rename` is atomic on POSIX, so the worst case under racing runs is one of them wins; no partial-write corruption.
- **Symlinked target paths:** `Upsert` does not follow symlinks. Go's `os.Rename` operates on the link entry, not its target; renaming over a symlink would break the link. All three current callers (`SetupZshrc`, `SetupGitconfig`, `SetupGpg`) operate on regular files materialized by `configgen` itself, not symlinks. If a future caller needs symlink-following, it should `filepath.EvalSymlinks` the path before calling `Upsert`.
- **Permission preservation** uses the *existing* file's mode bits. If a user has chmodded their `~/.zshrc` to something nonstandard, we keep that. Fresh files use `0o644`. (`gpg-agent.conf` lives under `~/.gnupg/` which is `0o700`; the file itself is `0o644` per phase-1's existing behavior, unchanged here.)
