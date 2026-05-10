package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// updateCheckInterval is how long check-update waits between
// prompts. Matches the 14-day window that the previous
// _melvin_check_update shell function enforced before this logic
// moved into Go.
const updateCheckInterval = 14 * 24 * time.Hour

// updateStampFile is the filename (under configDir) that records
// when check-update last fired. Both check-update and setup write
// it after a successful run.
const updateStampFile = ".last_update_check"

// execBootstrap is the function actually invoked to hand off to
// install.bootstrap.sh. It's a package-level variable so tests can
// substitute a recording version without the production
// implementation (syscall.Exec) replacing the test binary.
var execBootstrap = doExecBootstrap

// isInteractive reports whether in is a terminal (rather than a
// pipe / redirected file / closed FD). check-update consults this
// before prompting so non-interactive shells (`ssh host cmd`,
// IDE-spawned shells, script subshells) don't get an EOF-error
// printed to stderr on every shell startup.
//
// Package-level seam so tests can force interactive=true even when
// stdin is a strings.Reader.
var isInteractive = osStdinIsTerminal

// osStdinIsTerminal is the production isInteractive implementation:
// in must be a *os.File whose file descriptor is a real terminal.
// We use golang.org/x/term.IsTerminal (which round-trips a
// TTY-specific ioctl) rather than the os.ModeCharDevice bit —
// /dev/null is also a character device, so the bit alone would
// classify `cmd </dev/null` as interactive and defeat the whole
// point of the check.
func osStdinIsTerminal(in io.Reader) bool {
	f, ok := in.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// doExecBootstrap replaces the current process with
// `zsh <configDir>/install.bootstrap.sh <args...>` via
// syscall.Exec. Bootstrap then pulls the repo, rebuilds $BIN, and
// execs into `melvin-config setup` with the forwarded args.
//
// Using syscall.Exec rather than a subprocess eliminates the
// in-place-binary-replacement race entirely: the old binary's
// process image is gone before `go build -o $BIN` rewrites the
// file on disk, so there's no possibility of the old binary
// holding a stale view of anything.
//
// Returns only on failure — syscall.Exec doesn't return on
// success because the process image is replaced.
func doExecBootstrap(cfg *appConfig, args []string) error {
	configDir, err := resolveConfigDir(cfg)
	if err != nil {
		return err
	}
	script := filepath.Join(configDir, "install.bootstrap.sh")
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("locate bootstrap: %w", err)
	}
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		return fmt.Errorf("zsh on PATH: %w", err)
	}
	execArgs := append([]string{"zsh", script}, args...)
	if err := syscall.Exec(zsh, execArgs, os.Environ()); err != nil { //nolint:gosec // zsh is the fixed binary located via exec.LookPath; script path is configDir/install.bootstrap.sh; remaining args flow through argv (no shell interpolation)
		return fmt.Errorf("exec %s: %w", zsh, err)
	}
	// Unreachable on success — syscall.Exec replaces the process.
	return errors.New("unreachable: syscall.Exec returned nil error without replacing process")
}

// newUpdateCmd builds `melvin-config update`. Pulls the latest
// config, rebuilds the binary, and re-runs setup with whatever
// args the user forwarded.
//
// DisableFlagParsing makes cobra treat every post-`update` token as
// a positional arg, so flags like `--dry-run` or `--personal` reach
// the bootstrap (and through it, the rebuilt `melvin-config setup`)
// verbatim without needing to be redeclared here.
func newUpdateCmd(cfg *appConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update [args...]",
		Short: "Pull the latest config, rebuild, and re-run setup",
		Long: `Re-execs into install.bootstrap.sh so the bootstrap can pull the
latest config-repo commits, rebuild the melvin-config binary, and
re-run setup. All arguments after "update" pass through to setup
verbatim — e.g. melvin-config update --dry-run.

The current process is replaced via syscall.Exec, so the running
binary doesn't linger after the rebuild.`,
		DisableFlagParsing: true,
	}
	cmd.RunE = func(cobraCmd *cobra.Command, args []string) error {
		// DisableFlagParsing means cobra doesn't see --help / -h; if we
		// don't intercept it ourselves we'd shell out to bootstrap with
		// "--help" as an arg, which is not what the user asked for.
		for _, a := range args {
			if a == "--help" || a == "-h" {
				return cobraCmd.Help()
			}
		}
		return execBootstrap(cfg, args)
	}
	return cmd
}

