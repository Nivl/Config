package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Nivl/config/internal/appsetup"
	"github.com/Nivl/config/internal/brew"
	"github.com/Nivl/config/internal/claude/sync"
	"github.com/Nivl/config/internal/claude/sync/prompt"
	"github.com/Nivl/config/internal/configgen"
	"github.com/Nivl/config/internal/dotfiles"
	"github.com/Nivl/config/internal/dryrun"
	"github.com/Nivl/config/internal/packages"
	"github.com/Nivl/config/internal/userinput"

	"github.com/spf13/cobra"
)

// newSetupCmd builds the `melvin-config setup` subcommand. The RunE
// wrapper resolves all 7 config knobs (flag → env → fallback/prompt)
// and passes the resulting values down via setupParams; the body
// never touches process env.
func newSetupCmd(cfg *appConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Run the full bootstrap",
		Args:  cobra.NoArgs,
	}
	personal := cmd.Flags().Bool("personal", false, "Personal computer (overrides PERSONAL_COMPUTER env)")
	mergeResolution := cmd.Flags().String("merge-resolution", "", "Pre-resolve claude merge conflicts: keep-local|take-remote|skip (overrides CLAUDE_MERGE_RESOLUTION env)")
	homebrewPrefix := cmd.Flags().String("homebrew-prefix", "", "Homebrew prefix path (overrides HOMEBREW_PREFIX env; falls back to $(brew --prefix))")
	devRoot := cmd.Flags().String("dev-root", "", "Root directory for dev repos (overrides DEV_ROOT env)")
	gitOrg := cmd.Flags().String("git-org", "", "GitHub org / user (overrides GIT_CLONE_USER_NAME env)")
	gitHost := cmd.Flags().String("git-host", "", "Git SSH host (overrides GIT_HOST env)")
	dryRun := cmd.Flags().Bool("dry-run", false,
		"preview changes without applying them (also MELVIN_DRY_RUN env)")

	cmd.RunE = func(cobraCmd *cobra.Command, _ []string) error {
		p := setupParams{
			personalArg: resolveBoolAsString(cobraCmd, "personal", "PERSONAL_COMPUTER", *personal),
		}
		var err error
		if p.mergeResolution, err = resolveString(cobraCmd, "merge-resolution", "CLAUDE_MERGE_RESOLUTION", *mergeResolution, nil); err != nil {
			return fmt.Errorf("resolve merge-resolution flag: %w", err)
		}
		if err := validateMergeResolution(p.mergeResolution); err != nil {
			return fmt.Errorf("validate merge resolution: %w", err)
		}
		ctx := cobraCmd.Context()
		if p.homebrewPrefix, err = resolveString(cobraCmd, "homebrew-prefix", "HOMEBREW_PREFIX", *homebrewPrefix, func() (string, error) {
			return brewPrefixFallback(ctx)
		}); err != nil {
			return fmt.Errorf("resolve homebrew-prefix flag: %w", err)
		}
		if p.devRoot, err = resolveString(cobraCmd, "dev-root", "DEV_ROOT", *devRoot, nil); err != nil {
			return fmt.Errorf("resolve dev-root flag: %w", err)
		}
		if p.gitOrg, err = resolveString(cobraCmd, "git-org", "GIT_CLONE_USER_NAME", *gitOrg, nil); err != nil {
			return fmt.Errorf("resolve git-org flag: %w", err)
		}
		if p.gitHost, err = resolveString(cobraCmd, "git-host", "GIT_HOST", *gitHost, nil); err != nil {
			return fmt.Errorf("resolve git-host flag: %w", err)
		}
		resolvedDryRun, err := resolveBool(cobraCmd, "dry-run", "MELVIN_DRY_RUN", *dryRun, nil)
		if err != nil {
			return fmt.Errorf("resolve dry-run flag: %w", err)
		}
		cfg.dryRun = resolvedDryRun
		if resolvedDryRun {
			cfg.reporter = dryrun.NewReporter(cfg.streams.Err)
		}
		return setupCmd(cobraCmd.Context(), cfg, p)
	}
	return cmd
}

