package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Nivl/config/internal/claude/perms"
	"github.com/Nivl/config/internal/dryrun"
	"github.com/Nivl/config/internal/iox"
)

// setupPermsTestRepo initializes a fresh git repo at the returned
// path with `shared_config/.claude/settings.json` pre-populated, then
// returns a perms-ready appConfig pointing at it. Used by every test
// in this file so they don't reach into the user's real config dir.
// Also disables PATH resolution in perms.Variants so Bash() rule
// assertions stay deterministic regardless of the test machine's PATH.
func setupPermsTestRepo(t *testing.T) (configDir string, cfg *appConfig, stderr *bytes.Buffer) {
	t.Helper()
	t.Cleanup(perms.SetLookPath(func(string) (string, error) {
		return "", errors.New("disabled in test")
	}))
	configDir = t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(configDir, "shared_config", ".claude", "hooks"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(configDir, "shared_config", ".claude", "settings.json"),
		[]byte(`{
  "permissions": {
    "allow": [],
    "ask": [],
    "deny": []
  }
}
`), 0o644))
	require.NoError(t, os.WriteFile(
		filepath.Join(configDir, "shared_config", ".claude", "hooks", "git-safe-subcommands.py"),
		[]byte(`#!/usr/bin/env python3
# Minimal test fixture: just the three prefix tables runPermsCmd
# expects. Production-shape file has more scaffolding (helpers, the
# main function); the parser doesn't care about anything outside the
# three blocks.
ALLOW_PREFIXES = ()

ASK_PREFIXES = ()

DENY_PREFIXES = ()
`), 0o644))

	for _, args := range [][]string{
		{"init", "-q", "--initial-branch=main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
		{"config", "commit.gpgsign", "false"},
		{"add", "."},
		{"commit", "-q", "-m", "initial"},
	} {
		c := exec.CommandContext(t.Context(), "git", append([]string{"-C", configDir}, args...)...)
		out, err := c.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}

	stderr = &bytes.Buffer{}
	cfg = &appConfig{
		streams:   iox.Streams{In: strings.NewReader(""), Out: io.Discard, Err: stderr},
		reporter:  dryrun.NewNullReporter(),
		configDir: configDir,
	}
	t.Setenv("PERSONAL_COMPUTER", "true")
	return configDir, cfg, stderr
}

// readPermsList re-reads the post-commit settings.json and returns
// the named list. Sole reason this exists is so tests don't have to
// re-implement the JSON traversal.
func readPermsList(t *testing.T, configDir string, name perms.ListName) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(configDir, settingsRelPath))
	require.NoError(t, err)
	var parsed struct {
		Permissions map[perms.ListName][]string `json:"permissions"`
	}
	require.NoError(t, json.Unmarshal(data, &parsed))
	return parsed.Permissions[name]
}

// gitLogSubject returns the subject line of HEAD's commit message
// in configDir, so tests can assert on commit-message formatting.
func gitLogSubject(t *testing.T, configDir string) string {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), "git", "-C", configDir, "log", "-1", "--pretty=%s").CombinedOutput()
	require.NoError(t, err, "git log: %s", out)
	return strings.TrimSpace(string(out))
}

// gitLogBody returns the body of HEAD's commit message.
func gitLogBody(t *testing.T, configDir string) string {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), "git", "-C", configDir, "log", "-1", "--pretty=%b").CombinedOutput()
	require.NoError(t, err, "git log: %s", out)
	return strings.TrimSpace(string(out))
}

// gitLogChangedFiles returns the repo-relative paths touched by
// HEAD's commit. Used to verify that mixed-store changes only stage
// the file(s) the diff actually touched.
func gitLogChangedFiles(t *testing.T, configDir string) []string {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), "git", "-C", configDir, "show", "--name-only", "--pretty=", "HEAD").CombinedOutput()
	require.NoError(t, err, "git show: %s", out)
	var paths []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			paths = append(paths, line)
		}
	}
	return paths
}

