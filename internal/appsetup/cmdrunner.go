package appsetup

import (
	"context"
	"os/exec"

	"github.com/Nivl/config/internal/dryrun"
	"github.com/Nivl/config/internal/iox"
)

// CmdRunner abstracts shellouts so unit tests can inject deterministic
// behavior without spawning real processes. Distinct from
// internal/configgen/CmdRunner in two ways:
//  1. Run inherits stdio in production (ssh-keygen's passphrase
//     prompt and `gh auth login -w`'s browser flow need it).
//  2. Capture returns stdout bytes for non-interactive commands like
//     `gh auth status --json hosts`.
type CmdRunner interface {
	// Run invokes name with args, inheriting Stdin/Stdout/Stderr from
	// the parent process. Returns the command's exit error verbatim
	// (including *exec.ExitError).
	Run(ctx context.Context, name string, args ...string) error
	// Capture invokes name with args and returns stdout as bytes.
	// Stderr inherits (informational output). Returns the command's
	// exit error verbatim on failure.
	Capture(ctx context.Context, name string, args ...string) ([]byte, error)
}

// realCmdRunner is the production CmdRunner. Unexported so consumers
// always go through the interface. Carries the streams bundle that
// gets wired into each subprocess.
type realCmdRunner struct {
	streams iox.Streams
}

// Run implements CmdRunner. Wires Stdin/Stdout/Stderr from the bundle
// so interactive prompts (ssh-keygen passphrase, gh auth login -w
// browser flow) work end-to-end with the user's terminal in production
// and with test buffers in tests.
func (r realCmdRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // commands are package-level literals in callers
	cmd.Stdin = r.streams.In
	cmd.Stdout = r.streams.Out
	cmd.Stderr = r.streams.Err
	return cmd.Run()
}

// Capture implements CmdRunner. Returns stdout bytes; stderr passes
// through to the bundle's Err writer for informational output (e.g.
// gh auth status writes warnings to stderr while its JSON goes to
// stdout).
func (r realCmdRunner) Capture(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // commands are package-level literals in callers
	cmd.Stderr = r.streams.Err
	return cmd.Output()
}

// NewCmdRunner returns the production CmdRunner with the given streams
// bundle wired into every subprocess it spawns. Tests should construct
// a FakeCmdRunner from internal/appsetup/appsetuptest instead.
func NewCmdRunner(streams iox.Streams) CmdRunner {
	return realCmdRunner{streams: streams}
}

// dryRunCmdRunner wraps a real CmdRunner: Run is suppressed +
// reported; Capture delegates to the wrapped runner so reads (gh
// auth status, etc.) still produce real data for dry-run output.
type dryRunCmdRunner struct {
	wrapped  CmdRunner
	reporter dryrun.Reporter
}

// NewDryRunCmdRunner returns a CmdRunner that no-ops every Run call
// (reporting it via the supplied Reporter) and delegates every
// Capture call to the wrapped runner.
func NewDryRunCmdRunner(wrapped CmdRunner, reporter dryrun.Reporter) CmdRunner {
	return &dryRunCmdRunner{wrapped: wrapped, reporter: reporter}
}

// Run reports the subprocess invocation and returns nil without
// invoking exec.
func (r *dryRunCmdRunner) Run(_ context.Context, name string, args ...string) error {
	r.reporter.Shellout(name, args, "appsetup subprocess")
	return nil
}

// Capture delegates to the wrapped runner — reads run normally so
// dry-run output reflects the actual state of the system.
func (r *dryRunCmdRunner) Capture(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.wrapped.Capture(ctx, name, args...)
}
