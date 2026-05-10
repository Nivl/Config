package configgen

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/Nivl/config/internal/dryrun"
	"github.com/Nivl/config/internal/managedblock"
)

// gpgAgentConfPriorLine matches any line beginning with
// `pinentry-program ` so SetupGpg can migrate files written before
// the marker convention regardless of the prefix recorded at that
// time.
var gpgAgentConfPriorLine = regexp.MustCompile( //nolint:gochecknoglobals // immutable regex
	`(?m)^pinentry-program\s+.*$`,
)

// pinentryLine returns the single pinentry-program line written into
// ~/.gnupg/gpg-agent.conf as the managed-block payload.
func pinentryLine(brewPrefix string) string {
	return fmt.Sprintf("pinentry-program %s/bin/pinentry-mac", brewPrefix)
}

// SetupGpg materializes ~/.gnupg/gpg-agent.conf:
//
//   - First-install only (conf absent): mkdir ~/.gnupg with 0o700.
//   - Every run: upsert the managed-block region containing the
//     pinentry-program line via managedblock.Upsert.
//   - First-install only (conf was absent): killall gpg-agent +
//     gpg-agent --daemon, so the freshly-written conf is picked up
//     by a fresh agent process. Subsequent runs skip the restart —
//     the existing agent re-reads pinentry-program on its next
//     pinentry invocation.
//
// killall exit code 1 means "no such process" — expected on first
// install where no agent is running yet. Any other error (including
// non-ExitError) propagates.
//
// When dryRun is true, no disk writes or shellouts occur. Instead,
// setupGpgDryRun handles all three phases via reporter calls.
//
// runner allows tests to inject a FakeCmdRunner. Production callers
// pass NewCmdRunner().
func SetupGpg(ctx context.Context, homeDir, brewPrefix string, runner CmdRunner, dryRun bool, reporter dryrun.Reporter) error {
	conf := filepath.Join(homeDir, ".gnupg", "gpg-agent.conf")

	if dryRun {
		return setupGpgDryRun(conf, brewPrefix, reporter)
	}

	_, statErr := os.Stat(conf)
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", conf, statErr)
	}
	// After the guard, the only non-nil statErr is os.ErrNotExist.
	firstInstall := statErr != nil

	if firstInstall {
		confDir := filepath.Dir(conf)
		if err := os.MkdirAll(confDir, 0o700); err != nil {
			return fmt.Errorf("mkdir %s: %w", confDir, err)
		}
	}

	payload := pinentryLine(brewPrefix)
	if err := managedblock.Upsert(conf, payload, gpgAgentConfPriorLine, managedblock.UpsertOpts{Reporter: reporter}); err != nil {
		return fmt.Errorf("upsert managed block in %s: %w", conf, err)
	}

	if !firstInstall {
		return nil
	}

	if err := runner.Run(ctx, "killall", "gpg-agent"); err != nil {
		// killall exit code 1 means "no such process" — expected on
		// first install where no agent is running yet.
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			return fmt.Errorf("killall gpg-agent: %w", err)
		}
	}

	if err := runner.Run(ctx, "gpg-agent", "--daemon"); err != nil {
		return fmt.Errorf("gpg-agent --daemon: %w", err)
	}
	return nil
}

// setupGpgDryRun handles all three phases of SetupGpg (mkdir, upsert,
// killall/daemon) without performing any I/O. It reports what would
// happen via reporter calls.
func setupGpgDryRun(conf, brewPrefix string, reporter dryrun.Reporter) error {
	confDir := filepath.Dir(conf)
	_, statErr := os.Stat(conf)
	firstInstall := errors.Is(statErr, os.ErrNotExist)
	if statErr != nil && !firstInstall {
		return fmt.Errorf("stat %s: %w", conf, statErr)
	}
	if firstInstall {
		// The live path runs os.MkdirAll, which is a no-op when
		// confDir already exists (~/.gnupg may pre-date melvin-config
		// — e.g. user ran `gpg --gen-key` earlier). Stat first so the
		// preview matches.
		if info, err := os.Stat(confDir); err == nil && info.IsDir() {
			reporter.FileNoOp(confDir, "dir already exists")
		} else {
			// Empty before/after; udiff returns "" so only the one-liner is printed.
			reporter.FileChange(confDir, nil, nil, "would mkdir 0o700")
		}
	}

	realExisting, _ := os.ReadFile(conf) //nolint:gosec // caller-supplied home-relative config-file path, read-by-design
	virtualExisting := realExisting
	if firstInstall {
		virtualExisting = nil
	}
	payload := pinentryLine(brewPrefix)
	proposed, err := managedblock.ComputeNext(
		string(virtualExisting), payload, gpgAgentConfPriorLine, conf,
	)
	if err != nil {
		return err
	}
	if proposed == string(realExisting) {
		reporter.FileNoOp(conf, "gpg-agent.conf in sync")
		return nil
	}
	summary := "would update gpg-agent.conf"
	if firstInstall {
		summary = "would create gpg-agent.conf (fresh install)"
	}
	reporter.FileChange(conf, realExisting, []byte(proposed), summary)
	if firstInstall {
		reporter.Shellout("killall", []string{"gpg-agent"}, "stop existing agent if any")
		reporter.Shellout("gpg-agent", []string{"--daemon"}, "spawn fresh agent")
	}
	return nil
}
