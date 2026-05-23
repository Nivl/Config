package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Nivl/config/internal/claude/perms"
	"github.com/Nivl/config/internal/dryrun"
)

// settingsRelPath is the location of the permissions JSON inside the
// config repo, relative to the repo root. Used both as the file to
// edit and as the path passed to `git add` so commits stay scoped.
const settingsRelPath = "shared_config/.claude/settings.json"

// newPermsCmd builds the `melvin-config claude perms` parent
// command. It has no body — only the three list-subcommands
// (allow/ask/deny). Each list subcommand has add/remove children.
//
// --dry-run lives at this level as a persistent flag so every
// leaf (allow/ask/deny × add/remove) inherits it; each leaf RunE
// resolves it against MELVIN_DRY_RUN at run-time before calling
// into runPermsCmd.
func newPermsCmd(cfg *appConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "perms",
		Short: "Manage Claude Code permission rules in shared_config/.claude/settings.json",
		Long: `Add or remove entries in permissions.{allow,ask,deny} of
shared_config/.claude/settings.json, then commit the change.

Each --bash value produces a single Bash() rule; git commands
additionally get the ` + "`git -C /*`" + ` cwd-bypass variant.
--read, --fetch, and --skill each produce a single rule.
The command only runs on a personal computer (PERSONAL_COMPUTER=true).`,
	}
	cmd.PersistentFlags().Bool("dry-run", false,
		"preview changes without writing settings.json or making a commit (also MELVIN_DRY_RUN env)")
	cmd.AddCommand(newPermsListCmd(cfg, perms.ListAllow))
	cmd.AddCommand(newPermsListCmd(cfg, perms.ListAsk))
	cmd.AddCommand(newPermsListCmd(cfg, perms.ListDeny))
	return cmd
}

// newPermsListCmd builds one of the three list parents
// (`claude perms allow`, `claude perms ask`, `claude perms deny`).
// Each carries an `add` and a `remove` child built from the same
// flag wiring, so the three lists stay in sync.
func newPermsListCmd(cfg *appConfig, list perms.ListName) *cobra.Command {
	cmd := &cobra.Command{
		Use:   string(list),
		Short: fmt.Sprintf("Manage the permissions.%s list", list),
	}
	cmd.AddCommand(newPermsAddCmd(cfg, list))
	cmd.AddCommand(newPermsRemoveCmd(cfg, list))
	return cmd
}

// permsFlags is the shared (--bash, --read, --fetch, --skill) flag
// wiring used by both add and remove. Each flag is a StringSlice so
// it can be repeated AND accept comma-separated values in a single
// invocation: `--bash 'foo *' --bash 'bar,baz'` is three values.
type permsFlags struct {
	bash  []string
	read  []string
	fetch []string
	skill []string
}

// register attaches the four flags to cmd. Returns the same flags
// struct for fluent use at the call site.
func (f *permsFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringSliceVar(&f.bash, "bash", nil,
		"Bash rule to add/remove (repeatable, comma-separated; git commands also get a `git -C /*` variant)")
	cmd.Flags().StringSliceVar(&f.read, "read", nil,
		"Read rule to add/remove (repeatable, comma-separated)")
	cmd.Flags().StringSliceVar(&f.fetch, "fetch", nil,
		"WebFetch domain to add/remove (repeatable, comma-separated)")
	cmd.Flags().StringSliceVar(&f.skill, "skill", nil,
		"Skill name to add/remove (repeatable, comma-separated; "+
			"e.g. code-review:code-review for a plugin-namespaced skill)")
}

// toOps converts the parsed flag slices into a flat []Op preserving
// flag-declaration order (bash → read → fetch → skill). Empty
// entries are skipped silently — StringSlice happily produces them
// for `--bash ,,foo`, and rejecting at this layer keeps the variants
// layer free of input-sanitization.
func (f *permsFlags) toOps() []perms.Op {
	var out []perms.Op
	collect := func(values []string, kind perms.Kind) {
		for _, v := range values {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			out = append(out, perms.Op{Kind: kind, Value: v})
		}
	}
	collect(f.bash, perms.KindBash)
	collect(f.read, perms.KindRead)
	collect(f.fetch, perms.KindFetch)
	collect(f.skill, perms.KindSkill)
	return out
}

