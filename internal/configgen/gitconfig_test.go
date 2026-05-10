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

// TestSetupGitconfig_PersonalContent — PERSONAL_COMPUTER=true branch
// writes the personal email + signingkey + gpgsign=true.
func TestSetupGitconfig_PersonalContent(t *testing.T) {
	tmp := t.TempDir()

	err := SetupGitconfig(tmp, "/irrelevant/configdir", true, false, dryrun.NewNullReporter())
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(tmp, ".gitconfig"))
	require.NoError(t, err)

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
}

// TestSetupGitconfig_WorkContent — personalComputer=false branch
// writes the work email + commented signingkey + gpgsign=false.
func TestSetupGitconfig_WorkContent(t *testing.T) {
	tmp := t.TempDir()

	err := SetupGitconfig(tmp, "/irrelevant/configdir", false, false, dryrun.NewNullReporter())
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(tmp, ".gitconfig"))
	require.NoError(t, err)

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
}

// TestSetupGitconfig_FirstInstallGuard — when the file pre-exists,
// the first-install identity/url/commit write is skipped, but the
// every-run upsert appends the managed [include] block at EOF.
func TestSetupGitconfig_FirstInstallGuard(t *testing.T) {
	tmp := t.TempDir()
	preExisting := "# user-edited gitconfig\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmp, ".gitconfig"), []byte(preExisting), 0o644))

	err := SetupGitconfig(tmp, "/x", true, false, dryrun.NewNullReporter())
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(tmp, ".gitconfig"))
	require.NoError(t, err)
	// First-install step skipped: no [user] / [commit] added.
	assert.NotContains(t, string(got), "[user]")
	assert.NotContains(t, string(got), "[commit]")
	// Every-run step always executes: managed block appended at EOF.
	expected := preExisting +
		"# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"[include]\n\tpath = \"" + tmp + "/.melvin/config/shared_config/.gitconfig\"\n" +
		"# <<< melvin-config managed <<<\n"
	assert.Equal(t, expected, string(got))
}

// TestSetupGitconfig_RerunUpdatesManagedBlock — when the file
// already has a managed block whose [include] path has drifted, the
// fresh SetupGitconfig rewrites the inter-marker region while
// leaving the [user] / [commit] blocks untouched (the first-install
// identity write is skipped because the file exists).
func TestSetupGitconfig_RerunUpdatesManagedBlock(t *testing.T) {
	tmp := t.TempDir()
	preExisting := "[user]\n\temail = pinned@example.com\n\n" +
		"# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"[include]\n\tpath = \"" + tmp + "/.melvin/config/OLD/.gitconfig\"\n" +
		"# <<< melvin-config managed <<<\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmp, ".gitconfig"), []byte(preExisting), 0o644))

	err := SetupGitconfig(tmp, "/irrelevant", true, false, dryrun.NewNullReporter())
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(tmp, ".gitconfig"))
	require.NoError(t, err)
	// First-install step skipped: existing identity preserved.
	assert.Contains(t, string(got), "email = pinned@example.com")
	assert.NotContains(t, string(got), "noreply@melvin.la")
	// Every-run step executed: managed block now points at shared_config.
	assert.Contains(t, string(got),
		"path = \""+tmp+"/.melvin/config/shared_config/.gitconfig\"")
	assert.NotContains(t, string(got), "OLD/.gitconfig")
}

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

	err := SetupGitconfig(tmp, "/irrelevant", true, false, dryrun.NewNullReporter())
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

// TestSetupGitconfigDryRun_FreshInstall — file absent: dry-run reports
// a single FileChange containing both identity block and managed-block
// [include] section. No file is written to disk.
func TestSetupGitconfigDryRun_FreshInstall(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, ".gitconfig")

	var buf bytes.Buffer
	reporter := dryrun.NewReporter(&buf)
	err := SetupGitconfig(tmp, "/irrelevant", true, true, reporter)
	require.NoError(t, err)

	// No file written.
	_, statErr := os.Stat(target)
	require.True(t, os.IsNotExist(statErr), "file must not be written in dry-run mode")

	out := buf.String()
	assert.Contains(t, out, "would create gitconfig (fresh install)")
	assert.Contains(t, out, "[user]")
	assert.Contains(t, out, "melvin-config managed")
}

// TestSetupGitconfigDryRun_Migration — pre-marker file (bare [include]
// block): dry-run reports a single FileChange wrapping the block in
// markers. No file is written.
func TestSetupGitconfigDryRun_Migration(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, ".gitconfig")
	preExisting := "[include]\n\tpath = \"" + tmp + "/.melvin/config/shared_config/.gitconfig\"" +
		"\n\n[user]\n\temail = pinned@example.com"
	require.NoError(t, os.WriteFile(target, []byte(preExisting), 0o644))

	origMtime, err := os.Stat(target)
	require.NoError(t, err)

	var buf bytes.Buffer
	reporter := dryrun.NewReporter(&buf)
	err = SetupGitconfig(tmp, tmp, true, true, reporter)
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
	assert.Contains(t, out, "would update gitconfig")
	assert.Contains(t, out, "melvin-config managed")
}

// TestSetupGitconfigDryRun_InSync — file already has the current
// managed block shape: dry-run reports FileNoOp. No file is written.
func TestSetupGitconfigDryRun_InSync(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, ".gitconfig")
	// Pre-populate with the exact output SetupGitconfig would produce.
	preExisting := "[user]\n\temail = noreply@melvin.la" +
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
	require.NoError(t, os.WriteFile(target, []byte(preExisting), 0o644))

	var buf bytes.Buffer
	reporter := dryrun.NewReporter(&buf)
	err := SetupGitconfig(tmp, tmp, true, true, reporter)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "gitconfig in sync")
	assert.NotContains(t, out, "would")
}