// setupParams carries the resolved (flag-or-env-or-fallback) values
// setupCmd needs. Resolution happens in the RunE wrapper so the body
// is decoupled from cobra, env, and the brew shellout.
type setupParams struct {
	// personalArg is a tri-state string: "" means "neither flag nor
	// env set; userinput.Personal should prompt"; "true" or "false"
	// means "skip the prompt, use this value verbatim." The bool form
	// is derived inside setupCmd after the prompt helper normalizes
	// the value.
	personalArg string
	// mergeResolution is empty when no override is requested.
	mergeResolution string
	// homebrewPrefix is always populated by RunE: flag → env →
	// brewPrefixFallback (which shells to `brew --prefix`). If even
	// that errors, RunE surfaces the error and setupCmd is never
	// entered.
	homebrewPrefix string
	// devRoot / gitOrg / gitHost are empty when neither flag nor env
	// is set; the corresponding userinput.X prompt will fire.
	devRoot string
	gitOrg  string
	gitHost string
}

// brewPrefixFallback shells out to `brew --prefix` when
// HOMEBREW_PREFIX is provided by neither flag nor env. This is the
// lazy default: it only runs when nothing else resolved, so callers
// that explicitly pass --homebrew-prefix or set the env never pay the
// shellout cost (or fail when brew isn't on PATH). The ctx wires
// SIGINT/SIGTERM through to the brew process via CommandContext.
func brewPrefixFallback(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "brew", "--prefix").Output()
	if err != nil {
		return "", fmt.Errorf("brew --prefix: %w (set HOMEBREW_PREFIX or pass --homebrew-prefix)", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// setupCmd is the body of `melvin-config setup`. In order, it:
//
//	resolves user inputs (internal/userinput);
//	installs packages (internal/packages + brew);
//	runs the full claude config sync (internal/claude/sync);
//	copies dotfiles + generates config files (internal/dotfiles,
//	  internal/configgen);
//	performs final application setup — SetupSSH + SetupGitHub
//	  (internal/appsetup);
//	writes the success stamp at ~/.melvin/config/.last_update_check;
//	prints the post-install reminder list + zshrc-reload hint.
//
// install.bootstrap.sh remains as the chicken-and-egg shim (homebrew
// install + git + go + clone + binary build + exec melvin-config).
// Everything else is Go.
//
// All env-backed config knobs flow in via setupParams (populated in
// the RunE wrapper). The body reads neither env vars nor cobra flags
// directly.
func setupCmd(ctx context.Context, cfg *appConfig, p setupParams) error {
	stdin := cfg.streams.In
	stdout := cfg.streams.Out
	stderr := cfg.streams.Err

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home: %w", err)
	}
	configDir := cfg.configDir
	if configDir == "" {
		configDir = filepath.Join(home, ".melvin", "config")
	}

	// Resolve user inputs. Each userinput.X helper short-circuits when
	// its prearg is non-empty (flag/env already set the answer) and
	// otherwise prompts.
	personalStr, err := userinput.Personal(p.personalArg, stdin, stderr, home, cfg.dryRun, cfg.reporter)
	if err != nil {
		return fmt.Errorf("personal: %w", err)
	}
	personal := personalStr == "true"
	devRoot, err := userinput.DevRoot(p.devRoot, stdin, stderr, home)
	if err != nil {
		return fmt.Errorf("dev root: %w", err)
	}
	gitOrg, err := userinput.GitOrg(p.gitOrg, stdin, stderr, personal)
	if err != nil {
		return fmt.Errorf("git org: %w", err)
	}
	gitHost, err := userinput.GitHost(p.gitHost, stdin, stderr)
	if err != nil {
		return fmt.Errorf("git host: %w", err)
	}

	// Install packages first.
	cfg.reporter.Section("packages")
	brewRunner := cfg.newBrewRunner(cfg.streams)
	if cfg.dryRun {
		brewRunner = brew.NewDryRunWrapper(brewRunner, cfg.reporter)
	}
	summary, err := packages.InstallWithRetry(ctx, stdin, stdout, brewRunner, packages.Opts{
		Personal: personal,
	})
	if err != nil {
		return fmt.Errorf("install packages: %w", err)
	}
	summary.Print(stdout)

	// Run the full claude config sync. Sync owns precommit hook
	// install, both modes, and last-sync-commit advancement.
	cfg.reporter.Section("claude sync")
	claudePaths, err := resolveClaudePaths(cfg)
	if err != nil {
		return fmt.Errorf("resolve claude paths: %w", err)
	}
	mode := sync.ModeCopy
	if personal {
		mode = sync.ModeSymlink
	}
	// Progress lines, prompts, and warnings go to stderr.
	claudePrompter := prompt.NewPrompter(claudePaths, stdin, stderr, p.mergeResolution,
		cfg.dryRun, cfg.reporter)
	_, err = cfg.newClaudeSync(ctx, claudePaths, sync.Options{
		Mode:     mode,
		Out:      stderr,
		Prompter: claudePrompter,
		DryRun:   cfg.dryRun,
		Reporter: cfg.reporter,
	})
	if err != nil {
		return fmt.Errorf("claude sync: %w", err)
	}

	// Copy dotfiles + write the materialized config files.
	cfg.reporter.Section("dotfiles")
	if err := dotfiles.CopyConfigFiles(ctx, configDir, home, stderr, cfg.dryRun, cfg.reporter); err != nil {
		return fmt.Errorf("copy config files: %w", err)
	}
	cfg.reporter.Section("configgen")
	if err := configgen.SetupZshrc(home, configDir, configgen.ZshrcOpts{
		PersonalComputer: personalStr,
		DevRoot:          devRoot,
		GitHost:          gitHost,
		GitCloneUserName: gitOrg,
	}, cfg.dryRun, cfg.reporter); err != nil {
		return fmt.Errorf("setup zshrc: %w", err)
	}
	if err := configgen.SetupGitconfig(home, configDir, personal, cfg.dryRun, cfg.reporter); err != nil {
		return fmt.Errorf("setup gitconfig: %w", err)
	}
	// SetupGpg branches on cfg.dryRun internally (see setupGpgDryRun)
	// and never touches the runner under dryRun=true, so no wrap is
	// needed here.
	if err := configgen.SetupGpg(ctx, home, p.homebrewPrefix, cfg.newConfiggenRunner(), cfg.dryRun, cfg.reporter); err != nil {
		return fmt.Errorf("setup gpg: %w", err)
	}

	// Final application setup.
	cfg.reporter.Section("appsetup")
	appRunner := cfg.newAppRunner(cfg.streams)
	if cfg.dryRun {
		appRunner = appsetup.NewDryRunCmdRunner(appRunner, cfg.reporter)
	}
	if err := appsetup.SetupSSH(ctx, home, appRunner, cfg.dryRun, cfg.reporter); err != nil {
		return fmt.Errorf("setup ssh: %w", err)
	}
	githubAuthed, err := appsetup.SetupGitHub(ctx,
		appsetup.NewYesNoPrompter(stdin, stderr), appRunner)
	if err != nil {
		return fmt.Errorf("setup github: %w", err)
	}

	// Write the success stamp (unix seconds, newline-terminated). The
	// stamp is consumed by `melvin-config check-update` (invoked from
	// base.zshrc on shell startup) to gate the 14-day update prompt.
	stampPath := filepath.Join(configDir, ".last_update_check")
	stamp := strconv.FormatInt(time.Now().Unix(), 10) + "\n"
	existing, _ := os.ReadFile(stampPath) //nolint:gosec // success stamp under configDir, read-by-design
	cfg.reporter.FileChange(stampPath, existing, []byte(stamp), "advance update check")
	if !cfg.dryRun {
		if err := os.WriteFile(stampPath, []byte(stamp), 0o644); err != nil { //nolint:gosec // 0o644 success-stamp default
			return fmt.Errorf("write %s: %w", stampPath, err)
		}
	}

	// Dry-run summary line. NullReporter no-ops this in production.
	cfg.reporter.FinalSummary()

	// Post-install reminder + zshrc-reload hint. Sourcing a fresh
	// zshrc from a child process can't affect the user's interactive
	// shell; the stderr hint achieves the same practical effect.
	appsetup.PrintRemainingTasks(stderr, home, githubAuthed)
	_, _ = fmt.Fprintln(stderr, "\nRun \"source ~/.zshrc\" to pick up the new environment.")

	// If any casks failed earlier, exit non-zero at the end so the
	// user-facing reminder list still gets printed first.
	if summary.HasFailures() {
		return errors.New("some casks failed to install (see summary above)")
	}
	return nil
}
