package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"

	"github.com/Nivl/config/internal/claude/sync"
	"github.com/Nivl/config/internal/claude/sync/prompt"
	"github.com/Nivl/config/internal/claude/sync/state"
)

// validMergeResolutions is the closed set of values
// CLAUDE_MERGE_RESOLUTION (and --merge-resolution) may take. Mirrors
// the case statement in parseMergeResolution (internal/claude/sync/
// prompt/envoverride.go).
var validMergeResolutions = []string{"keep-local", "take-remote", "skip"}

// newClaudeCmd builds the `melvin-config claude` parent command. It has
// no body of its own — it only hosts the per-subsystem subcommands.
func newClaudeCmd(cfg *appConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claude",
		Short: "Claude Code config sync",
	}
	cmd.AddCommand(newClaudeSyncCmd(cfg))
	cmd.AddCommand(newPermsCmd(cfg))
	return cmd
}

// newClaudeSyncCmd builds `melvin-config claude sync`, which runs the
// full claude sync pipeline (precommit hook in both modes; then
// symlink-install or copy-mode merges + last-sync-commit advance). All
// three flags resolve (flag → env → default) in the RunE wrapper and
// flow down to claudeSyncCmd via claudeSyncParams.
func newClaudeSyncCmd(cfg *appConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync Claude Code config from the repo to ~/.claude",
		Args:  cobra.NoArgs,
	}
	personal := cmd.Flags().Bool("personal", false, "Force symlink mode (overrides PERSONAL_COMPUTER env)")
	mergeResolution := cmd.Flags().String("merge-resolution", "", "Pre-resolve merge conflicts: keep-local|take-remote|skip (overrides CLAUDE_MERGE_RESOLUTION env)")
	cmd.RunE = func(cobraCmd *cobra.Command, _ []string) error {
		resolvedPersonal, err := resolveBool(cobraCmd, "personal", "PERSONAL_COMPUTER", *personal, nil)
		if err != nil {
			return fmt.Errorf("resolve personal flag: %w", err)
		}
		resolvedMergeResolution, err := resolveString(cobraCmd, "merge-resolution", "CLAUDE_MERGE_RESOLUTION", *mergeResolution, nil)
		if err != nil {
			return fmt.Errorf("resolve merge-resolution flag: %w", err)
		}
		if err := validateMergeResolution(resolvedMergeResolution); err != nil {
			return fmt.Errorf("validate merge resolution: %w", err)
		}
		return claudeSyncCmd(cobraCmd.Context(), cfg, claudeSyncParams{
			personal:        resolvedPersonal,
			mergeResolution: resolvedMergeResolution,
		})
	}
	return cmd
}

// validateMergeResolution rejects values outside the closed set when
// the flag is non-empty. Cobra has no built-in enum validator that
// also tolerates "unset"; this is the smallest custom check that does.
func validateMergeResolution(v string) error {
	if v == "" || slices.Contains(validMergeResolutions, v) {
		return nil
	}
	return fmt.Errorf("--merge-resolution: %q is not one of %v", v, validMergeResolutions)
}

// claudeSyncParams carries the resolved values claudeSyncCmd needs.
// Resolution happens in the RunE wrapper; this struct is the contract
// between the cmd glue and the body, decoupling the body from cobra
// and env.
type claudeSyncParams struct {
	// mergeResolution pre-resolves copy-mode merge conflicts (one of
	// keep-local|take-remote|skip; "" means "prompt interactively").
	// Validated by validateMergeResolution at flag-parse time.
	mergeResolution string
	// personal forces symlink mode when true; otherwise copy mode.
	personal bool
}

// claudeSyncCmd executes the claude sync. Copy mode runs settings +
// top-files + dir merges; Symlink mode installs symlinks for the
// curated items. Returns a non-nil error only for catastrophic
// failures (a user-requested Skip is not an error).
//
// stdin (read by the conflict + existing-file prompters) and stderr
// (progress lines, prompts, warnings) come from cfg.streams. Stdout
// is deliberately untouched so `melvin-config claude sync > x.json`
// doesn't scoop diagnostic output.
func claudeSyncCmd(ctx context.Context, cfg *appConfig, p claudeSyncParams) error {
	paths, err := resolveClaudePaths(cfg)
	if err != nil {
		return fmt.Errorf("resolve claude paths: %w", err)
	}

	mode := sync.ModeCopy
	if p.personal {
		mode = sync.ModeSymlink
	}

	prompter := prompt.NewPrompter(paths, cfg.streams.In, cfg.streams.Err, p.mergeResolution,
		cfg.dryRun, cfg.reporter)
	_, err = cfg.newClaudeSync(ctx, paths, sync.Options{
		Mode:     mode,
		Out:      cfg.streams.Err,
		Prompter: prompter,
		DryRun:   cfg.dryRun,
		Reporter: cfg.reporter,
		// NewGit is left nil; sync.Sync defaults to state.NewGit.
	})
	if err != nil {
		return fmt.Errorf("claude sync: %w", err)
	}
	return nil
}

// resolveClaudePaths returns a state.Paths derived from cfg.configDir
// or, if empty, $HOME/.melvin/config + $HOME. Returns an error when
// os.UserHomeDir() fails (HOME unset, /etc/passwd unreadable, etc.) so
// callers don't silently end up with a relative "./.claude" path.
func resolveClaudePaths(cfg *appConfig) (state.Paths, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return state.Paths{}, fmt.Errorf("resolve home: %w", err)
	}
	configDir := cfg.configDir
	if configDir == "" {
		configDir = filepath.Join(homeDir, ".melvin", "config")
	}
	return state.NewPaths(configDir, homeDir), nil
}
