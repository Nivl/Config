//go:build integration

package configgen

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Nivl/config/internal/dryrun"
)

// TestSetupGpg_RealGpgAgent — integration test that uses the real
// CmdRunner against the system gpg-agent binary. Only runs on
// darwin with gpg-agent on PATH. Build tagged so default `go test`
// skips it; CI's go-integration-configgen job runs it explicitly.
func TestSetupGpg_RealGpgAgent(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("integration test only runs on darwin")
	}
	if _, err := exec.LookPath("gpg-agent"); err != nil {
		t.Skip("gpg-agent not on PATH")
	}

	tmp := t.TempDir()
	brewPrefix := os.Getenv("HOMEBREW_PREFIX")
	if brewPrefix == "" {
		brewPrefix = "/opt/homebrew" // sensible default for Apple Silicon
	}

	err := SetupGpg(context.Background(), tmp, brewPrefix, NewCmdRunner(), false, dryrun.NewNullReporter())
	require.NoError(t, err)

	// Verify conf written.
	got, err := os.ReadFile(filepath.Join(tmp, ".gnupg", "gpg-agent.conf"))
	require.NoError(t, err)
	assert.Contains(t, string(got), "pinentry-program "+brewPrefix+"/bin/pinentry-mac")

	// Cleanup: kill the just-launched daemon.
	_ = exec.Command("killall", "gpg-agent").Run()
}
