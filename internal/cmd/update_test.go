package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Nivl/config/internal/dryrun"
	"github.com/Nivl/config/internal/iox"
)

// withFakeBootstrap swaps execBootstrap for a recorder + returns
// the slice the recorder writes to. The previous implementation is
// restored at test cleanup so the package-level seam doesn't leak.
func withFakeBootstrap(t *testing.T) *[]bootstrapCall {
	t.Helper()
	calls := &[]bootstrapCall{}
	prev := execBootstrap
	execBootstrap = func(_ *appConfig, args []string) error {
		*calls = append(*calls, bootstrapCall{args: append([]string(nil), args...)})
		return nil
	}
	t.Cleanup(func() { execBootstrap = prev })
	return calls
}

// withInteractive forces isInteractive to return the given value
// for the duration of the test. Most prompt-flow tests pass a
// strings.Reader as stdin (which is correctly classified as
// non-interactive by the production check); they need this to
// reach the prompt code path at all.
func withInteractive(t *testing.T, interactive bool) {
	t.Helper()
	prev := isInteractive
	isInteractive = func(io.Reader) bool { return interactive }
	t.Cleanup(func() { isInteractive = prev })
}

// bootstrapCall records one invocation of the bootstrap seam.
type bootstrapCall struct {
	args []string
}

// updateTestCfg builds an appConfig pointing at configDir, with
// the given stdin and a discardable stdout. Stderr is wired to
// the returned buffer so tests can assert prompt output.
func updateTestCfg(configDir string, stdin io.Reader) (*appConfig, *bytes.Buffer) {
	stderr := &bytes.Buffer{}
	return &appConfig{
		streams:   iox.Streams{In: stdin, Out: io.Discard, Err: stderr},
		reporter:  dryrun.NewNullReporter(),
		configDir: configDir,
	}, stderr
}

// TestRunCheckUpdate_FreshStampIsNoOp — when the stamp file is
// less than updateCheckInterval old, runCheckUpdate exits silently
// without calling execBootstrap and without prompting.
func TestRunCheckUpdate_FreshStampIsNoOp(t *testing.T) {
	configDir := t.TempDir()
	now := time.Now()
	// Stamp written 1 day ago — well under 14 days.
	stamp := now.Add(-24 * time.Hour)
	require.NoError(t, writeUpdateStamp(filepath.Join(configDir, updateStampFile), stamp))

	calls := withFakeBootstrap(t)
	cfg, stderr := updateTestCfg(configDir, strings.NewReader(""))

	require.NoError(t, runCheckUpdate(cfg, now))
	assert.Empty(t, *calls, "fresh stamp must skip the bootstrap call")
	assert.Empty(t, stderr.String(), "fresh stamp must not print a prompt")
}

// TestRunCheckUpdate_MissingStampPromptsAndCallsBootstrap — a
// missing stamp file (e.g. first run on a clean machine) is
// treated as overdue: stamp is created, user is prompted, and
// a Y answer hands off to the bootstrap.
func TestRunCheckUpdate_MissingStampPromptsAndCallsBootstrap(t *testing.T) {
	configDir := t.TempDir()
	calls := withFakeBootstrap(t)
	withInteractive(t, true)
	cfg, stderr := updateTestCfg(configDir, strings.NewReader("y\n"))

	now := time.Unix(2_000_000_000, 0)
	require.NoError(t, runCheckUpdate(cfg, now))

	require.Len(t, *calls, 1)
	assert.Empty(t, (*calls)[0].args, "check-update forwards no args; explicit update is for that")

	got := readUpdateStamp(filepath.Join(configDir, updateStampFile))
	assert.Equal(t, now.Unix(), got.Unix(), "stamp should be refreshed to now")
	assert.Contains(t, stderr.String(), "Would you like to update your config?")
}

// TestRunCheckUpdate_UserAnsweringNoSkipsBootstrap — when the user
// answers N, the stamp is still refreshed (so we don't prompt
// again on every shell), but execBootstrap is not called.
func TestRunCheckUpdate_UserAnsweringNoSkipsBootstrap(t *testing.T) {
	configDir := t.TempDir()
	calls := withFakeBootstrap(t)
	withInteractive(t, true)
	cfg, _ := updateTestCfg(configDir, strings.NewReader("n\n"))

	now := time.Unix(2_000_000_000, 0)
	require.NoError(t, runCheckUpdate(cfg, now))

	assert.Empty(t, *calls, "N answer must not invoke the bootstrap")
	got := readUpdateStamp(filepath.Join(configDir, updateStampFile))
	assert.Equal(t, now.Unix(), got.Unix(), "stamp is refreshed even on N so we don't re-prompt every shell")
}

