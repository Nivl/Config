package configgen

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/Nivl/config/internal/configgen/configgentest"
	"github.com/Nivl/config/internal/dryrun"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// fakeExitError wraps an integer exit code in a way that
// errors.As(*exec.ExitError) extracts it. exec.Cmd actually returns
// *exec.ExitError, which embeds *os.ProcessState — we can't construct
// one of those by hand, so the tests below use real exec.Command on
// /bin/false (exit 1) and /bin/sh -c "exit 2" (exit 2) to produce
// genuine *exec.ExitError values with the desired ExitCode.
func failWithExitCode(t *testing.T, code int) error {
	t.Helper()
	err := exec.CommandContext(t.Context(), "/bin/sh", "-c", "exit "+strconv.Itoa(code)).Run()
	require.Error(t, err)
	return err
}

// TestSetupGpg_FreshWrite — happy path: target doesn't exist, ~/.gnupg
// created with 0o700, conf file written with pinentry-program line,
// killall + daemon called in order.
func TestSetupGpg_FreshWrite(t *testing.T) {
	tmp := t.TempDir()
	runner := configgentest.NewFakeCmdRunner()
	runner.On("Run", mock.Anything, "killall", "gpg-agent").Return(nil)
	runner.On("Run", mock.Anything, "gpg-agent", "--daemon").Return(nil)

	err := SetupGpg(context.Background(), tmp, "/opt/homebrew", runner, false, dryrun.NewNullReporter())
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(tmp, ".gnupg"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())

	got, err := os.ReadFile(filepath.Join(tmp, ".gnupg", "gpg-agent.conf"))
	require.NoError(t, err)
	expected := "# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"pinentry-program /opt/homebrew/bin/pinentry-mac\n" +
		"# <<< melvin-config managed <<<\n"
	assert.Equal(t, expected, string(got))

	runner.AssertExpectations(t)
}

// TestSetupGpg_FirstInstallGuard — when the conf file pre-exists,
// no mkdir + no killall/daemon shellouts. The managed block IS
// upserted (replacing the existing pinentry-program line in place).
func TestSetupGpg_FirstInstallGuard(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, ".gnupg"), 0o700))
	preExisting := "pinentry-program /old/path\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmp, ".gnupg", "gpg-agent.conf"), []byte(preExisting), 0o644))
	runner := configgentest.NewFakeCmdRunner() // no expectations: no Run calls allowed

	err := SetupGpg(context.Background(), tmp, "/opt/homebrew", runner, false, dryrun.NewNullReporter())
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

// TestSetupGpg_KillallExitCode1Ignored — when killall returns
// *exec.ExitError with ExitCode==1 ("no such process"), the error is
// swallowed and daemon launch proceeds. Fixes the latent bash bug
// under set -e (Q4-B).
func TestSetupGpg_KillallExitCode1Ignored(t *testing.T) {
	tmp := t.TempDir()
	exit1 := failWithExitCode(t, 1)
	runner := configgentest.NewFakeCmdRunner()
	runner.On("Run", mock.Anything, "killall", "gpg-agent").Return(exit1)
	runner.On("Run", mock.Anything, "gpg-agent", "--daemon").Return(nil)

	err := SetupGpg(context.Background(), tmp, "/opt/homebrew", runner, false, dryrun.NewNullReporter())
	require.NoError(t, err)
	runner.AssertExpectations(t)
}

// TestSetupGpg_KillallExitCodeOtherPropagates — exit codes other than
// 1 propagate as errors (real failures, not "no process").
func TestSetupGpg_KillallExitCodeOtherPropagates(t *testing.T) {
	tmp := t.TempDir()
	exit2 := failWithExitCode(t, 2)
	runner := configgentest.NewFakeCmdRunner()
	runner.On("Run", mock.Anything, "killall", "gpg-agent").Return(exit2)
	// daemon should NOT be called when killall returns a real error.

	err := SetupGpg(context.Background(), tmp, "/opt/homebrew", runner, false, dryrun.NewNullReporter())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "killall gpg-agent")
	runner.AssertNotCalled(t, "Run", mock.Anything, "gpg-agent", "--daemon")
}

// TestSetupGpg_KillallNonExitErrorPropagates — non-ExitError errors
// (e.g. binary missing) propagate.
func TestSetupGpg_KillallNonExitErrorPropagates(t *testing.T) {
	tmp := t.TempDir()
	runner := configgentest.NewFakeCmdRunner()
	runner.On("Run", mock.Anything, "killall", "gpg-agent").Return(errors.New("binary missing"))

	err := SetupGpg(context.Background(), tmp, "/opt/homebrew", runner, false, dryrun.NewNullReporter())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "binary missing")
}

