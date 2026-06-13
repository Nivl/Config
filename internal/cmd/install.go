package cmd

import (
	"context"
	"fmt"
	"slices"

	"github.com/Nivl/config/internal/packages"

	"github.com/spf13/cobra"
)

// newInstallCmd builds the `melvin-config install` parent command. It has
// no body of its own — it only hosts the per-subsystem subcommands.
func newInstallCmd(cfg *appConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install or update components",
	}
	cmd.AddCommand(newInstallPackagesCmd(cfg))
	return cmd
}

// newInstallPackagesCmd builds the `melvin-config install packages`
// subcommand. The RunE wrapper resolves --personal against
// PERSONAL_COMPUTER and --install-failure-resolution against
// INSTALL_FAILURE_RESOLUTION (flag wins via .Changed) and passes the
// resolved values down through installPackagesParams; the body never
// reads env.
func newInstallPackagesCmd(cfg *appConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "packages",
		Short: "Install Homebrew formulae and casks",
		Args:  cobra.NoArgs,
	}
	personal := cmd.Flags().Bool("personal", false, "Include personal casks (overrides PERSONAL_COMPUTER env)")
	failureResolution := cmd.Flags().String("install-failure-resolution", "",
		"Pre-answer the failed-packages prompt: abort|retry|ignore (overrides INSTALL_FAILURE_RESOLUTION env)")
	cmd.RunE = func(cobraCmd *cobra.Command, _ []string) error {
		resolvedPersonal, err := resolveBool(cobraCmd, "personal", "PERSONAL_COMPUTER", *personal, nil)
		if err != nil {
			return fmt.Errorf("resolve personal flag: %w", err)
		}
		resolvedFailureResolution, err := resolveString(cobraCmd, "install-failure-resolution",
			"INSTALL_FAILURE_RESOLUTION", *failureResolution, nil)
		if err != nil {
			return fmt.Errorf("resolve install-failure-resolution flag: %w", err)
		}
		if err := validateInstallFailureResolution(resolvedFailureResolution); err != nil {
			return fmt.Errorf("validate install failure resolution: %w", err)
		}
		return installPackagesCmd(cobraCmd.Context(), cfg, installPackagesParams{
			personal:          resolvedPersonal,
			failureResolution: resolvedFailureResolution,
		})
	}
	return cmd
}

// validInstallFailureResolutions is the closed set accepted by
// --install-failure-resolution / INSTALL_FAILURE_RESOLUTION.
var validInstallFailureResolutions = []string{"abort", "retry", "ignore"}

// validateInstallFailureResolution rejects values outside the closed
// set when the value is non-empty — empty means "prompt interactively."
func validateInstallFailureResolution(v string) error {
	if v == "" || slices.Contains(validInstallFailureResolutions, v) {
		return nil
	}
	return fmt.Errorf("--install-failure-resolution: %q is not one of %v", v, validInstallFailureResolutions)
}

// installPackagesParams carries the resolved (flag-or-env-or-default)
// values that installPackagesCmd needs. Resolution happens in the RunE
// wrapper so the body is decoupled from cobra and env.
type installPackagesParams struct {
	// failureResolution pre-answers the failed-packages prompt (one of
	// abort|retry|ignore; "" means "prompt interactively"). Resolved
	// from --install-failure-resolution / INSTALL_FAILURE_RESOLUTION
	// and validated by validateInstallFailureResolution.
	failureResolution string
	// personal controls whether PersonalCasks (proton-*, daisydisk, …)
	// are included. Resolved from --personal / PERSONAL_COMPUTER.
	personal bool
}

// installPackagesCmd installs homebrew packages: brew upgrade,
// formulae, then casks — every step limps past per-package failures.
// When failures remain, InstallWithRetry prompts the user (on stderr,
// like every sibling prompt) to abort, retry just the failed packages,
// or ignore and continue. Returns a non-nil error for catastrophic
// failures (brew binary missing or unrunnable, context cancelled), an
// explicit abort (packages.ErrAborted), or a prompt that could not be
// answered (stdin EOF). An answered "ignore" exits 0 — the user
// accepted the failures. Only the skipped-casks trailer prints at the
// end: the failure list is owned by the prompt loop (a catastrophic
// abort surfaces only the error — the loop either never ran, or
// already listed the failures before the retry that died).
func installPackagesCmd(ctx context.Context, cfg *appConfig, p installPackagesParams) error {
	summary, err := packages.InstallWithRetry(ctx, cfg.streams.In, cfg.streams.Out, cfg.streams.Err,
		cfg.newBrewRunner(cfg.streams), packages.Opts{
			Personal:          p.personal,
			FailureResolution: p.failureResolution,
		})
	// The trailer is informational and its casks really were skipped —
	// show it however the run ended (abort, prompt EOF, catastrophic
	// failure included).
	summary.PrintSkipped(cfg.streams.Out)
	if err != nil {
		return fmt.Errorf("install packages: %w", err)
	}
	return nil
}
