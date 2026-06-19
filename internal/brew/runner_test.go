package brew

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Nivl/config/internal/dryrun"
	"github.com/Nivl/config/internal/iox"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestRunner_Install_NoFormulae asserts that calling Install with no
// formulae is a no-op (no `brew install` subprocess).
func TestRunner_Install_NoFormulae(t *testing.T) {
	r := NewRunner(iox.Streams{Out: io.Discard, Err: io.Discard})
	err := r.Install(context.Background())
	require.NoError(t, err)
}

// fakeBrew is a tiny helper that creates a tempdir containing an executable
// shell script named "brew" with the given body, then returns the dir. The
// caller prepends it to PATH via t.Setenv so subprocesses calling "brew"
// hit the fake. Used for one integration test per shell-out method.
func fakeBrew(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "brew")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755))
	return dir
}

// TestRunner_InstallCask_FakeBinarySubprocess verifies the real
// exec.Command path: stderr capture, exit-code-to-CaskStatus mapping,
// extractCaskFailureReason wiring. Uses a fake `brew` on PATH so we
// don't depend on a real homebrew install.
func TestRunner_InstallCask_FakeBinarySubprocess(t *testing.T) {
	body := `case "$1 $2" in
"list --cask") exit 1 ;;
"install --cask") echo "Error: synthetic failure" >&2 ; exit 1 ;;
*) echo "unexpected: $*" >&2 ; exit 99 ;;
esac`
	dir := fakeBrew(t, body)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	r := NewRunner(iox.Streams{Out: io.Discard, Err: io.Discard})
	outcome, err := r.InstallCask(context.Background(), "synthetic-cask")
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, outcome.Status)
	assert.Equal(t, "Error: synthetic failure", outcome.Reason)
}

// TestRunner_InstallCask_UpgradesInstalledQuiescentCask verifies the
// installed-and-not-running branch: the cask is upgraded with
// `brew upgrade --cask` (NOT a no-op `brew install --cask`). The app
// name returned by `brew info` is the same guaranteed-absent name
// TestRunner_IsAppRunning_NotRunning relies on, so the real
// pgrep/osascript report it not-running and the upgrade proceeds.
func TestRunner_InstallCask_UpgradesInstalledQuiescentCask(t *testing.T) {
	body := `case "$1 $2" in
"list --cask") exit 0 ;;
"info --cask") echo '{"casks":[{"artifacts":[{"app":["ThisAppDefinitelyDoesNotExist_xyzzy.app"]}]}]}' ;;
"upgrade --cask") echo "$@" > "$BREW_LOG" ; exit 0 ;;
*) echo "unexpected: $*" >&2 ; exit 99 ;;
esac`
	dir := fakeBrew(t, body)
	logFile := filepath.Join(dir, "brew.log")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BREW_LOG", logFile)

	r := NewRunner(iox.Streams{Out: io.Discard, Err: io.Discard})
	outcome, err := r.InstallCask(context.Background(), "synthetic-cask")
	require.NoError(t, err)
	assert.Equal(t, StatusInstalled, outcome.Status)
	got, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Equal(t, "upgrade --cask synthetic-cask\n", string(got))
}

// TestRunner_ListCaskApps_MalformedJSONYieldsEmptyList verifies that a
// brew `info` exit-zero but JSON-garbage body collapses to "no apps
// known" rather than a propagated parse error. A real-world brew bug
// or partial output must not abort the cask loop.
func TestRunner_ListCaskApps_MalformedJSONYieldsEmptyList(t *testing.T) {
	body := `echo "not json {{"`
	dir := fakeBrew(t, body)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	r := NewRunner(iox.Streams{Out: io.Discard, Err: io.Discard})
	apps, err := r.ListCaskApps(context.Background(), "synthetic-cask")
	require.NoError(t, err)
	assert.Nil(t, apps)
}

