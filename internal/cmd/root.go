// Package cmd hosts the cobra subcommand graph for the melvin-config
// binary. The actual binary entrypoint lives in cmd/melvin-config/main.go;
// this package factors out the subcommand wiring so tests can construct
// the root command + inject fakes without going through os.Args.
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/Nivl/config/internal/appsetup"
	"github.com/Nivl/config/internal/brew"
	"github.com/Nivl/config/internal/claude/sync"
	"github.com/Nivl/config/internal/claude/sync/state"
	"github.com/Nivl/config/internal/configgen"
	"github.com/Nivl/config/internal/dryrun"
	"github.com/Nivl/config/internal/iox"

	"github.com/spf13/cobra"
)

// appConfig holds dependencies and shared process-level state accessible
// to every subcommand. Constructors (newXxxCmd) capture this by closure;
// business-logic functions take it as a param alongside context.Context.
// I/O streams live on it as an iox.Streams value (wired by NewRoot) so
// every subcommand body reads them the same way without an extra
// argument.
type appConfig struct { //nolint:govet // fieldalignment: logical grouping (streams neighbours dryRun+reporter; config strings at the end) takes priority over struct packing.
	// newBrewRunner is the brew.Runner factory. Defaults to brew.NewRunner;
	// tests substitute one returning a *brewtest.FakeRunner. Receives the
	// command's streams bundle so brew subprocesses inherit the same
	// writers as the rest of the subcommand.
	newBrewRunner func(iox.Streams) brew.Runner
	// newClaudeSync is the sync.Sync factory. Defaults to a
	// wrapper around sync.Sync; tests substitute a stub. Same
	// pattern as newBrewRunner above.
	newClaudeSync func(ctx context.Context, paths state.Paths, opts sync.Options) (sync.Summary, error)
	// newAppRunner is the appsetup.CmdRunner factory used by
	// SetupSSH + SetupGitHub. Defaults to appsetup.NewCmdRunner;
	// tests substitute a FakeCmdRunner from appsetup/appsetuptest.
	// Same factory-injection pattern as newBrewRunner so the
	// appsetup phase of setupCmd is uniformly mockable.
	newAppRunner func(iox.Streams) appsetup.CmdRunner
	// newConfiggenRunner is the configgen.CmdRunner factory used by
	// SetupGpg (for the killall/daemon shellouts). Defaults to
	// configgen.NewCmdRunner; tests substitute a FakeCmdRunner from
	// configgen/configgentest. The factory takes no args because
	// configgen.NewCmdRunner doesn't take a streams bundle — gpg
	// subprocess output is intentionally swallowed.
	newConfiggenRunner func() configgen.CmdRunner
	// streams is the In/Out/Err triple wired into every subprocess,
	// prompt, and progress writer. Populated by NewRoot.
	streams iox.Streams
	// dryRun is set by subcommand RunE wrappers when --dry-run /
	// MELVIN_DRY_RUN resolves true. Threaded down into subsystem
	// entry points so chokepoints can suppress side effects and
	// route through reporter instead.
	dryRun bool
	// reporter is dryrun.NewNullReporter() in production runs and
	// dryrun.NewReporter(streams.Err) in dry-run runs. Chokepoint
	// code calls its methods unconditionally; the Null impl makes
	// production silent.
	reporter dryrun.Reporter
	// cwd is the working directory at process start.
	cwd string
	// configDir overrides the default ~/.melvin/config path. Empty in
	// production; tests set it to a tempdir.
	configDir string
	// version is the build-time version string (set via -ldflags on
	// main.version, passed through from main into NewRoot). Surfaced by
	// `melvin-config version`.
	version string
}