// TestRunPermsCmd_AddBashGitRoutesToHook — `--bash 'git status *'`
// now routes to git-safe-subcommands.py instead of settings.json.
// settings.json stays empty; the hook's ALLOW_PREFIXES gains a
// ("status",) entry; the commit covers the .py file with the
// user-facing rule string in the body.
func TestRunPermsCmd_AddBashGitRoutesToHook(t *testing.T) {
	configDir, cfg, _ := setupPermsTestRepo(t)
	err := runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{{Kind: perms.KindBash, Value: "git status *"}},
		addAction{},
	)
	require.NoError(t, err)

	// settings.json is untouched.
	assert.Empty(t, readPermsList(t, configDir, perms.ListAllow),
		"git rules must not leak into settings.json")

	// Hook file picked up the prefix.
	hook, err := perms.LoadGitHook(filepath.Join(configDir, gitHookRelPath))
	require.NoError(t, err)
	assert.True(t, hook.Has(perms.GitHookAllow, []string{"status"}),
		"ALLOW_PREFIXES should contain (\"status\",)")

	// Commit covers the hook file and shows the canonical rule string.
	assert.Equal(t, "feat(claude,perms): add 1 allow rule (Bash)", gitLogSubject(t, configDir))
	assert.Contains(t, gitLogBody(t, configDir), "+ Bash(git status *)")
	committedFiles := gitLogChangedFiles(t, configDir)
	assert.Contains(t, committedFiles, gitHookRelPath)
	assert.NotContains(t, committedFiles, settingsRelPath,
		"settings.json should not be in the commit when only the hook changed")
}

// TestRunPermsCmd_NoOpAddDoesNotCommit — adding rules that already
// exist exits 0 without producing a new commit. The Skipped listing
// still prints; the file is untouched.
func TestRunPermsCmd_NoOpAddDoesNotCommit(t *testing.T) {
	configDir, cfg, stderr := setupPermsTestRepo(t)
	// First add seeds the rules.
	require.NoError(t, runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{{Kind: perms.KindRead, Value: "file.txt"}},
		addAction{},
	))
	headBefore, err := exec.CommandContext(t.Context(), "git", "-C", configDir, "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	stderr.Reset()

	require.NoError(t, runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{{Kind: perms.KindRead, Value: "file.txt"}},
		addAction{},
	))
	headAfter, err := exec.CommandContext(t.Context(), "git", "-C", configDir, "rev-parse", "HEAD").Output()
	require.NoError(t, err)

	assert.Equal(t, string(headBefore), string(headAfter), "no commit should be produced for a no-op")
	assert.Contains(t, stderr.String(), "skipped (add already in correct state): Read(file.txt)")
}

// TestRunPermsCmd_RemoveExisting — remove drops every variant that
// lives in the target list and commits with the right subject.
func TestRunPermsCmd_RemoveExisting(t *testing.T) {
	configDir, cfg, _ := setupPermsTestRepo(t)
	require.NoError(t, runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{{Kind: perms.KindBash, Value: "ls"}},
		addAction{},
	))

	require.NoError(t, runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{{Kind: perms.KindBash, Value: "ls"}},
		removeAction{},
	))

	assert.Empty(t, readPermsList(t, configDir, perms.ListAllow))
	assert.Equal(t, "feat(claude,perms): remove 1 allow rule (Bash)", gitLogSubject(t, configDir))
}

