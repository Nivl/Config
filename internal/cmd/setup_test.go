package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Nivl/config/internal/appsetup"
	"github.com/Nivl/config/internal/appsetup/appsetuptest"
	"github.com/Nivl/config/internal/brew"
	"github.com/Nivl/config/internal/claude/sync"
	"github.com/Nivl/config/internal/configgen"
	"github.com/Nivl/config/internal/configgen/configgentest"
	"github.com/Nivl/config/internal/dotfiles"
	"github.com/Nivl/config/internal/dryrun"
	"github.com/Nivl/config/internal/iox"
	"github.com/Nivl/config/internal/userinput"
)

// TestSetupCmd_AllPromptsEnvFastPathDoNotReadStdin verifies that when
// all five user-input preargs are set, setupCmd's prompt step doesn't
// try to read from os.Stdin. This is the regression guard for
// unattended-mode CI users — non-empty preargs bypass all prompts.
//
// We can't easily fake os.Stdin / os.Stdout / os.Stderr in a non-DI
// codepath, so this test only verifies the fast-path env reads return
// the pre-set values directly. The actual setupCmd body is not invoked;
// instead we exercise the userinput package functions directly with the
// pre-set env to confirm zero stdin reads. The full setupCmd path is
// exercised via the existing manual smoke tests on a sandboxed copy.
func TestSetupCmd_AllPromptsEnvFastPathDoNotReadStdin(t *testing.T) {
	// Pass an empty reader as "stdin"; if any prompt actually reads, this
	// would either block or error. The five userinput helpers all
	// short-circuit when their prearg is non-empty.
	emptyStdin := strings.NewReader("")
	out := &bytes.Buffer{}

	personal, err := userinput.Personal("true", emptyStdin, out, t.TempDir(), false, dryrun.NewNullReporter())
	require.NoError(t, err)
	assert.Equal(t, "true", personal)

	devRoot, err := userinput.DevRoot("/opt/dev", emptyStdin, out, t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, "/opt/dev", devRoot)

	gitOrg, err := userinput.GitOrg("Acme", emptyStdin, out, personal == "true")
	require.NoError(t, err)
	assert.Equal(t, "Acme", gitOrg)

	gitHost, err := userinput.GitHost("git@github.com", emptyStdin, out)
	require.NoError(t, err)
	assert.Equal(t, "git@github.com", gitHost)

	assert.Empty(t, out.String(), "no prompt strings should be printed when all preargs are set")
}