// newPermsAddCmd builds `claude perms <list> add`. The -f/--force
// flag opts into the cross-list re-categorization behaviour
// (otherwise such conflicts fail loudly).
func newPermsAddCmd(cfg *appConfig, list perms.ListName) *cobra.Command {
	flags := &permsFlags{}
	var force bool
	cmd := &cobra.Command{
		Use:   "add",
		Short: fmt.Sprintf("Add rules to permissions.%s", list),
		Args:  cobra.NoArgs,
	}
	flags.register(cmd)
	cmd.Flags().BoolVarP(&force, "force", "f", false,
		"Move rules that currently live in a different list (allow/ask/deny)")
	cmd.RunE = func(cobraCmd *cobra.Command, _ []string) error {
		if err := applyDryRunFromFlags(cobraCmd, cfg); err != nil {
			return err
		}
		return runPermsCmd(cobraCmd.Context(), cfg, list, flags.toOps(), addAction{force: force})
	}
	return cmd
}

// newPermsRemoveCmd builds `claude perms <list> remove`. There is
// no --force because remove never reaches into other lists.
func newPermsRemoveCmd(cfg *appConfig, list perms.ListName) *cobra.Command {
	flags := &permsFlags{}
	cmd := &cobra.Command{
		Use:   "remove",
		Short: fmt.Sprintf("Remove rules from permissions.%s", list),
		Args:  cobra.NoArgs,
	}
	flags.register(cmd)
	cmd.RunE = func(cobraCmd *cobra.Command, _ []string) error {
		if err := applyDryRunFromFlags(cobraCmd, cfg); err != nil {
			return err
		}
		return runPermsCmd(cobraCmd.Context(), cfg, list, flags.toOps(), removeAction{})
	}
	return cmd
}

// applyDryRunFromFlags resolves the --dry-run flag (inherited from
// the perms parent) against MELVIN_DRY_RUN and, when true, swaps
// cfg.reporter from the null reporter to a real one that writes
// would-be effects to cfg.streams.Err. Mirrors the wiring in
// newSetupCmd so the dry-run surface stays consistent across
// commands.
func applyDryRunFromFlags(cobraCmd *cobra.Command, cfg *appConfig) error {
	flagVal, err := cobraCmd.Flags().GetBool("dry-run")
	if err != nil {
		return fmt.Errorf("read dry-run flag: %w", err)
	}
	resolved, err := resolveBool(cobraCmd, "dry-run", "MELVIN_DRY_RUN", flagVal, nil)
	if err != nil {
		return fmt.Errorf("resolve dry-run flag: %w", err)
	}
	cfg.dryRun = resolved
	if resolved {
		cfg.reporter = dryrun.NewReporter(cfg.streams.Err)
	}
	return nil
}

// permsAction is the strategy interface for "what does this command
// do to the Settings?" — Add and Remove differ only in (a) the
// perms call and (b) the verb in the commit message.
type permsAction interface {
	apply(s *perms.Settings, target perms.ListName, ops []perms.Op) (perms.Diff, error)
	verb() string
}

// addAction wraps perms.Add with the user's --force choice.
type addAction struct{ force bool }

func (a addAction) apply(s *perms.Settings, target perms.ListName, ops []perms.Op) (perms.Diff, error) {
	return perms.Add(s, target, ops, a.force)
}
func (addAction) verb() string { return "add" }

// removeAction wraps perms.Remove.
type removeAction struct{}

func (removeAction) apply(s *perms.Settings, target perms.ListName, ops []perms.Op) (perms.Diff, error) {
	return perms.Remove(s, target, ops)
}
func (removeAction) verb() string { return "remove" }

// runPermsCmd is the shared body for both add and remove. Order:
//  1. Personal-computer gate.
//  2. Reject zero ops (cobra can't detect "no --bash/--read/--fetch
//     at all" through its required-flags API because each is
//     individually optional).
//  3. Resolve configDir → settings.json path.
//  4. Load + action.apply.
//  5. Print per-rule listing (verbs flip to "would …" under
//     cfg.dryRun).
//  6. If the Diff records mutations:
//     - Under dryRun, route the would-be `git add` and `git commit`
//     shellouts through cfg.reporter and skip Save + commit.
//     - Otherwise, Save the file then git add + git commit.
func runPermsCmd(ctx context.Context, cfg *appConfig, list perms.ListName, ops []perms.Op, action permsAction) error {
	if err := requirePersonalComputer(); err != nil {
		return err
	}
	if len(ops) == 0 {
		return errors.New("at least one of --bash, --read, or --fetch is required")
	}

	configDir, err := resolveConfigDir(cfg)
	if err != nil {
		return err
	}
	settingsPath := filepath.Join(configDir, settingsRelPath)

	settings, err := perms.Load(settingsPath)
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}

	diff, err := action.apply(settings, list, ops)
	if err != nil {
		return formatPermsError(err)
	}

	stderr := cfg.streams.Err
	printDiff(stderr, diff, list, action.verb(), cfg.dryRun)

	if diff.Empty() {
		return nil
	}
	if cfg.dryRun {
		reportDryRunCommit(cfg.reporter, configDir, list, action.verb(), diff)
		return nil
	}
	if err := settings.Save(settingsPath); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	if err := gitCommitSettings(ctx, cfg, configDir, list, action.verb(), diff); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	return nil
}