// TestRunner_ListCaskApps_ExitErrorYieldsEmptyList verifies that a
// non-zero `brew info` exit is treated as "no apps known" rather than
// a propagated error — a transient brew flake must not abort the
// cask loop.
func TestRunner_ListCaskApps_ExitErrorYieldsEmptyList(t *testing.T) {
	body := `echo "Error: synthetic info failure" >&2 ; exit 1`
	dir := fakeBrew(t, body)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	r := NewRunner(iox.Streams{Out: io.Discard, Err: io.Discard})
	apps, err := r.ListCaskApps(context.Background(), "synthetic-cask")
	require.NoError(t, err)
	assert.Nil(t, apps)
}

// TestRunner_InstallCask_CancelledContextReturnsError verifies that a
// cancelled context surfaces as a returned error (the "catastrophic"
// branch of the docstring), NOT as Status=Failed. Without this, a
// SIGINT during a long install would silently look like a flaky cask.
func TestRunner_InstallCask_CancelledContextReturnsError(t *testing.T) {
	body := `case "$1 $2" in
"list --cask") exit 1 ;;
"install --cask") sleep 5 ;;
*) exit 99 ;;
esac`
	dir := fakeBrew(t, body)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	r := NewRunner(iox.Streams{Out: io.Discard, Err: io.Discard})
	_, err := r.InstallCask(ctx, "synthetic-cask")
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// TestRunner_Install_FakeBinarySubprocess verifies that formulae are
// passed as positional args after "install".
func TestRunner_Install_FakeBinarySubprocess(t *testing.T) {
	body := `echo "$@" > "$BREW_LOG"
exit 0`
	dir := fakeBrew(t, body)
	logFile := filepath.Join(dir, "brew.log")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BREW_LOG", logFile)

	r := NewRunner(iox.Streams{Out: io.Discard, Err: io.Discard})
	require.NoError(t, r.Install(context.Background(), "foo", "bar", "baz"))
	got, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Equal(t, "install foo bar baz\n", string(got))
}

// TestRunner_Upgrade_FakeBinarySubprocess verifies the upgrade call.
func TestRunner_Upgrade_FakeBinarySubprocess(t *testing.T) {
	body := `echo "$@" > "$BREW_LOG"
exit 0`
	dir := fakeBrew(t, body)
	logFile := filepath.Join(dir, "brew.log")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BREW_LOG", logFile)

	r := NewRunner(iox.Streams{Out: io.Discard, Err: io.Discard})
	require.NoError(t, r.Upgrade(context.Background()))
	got, err := os.ReadFile(logFile)
	require.NoError(t, err)
	// The blanket pass is scoped to --formula so it never quits/upgrades
	// a running cask app; casks go through InstallCask instead.
	assert.Equal(t, "upgrade --formula\n", string(got))

	// Scoped form: names ride as positional args (used by the retry path).
	require.NoError(t, r.Upgrade(context.Background(), "foo", "bar"))
	got, err = os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Equal(t, "upgrade foo bar\n", string(got))
}

// TestRunner_Outdated_FakeBinarySubprocess verifies the arg shape and
// the one-name-per-line parse, including blank-line trimming.
func TestRunner_Outdated_FakeBinarySubprocess(t *testing.T) {
	body := `echo "$@" > "$BREW_LOG"
printf 'the-unarchiver\n\nzoom\n'`
	dir := fakeBrew(t, body)
	logFile := filepath.Join(dir, "brew.log")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BREW_LOG", logFile)

	r := NewRunner(iox.Streams{Out: io.Discard, Err: io.Discard})
	names, err := r.Outdated(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"the-unarchiver", "zoom"}, names)
	got, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Equal(t, "outdated --quiet\n", string(got))
}

// TestRunner_OutdatedCasks_FakeBinarySubprocess verifies OutdatedCasks
// shells to `brew outdated --quiet --cask` and parses one-name-per-line.
func TestRunner_OutdatedCasks_FakeBinarySubprocess(t *testing.T) {
	body := `echo "$@" > "$BREW_LOG"
printf 'brave-browser\n\nrectangle\n'`
	dir := fakeBrew(t, body)
	logFile := filepath.Join(dir, "brew.log")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BREW_LOG", logFile)

	r := NewRunner(iox.Streams{Out: io.Discard, Err: io.Discard})
	names, err := r.OutdatedCasks(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"brave-browser", "rectangle"}, names)
	got, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Equal(t, "outdated --quiet --cask\n", string(got))
}