// TestRunPermsCmd_CrossListConflictWithoutForceErrors — adding a
// rule that lives in `ask` to `allow` fails without --force and
// leaves the file untouched (no commit).
func TestRunPermsCmd_CrossListConflictWithoutForceErrors(t *testing.T) {
	configDir, cfg, _ := setupPermsTestRepo(t)
	require.NoError(t, runPermsCmd(
		context.Background(), cfg, perms.ListAsk,
		[]perms.Op{{Kind: perms.KindRead, Value: "secrets.txt"}},
		addAction{},
	))
	headBefore, err := exec.CommandContext(t.Context(), "git", "-C", configDir, "rev-parse", "HEAD").Output()
	require.NoError(t, err)

	err = runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{{Kind: perms.KindRead, Value: "secrets.txt"}},
		addAction{},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cross-list conflict")
	assert.Contains(t, err.Error(), "--force")

	headAfter, err := exec.CommandContext(t.Context(), "git", "-C", configDir, "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	assert.Equal(t, string(headBefore), string(headAfter), "conflict must not produce a commit")
}

// TestRunPermsCmd_CrossListConflictWithForceMoves — same scenario
// with --force re-categorizes: the rule leaves `ask` and lands in
// `allow`, with a single commit reflecting the move.
func TestRunPermsCmd_CrossListConflictWithForceMoves(t *testing.T) {
	configDir, cfg, stderr := setupPermsTestRepo(t)
	require.NoError(t, runPermsCmd(
		context.Background(), cfg, perms.ListAsk,
		[]perms.Op{{Kind: perms.KindRead, Value: "secrets.txt"}},
		addAction{},
	))
	stderr.Reset()

	require.NoError(t, runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{{Kind: perms.KindRead, Value: "secrets.txt"}},
		addAction{force: true},
	))

	assert.Contains(t, readPermsList(t, configDir, perms.ListAllow), "Read(secrets.txt)")
	assert.NotContains(t, readPermsList(t, configDir, perms.ListAsk), "Read(secrets.txt)")
	assert.Contains(t, stderr.String(), "moved   Read(secrets.txt): ask -> allow")
}

// TestRunPermsCmd_PersonalComputerRequired — without
// PERSONAL_COMPUTER=true the command refuses to run.
func TestRunPermsCmd_PersonalComputerRequired(t *testing.T) {
	_, cfg, _ := setupPermsTestRepo(t)
	t.Setenv("PERSONAL_COMPUTER", "")

	err := runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{{Kind: perms.KindBash, Value: "ls"}},
		addAction{},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "personal computer")
}

// TestRunPermsCmd_NoFlagsErrors — empty ops list is a usage error
// (cobra can't catch this through "required" because each flag is
// individually optional).
func TestRunPermsCmd_NoFlagsErrors(t *testing.T) {
	_, cfg, _ := setupPermsTestRepo(t)
	err := runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		nil, addAction{},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--bash, --read, or --fetch")
}

// TestRunPermsCmd_CommitSubjectPluralization — 1 rule vs N rules
// gets the correct singular/plural in the commit subject.
func TestRunPermsCmd_CommitSubjectPluralization(t *testing.T) {
	configDir, cfg, _ := setupPermsTestRepo(t)
	require.NoError(t, runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{{Kind: perms.KindRead, Value: "file.txt"}},
		addAction{},
	))
	assert.Equal(t, "feat(claude,perms): add 1 allow rule (Read)", gitLogSubject(t, configDir))
}

// TestRunPermsCmd_MultipleKindsCommitSubject — a mixed-kind batch
// produces a comma-separated "(Bash, Read)"-style kind summary.
func TestRunPermsCmd_MultipleKindsCommitSubject(t *testing.T) {
	configDir, cfg, _ := setupPermsTestRepo(t)
	require.NoError(t, runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{
			{Kind: perms.KindBash, Value: "ls"},
			{Kind: perms.KindRead, Value: "file.txt"},
		},
		addAction{},
	))
	subject := gitLogSubject(t, configDir)
	assert.Contains(t, subject, "(Bash, Read)",
		"kinds should appear in canonical order: %s", subject)
}

// TestRunPermsCmd_SkillKindEndToEnd — a Skill op produces exactly
// one Skill(<value>) rule in the target list and shows up in the
// commit subject's kind summary, proving the new kind is wired
// through Variants → Apply → printDiff → buildCommitSubject.
func TestRunPermsCmd_SkillKindEndToEnd(t *testing.T) {
	configDir, cfg, _ := setupPermsTestRepo(t)
	require.NoError(t, runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{{Kind: perms.KindSkill, Value: "code-review:code-review"}},
		addAction{},
	))

	assert.Contains(t, readPermsList(t, configDir, perms.ListAllow),
		"Skill(code-review:code-review)",
		"the rule should land in permissions.allow verbatim")
	subject := gitLogSubject(t, configDir)
	assert.Contains(t, subject, "(Skill)",
		"single-kind subject should name Skill: %s", subject)
}

