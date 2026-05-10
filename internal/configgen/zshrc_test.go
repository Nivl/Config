package configgen

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nivl/config/internal/dryrun"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetupZshrc_FreshWrite — happy path: target doesn't exist, the
// file gets the 7 exports + source line in a fixed order, with a
// leading newline.
func TestSetupZshrc_FreshWrite(t *testing.T) {
	tmp := t.TempDir()
	opts := ZshrcOpts{
		PersonalComputer: "true",
		DevRoot:          "/home/user/dev",
		GitHost:          "git@github.com",
		GitCloneUserName: "Nivl",
	}

	err := SetupZshrc(tmp, "/irrelevant/configdir", opts, false, dryrun.NewNullReporter())
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(tmp, ".zshrc"))
	require.NoError(t, err)

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
}

// TestSetupZshrc_FirstInstallGuard — when the file pre-exists, the
// seven exports are NOT added (first-install step skipped), but the
// managed block IS appended at EOF (every-run step always executes).
func TestSetupZshrc_FirstInstallGuard(t *testing.T) {
	tmp := t.TempDir()
	preExisting := "# user-edited zshrc\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmp, ".zshrc"), []byte(preExisting), 0o644))

	err := SetupZshrc(tmp, "/x", ZshrcOpts{DevRoot: "/home/u/dev"}, false, dryrun.NewNullReporter())
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(tmp, ".zshrc"))
	require.NoError(t, err)
	// First-install step skipped: no export lines added.
	assert.NotContains(t, string(got), "export DEV_ROOT=")
	// Every-run step always executes: managed block appended at EOF
	// (regex doesn't match the pre-existing comment, so append-at-EOF).
	expected := preExisting +
		"# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"source \"$HOME/.melvin/config/shared_config/base.zshrc\"\n" +
		"# <<< melvin-config managed <<<\n"
	assert.Equal(t, expected, string(got))
}

// TestSetupZshrc_PersonalComputerPassthrough — non-canonical
// PERSONAL_COMPUTER values pass through verbatim. The resulting
// export line embeds whatever the caller supplied.
func TestSetupZshrc_PersonalComputerPassthrough(t *testing.T) {
	tmp := t.TempDir()
	err := SetupZshrc(tmp, "/x", ZshrcOpts{
		PersonalComputer: "garbage",
		DevRoot:          "/d",
		GitHost:          "g",
		GitCloneUserName: "u",
	}, false, dryrun.NewNullReporter())
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(tmp, ".zshrc"))
	require.NoError(t, err)
	assert.Contains(t, string(got), "export PERSONAL_COMPUTER=\"garbage\"")
}

// TestSetupZshrc_DevRootDerivedRoots — WORKTREES_ROOT/REPOS_ROOT/
// SDKS_ROOT are derived as DevRoot + "/worktrees"|"/repos"|"/sdks".
func TestSetupZshrc_DevRootDerivedRoots(t *testing.T) {
	tmp := t.TempDir()
	err := SetupZshrc(tmp, "/x", ZshrcOpts{
		PersonalComputer: "true",
		DevRoot:          "/opt/work",
		GitHost:          "g",
		GitCloneUserName: "u",
	}, false, dryrun.NewNullReporter())
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(tmp, ".zshrc"))
	require.NoError(t, err)
	assert.Contains(t, string(got), "export WORKTREES_ROOT=\"/opt/work/worktrees\"")
	assert.Contains(t, string(got), "export REPOS_ROOT=\"/opt/work/repos\"")
	assert.Contains(t, string(got), "export SDKS_ROOT=\"/opt/work/sdks\"")
}

