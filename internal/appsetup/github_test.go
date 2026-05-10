package appsetup

import (
	"context"
	"errors"
	"testing"

	"github.com/Nivl/config/internal/appsetup/appsetuptest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestSetupGitHub_AuthedSuccessPath — gh auth status returns success
// for github.com; SetupGitHub returns (true, nil) without calling
// the prompter or gh auth login.
func TestSetupGitHub_AuthedSuccessPath(t *testing.T) {
	runner := appsetuptest.NewFakeCmdRunner()
	runner.On("Capture", mock.Anything, "gh", "auth", "status", "-a", "--json", "hosts").
		Return([]byte(`{"hosts":{"github.com":[{"state":"success"}]}}`), nil)

	prompter := appsetuptest.NewFakeYesNoPrompter()

	got, err := SetupGitHub(context.Background(), prompter, runner)
	require.NoError(t, err)
	assert.True(t, got)

	prompter.AssertNotCalled(t, "AskYesNo", mock.Anything)
	runner.AssertNotCalled(t, "Run", mock.Anything, "gh", "auth", "login", "-w")
}

// TestSetupGitHub_NotAuthedUserDeclines — gh auth status returns
// non-success (e.g., missing token); prompter says no; SetupGitHub
// returns (false, nil) without invoking gh auth login.
func TestSetupGitHub_NotAuthedUserDeclines(t *testing.T) {
	runner := appsetuptest.NewFakeCmdRunner()
	runner.On("Capture", mock.Anything, "gh", "auth", "status", "-a", "--json", "hosts").
		Return([]byte(`{"hosts":{"github.com":[{"state":"not_authenticated"}]}}`), nil)

	prompter := appsetuptest.NewFakeYesNoPrompter()
	prompter.On("AskYesNo", "Setup Github").Return(false, nil)

	got, err := SetupGitHub(context.Background(), prompter, runner)
	require.NoError(t, err)
	assert.False(t, got)

	runner.AssertNotCalled(t, "Run", mock.Anything, "gh", "auth", "login", "-w")
}

// TestSetupGitHub_NotAuthedUserAccepts — prompter says yes; gh auth
// login -w is invoked; SetupGitHub returns (true, nil) on success.
func TestSetupGitHub_NotAuthedUserAccepts(t *testing.T) {
	runner := appsetuptest.NewFakeCmdRunner()
	runner.On("Capture", mock.Anything, "gh", "auth", "status", "-a", "--json", "hosts").
		Return([]byte(`{"hosts":{}}`), nil)
	runner.On("Run", mock.Anything, "gh", "auth", "login", "-w").Return(nil)

	prompter := appsetuptest.NewFakeYesNoPrompter()
	prompter.On("AskYesNo", "Setup Github").Return(true, nil)

	got, err := SetupGitHub(context.Background(), prompter, runner)
	require.NoError(t, err)
	assert.True(t, got)

	runner.AssertExpectations(t)
	prompter.AssertExpectations(t)
}

// TestSetupGitHub_LoginErrorPropagates — gh auth login -w failure
// surfaces as an error.
func TestSetupGitHub_LoginErrorPropagates(t *testing.T) {
	runner := appsetuptest.NewFakeCmdRunner()
	runner.On("Capture", mock.Anything, "gh", "auth", "status", "-a", "--json", "hosts").
		Return([]byte(`{"hosts":{}}`), nil)
	loginErr := errors.New("user canceled login")
	runner.On("Run", mock.Anything, "gh", "auth", "login", "-w").Return(loginErr)

	prompter := appsetuptest.NewFakeYesNoPrompter()
	prompter.On("AskYesNo", "Setup Github").Return(true, nil)

	got, err := SetupGitHub(context.Background(), prompter, runner)
	require.Error(t, err)
	assert.False(t, got)
	assert.ErrorIs(t, err, loginErr)
}

// TestSetupGitHub_MalformedJSON — gh returns non-JSON output; error
// surfaces as a wrapped json error.
func TestSetupGitHub_MalformedJSON(t *testing.T) {
	runner := appsetuptest.NewFakeCmdRunner()
	runner.On("Capture", mock.Anything, "gh", "auth", "status", "-a", "--json", "hosts").
		Return([]byte(`not json at all`), nil)

	prompter := appsetuptest.NewFakeYesNoPrompter()

	_, err := SetupGitHub(context.Background(), prompter, runner)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse gh auth status JSON")
}

// TestSetupGitHub_GhMissing — Capture returns a process-missing
// error; SetupGitHub propagates without calling the prompter.
func TestSetupGitHub_GhMissing(t *testing.T) {
	runner := appsetuptest.NewFakeCmdRunner()
	ghErr := errors.New("exec: \"gh\": executable file not found in $PATH")
	runner.On("Capture", mock.Anything, "gh", "auth", "status", "-a", "--json", "hosts").
		Return([]byte(nil), ghErr)

	prompter := appsetuptest.NewFakeYesNoPrompter()

	_, err := SetupGitHub(context.Background(), prompter, runner)
	require.Error(t, err)
	require.ErrorIs(t, err, ghErr)
	prompter.AssertNotCalled(t, "AskYesNo", mock.Anything)
}

// TestSetupGitHub_EmptyHostsArray — gh returns {"hosts":{"github.com":[]}};
// SetupGitHub treats it as not-authenticated and prompts.
func TestSetupGitHub_EmptyHostsArray(t *testing.T) {
	runner := appsetuptest.NewFakeCmdRunner()
	runner.On("Capture", mock.Anything, "gh", "auth", "status", "-a", "--json", "hosts").
		Return([]byte(`{"hosts":{"github.com":[]}}`), nil)

	prompter := appsetuptest.NewFakeYesNoPrompter()
	prompter.On("AskYesNo", "Setup Github").Return(false, nil)

	got, err := SetupGitHub(context.Background(), prompter, runner)
	require.NoError(t, err)
	assert.False(t, got)
	prompter.AssertExpectations(t)
}
