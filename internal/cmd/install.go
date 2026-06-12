package cmd

import (
	"context"
	"fmt"

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
// PERSONAL_COMPUTER (flag wins via .Changed) and passes the resulting
// bool down through installPackagesParams; the body never reads env.
func newInstallPackagesCmd(cfg *appConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "packages",
		Short: "Install Homebrew formulae and casks",
		Args:  cobra.NoArgs,
	}
	personal := cmd.Flags().Bool("personal", false, "Include personal casks (overrides PERSONAL_COMPUTER env)")
	cmd.RunE = func(cobraCmd *cobra.Command, _ []string) error {
		resolvedPersonal, err := resolveBool(cobraCmd, "personal", "PERSONAL_COMPUTER", *personal, nil)
		if err != nil {
			return fmt.Errorf("resolve personal flag: %w", err)
		}
		return installPackagesCmd(cobraCmd.Context(), cfg, installPackagesParams{
			personal: resolvedPersonal,
		})
	}
	return cmd
}

// installPackagesParams carries the resolved (flag-or-env-or-default)
// values that installPackagesCmd needs. Resolution happens in the RunE
// wrapper so the body is decoupled from cobra and env.
type installPackagesParams struct {
	// personal controls whether PersonalCasks (proton-*, daisydisk, …)
	// are included. Resolved from --personal / PERSONAL_COMPUTER.
	personal bool
}

// installPackagesCmd installs homebrew packages: brew upgrade,
// formulae, then casks — every step limps past per-package failures.
// When failures remain, InstallWithRetry prompts the user to abort,
// retry just the failed packages, or ignore and continue. Returns a
// non-nil error for catastrophic failures (brew binary missing,
// context cancelled), an explicit abort (packages.ErrAborted), or a
// prompt that could not be answered (stdin EOF). An answered "ignore"
// exits 0 — the user accepted the failures.
func installPackagesCmd(ctx context.Context, cfg *appConfig, p installPackagesParams) error {
	summary, err := packages.InstallWithRetry(ctx, cfg.streams.In, cfg.streams.Out,
		cfg.newBrewRunner(cfg.streams), packages.Opts{
			Personal: p.personal,
		})
	if err != nil {
		return fmt.Errorf("install packages: %w", err)
	}
	summary.Print(cfg.streams.Out)
	return nil
}