// TestRunPermsCmd_SkillKindCanonicalOrderInSubject — when Skill is
// batched with other kinds, the kind summary places it in
// Bash < Read < Skill < WebFetch order, matching the alphabetical
// layout in settings.json so a future grep is predictable.
func TestRunPermsCmd_SkillKindCanonicalOrderInSubject(t *testing.T) {
	configDir, cfg, _ := setupPermsTestRepo(t)
	require.NoError(t, runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{
			{Kind: perms.KindSkill, Value: "code-review:code-review"},
			{Kind: perms.KindBash, Value: "ls"},
			{Kind: perms.KindFetch, Value: "npm.io"},
		},
		addAction{},
	))
	subject := gitLogSubject(t, configDir)
	assert.Contains(t, subject, "(Bash, Skill, WebFetch)",
		"kinds should appear in canonical order Bash → Skill → WebFetch: %s", subject)
}

// readSettingsFile is a tiny helper returning the raw file bytes,
// used by dry-run tests to assert the file wasn't touched.
func readSettingsFile(t *testing.T, configDir string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(configDir, settingsRelPath))
	require.NoError(t, err)
	return data
}

// withDryRun returns a copy of cfg with dryRun=true and a real
// reporter wired to stderr, mirroring what applyDryRunFromFlags does
// at the cobra layer. Used to exercise runPermsCmd directly under
// dry-run without going through cobra flag parsing.
func withDryRun(cfg *appConfig, stderr *bytes.Buffer) *appConfig {
	cfg.dryRun = true
	cfg.reporter = dryrun.NewReporter(stderr)
	cfg.streams.Err = stderr
	return cfg
}

// TestRunPermsCmd_DryRunAddDoesNotTouchFile — under dryRun the
// settings.json file is unchanged and no commit is produced.
func TestRunPermsCmd_DryRunAddDoesNotTouchFile(t *testing.T) {
	configDir, cfg, _ := setupPermsTestRepo(t)
	before := readSettingsFile(t, configDir)
	headBefore, err := exec.CommandContext(t.Context(), "git", "-C", configDir, "rev-parse", "HEAD").Output()
	require.NoError(t, err)

	var stderr bytes.Buffer
	cfg = withDryRun(cfg, &stderr)
	require.NoError(t, runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{{Kind: perms.KindBash, Value: "ls"}},
		addAction{},
	))

	after := readSettingsFile(t, configDir)
	assert.Equal(t, before, after, "dry-run must not modify settings.json")
	headAfter, err := exec.CommandContext(t.Context(), "git", "-C", configDir, "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	assert.Equal(t, string(headBefore), string(headAfter), "dry-run must not produce a commit")
}

// TestRunPermsCmd_DryRunAddPrintsWouldVerbs — the per-rule listing
// shows "would add" instead of "added"; the reporter emits the
// would-be git add and git commit shellouts so the preview is
// self-contained.
func TestRunPermsCmd_DryRunAddPrintsWouldVerbs(t *testing.T) {
	_, cfg, _ := setupPermsTestRepo(t)
	var stderr bytes.Buffer
	cfg = withDryRun(cfg, &stderr)

	require.NoError(t, runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{{Kind: perms.KindBash, Value: "ls"}},
		addAction{},
	))

	out := stderr.String()
	assert.Contains(t, out, "would add", "verbs should flip to would-form under dry-run")
	assert.NotContains(t, out, "\nadded   allow", "non-dry-run verb form must not leak through")
	// Reporter-driven shellout previews for the git commands. Match
	// loosely on the substring after `git -C <configDir>` so the test
	// doesn't depend on the temp-dir path.
	assert.Contains(t, out, "add "+settingsRelPath, "would-be git add shellout missing")
	assert.Contains(t, out, "feat(claude,perms): add 1 allow rule (Bash)", "would-be commit subject missing")
}