// newCheckUpdateCmd builds `melvin-config check-update`. Designed
// to be called from shell startup (shared_config/base.zshrc): exits
// silently when the last check was less than updateCheckInterval
// ago, otherwise refreshes the stamp (BEFORE prompting, so
// concurrent shells don't all queue up the same prompt) and asks
// the user whether to update.
//
// Splitting this from `update` keeps the explicit
// `melvin-config update` invocation prompt-free.
func newCheckUpdateCmd(cfg *appConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check-update",
		Short: "Prompt to update when 14+ days have passed (shell-startup hook)",
		Args:  cobra.NoArgs,
	}
	cmd.RunE = func(_ *cobra.Command, _ []string) error {
		return runCheckUpdate(cfg, time.Now())
	}
	return cmd
}

// runCheckUpdate reads the staleness stamp, returns silently when
// the last check was recent, otherwise refreshes the stamp and
// prompts (Y/N). On Y, replaces the process with the bootstrap
// via execBootstrap; on N (or anything else parsed as no), returns
// nil so the shell startup continues.
//
// Non-interactive shells (script-spawned, `ssh host cmd`, IDE-
// spawned) skip both the stamp and the prompt: prompting an EOF
// stdin would surface as an error every shell startup, and burning
// the 14-day window on shells that can't actually answer would
// suppress the prompt for real interactive shells too.
//
// For interactive shells, the stamp is written BEFORE the prompt
// so two shells opening at the same time don't both prompt: the
// second sees the fresh stamp and exits silently. The shell-side
// equivalent had the same shape — keeping it on the Go side
// preserves the behavior.
func runCheckUpdate(cfg *appConfig, now time.Time) error {
	configDir, err := resolveConfigDir(cfg)
	if err != nil {
		return err
	}
	stampPath := filepath.Join(configDir, updateStampFile)

	last := readUpdateStamp(stampPath)
	if now.Sub(last) < updateCheckInterval {
		return nil
	}

	if !isInteractive(cfg.streams.In) {
		return nil
	}

	if err := writeUpdateStamp(stampPath, now); err != nil {
		return fmt.Errorf("stamp update check: %w", err)
	}

	yes, err := promptYesNo(cfg.streams.In, cfg.streams.Err, "Would you like to update your config?")
	if err != nil {
		return fmt.Errorf("prompt: %w", err)
	}
	if !yes {
		return nil
	}
	return execBootstrap(cfg, nil)
}

// readUpdateStamp returns the time recorded in the stamp file.
// Returns the zero time when the file is missing or malformed —
// callers treat that as "long overdue" so the next prompt fires
// regardless of whether we've ever stamped before.
func readUpdateStamp(path string) time.Time {
	data, err := os.ReadFile(path) //nolint:gosec // configDir-relative path; written only by this binary
	if err != nil {
		return time.Time{}
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(n, 0)
}

// writeUpdateStamp records t as a unix-seconds integer (newline-
// terminated for line-tool friendliness) at path.
func writeUpdateStamp(path string, t time.Time) error {
	contents := strconv.FormatInt(t.Unix(), 10) + "\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil { //nolint:gosec // conventional config-file perms
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// promptYesNo asks `<prompt> (Y/N): ` and reads the first non-
// blank line of input. y/Y → true, n/N → false; anything else
// prints a retry hint and loops. EOF on a blank first read
// surfaces as an error so callers (e.g. a piped invocation with
// no input) don't silently take the "no" branch.
func promptYesNo(in io.Reader, out io.Writer, prompt string) (bool, error) {
	r := bufio.NewReader(in)
	for {
		if _, err := fmt.Fprintf(out, "%s (Y/N): ", prompt); err != nil {
			return false, fmt.Errorf("write prompt: %w", err)
		}
		line, err := r.ReadString('\n')
		if err != nil && line == "" {
			return false, errors.New("no input")
		}
		line = strings.TrimSpace(line)
		if line == "" {
			// Blank-but-not-EOF (terminal user pressed Enter); re-prompt.
			continue
		}
		switch line[0] {
		case 'y', 'Y':
			return true, nil
		case 'n', 'N':
			return false, nil
		}
		if _, err := fmt.Fprintln(out, "Please answer Y or N."); err != nil {
			return false, fmt.Errorf("write hint: %w", err)
		}
	}
}
