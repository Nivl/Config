package configgen

import (
	"context"
	"os/exec"
)

// CmdRunner abstracts shellouts so unit tests can inject deterministic
// errors (e.g. "no such process" from killall) without touching the
// real system.
type CmdRunner interface {
	Run(ctx context.Context, name string, args ...string) error
}

// realCmdRunner is the production implementation. Unexported so
// consumers always go through the CmdRunner interface.
type realCmdRunner struct{}

// Run executes the command via exec.CommandContext. Returns the
// command's exit error verbatim (including *exec.ExitError) so
// callers can inspect ExitCode for branch logic.
func (realCmdRunner) Run(ctx context.Context, name string, args ...string) error {
	return exec.CommandContext(ctx, name, args...).Run() //nolint:gosec // command names are package-level constants in SetupGpg
}

// NewCmdRunner returns the production CmdRunner. Tests should construct
// a FakeCmdRunner from internal/configgen/configgentest instead.
func NewCmdRunner() CmdRunner { return realCmdRunner{} }