// TestRunPermsCmd_DryRunNoOpStaysQuiet — when the add would be a
// no-op (all rules already present), dry-run prints the skipped
// listing AND emits no would-run shellouts (nothing would commit).
func TestRunPermsCmd_DryRunNoOpStaysQuiet(t *testing.T) {
	_, cfg, _ := setupPermsTestRepo(t)
	require.NoError(t, runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{{Kind: perms.KindRead, Value: "a.txt"}},
		addAction{},
	))

	var stderr bytes.Buffer
	cfg = withDryRun(cfg, &stderr)
	require.NoError(t, runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{{Kind: perms.KindRead, Value: "a.txt"}},
		addAction{},
	))

	out := stderr.String()
	assert.Contains(t, out, "skipped (add already in correct state): Read(a.txt)")
	assert.NotContains(t, out, "would run", "no-op dry-run must not emit a would-commit shellout")
}

// TestRunPermsCmd_DryRunRemove — remove under dry-run prints
// "would remove" and emits the would-be commit shellout; the file
// stays put.
func TestRunPermsCmd_DryRunRemove(t *testing.T) {
	configDir, cfg, _ := setupPermsTestRepo(t)
	require.NoError(t, runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{{Kind: perms.KindRead, Value: "a.txt"}},
		addAction{},
	))
	before := readSettingsFile(t, configDir)

	var stderr bytes.Buffer
	cfg = withDryRun(cfg, &stderr)
	require.NoError(t, runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{{Kind: perms.KindRead, Value: "a.txt"}},
		removeAction{},
	))

	assert.Equal(t, before, readSettingsFile(t, configDir))
	out := stderr.String()
	assert.Contains(t, out, "would remove allow <- Read(a.txt)")
	assert.Contains(t, out, "feat(claude,perms): remove 1 allow rule (Read)")
}

// TestRunPermsCmd_DryRunCrossListConflictStillErrors — dry-run does
// not bypass the cross-list conflict check; without --force, the
// preview itself fails so the user can't be misled into thinking
// the operation would succeed.
func TestRunPermsCmd_DryRunCrossListConflictStillErrors(t *testing.T) {
	configDir, cfg, _ := setupPermsTestRepo(t)
	require.NoError(t, runPermsCmd(
		context.Background(), cfg, perms.ListAsk,
		[]perms.Op{{Kind: perms.KindRead, Value: "s.txt"}},
		addAction{},
	))
	before := readSettingsFile(t, configDir)

	var stderr bytes.Buffer
	cfg = withDryRun(cfg, &stderr)
	err := runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{{Kind: perms.KindRead, Value: "s.txt"}},
		addAction{},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cross-list conflict")
	assert.Equal(t, before, readSettingsFile(t, configDir))
}

// TestRunPermsCmd_CrossListConflictErrorIsStillErrorsAs — the
// formatted error rendered to the user must still wrap the
// underlying *perms.ConflictError so downstream code (and
// future tests) can errors.As it without depending on the message
// string.
func TestRunPermsCmd_CrossListConflictErrorIsStillErrorsAs(t *testing.T) {
	_, cfg, _ := setupPermsTestRepo(t)
	require.NoError(t, runPermsCmd(
		context.Background(), cfg, perms.ListAsk,
		[]perms.Op{{Kind: perms.KindRead, Value: "secrets.txt"}},
		addAction{},
	))

	err := runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{{Kind: perms.KindRead, Value: "secrets.txt"}},
		addAction{},
	)
	require.Error(t, err)
	var conflictErr *perms.ConflictError
	require.ErrorAs(t, err, &conflictErr,
		"formatted error must still wrap the *ConflictError")
	require.Len(t, conflictErr.Conflicts, 1)
	assert.Equal(t, perms.ListAsk, conflictErr.Conflicts[0].In)
}

// TestPermsFlags_ToOpsTrimsAndDropsEmpties — `--bash ,,foo, bar ` is
// equivalent to two distinct Op values "foo" and "bar", no empty
// entries leaking through.
func TestPermsFlags_ToOpsTrimsAndDropsEmpties(t *testing.T) {
	f := &permsFlags{bash: []string{"", "  ", "foo", "  bar  "}}
	ops := f.toOps()
	require.Len(t, ops, 2)
	assert.Equal(t, "foo", ops[0].Value)
	assert.Equal(t, "bar", ops[1].Value)
}