// TestRunCheckUpdate_StampWrittenBeforePrompt — the stamp file
// must exist on disk by the time the user is asked Y/N. This is
// the concurrent-shells coordination contract: a second shell
// opening during the prompt sees the refreshed stamp and exits
// silently rather than queueing up its own prompt.
//
// We exercise this by reading the stamp from inside the fake
// stdin, before the answer is returned.
func TestRunCheckUpdate_StampWrittenBeforePrompt(t *testing.T) {
	configDir := t.TempDir()
	stampPath := filepath.Join(configDir, updateStampFile)

	now := time.Unix(2_000_000_000, 0)
	// Custom reader that snapshots the stamp file the moment the
	// prompt tries to read. Mirrors what a concurrent shell would
	// see at that point.
	stampedAtPromptTime := time.Time{}
	stdin := &readerHook{
		body: "y\n",
		onRead: func() {
			stampedAtPromptTime = readUpdateStamp(stampPath)
		},
	}

	withFakeBootstrap(t)
	withInteractive(t, true)
	cfg, _ := updateTestCfg(configDir, stdin)
	require.NoError(t, runCheckUpdate(cfg, now))

	assert.Equal(t, now.Unix(), stampedAtPromptTime.Unix(),
		"stamp must be written BEFORE the prompt reads — concurrent shells need to see it")
}

// readerHook is an io.Reader that invokes onRead once on the
// first Read call before delegating to a strings.Reader. Used by
// the stamp-ordering test above.
type readerHook struct {
	body   string
	onRead func()
	pos    int
	fired  bool
}

func (r *readerHook) Read(p []byte) (int, error) {
	if !r.fired {
		r.fired = true
		r.onRead()
	}
	if r.pos >= len(r.body) {
		return 0, io.EOF
	}
	n := copy(p, r.body[r.pos:])
	r.pos += n
	return n, nil
}

// TestRunCheckUpdate_MalformedStampTreatedAsOverdue — when the
// stamp file is corrupted with non-numeric content, readUpdateStamp
// returns the zero time, which is always > 14 days ago, so the
// prompt fires (and the bad stamp gets overwritten with a fresh
// one).
func TestRunCheckUpdate_MalformedStampTreatedAsOverdue(t *testing.T) {
	configDir := t.TempDir()
	stampPath := filepath.Join(configDir, updateStampFile)
	require.NoError(t, os.WriteFile(stampPath, []byte("not-a-number"), 0o644))

	calls := withFakeBootstrap(t)
	withInteractive(t, true)
	cfg, _ := updateTestCfg(configDir, strings.NewReader("y\n"))

	require.NoError(t, runCheckUpdate(cfg, time.Unix(2_000_000_000, 0)))
	require.Len(t, *calls, 1, "malformed stamp must be treated as overdue")
}

// TestPromptYesNo_AcceptsCaseInsensitive — both y/Y → true, both
// n/N → false. The shell version had this behavior; we match it.
func TestPromptYesNo_AcceptsCaseInsensitive(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"y\n", true},
		{"Y\n", true},
		{"yes\n", true},
		{"n\n", false},
		{"N\n", false},
		{"no\n", false},
	}
	for _, tc := range cases {
		got, err := promptYesNo(strings.NewReader(tc.in), &bytes.Buffer{}, "?")
		require.NoError(t, err, "input %q", tc.in)
		assert.Equal(t, tc.want, got, "input %q", tc.in)
	}
}

// TestPromptYesNo_LoopsOnGarbageThenAccepts — invalid input
// re-prompts; the loop ends on the first valid answer.
func TestPromptYesNo_LoopsOnGarbageThenAccepts(t *testing.T) {
	in := strings.NewReader("maybe\nblah\nyes\n")
	out := &bytes.Buffer{}
	got, err := promptYesNo(in, out, "?")
	require.NoError(t, err)
	assert.True(t, got)
	assert.Equal(t, 2, strings.Count(out.String(), "Please answer Y or N."))
}

// TestPromptYesNo_EOFOnFirstReadErrors — a piped or redirected
// invocation with no input shouldn't silently fall through to "no";
// it errors so the caller can distinguish "user said no" from
// "couldn't ask the user."
func TestPromptYesNo_EOFOnFirstReadErrors(t *testing.T) {
	_, err := promptYesNo(strings.NewReader(""), &bytes.Buffer{}, "?")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no input")
}