// reportDryRunCommit emits the would-be `git add` and `git commit`
// shellouts via the dry-run reporter so the preview output is
// self-contained — the user sees both the semantic changes (from
// printDiff) and the underlying subprocess invocations that would
// run. No file or process is touched.
func reportDryRunCommit(reporter dryrun.Reporter, configDir string, list perms.ListName, verb string, diff perms.Diff) {
	subject := buildCommitSubject(list, verb, diff)
	reporter.Shellout("git",
		[]string{"-C", configDir, "add", settingsRelPath},
		"stage settings change")
	reporter.Shellout("git",
		[]string{"-C", configDir, "commit", "-m", subject},
		"commit settings change")
}

// requirePersonalComputer enforces the personal-only restriction.
// Reads PERSONAL_COMPUTER at command-run time (not flag-parse) so
// `--help` still works without the env set.
func requirePersonalComputer() error {
	if os.Getenv("PERSONAL_COMPUTER") != "true" {
		return errors.New("claude perms commands only run on a personal computer (PERSONAL_COMPUTER=true)")
	}
	return nil
}

// resolveConfigDir returns the directory holding the config repo —
// either cfg.configDir (used by tests) or $HOME/.melvin/config.
func resolveConfigDir(cfg *appConfig) (string, error) {
	if cfg.configDir != "" {
		return cfg.configDir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	return filepath.Join(home, ".melvin", "config"), nil
}

// formatPermsError renders a perms.ConflictError with one rule
// per line so the user can copy/paste a corrective command, and
// passes any other error through unwrapped. The original
// *ConflictError is preserved as the wrapped error so callers (and
// tests) can still errors.As it.
func formatPermsError(err error) error {
	var conflictErr *perms.ConflictError
	if !errors.As(err, &conflictErr) {
		return err
	}
	var b strings.Builder
	b.WriteString("cross-list conflict (re-run with --force to move):\n")
	for _, c := range conflictErr.Conflicts {
		fmt.Fprintf(&b, "  %s currently in %s\n", c.Rule, c.In)
	}
	return fmt.Errorf("%s: %w", strings.TrimRight(b.String(), "\n"), conflictErr)
}

// printDiff writes a per-rule listing of the operation's effects to
// out. Each line names the list and rule so a batched invocation
// (e.g. `add --bash X --read Y`) reads top-to-bottom. Under dryRun
// the action verbs flip to their "would" forms so the listing reads
// as a preview rather than a record of work done.
func printDiff(out io.Writer, diff perms.Diff, list perms.ListName, verb string, dryRun bool) {
	addedLabel, removedLabel, movedLabel := "added  ", "removed", "moved  "
	if dryRun {
		addedLabel, removedLabel, movedLabel = "would add  ", "would remove", "would move "
	}
	for _, rule := range diff.Added {
		_, _ = fmt.Fprintf(out, "%s %s -> %s\n", addedLabel, list, rule)
	}
	for _, rule := range diff.Removed {
		_, _ = fmt.Fprintf(out, "%s %s <- %s\n", removedLabel, list, rule)
	}
	for _, moved := range diff.Moved {
		_, _ = fmt.Fprintf(out, "%s %s: %s -> %s\n", movedLabel, moved.Rule, moved.From, list)
	}
	for _, rule := range diff.Skipped {
		_, _ = fmt.Fprintf(out, "skipped (%s already in correct state): %s\n", verb, rule)
	}
}

// gitCommitSettings stages just the settings file and creates a
// commit. Subject follows the compact format the user picked:
// `feat(claude,perms): <verb> N <list> rules (<kinds>)`. The body lists every
// rule so the commit message stays auditable for big batches.
func gitCommitSettings(ctx context.Context, cfg *appConfig, configDir string, list perms.ListName, verb string, diff perms.Diff) error {
	relPath := settingsRelPath
	if err := runGit(ctx, cfg, configDir, "add", relPath); err != nil {
		return err
	}

	subject := buildCommitSubject(list, verb, diff)
	body := buildCommitBody(diff)
	msg := subject
	if body != "" {
		msg = subject + "\n\n" + body
	}
	return runGit(ctx, cfg, configDir, "commit", "-m", msg)
}

// buildCommitSubject produces a compact 1-line subject summarizing
// the operation. Example: `feat(claude,perms): add 3 allow rules (Bash, Read)`.
// Excludes the Skipped count from the subject — skipped rules don't
// contribute to the file diff, so they shouldn't influence what the
// commit log claims happened.
func buildCommitSubject(list perms.ListName, verb string, diff perms.Diff) string {
	var changedCount int
	switch verb {
	case "add":
		changedCount = len(diff.Added) + len(diff.Moved)
	case "remove":
		changedCount = len(diff.Removed)
	}
	kinds := summarizeKinds(diff)
	subject := fmt.Sprintf("feat(claude,perms): %s %d %s rule", verb, changedCount, list)
	if changedCount != 1 {
		subject += "s"
	}
	if kinds != "" {
		subject += " (" + kinds + ")"
	}
	return subject
}

// buildCommitBody lists every rule the operation touched, one per
// line, grouped by action (added → moved → removed). Skipped rules
// are intentionally omitted from the body — they didn't change the
// file, so they don't belong in the commit message either.
func buildCommitBody(diff perms.Diff) string {
	var b strings.Builder
	for _, rule := range diff.Added {
		fmt.Fprintf(&b, "+ %s\n", rule)
	}
	for _, moved := range diff.Moved {
		fmt.Fprintf(&b, "~ %s (from %s)\n", moved.Rule, moved.From)
	}
	for _, rule := range diff.Removed {
		fmt.Fprintf(&b, "- %s\n", rule)
	}
	return strings.TrimRight(b.String(), "\n")
}

// summarizeKinds returns a comma-separated, alphabetically-sorted
// listing of the rule kinds touched by the diff (Bash, Read, Skill,
// WebFetch). Used in the commit subject to give a quick
// at-a-glance signal about scope.
func summarizeKinds(diff perms.Diff) string {
	seen := map[string]struct{}{}
	add := func(rule string) {
		switch {
		case strings.HasPrefix(rule, "Bash("):
			seen["Bash"] = struct{}{}
		case strings.HasPrefix(rule, "Read("):
			seen["Read"] = struct{}{}
		case strings.HasPrefix(rule, "WebFetch("):
			seen["WebFetch"] = struct{}{}
		case strings.HasPrefix(rule, "Skill("):
			seen["Skill"] = struct{}{}
		}
	}
	for _, r := range diff.Added {
		add(r)
	}
	for _, r := range diff.Removed {
		add(r)
	}
	for _, m := range diff.Moved {
		add(m.Rule)
	}
	if len(seen) == 0 {
		return ""
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	// Alphabetical sort — matches the layout in
	// shared_config/.claude/settings.json and means future kinds
	// land in the right spot without updating a hand-maintained
	// index map (all current kind names happen to sort into the
	// canonical Bash → Read → Skill → WebFetch order).
	sort.Strings(out)
	return strings.Join(out, ", ")
}

// runGit invokes `git -C <configDir> <args...>` with both stdout and
// stderr piped to cfg.streams.Err so the user sees real-time git
// output (helpful when a hook fails or an editor opens for a merge
// commit message) and so tests can assert against a captured stream.
// Routing through cfg.streams (rather than reaching into os.Stderr
// directly) keeps the package consistent with the rest of the cmd
// layer.
func runGit(ctx context.Context, cfg *appConfig, configDir string, args ...string) error {
	full := append([]string{"-C", configDir}, args...)
	c := exec.CommandContext(ctx, "git", full...) //nolint:gosec // binary is the fixed literal "git"; args (configDir from cfg, rule/subject strings) flow through argv so no shell interpolation is possible
	c.Stderr = cfg.streams.Err
	c.Stdout = cfg.streams.Err
	if err := c.Run(); err != nil {
		return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return nil
}