// makeApplyDryRunFixture returns a cmd with --dry-run declared
// directly (not as a persistent flag inherited from a parent), plus
// an appConfig that starts in non-dry-run state. The persistent-flag
// inheritance path is covered end-to-end by the TestRunPermsCmd_*
// integration tests; this fixture isolates the resolve+swap logic.
func makeApplyDryRunFixture(t *testing.T) (*cobra.Command, *appConfig) {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool("dry-run", false, "test")
	cfg := &appConfig{
		streams:  iox.Streams{In: strings.NewReader(""), Out: io.Discard, Err: &bytes.Buffer{}},
		reporter: dryrun.NewNullReporter(),
	}
	return cmd, cfg
}

// TestApplyDryRunFromFlags_FlagWinsOverEnv — explicit `--dry-run=true`
// flips cfg.dryRun + swaps in a real reporter. Flag-set wins even
// when the env var is unset.
func TestApplyDryRunFromFlags_FlagWinsOverEnv(t *testing.T) {
	t.Setenv("MELVIN_DRY_RUN", "")
	cmd, cfg := makeApplyDryRunFixture(t)
	require.NoError(t, cmd.Flags().Set("dry-run", "true"))

	require.NoError(t, applyDryRunFromFlags(cmd, cfg))
	assert.True(t, cfg.dryRun)
	assert.NotEqual(t, dryrun.NewNullReporter(), cfg.reporter,
		"reporter should be swapped to the real impl, not stay as null")
}

// TestApplyDryRunFromFlags_EnvWinsWhenFlagUnchanged — flag stays at
// its default (false, .Changed=false), env=true → resolved true.
// The whole point of resolveBool's flag→env→fallback chain.
func TestApplyDryRunFromFlags_EnvWinsWhenFlagUnchanged(t *testing.T) {
	t.Setenv("MELVIN_DRY_RUN", "true")
	cmd, cfg := makeApplyDryRunFixture(t)

	require.NoError(t, applyDryRunFromFlags(cmd, cfg))
	assert.True(t, cfg.dryRun)
}

// TestApplyDryRunFromFlags_NeitherFlagNorEnv — both unset → resolved
// false, cfg.reporter stays at the null impl (no real reporter
// swap-in).
func TestApplyDryRunFromFlags_NeitherFlagNorEnv(t *testing.T) {
	t.Setenv("MELVIN_DRY_RUN", "")
	cmd, cfg := makeApplyDryRunFixture(t)
	initialReporter := cfg.reporter

	require.NoError(t, applyDryRunFromFlags(cmd, cfg))
	assert.False(t, cfg.dryRun)
	assert.Equal(t, initialReporter, cfg.reporter,
		"reporter must not be re-swapped when dry-run is false")
}

// TestRunPermsCmd_AddBashGitToDenyList — the routing covers all
// three lists, not just allow. A `--bash 'git push --force *'` add
// to deny lands in DENY_PREFIXES and only stages the hook file.
func TestRunPermsCmd_AddBashGitToDenyList(t *testing.T) {
	configDir, cfg, _ := setupPermsTestRepo(t)
	require.NoError(t, runPermsCmd(
		context.Background(), cfg, perms.ListDeny,
		[]perms.Op{{Kind: perms.KindBash, Value: "git push --force *"}},
		addAction{},
	))

	hook, err := perms.LoadGitHook(filepath.Join(configDir, gitHookRelPath))
	require.NoError(t, err)
	assert.True(t, hook.Has(perms.GitHookDeny, []string{"push", "--force"}))
	assert.Equal(t, "feat(claude,perms): add 1 deny rule (Bash)", gitLogSubject(t, configDir))
}

