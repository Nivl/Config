package managedblock

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Nivl/config/internal/dryrun"
)

// TestUpsert_FreshFile — when the target file does not exist, Upsert
// creates it containing just the wrapped block (markers + notice +
// payload + closing marker + trailing newline), perm 0o644.
func TestUpsert_FreshFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "file")

	err := Upsert(path, "payload-line", nil, UpsertOpts{Reporter: dryrun.NewNullReporter()})
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

// TestUpsert_IdempotentNoOp — markers present and payload unchanged,
// Upsert returns nil without writing. Verified by byte equality of
// the file content AND mtime preservation: content equality alone
// would pass even if the file were rewritten with identical bytes;
// mtime equality proves no rename happened (writeAtomic uses
// os.Rename, which always bumps mtime).
func TestUpsert_IdempotentNoOp(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "file")
	require.NoError(t, Upsert(path, "stable-payload", nil, UpsertOpts{Reporter: dryrun.NewNullReporter()}))

	beforeBytes, err := os.ReadFile(path)
	require.NoError(t, err)
	beforeStat, err := os.Stat(path)
	require.NoError(t, err)

	require.NoError(t, Upsert(path, "stable-payload", nil, UpsertOpts{Reporter: dryrun.NewNullReporter()}))

	afterBytes, err := os.ReadFile(path)
	require.NoError(t, err)
	afterStat, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, beforeBytes, afterBytes,
		"second identical Upsert must not rewrite the file content")
	assert.Equal(t, beforeStat.ModTime(), afterStat.ModTime(),
		"second identical Upsert must not touch the file's mtime")
}

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

	require.NoError(t, Upsert(path, "new-payload", nil, UpsertOpts{Reporter: dryrun.NewNullReporter()}))

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
	require.NoError(t, Upsert(path, "new-payload", priorLine, UpsertOpts{Reporter: dryrun.NewNullReporter()}))

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

// TestUpsert_MigrationRegexNoMatch — file has no markers, priorLine
// doesn't match anything; the wrapped block is appended at EOF.
func TestUpsert_MigrationRegexNoMatch(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "file")
	initial := "header-line\n" +
		"footer-line\n"
	require.NoError(t, os.WriteFile(path, []byte(initial), 0o644))

	priorLine := regexp.MustCompile(`(?m)^STALE-LINE$`)
	require.NoError(t, Upsert(path, "appended-payload", priorLine, UpsertOpts{Reporter: dryrun.NewNullReporter()}))

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

// TestUpsert_MigrationFileNoTrailingNewline — when the existing file
// doesn't end in '\n' and there are no markers + no regex match,
// append-at-EOF adds a leading '\n' so the wrapped block starts on a
// new line.
func TestUpsert_MigrationFileNoTrailingNewline(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "file")
	require.NoError(t, os.WriteFile(path, []byte("no-trailing-newline"), 0o644))

	require.NoError(t, Upsert(path, "payload", nil, UpsertOpts{Reporter: dryrun.NewNullReporter()}))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	want := "no-trailing-newline\n" +
		"# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"payload\n" +
		"# <<< melvin-config managed <<<\n"
	assert.Equal(t, want, string(got))
}

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
	require.NoError(t, Upsert(path, "payload", priorLine, UpsertOpts{Reporter: dryrun.NewNullReporter()}))

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
			errSubstr: "BeginMarker appears 2 time(s)",
		},
		{
			name: "duplicate end",
			content: "# >>> melvin-config managed >>>\nA\n" +
				"# <<< melvin-config managed <<<\n" +
				"# <<< melvin-config managed <<<\n",
			errSubstr: "EndMarker appears 2 time(s)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			path := filepath.Join(tmp, "file")
			require.NoError(t, os.WriteFile(path, []byte(tc.content), 0o644))

			err := Upsert(path, "irrelevant", nil, UpsertOpts{Reporter: dryrun.NewNullReporter()})
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

// TestUpsert_PreservesExistingPermissions — when the file already
// exists with non-default perms, the rewrite preserves them.
// TestUpsert_RenameFailureCleansUpTempfile — when writeAtomic fails
// at the rename step (here forced by aiming at a non-empty directory),
// the sibling tempfile must be removed so the user doesn't end up
// with orphan .tmp.XXXX files next to their dotfiles.
func TestUpsert_RenameFailureCleansUpTempfile(t *testing.T) {
	tmp := t.TempDir()
	// Make the "target" path actually be a non-empty directory so
	// os.Rename can't replace it on either macOS or Linux.
	target := filepath.Join(tmp, "target")
	require.NoError(t, os.MkdirAll(target, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, "child"), []byte("x"), 0o644))

	err := Upsert(target, "payload", nil, UpsertOpts{Reporter: dryrun.NewNullReporter()})
	require.Error(t, err, "Upsert must fail when target is a non-empty directory")

	matches, globErr := filepath.Glob(filepath.Join(tmp, "target.tmp.*"))
	require.NoError(t, globErr)
	assert.Empty(t, matches, "writeAtomic must remove its tempfile on rename failure (found %v)", matches)
}