// TestRunner_Outdated_MissingBrewReturnsErrNotFound pins the
// cross-package catastrophic contract: when the brew binary is absent,
// the wrapped error must satisfy errors.Is(err, exec.ErrNotFound) so
// packages.catastrophic aborts the run instead of limping through
// every package against a missing binary.
func TestRunner_Outdated_MissingBrewReturnsErrNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	r := NewRunner(iox.Streams{Out: io.Discard, Err: io.Discard})
	_, err := r.Outdated(context.Background())
	require.Error(t, err)
	require.ErrorIs(t, err, exec.ErrNotFound)
}

// TestRunner_Outdated_ExitErrorFakeBinarySubprocess pins the other
// half of the contract: an ordinary non-zero exit returns an error
// that does NOT match exec.ErrNotFound, so the packages layer records
// the UpgradeAll sentinel instead of aborting the run.
func TestRunner_Outdated_ExitErrorFakeBinarySubprocess(t *testing.T) {
	body := `echo "Error: synthetic outdated failure" >&2 ; exit 1`
	dir := fakeBrew(t, body)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	r := NewRunner(iox.Streams{Out: io.Discard, Err: io.Discard})
	_, err := r.Outdated(context.Background())
	require.Error(t, err)
	require.NotErrorIs(t, err, exec.ErrNotFound)
	assert.Contains(t, err.Error(), "brew outdated")
}

// TestRunner_IsCaskInstalled_FakeBinarySubprocess covers both branches:
// brew list --cask returns 0 (installed) and returns 1 (not installed).
func TestRunner_IsCaskInstalled_FakeBinarySubprocess(t *testing.T) {
	body := `case "$3" in
"docker") exit 0 ;;
*) exit 1 ;;
esac`
	dir := fakeBrew(t, body)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	r := NewRunner(iox.Streams{Out: io.Discard, Err: io.Discard})
	ok, err := r.IsCaskInstalled(context.Background(), "docker")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = r.IsCaskInstalled(context.Background(), "missing")
	require.NoError(t, err)
	assert.False(t, ok)
}

// TestRunner_ListCaskApps_FakeBinarySubprocess covers JSON parsing
// through the subprocess boundary.
func TestRunner_ListCaskApps_FakeBinarySubprocess(t *testing.T) {
	body := `case "$4" in
"docker") echo '{"casks":[{"artifacts":[{"app":["Docker.app"]}]}]}' ;;
*) echo '{"casks":[]}' ;;
esac`
	dir := fakeBrew(t, body)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	r := NewRunner(iox.Streams{Out: io.Discard, Err: io.Discard})
	apps, err := r.ListCaskApps(context.Background(), "docker")
	require.NoError(t, err)
	assert.Equal(t, []string{"Docker"}, apps)

	apps, err = r.ListCaskApps(context.Background(), "missing")
	require.NoError(t, err)
	assert.Nil(t, apps)
}

// fakeRunner is a minimal testify-mock-backed Runner used in
// dry-run tests. It lives here (not brewtest) to avoid an import
// cycle: brewtest imports brew, so brew_test cannot import brewtest.
type fakeRunner struct {
	mock.Mock
}

// Upgrade implements Runner via the embedded mock. Variadic args are
// bundled into a single []string slot in the mock call.
func (f *fakeRunner) Upgrade(ctx context.Context, packages ...string) error {
	return f.Called(ctx, packages).Error(0)
}

// Outdated implements Runner via the embedded mock.
func (f *fakeRunner) Outdated(ctx context.Context, _ ...string) ([]string, error) {
	args := f.Called(ctx)
	v, _ := args.Get(0).([]string)
	return v, args.Error(1)
}

