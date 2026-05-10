package appsetup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Nivl/config/internal/appsetup/appsetuptest"
	"github.com/Nivl/config/internal/dryrun"
)

// TestSetupSSH_FreshInstall — homeDir/.ssh doesn't exist; SetupSSH
// creates it with 0o700, invokes ssh-keygen with the expected args,
// and writes ~/.ssh/config with IdentityFile pointing at default.
func TestSetupSSH_FreshInstall(t *testing.T) {
	tmp := t.TempDir()
	runner := appsetuptest.NewFakeCmdRunner()
	defaultKey := filepath.Join(tmp, ".ssh", "default")
	runner.On("Run", mock.Anything, "ssh-keygen", "-o", "-a", "100", "-t", "ed25519", "-f", defaultKey).
		Return(nil)

	err := SetupSSH(context.Background(), tmp, runner, false, dryrun.NewNullReporter())
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(tmp, ".ssh"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())

	got, err := os.ReadFile(filepath.Join(tmp, ".ssh", "config"))
	require.NoError(t, err)
	assert.Equal(t, "IdentityFile "+defaultKey+"\n", string(got))

	runner.AssertExpectations(t)
}

// TestSetupSSH_KeyExistsSkipsKeygen — homeDir/.ssh/default.pub
// already exists; ssh-keygen is NOT invoked. The .ssh/config write
// happens normally.
func TestSetupSSH_KeyExistsSkipsKeygen(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, ".ssh"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, ".ssh", "default.pub"), []byte("ssh-ed25519 ..."), 0o644))

	runner := appsetuptest.NewFakeCmdRunner()
	// Deliberately no Run expectation — if keygen runs, AssertExpectations would still pass
	// but AssertNotCalled is the explicit guard.

	err := SetupSSH(context.Background(), tmp, runner, false, dryrun.NewNullReporter())
	require.NoError(t, err)
	runner.AssertNotCalled(t, "Run", mock.Anything, "ssh-keygen", mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// TestSetupSSH_ConfigExistsSkipsWrite — homeDir/.ssh/config already
// exists; SetupSSH does NOT overwrite. The keygen still runs if
// default.pub is absent.
func TestSetupSSH_ConfigExistsSkipsWrite(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, ".ssh"), 0o700))
	preExisting := "Host foo.example.com\n  IdentityFile ~/.ssh/foo\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmp, ".ssh", "config"), []byte(preExisting), 0o600))

	runner := appsetuptest.NewFakeCmdRunner()
	defaultKey := filepath.Join(tmp, ".ssh", "default")
	runner.On("Run", mock.Anything, "ssh-keygen", "-o", "-a", "100", "-t", "ed25519", "-f", defaultKey).
		Return(nil)

	err := SetupSSH(context.Background(), tmp, runner, false, dryrun.NewNullReporter())
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(tmp, ".ssh", "config"))
	require.NoError(t, err)
	assert.Equal(t, preExisting, string(got), "user's pre-existing config preserved")
}

// TestSetupSSH_AllPresentNoOp — both default.pub and config exist;
// SetupSSH is a no-op (no shellouts, no writes).
func TestSetupSSH_AllPresentNoOp(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, ".ssh"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, ".ssh", "default.pub"), []byte("pubkey"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, ".ssh", "config"), []byte("preserved"), 0o600))

	runner := appsetuptest.NewFakeCmdRunner()

	err := SetupSSH(context.Background(), tmp, runner, false, dryrun.NewNullReporter())
	require.NoError(t, err)
	runner.AssertNotCalled(t, "Run", mock.Anything, "ssh-keygen", mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	got, err := os.ReadFile(filepath.Join(tmp, ".ssh", "config"))
	require.NoError(t, err)
	assert.Equal(t, "preserved", string(got))
}

// TestSetupSSH_KeygenErrorPropagates — ssh-keygen failure surfaces as
// a wrapped error.
func TestSetupSSH_KeygenErrorPropagates(t *testing.T) {
	tmp := t.TempDir()
	runner := appsetuptest.NewFakeCmdRunner()
	keygenErr := errors.New("ssh-keygen failed")
	defaultKey := filepath.Join(tmp, ".ssh", "default")
	runner.On("Run", mock.Anything, "ssh-keygen", "-o", "-a", "100", "-t", "ed25519", "-f", defaultKey).
		Return(keygenErr)

	err := SetupSSH(context.Background(), tmp, runner, false, dryrun.NewNullReporter())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ssh-keygen")
	assert.ErrorIs(t, err, keygenErr)
}