func TestUpsert_PreservesExistingPermissions(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "file")
	initial := "# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"old-payload\n" +
		"# <<< melvin-config managed <<<\n"
	require.NoError(t, os.WriteFile(path, []byte(initial), 0o600))

	require.NoError(t, Upsert(path, "new-payload", nil, UpsertOpts{Reporter: dryrun.NewNullReporter()}))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

// fakeReporter records calls for assertion. Embeds NullReporter so
// methods we don't override stay silent.
type fakeReporter struct {
	dryrun.Reporter

	fileChangeCalls []fakeFileChange
	fileNoOpCalls   []fakeFileNoOp
}

// fakeFileChange holds the arguments of a single FileChange call.
type fakeFileChange struct {
	target  string
	before  []byte
	after   []byte
	summary string
}

// fakeFileNoOp holds the arguments of a single FileNoOp call.
type fakeFileNoOp struct {
	target  string
	summary string
}

// FileChange records the call.
func (f *fakeReporter) FileChange(target string, before, after []byte, summary string) {
	f.fileChangeCalls = append(f.fileChangeCalls,
		fakeFileChange{target: target, before: before, after: after, summary: summary})
}

// FileNoOp records the call.
func (f *fakeReporter) FileNoOp(target, summary string) {
	f.fileNoOpCalls = append(f.fileNoOpCalls,
		fakeFileNoOp{target: target, summary: summary})
}

// TestUpsert_DryRunSkipsWriteReportsChange — when DryRun=true and
// the resulting content differs from existing, Upsert calls
// reporter.FileChange and does NOT write the file.
func TestUpsert_DryRunSkipsWriteReportsChange(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "file")
	require.NoError(t, os.WriteFile(path, []byte("old\n"), 0o644))

	rep := &fakeReporter{Reporter: dryrun.NewNullReporter()}
	err := Upsert(path, "new payload", nil, UpsertOpts{
		DryRun:   true,
		Reporter: rep,
	})
	require.NoError(t, err)

	// File is unchanged on disk.
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "old\n", string(got))

	// Reporter received the FileChange call.
	require.Len(t, rep.fileChangeCalls, 1)
	assert.Equal(t, path, rep.fileChangeCalls[0].target)
	assert.Equal(t, []byte("old\n"), rep.fileChangeCalls[0].before)
	assert.Empty(t, rep.fileNoOpCalls)
}

// TestUpsert_DryRunFreshFileReportsChange — when DryRun=true and the
// file does not exist, Upsert calls reporter.FileChange with a nil
// before-slice and does NOT create the file. This pins the fresh-write
// branch (ErrNotExist) under dry-run.
func TestUpsert_DryRunFreshFileReportsChange(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "file")
	// File intentionally absent.

	rep := &fakeReporter{Reporter: dryrun.NewNullReporter()}
	err := Upsert(path, "payload", nil, UpsertOpts{
		DryRun:   true,
		Reporter: rep,
	})
	require.NoError(t, err)

	// File must not have been created.
	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr), "file must not be created in dry-run")

	// Reporter received FileChange with nil before.
	require.Len(t, rep.fileChangeCalls, 1)
	assert.Equal(t, path, rep.fileChangeCalls[0].target)
	assert.Nil(t, rep.fileChangeCalls[0].before)
	assert.Empty(t, rep.fileNoOpCalls)
}

// TestUpsert_DryRunNoOpReportsFileNoOp — when DryRun=true and the
// resulting content equals existing content, Upsert calls
// reporter.FileNoOp and skips the write.
func TestUpsert_DryRunNoOpReportsFileNoOp(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "file")
	// Seed the file with content that already matches the would-be
	// upsert output (just the wrapped block).
	require.NoError(t, Upsert(path, "p", nil, UpsertOpts{Reporter: dryrun.NewNullReporter()}))

	rep := &fakeReporter{Reporter: dryrun.NewNullReporter()}
	err := Upsert(path, "p", nil, UpsertOpts{
		DryRun:   true,
		Reporter: rep,
	})
	require.NoError(t, err)

	require.Len(t, rep.fileNoOpCalls, 1)
	assert.Equal(t, path, rep.fileNoOpCalls[0].target)
	assert.Empty(t, rep.fileChangeCalls)
}

// TestUpsert_DryRunMalformedStillErrors — malformed markers return
// an error regardless of dry-run.
func TestUpsert_DryRunMalformedStillErrors(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "file")
	require.NoError(t, os.WriteFile(path,
		[]byte("# >>> melvin-config managed >>>\nbody\n"), 0o644))

	rep := &fakeReporter{Reporter: dryrun.NewNullReporter()}
	err := Upsert(path, "p", nil, UpsertOpts{
		DryRun:   true,
		Reporter: rep,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "found BeginMarker without EndMarker")
	assert.Empty(t, rep.fileChangeCalls)
	assert.Empty(t, rep.fileNoOpCalls)
}