// OutdatedCasks implements Runner via the embedded mock.
func (f *fakeRunner) OutdatedCasks(ctx context.Context) ([]string, error) {
	args := f.Called(ctx)
	v, _ := args.Get(0).([]string)
	return v, args.Error(1)
}

// Install implements Runner via the embedded mock.
func (f *fakeRunner) Install(ctx context.Context, formulae ...string) error {
	return f.Called(ctx, formulae).Error(0)
}

// InstallCask implements Runner via the embedded mock.
func (f *fakeRunner) InstallCask(ctx context.Context, cask string) (CaskOutcome, error) {
	args := f.Called(ctx, cask)
	v, _ := args.Get(0).(CaskOutcome)
	return v, args.Error(1)
}

// IsCaskInstalled implements Runner via the embedded mock.
func (f *fakeRunner) IsCaskInstalled(ctx context.Context, cask string) (bool, error) {
	args := f.Called(ctx, cask)
	return args.Bool(0), args.Error(1)
}

// IsAppRunning implements Runner via the embedded mock.
func (f *fakeRunner) IsAppRunning(ctx context.Context, app string) (bool, error) {
	args := f.Called(ctx, app)
	return args.Bool(0), args.Error(1)
}

// ListCaskApps implements Runner via the embedded mock.
func (f *fakeRunner) ListCaskApps(ctx context.Context, cask string) ([]string, error) {
	args := f.Called(ctx, cask)
	v, _ := args.Get(0).([]string)
	return v, args.Error(1)
}

// fakeReporter is a minimal dryrun.Reporter for brew dry-run tests.
// It records every Shellout call for assertion.
type fakeReporter struct {
	dryrun.Reporter

	shellouts []fakeShellout
}

// fakeShellout holds the command and args recorded by fakeReporter.Shellout.
type fakeShellout struct {
	command string
	args    []string
}

// Shellout records the call in the shellouts slice.
func (f *fakeReporter) Shellout(command string, args []string, _ string) {
	f.shellouts = append(f.shellouts,
		fakeShellout{command: command, args: append([]string(nil), args...)})
}

// TestDryRunRunner_WriteMethodsReportNoOp — Upgrade (bare and scoped),
// Install, and InstallCask report via the Reporter and never invoke
// the wrapped runner's writes. The scoped Upgrade form must report
// the package names it would hand to brew.
// InstallCask also reports a would-install when the cask is not yet
// present (the live path would shell out); the skip-when-running
// branch is covered separately below.
func TestDryRunRunner_WriteMethodsReportNoOp(t *testing.T) {
	inner := &fakeRunner{}
	// InstallCask consults the read methods to mirror the production
	// decision tree: an absent cask means the live path would install,
	// so the wrapper reports a would-install.
	inner.On("IsCaskInstalled", mock.Anything, "iterm2").Return(false, nil)
	rep := &fakeReporter{Reporter: dryrun.NewNullReporter()}
	r := NewDryRunWrapper(inner, rep)

	require.NoError(t, r.Upgrade(context.Background()))
	require.NoError(t, r.Upgrade(context.Background(), "foo", "bar"))
	require.NoError(t, r.Install(context.Background(), "git", "go"))
	_, err := r.InstallCask(context.Background(), "iterm2")
	require.NoError(t, err)

	require.Len(t, rep.shellouts, 4)
	assert.Equal(t, "brew", rep.shellouts[0].command)
	assert.Equal(t, []string{"upgrade", "--formula"}, rep.shellouts[0].args)
	assert.Equal(t, []string{"upgrade", "foo", "bar"}, rep.shellouts[1].args)
	assert.Equal(t, []string{"install", "git", "go"}, rep.shellouts[2].args)
	assert.Equal(t, []string{"install", "--cask", "iterm2"}, rep.shellouts[3].args)
	// Write methods on the wrapped runner are never called.
	inner.AssertNotCalled(t, "Upgrade", mock.Anything, mock.Anything)
	inner.AssertNotCalled(t, "Install", mock.Anything, mock.Anything)
	inner.AssertNotCalled(t, "InstallCask", mock.Anything, mock.Anything)
}