// TestRunPermsCmd_MixedBatchStagesBothFiles — a single invocation
// mixing one settings rule (Read) and one git rule (Bash) ends up
// with both files modified and both staged in one commit.
func TestRunPermsCmd_MixedBatchStagesBothFiles(t *testing.T) {
	configDir, cfg, _ := setupPermsTestRepo(t)
	require.NoError(t, runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{
			{Kind: perms.KindRead, Value: "file.txt"},
			{Kind: perms.KindBash, Value: "git fetch *"},
		},
		addAction{},
	))

	// Settings file has the Read rule.
	assert.Contains(t, readPermsList(t, configDir, perms.ListAllow), "Read(file.txt)")
	// Hook file has the prefix.
	hook, err := perms.LoadGitHook(filepath.Join(configDir, gitHookRelPath))
	require.NoError(t, err)
	assert.True(t, hook.Has(perms.GitHookAllow, []string{"fetch"}))

	// Both files in the single commit.
	files := gitLogChangedFiles(t, configDir)
	assert.Contains(t, files, settingsRelPath)
	assert.Contains(t, files, gitHookRelPath)
	// Subject counts both, kinds summary covers both.
	assert.Equal(t, "feat(claude,perms): add 2 allow rules (Bash, Read)", gitLogSubject(t, configDir))
}

// TestRunPermsCmd_GitRuleWithoutTrailingStarErrors — `git status`
// (no wildcard) is rejected at the SplitOpsByBackend layer; neither
// file changes, no commit is produced, and the error message points
// the user at the trailing-`*` requirement.
func TestRunPermsCmd_GitRuleWithoutTrailingStarErrors(t *testing.T) {
	configDir, cfg, _ := setupPermsTestRepo(t)
	headBefore, err := exec.CommandContext(t.Context(), "git", "-C", configDir, "rev-parse", "HEAD").Output()
	require.NoError(t, err)

	err = runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{{Kind: perms.KindBash, Value: "git status"}},
		addAction{},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trailing `*`")

	headAfter, err := exec.CommandContext(t.Context(), "git", "-C", configDir, "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	assert.Equal(t, string(headBefore), string(headAfter),
		"parse error must not produce a commit")
}

// TestRunPermsCmd_RemoveGitRuleFromHook — `perms allow remove
// --bash 'git status *'` strips the prefix from the hook file
// without touching settings.json. Verifies the remove path
// mirrors the add path's routing.
func TestRunPermsCmd_RemoveGitRuleFromHook(t *testing.T) {
	configDir, cfg, _ := setupPermsTestRepo(t)
	require.NoError(t, runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{{Kind: perms.KindBash, Value: "git status *"}},
		addAction{},
	))

	require.NoError(t, runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{{Kind: perms.KindBash, Value: "git status *"}},
		removeAction{},
	))

	hook, err := perms.LoadGitHook(filepath.Join(configDir, gitHookRelPath))
	require.NoError(t, err)
	assert.False(t, hook.Has(perms.GitHookAllow, []string{"status"}))
	assert.Equal(t, "feat(claude,perms): remove 1 allow rule (Bash)", gitLogSubject(t, configDir))
}

// TestRunPermsCmd_DryRunHookOpDoesNotTouchFile — dry-run with a
// git op preview the prefix change without writing the hook file.
// The would-add line still prints to stderr; the file stays as it
// was on disk.
func TestRunPermsCmd_DryRunHookOpDoesNotTouchFile(t *testing.T) {
	configDir, cfg, stderr := setupPermsTestRepo(t)
	cfg.dryRun = true
	cfg.reporter = dryrun.NewReporter(stderr)

	require.NoError(t, runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{{Kind: perms.KindBash, Value: "git status *"}},
		addAction{},
	))

	// File on disk unchanged — no `status` prefix yet.
	hook, err := perms.LoadGitHook(filepath.Join(configDir, gitHookRelPath))
	require.NoError(t, err)
	assert.False(t, hook.Has(perms.GitHookAllow, []string{"status"}),
		"dry-run must not write to the hook file")

	// Preview still showed the would-add line.
	assert.Contains(t, stderr.String(), "would add")
	assert.Contains(t, stderr.String(), "Bash(git status *)")
}