// TestSetupGpg_DaemonLaunchErrorPropagates — daemon failure surfaces
// as an error.
func TestSetupGpg_DaemonLaunchErrorPropagates(t *testing.T) {
	tmp := t.TempDir()
	runner := configgentest.NewFakeCmdRunner()
	runner.On("Run", mock.Anything, "killall", "gpg-agent").Return(nil)
	runner.On("Run", mock.Anything, "gpg-agent", "--daemon").Return(errors.New("daemon failed"))

	err := SetupGpg(context.Background(), tmp, "/opt/homebrew", runner, false, dryrun.NewNullReporter())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gpg-agent --daemon")
	assert.Contains(t, err.Error(), "daemon failed")
}

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

	err := SetupGpg(context.Background(), tmp, "/opt/homebrew", runner, false, dryrun.NewNullReporter())
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(tmp, ".gnupg", "gpg-agent.conf"))
	require.NoError(t, err)
	assert.Contains(t, string(got), "pinentry-program /opt/homebrew/bin/pinentry-mac")
	assert.NotContains(t, string(got), "/old/prefix/")
	runner.AssertNotCalled(t, "Run", mock.Anything, "killall", mock.Anything)
	runner.AssertNotCalled(t, "Run", mock.Anything, "gpg-agent", mock.Anything)
}

// TestSetupGpgDryRun_FreshInstall — conf absent: dry-run reports a
// mkdir FileChange, a create FileChange for the conf file, and two
// Shellout calls (killall + daemon). No file is written to disk.
func TestSetupGpgDryRun_FreshInstall(t *testing.T) {
	tmp := t.TempDir()
	conf := filepath.Join(tmp, ".gnupg", "gpg-agent.conf")
	runner := configgentest.NewFakeCmdRunner() // no Run calls in dry-run mode

	var buf bytes.Buffer
	reporter := dryrun.NewReporter(&buf)
	err := SetupGpg(context.Background(), tmp, "/opt/homebrew", runner, true, reporter)
	require.NoError(t, err)

	// No file written.
	_, statErr := os.Stat(conf)
	require.True(t, os.IsNotExist(statErr), "conf must not be written in dry-run mode")

	out := buf.String()
	// mkdir reported.
	assert.Contains(t, out, "would mkdir 0o700")
	// conf create reported.
	assert.Contains(t, out, "would create gpg-agent.conf (fresh install)")
	assert.Contains(t, out, "pinentry-program /opt/homebrew/bin/pinentry-mac")
	// Both shellouts reported.
	assert.Contains(t, out, "would run: killall gpg-agent")
	assert.Contains(t, out, "would run: gpg-agent --daemon")

	runner.AssertNotCalled(t, "Run", mock.Anything, mock.Anything, mock.Anything)
}

// TestSetupGpgDryRun_Migration — pre-marker file (bare pinentry-program
// line): dry-run reports a single FileChange. No file is written.
func TestSetupGpgDryRun_Migration(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, ".gnupg"), 0o700))
	conf := filepath.Join(tmp, ".gnupg", "gpg-agent.conf")
	preExisting := "pinentry-program /old/path\n"
	require.NoError(t, os.WriteFile(conf, []byte(preExisting), 0o644))

	origMtime, err := os.Stat(conf)
	require.NoError(t, err)

	runner := configgentest.NewFakeCmdRunner() // no Run calls in dry-run mode

	var buf bytes.Buffer
	reporter := dryrun.NewReporter(&buf)
	err = SetupGpg(context.Background(), tmp, "/opt/homebrew", runner, true, reporter)
	require.NoError(t, err)

	// File unchanged on disk.
	got, err := os.ReadFile(conf)
	require.NoError(t, err)
	assert.Equal(t, preExisting, string(got))

	// Mtime not changed.
	afterMtime, err := os.Stat(conf)
	require.NoError(t, err)
	assert.Equal(t, origMtime.ModTime(), afterMtime.ModTime())

	out := buf.String()
	assert.Contains(t, out, "would update gpg-agent.conf")
	assert.Contains(t, out, "melvin-config managed")

	runner.AssertNotCalled(t, "Run", mock.Anything, mock.Anything, mock.Anything)
}

// TestSetupGpgDryRun_InSync — conf already has the current managed
// block shape: dry-run reports FileNoOp. No file is written.
func TestSetupGpgDryRun_InSync(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, ".gnupg"), 0o700))
	conf := filepath.Join(tmp, ".gnupg", "gpg-agent.conf")
	// Pre-populate with the exact output SetupGpg would produce.
	preExisting := "# >>> melvin-config managed >>>\n" +
		"# Do not edit this block by hand — it is rewritten by `melvin-config setup`.\n" +
		"pinentry-program /opt/homebrew/bin/pinentry-mac\n" +
		"# <<< melvin-config managed <<<\n"
	require.NoError(t, os.WriteFile(conf, []byte(preExisting), 0o644))

	runner := configgentest.NewFakeCmdRunner() // no Run calls in dry-run mode

	var buf bytes.Buffer
	reporter := dryrun.NewReporter(&buf)
	err := SetupGpg(context.Background(), tmp, "/opt/homebrew", runner, true, reporter)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "gpg-agent.conf in sync")
	assert.NotContains(t, out, "would")

	runner.AssertNotCalled(t, "Run", mock.Anything, mock.Anything, mock.Anything)
}