// TestDryRunRunner_InstallCaskSkipsWhenAppRunning — when the cask is
// already installed AND one of its apps is currently running, the
// production path returns StatusSkipped without shelling out. The
// dry-run wrapper must mirror that exactly so the preview reflects
// reality on an already-provisioned machine.
func TestDryRunRunner_InstallCaskSkipsWhenAppRunning(t *testing.T) {
	inner := &fakeRunner{}
	inner.On("IsCaskInstalled", mock.Anything, "rectangle").Return(true, nil)
	inner.On("ListCaskApps", mock.Anything, "rectangle").Return([]string{"Rectangle"}, nil)
	inner.On("IsAppRunning", mock.Anything, "Rectangle").Return(true, nil)
	rep := &fakeReporter{Reporter: dryrun.NewNullReporter()}
	r := NewDryRunWrapper(inner, rep)

	outcome, err := r.InstallCask(context.Background(), "rectangle")
	require.NoError(t, err)
	assert.Equal(t, StatusSkipped, outcome.Status)
	assert.Empty(t, rep.shellouts, "skipped cask must not appear in the dry-run preview")
}

// TestDryRunRunner_InstallCaskReportsWhenInstalledButQuiescent — an
// installed cask whose apps are NOT running falls through to a
// would-`upgrade --cask` report (matches the production path, which
// upgrades an installed cask rather than no-op `install --cask`-ing it).
func TestDryRunRunner_InstallCaskReportsWhenInstalledButQuiescent(t *testing.T) {
	inner := &fakeRunner{}
	inner.On("IsCaskInstalled", mock.Anything, "docker").Return(true, nil)
	inner.On("ListCaskApps", mock.Anything, "docker").Return([]string{"Docker"}, nil)
	inner.On("IsAppRunning", mock.Anything, "Docker").Return(false, nil)
	rep := &fakeReporter{Reporter: dryrun.NewNullReporter()}
	r := NewDryRunWrapper(inner, rep)

	_, err := r.InstallCask(context.Background(), "docker")
	require.NoError(t, err)
	require.Len(t, rep.shellouts, 1)
	assert.Equal(t, []string{"upgrade", "--cask", "docker"}, rep.shellouts[0].args)
}

// TestDryRunRunner_ReadMethodsPassThrough — read methods (all three:
// IsCaskInstalled, IsAppRunning, ListCaskApps) delegate to the
// wrapped runner verbatim and do not produce Shellout reports.
func TestDryRunRunner_ReadMethodsPassThrough(t *testing.T) {
	inner := &fakeRunner{}
	inner.On("IsCaskInstalled", mock.Anything, "rectangle").Return(true, nil)
	inner.On("IsAppRunning", mock.Anything, "Rectangle").Return(false, nil)
	inner.On("ListCaskApps", mock.Anything, "rectangle").Return([]string{"Rectangle.app"}, nil)
	inner.On("Outdated", mock.Anything).Return([]string{"zoom"}, nil)
	rep := &fakeReporter{Reporter: dryrun.NewNullReporter()}
	r := NewDryRunWrapper(inner, rep)

	gotOutdated, err := r.Outdated(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"zoom"}, gotOutdated)

	gotInstalled, err := r.IsCaskInstalled(context.Background(), "rectangle")
	require.NoError(t, err)
	assert.True(t, gotInstalled)

	gotRunning, err := r.IsAppRunning(context.Background(), "Rectangle")
	require.NoError(t, err)
	assert.False(t, gotRunning)

	gotApps, err := r.ListCaskApps(context.Background(), "rectangle")
	require.NoError(t, err)
	assert.Equal(t, []string{"Rectangle.app"}, gotApps)

	inner.AssertCalled(t, "IsCaskInstalled", mock.Anything, "rectangle")
	inner.AssertCalled(t, "IsAppRunning", mock.Anything, "Rectangle")
	inner.AssertCalled(t, "ListCaskApps", mock.Anything, "rectangle")
	// Reads do not bump the Shellout counter.
	assert.Empty(t, rep.shellouts)
}