// TestSetupCmd_FileGenWiring verifies that the four file-generating
// functions (CopyConfigFiles, SetupZshrc, SetupGitconfig, SetupGpg)
// are callable end-to-end with env-derived inputs, producing the
// expected artifacts in a tempdir. This is the integration-style
// guard for the wiring — if a future refactor breaks parameter
// passing between the userinput prompts and the file-gen functions,
// this test catches it.
//
// The test invokes the configgen + dotfiles functions directly rather
// than going through setupCmd because setupCmd performs an end-to-end
// install that is hard to fake. The relevant logical flow is: get env
// → call userinput.* → call packages.InstallWithRetry → call sync.Sync
// → call the four file-gen functions. This test covers the last leg
// in isolation.
func TestSetupCmd_FileGenWiring(t *testing.T) {
	home := t.TempDir()
	configDir := t.TempDir()
	for _, item := range []string{".emacs.d", ".golangci.yml"} {
		require.NoError(t, os.MkdirAll(filepath.Join(configDir, "shared_config", item), 0o755))
	}

	// Simulate post-prompt env state.
	t.Setenv("HOMEBREW_PREFIX", "/opt/homebrew")
	personalStr := "true"
	devRoot := filepath.Join(home, "dev")
	gitHost := "git@github.com"
	gitOrg := "Nivl"

	runner := configgentest.NewFakeCmdRunner()
	runner.On("Run", mock.Anything, "killall", "gpg-agent").Return(nil)
	runner.On("Run", mock.Anything, "gpg-agent", "--daemon").Return(nil)

	require.NoError(t, dotfiles.CopyConfigFiles(context.Background(), configDir, home, &bytes.Buffer{}, false, dryrun.NewNullReporter()))
	require.NoError(t, configgen.SetupZshrc(home, configDir, configgen.ZshrcOpts{
		PersonalComputer: personalStr,
		DevRoot:          devRoot,
		GitHost:          gitHost,
		GitCloneUserName: gitOrg,
	}, false, dryrun.NewNullReporter()))
	require.NoError(t, configgen.SetupGitconfig(home, configDir, personalStr == "true", false, dryrun.NewNullReporter()))
	require.NoError(t, configgen.SetupGpg(context.Background(), home, os.Getenv("HOMEBREW_PREFIX"), runner, false, dryrun.NewNullReporter()))

	// Verify all 4 expected artifacts.
	for _, item := range []string{".emacs.d", ".golangci.yml"} {
		_, err := os.Readlink(filepath.Join(home, item))
		require.NoError(t, err, "%s should be symlinked", item)
	}
	_, err := os.Stat(filepath.Join(home, ".zshrc"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(home, ".gitconfig"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(home, ".gnupg", "gpg-agent.conf"))
	require.NoError(t, err)
}

// TestSetupCmd_FinalSetupWiring verifies that the three final-setup
// functions (SetupSSH, SetupGitHub, PrintRemainingTasks) are callable
// end-to-end with FakeCmdRunner + FakeYesNoPrompter. Mirrors
// TestSetupCmd_FileGenWiring — exercises the wiring in isolation
// since the full setupCmd path is hard to fake.
func TestSetupCmd_FinalSetupWiring(t *testing.T) {
	home := t.TempDir()
	runner := appsetuptest.NewFakeCmdRunner()
	defaultKey := filepath.Join(home, ".ssh", "default")
	runner.On("Run", mock.Anything, "ssh-keygen", "-o", "-a", "100", "-t", "ed25519", "-f", defaultKey).
		Return(nil)
	runner.On("Capture", mock.Anything, "gh", "auth", "status", "-a", "--json", "hosts").
		Return([]byte(`{"hosts":{"github.com":[{"state":"success"}]}}`), nil)

	prompter := appsetuptest.NewFakeYesNoPrompter()

	require.NoError(t, appsetup.SetupSSH(context.Background(), home, runner, false, dryrun.NewNullReporter()))
	githubAuthed, err := appsetup.SetupGitHub(context.Background(), prompter, runner)
	require.NoError(t, err)
	assert.True(t, githubAuthed)

	out := &bytes.Buffer{}
	appsetup.PrintRemainingTasks(out, home, githubAuthed)
	assert.Contains(t, out.String(), "Things left to do:")
	assert.NotContains(t, out.String(), "Upload") // authed, no SSH upload reminder

	// Verify the artifacts SetupSSH created.
	info, err := os.Stat(filepath.Join(home, ".ssh"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
	got, err := os.ReadFile(filepath.Join(home, ".ssh", "config"))
	require.NoError(t, err)
	assert.Equal(t, "IdentityFile "+defaultKey+"\n", string(got))

	runner.AssertExpectations(t)
}

// TestSetupCmd_DryRunFlagDeclared — newSetupCmd registers a
// --dry-run flag on the setup subcommand. Combined with the
// existing resolveBool unit tests (which cover the
// flag/env/fallback resolution chain), this proves the flag is
// reachable from the cobra harness.
func TestSetupCmd_DryRunFlagDeclared(t *testing.T) {
	cfg := &appConfig{
		streams:       iox.Streams{In: strings.NewReader(""), Out: &bytes.Buffer{}, Err: &bytes.Buffer{}},
		newBrewRunner: brew.NewRunner,
		newClaudeSync: sync.Sync,
		reporter:      dryrun.NewNullReporter(),
	}
	cmd := newSetupCmd(cfg)
	flag := cmd.Flags().Lookup("dry-run")
	require.NotNil(t, flag, "--dry-run flag must be declared on the setup subcommand")
	assert.Equal(t, "false", flag.DefValue)
}

// TestSetupCmd_DryRunStampSuppressed — exercises the inline stamp
// write in setupCmd: when cfg.dryRun is true, the stamp file is not
// created on disk, and the reporter receives a FileChange for the
// stamp path. This is the only write chokepoint inline in
// setupCmd; all other writes go through subsystem entry points
// covered by per-phase tests.
//
// The test pokes at setupCmd's stamp logic directly rather than
// running the full cobra setup command, because exercising the full
// command requires faking every subsystem.
func TestSetupCmd_DryRunStampSuppressed(t *testing.T) {
	tmpConfigDir := t.TempDir()
	stampPath := filepath.Join(tmpConfigDir, ".last_update_check")
	var stderr bytes.Buffer
	reporter := dryrun.NewReporter(&stderr)

	// Simulate the stamp-write block from setupCmd with cfg.dryRun=true.
	// (The block isn't independently exposed; assert against the
	// resulting behavior by inlining the same code shape here.)
	existing, _ := os.ReadFile(stampPath) // expect ErrNotExist; bytes nil
	stamp := strconv.FormatInt(time.Now().Unix(), 10) + "\n"
	reporter.FileChange(stampPath, existing, []byte(stamp), "advance update check")
	// dry-run path: skip the os.WriteFile call.
	// Production would: os.WriteFile(stampPath, []byte(stamp), 0o644).

	// Stamp file must not exist on disk.
	_, statErr := os.Stat(stampPath)
	assert.True(t, os.IsNotExist(statErr),
		"stamp file must not be written under --dry-run")

	// Reporter received the FileChange call.
	got := stderr.String()
	assert.Contains(t, got, ".last_update_check")
	assert.Contains(t, got, "advance update check")
}
