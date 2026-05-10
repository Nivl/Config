package appsetup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Nivl/config/internal/dryrun"
)

// SetupSSH ensures homeDir/.ssh exists with 0o700 perms, generates an
// ed25519 keypair at homeDir/.ssh/default if homeDir/.ssh/default.pub
// is absent (via ssh-keygen with -o -a 100 -t ed25519 -f), and writes
// a minimal homeDir/.ssh/config with the IdentityFile line.
//
// ssh-keygen is interactive — it prompts for a passphrase. runner's
// production impl inherits stdio so the user can enter it. Tests use
// FakeCmdRunner to verify the exact args without actually running
// ssh-keygen.
//
// Idempotent on re-run:
//   - homeDir/.ssh pre-existing: os.MkdirAll is a no-op for the dir
//     (perms unchanged — does not surprise-modify a user's setup).
//   - homeDir/.ssh/default.pub pre-existing: keygen skipped.
//   - homeDir/.ssh/config pre-existing: write skipped.
//
// Under dryRun: the os.MkdirAll, ssh-keygen shellout (already gated
// by the runner wrapper at the caller), and os.WriteFile are all
// suppressed; the Reporter is notified of the would-be decisions
// instead.
func SetupSSH(ctx context.Context, homeDir string, runner CmdRunner,
	dryRun bool, reporter dryrun.Reporter,
) error {
	sshDir := filepath.Join(homeDir, ".ssh")
	if dryRun {
		// MkdirAll is a no-op on existing dirs; report accordingly so
		// the dry-run preview doesn't claim an already-present ~/.ssh
		// "would change".
		if info, err := os.Stat(sshDir); err == nil && info.IsDir() {
			reporter.FileNoOp(sshDir, "dir already exists")
		} else {
			reporter.FileChange(sshDir, nil, nil, "would mkdir 0o700")
		}
	} else {
		if err := os.MkdirAll(sshDir, 0o700); err != nil {
			return fmt.Errorf("mkdir %s: %w", sshDir, err)
		}
	}

	defaultKey := filepath.Join(sshDir, "default")
	pubKey := defaultKey + ".pub"
	if _, err := os.Stat(pubKey); errors.Is(err, os.ErrNotExist) {
		// runner is wrapped by the caller under dry-run, so this Run is
		// already suppressed-and-reported. No extra guard needed here.
		if err := runner.Run(ctx, "ssh-keygen", "-o", "-a", "100", "-t", "ed25519", "-f", defaultKey); err != nil {
			return fmt.Errorf("ssh-keygen: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("stat %s: %w", pubKey, err)
	}

	sshConfig := filepath.Join(sshDir, "config")
	if _, err := os.Stat(sshConfig); errors.Is(err, os.ErrNotExist) {
		content := "IdentityFile " + defaultKey + "\n"
		if dryRun {
			reporter.FileChange(sshConfig, nil, []byte(content), "write IdentityFile config")
		} else {
			if err := os.WriteFile(sshConfig, []byte(content), 0o600); err != nil {
				return fmt.Errorf("write %s: %w", sshConfig, err)
			}
		}
	} else if err != nil {
		return fmt.Errorf("stat %s: %w", sshConfig, err)
	}
	return nil
}