// TestNewUpdateCmd_ForwardsArgsToBootstrap — the cobra wrapper
// passes positional args (and unknown flags, thanks to
// DisableFlagParsing) straight through to execBootstrap.
func TestNewUpdateCmd_ForwardsArgsToBootstrap(t *testing.T) {
	calls := withFakeBootstrap(t)
	cfg, _ := updateTestCfg(t.TempDir(), strings.NewReader(""))

	cmd := newUpdateCmd(cfg)
	cmd.SetArgs([]string{"--dry-run", "--personal"})
	require.NoError(t, cmd.Execute())

	require.Len(t, *calls, 1)
	assert.Equal(t, []string{"--dry-run", "--personal"}, (*calls)[0].args,
		"DisableFlagParsing should pass cobra-unknown flags through verbatim")
}

// TestRunCheckUpdate_NonInteractiveSkipsSilently — when stdin is
// not a TTY (script-spawned shell, `ssh host cmd`, IDE), the
// check-update hook must exit 0 with no output, no stamp write,
// no bootstrap call. Without this guard, base.zshrc invoking
// `melvin-config check-update` on every shell startup would
// otherwise spam stderr with "Error: prompt: no input" for any
// non-interactive shell whose stamp happened to be stale.
func TestRunCheckUpdate_NonInteractiveSkipsSilently(t *testing.T) {
	configDir := t.TempDir()
	calls := withFakeBootstrap(t)
	withInteractive(t, false)
	cfg, stderr := updateTestCfg(configDir, strings.NewReader(""))

	now := time.Unix(2_000_000_000, 0)
	require.NoError(t, runCheckUpdate(cfg, now))

	assert.Empty(t, *calls, "non-interactive shell must not invoke the bootstrap")
	assert.Empty(t, stderr.String(), "non-interactive shell must not print anything")

	stampPath := filepath.Join(configDir, updateStampFile)
	_, err := os.Stat(stampPath)
	assert.True(t, os.IsNotExist(err),
		"non-interactive shell must NOT burn the 14-day window — leave the stamp alone so the next interactive shell still gets prompted")
}

// TestOsStdinIsTerminal_NonFileReturnsFalse — the production
// isInteractive impl returns false for anything that isn't *os.File
// (strings.Reader, bytes.Buffer, etc.) since those can't possibly
// be a terminal. Regression guard against accidentally narrowing
// the type assertion.
func TestOsStdinIsTerminal_NonFileReturnsFalse(t *testing.T) {
	assert.False(t, osStdinIsTerminal(strings.NewReader("anything")))
	assert.False(t, osStdinIsTerminal(&bytes.Buffer{}))
}

// TestOsStdinIsTerminal_DevNullIsNotATerminal — /dev/null is a
// character device (ModeCharDevice is set), so a naive
// ModeCharDevice-based check would falsely classify
// `cmd </dev/null` as interactive. The TTY-specific ioctl behind
// term.IsTerminal correctly rejects it.
func TestOsStdinIsTerminal_DevNullIsNotATerminal(t *testing.T) {
	f, err := os.Open(os.DevNull)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	assert.False(t, osStdinIsTerminal(f),
		"/dev/null must not be classified as a terminal even though it's a char device")
}

// TestOsStdinIsTerminal_RegularFileIsNotATerminal — sanity check
// for the opposite end of the spectrum: a plain disk file should
// also fail the TTY check.
func TestOsStdinIsTerminal_RegularFileIsNotATerminal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "in.txt")
	require.NoError(t, os.WriteFile(path, []byte("y\n"), 0o644))
	f, err := os.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	assert.False(t, osStdinIsTerminal(f))
}

// TestNewUpdateCmd_HelpFlagShortCircuits — --help and -h short-
// circuit to cobra's help output instead of being forwarded to the
// bootstrap. DisableFlagParsing means cobra doesn't intercept these
// automatically, so the RunE has to do it explicitly.
func TestNewUpdateCmd_HelpFlagShortCircuits(t *testing.T) {
	for _, flag := range []string{"--help", "-h"} {
		calls := withFakeBootstrap(t)
		cfg, _ := updateTestCfg(t.TempDir(), strings.NewReader(""))
		cmd := newUpdateCmd(cfg)
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{flag})
		require.NoError(t, cmd.Execute(), "help flag %q must not error", flag)
		assert.Empty(t, *calls, "help flag %q must not invoke the bootstrap", flag)
	}
}

// TestReadUpdateStamp_MissingFileReturnsZeroTime — the readback
// helper distinguishes "missing/corrupt" from "long-past" by
// returning the zero time; callers (runCheckUpdate) then compare
// against the interval and treat both as overdue.
func TestReadUpdateStamp_MissingFileReturnsZeroTime(t *testing.T) {
	got := readUpdateStamp(filepath.Join(t.TempDir(), "no-such-file"))
	assert.True(t, got.IsZero())
}