// TestSetupZshrc_RerunUpdatesManagedBlock — when the file already
// exists with a managed block whose payload has drifted (e.g. the
// source path was changed by a previous build of the code), a fresh
// SetupZshrc rewrites the inter-marker region. The first-install
// user-exports write is skipped because the file exists.
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
	}, false, dryrun.NewNullReporter())
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(tmp, ".zshrc"))
	require.NoError(t, err)
	// First-install step must NOT run: existing user-edited content stays as-is.
	assert.Contains(t, string(got), "# user header\n")
	assert.Contains(t, string(got), "export GIT_HOST=\"frozen\"")
	assert.NotContains(t, string(got), "export DEV_ROOT=") // would only exist if the first-install step ran
	// Every-run step must execute: the managed block now points at shared_config.
	assert.Contains(t, string(got), "source \"$HOME/.melvin/config/shared_config/base.zshrc\"")
	assert.NotContains(t, string(got), "OLD/base.zshrc")
}

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

	err := SetupZshrc(tmp, "/irrelevant", ZshrcOpts{}, false, dryrun.NewNullReporter())
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

// TestSetupZshrcDryRun_FreshInstall — file absent: dry-run reports a
// single FileChange containing both the export lines and the wrapped
// managed block. No file is written to disk.
func TestSetupZshrcDryRun_FreshInstall(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, ".zshrc")
	opts := ZshrcOpts{
		PersonalComputer: "true",
		DevRoot:          "/home/user/dev",
		GitHost:          "git@github.com",
		GitCloneUserName: "Nivl",
	}

	var buf bytes.Buffer
	reporter := dryrun.NewReporter(&buf)
	err := SetupZshrc(tmp, "/irrelevant", opts, true, reporter)
	require.NoError(t, err)

	// No file written.
	_, statErr := os.Stat(target)
	require.True(t, os.IsNotExist(statErr), "file must not be written in dry-run mode")

	out := buf.String()
	// One FileChange reported.
	assert.Contains(t, out, "would create zshrc (fresh install)")
	// The diff shows both exports and the managed block.
	assert.Contains(t, out, "export GIT_HOST=")
	assert.Contains(t, out, "melvin-config managed")
}

// TestSetupZshrcDryRun_Migration — pre-marker file (bare source line):
// dry-run reports a single FileChange replacing the line with the
// wrapped block. No file is written.
func TestSetupZshrcDryRun_Migration(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, ".zshrc")
	preExisting := "\nexport GIT_HOST=\"g\"" +
		"\nexport DEV_ROOT=\"/d\"" +
		"\nsource \"$HOME/.melvin/config/shared_config/base.zshrc\"\n"
	require.NoError(t, os.WriteFile(target, []byte(preExisting), 0o644))

	origMtime, err := os.Stat(target)
	require.NoError(t, err)

	var buf bytes.Buffer
	reporter := dryrun.NewReporter(&buf)
	err = SetupZshrc(tmp, "/irrelevant", ZshrcOpts{}, true, reporter)
	require.NoError(t, err)

	// File unchanged on disk.
	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, preExisting, string(got))

	// Mtime not changed.
	afterMtime, err := os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, origMtime.ModTime(), afterMtime.ModTime())

	out := buf.String()
	assert.Contains(t, out, "would update zshrc")
	assert.Contains(t, out, "melvin-config managed")
}

// TestSetupZshrcDryRun_InSync — file already has the current managed
// block shape: dry-run reports FileNoOp. No file is written.
func TestSetupZshrcDryRun_InSync(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, ".zshrc")
	// Pre-populate with the exact output SetupZshrc would produce.
	preExisting := "\nexport GIT_HOST=\"g\"" +
		"\nexport GIT_CLONE_USER_NAME=\"u\"" +
		"\nexport PERSONAL_COMPUTER=\"true\"" +
		"\nexport DEV_ROOT=\"/d\"" +
		"\nexport WORKTREES_ROOT=\"/d/worktrees\"" +
		"\nexport REPOS_ROOT=\"/d/repos\"" +
		"\nexport SDKS_ROOT=\"/d/sdks\"\n" +
		"# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"source \"$HOME/.melvin/config/shared_config/base.zshrc\"\n" +
		"# <<< melvin-config managed <<<\n"
	require.NoError(t, os.WriteFile(target, []byte(preExisting), 0o644))

	var buf bytes.Buffer
	reporter := dryrun.NewReporter(&buf)
	err := SetupZshrc(tmp, "/irrelevant", ZshrcOpts{}, true, reporter)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "zshrc in sync")
	assert.NotContains(t, out, "would")
}
