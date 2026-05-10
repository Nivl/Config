package appsetup

import (
	"context"
	"testing"

	"github.com/Nivl/config/internal/appsetup/appsetuptest"
	"github.com/Nivl/config/internal/dryrun"
	"github.com/Nivl/config/internal/iox"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestNewCmdRunner_ReturnsRealImpl verifies the constructor returns
// a non-nil CmdRunner. We don't test the real impl directly here —
// stdio inheritance is exercised in manual smoke + production use.
func TestNewCmdRunner_ReturnsRealImpl(t *testing.T) {
	got := NewCmdRunner(iox.Streams{})
	assert.NotNil(t, got)
}

// fakeReporter is a minimal dryrun.Reporter for appsetup dry-run tests.
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
	f.shellouts = append(f.shellouts, fakeShellout{
		command: command,
		args:    append([]string(nil), args...),
	})
}

// TestDryRunCmdRunner_RunReports — Run reports via the Reporter and
// does NOT delegate to the wrapped runner.
func TestDryRunCmdRunner_RunReports(t *testing.T) {
	inner := appsetuptest.NewFakeCmdRunner()
	rep := &fakeReporter{Reporter: dryrun.NewNullReporter()}
	r := NewDryRunCmdRunner(inner, rep)

	require.NoError(t, r.Run(context.Background(), "ssh-keygen", "-t", "ed25519"))

	require.Len(t, rep.shellouts, 1)
	assert.Equal(t, "ssh-keygen", rep.shellouts[0].command)
	assert.Equal(t, []string{"-t", "ed25519"}, rep.shellouts[0].args)
	inner.AssertNotCalled(t, "Run", mock.Anything, mock.Anything)
}

// TestDryRunCmdRunner_CapturePassesThrough — Capture delegates to
// the wrapped runner so reads run as normal.
func TestDryRunCmdRunner_CapturePassesThrough(t *testing.T) {
	want := []byte("gh-auth-status-output")
	inner := appsetuptest.NewFakeCmdRunner()
	inner.On("Capture", mock.Anything, "gh", "auth", "status").
		Return(want, nil)
	rep := &fakeReporter{Reporter: dryrun.NewNullReporter()}
	r := NewDryRunCmdRunner(inner, rep)

	got, err := r.Capture(context.Background(), "gh", "auth", "status")
	require.NoError(t, err)
	assert.Equal(t, want, got)
	assert.Empty(t, rep.shellouts)
}