// NewRoot builds the root cobra command. cwd is the process working
// directory; streams is the I/O bundle the binary will read and print
// to (production wires iox.System()); version is the build-time
// version string (consumed by the `version` subcommand). Streams are
// attached via cobra's SetIn/SetOut/SetErr so every subcommand can
// reach them through InOrStdin/OutOrStdout/ErrOrStderr.
func NewRoot(version, cwd string, streams iox.Streams) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "melvin-config",
		Short:         "bootstrap and config management for ~/.melvin/config",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd.SetIn(streams.In)
	cmd.SetOut(streams.Out)
	cmd.SetErr(streams.Err)
	cfg := &appConfig{
		cwd:                cwd,
		configDir:          "",
		version:            version,
		streams:            streams,
		newBrewRunner:      brew.NewRunner,
		newClaudeSync:      sync.Sync,
		newAppRunner:       appsetup.NewCmdRunner,
		newConfiggenRunner: configgen.NewCmdRunner,
		dryRun:             false,
		reporter:           dryrun.NewNullReporter(),
	}
	cmd.PersistentFlags().StringVar(&cfg.configDir, "config-dir", "", "override the default config directory")

	cmd.AddCommand(newInstallCmd(cfg))
	cmd.AddCommand(newClaudeCmd(cfg))
	cmd.AddCommand(newSetupCmd(cfg))
	cmd.AddCommand(newUpdateCmd(cfg))
	cmd.AddCommand(newCheckUpdateCmd(cfg))
	cmd.AddCommand(newVersionCmd(cfg))
	return cmd
}

// resolveBool resolves a bool-typed config knob backed by a flag + env
// var fallback chain. Precedence:
//  1. Explicit flag (.Changed) — returned verbatim, so --personal=false
//     wins over PERSONAL_COMPUTER=true.
//  2. Non-empty env var — compared strictly against "true".
//  3. fallback() — invoked only when neither flag nor env is set;
//     allowed to error. Pass nil for "no fallback; return false."
//
// resolveBool never mutates process state — the resolved value is
// returned for the caller to thread through its own params.
func resolveBool(cmd *cobra.Command, flagName, envName string, flagValue bool, fallback func() (bool, error)) (bool, error) {
	if cmd.Flag(flagName).Changed {
		return flagValue, nil
	}
	if env := os.Getenv(envName); env != "" {
		return env == "true", nil
	}
	if fallback == nil {
		return false, nil
	}
	return fallback()
}

// resolveBoolAsString is for callers (currently setup's userinput
// integration) that want a tri-state string back rather than a bool:
// "" means "neither flag nor env set; downstream should prompt or
// default", "true"/"false" means "use this exact value."
//
// This exists because the userinput.X prompt helpers take their
// pre-resolved value as a string with "" as the sentinel for "prompt
// the user." Funnelling a bool through that interface would require
// either a *bool prearg or a separate "explicit?" flag — string is
// simpler.
func resolveBoolAsString(cmd *cobra.Command, flagName, envName string, flagValue bool) string { //nolint:unparam // flagName/envName always ("personal","PERSONAL_COMPUTER") for now; the bool-resolver / userinput bridge stays parameterized for future tri-state knobs.
	if cmd.Flag(flagName).Changed {
		if flagValue {
			return "true"
		}
		return "false"
	}
	return os.Getenv(envName)
}

// resolveString is the string analogue of resolveBool. Empty flag
// values are treated as "not set" (cobra defaults string flags to "")
// so an env value can still win when the user didn't pass the flag.
// Pass nil for "no fallback; return empty string." The fallback path
// is the right place to put expensive operations like a $(brew
// --prefix) shellout: it runs only when nothing else resolved.
func resolveString(cmd *cobra.Command, flagName, envName, flagValue string, fallback func() (string, error)) (string, error) {
	if cmd.Flag(flagName).Changed && flagValue != "" {
		return flagValue, nil
	}
	if env := os.Getenv(envName); env != "" {
		return env, nil
	}
	if fallback == nil {
		return "", nil
	}
	return fallback()
}

// newVersionCmd builds the `melvin-config version` subcommand.
func newVersionCmd(cfg *appConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version info",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if _, err := cfg.streams.Out.Write([]byte("melvin-config " + cfg.version + "\n")); err != nil {
				return fmt.Errorf("write version: %w", err)
			}
			return nil
		},
	}
}
