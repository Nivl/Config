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
	require.NoError(t, os.MkdirAll(filepath.Join(configDir, "shared_config", ".claude"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(configDir, "shared_config", ".claude", "settings.json"),
		[]byte(`{
  "permissions": {
    "allow": [],
    "ask": [],
    "deny": []
  }
}
`), 0o644,
	))

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

// TestRunPermsCmd_AddBashGitSingleVariantAndCommits — a single
// `--bash 'git status *'` add writes just the raw rule into
// permissions.allow (git is no longer special-cased; the `git -C`
// form is denied at the hook layer) AND produces a git commit with
// the expected subject and body.
func TestRunPermsCmd_AddBashGitSingleVariantAndCommits(t *testing.T) {
	configDir, cfg, _ := setupPermsTestRepo(t)
	err := runPermsCmd(
		context.Background(), cfg, perms.ListAllow,
		[]perms.Op{{Kind: perms.KindBash, Value: "git status *"}},
		addAction{},
	)
	require.NoError(t, err)

	allow := readPermsList(t, configDir, perms.ListAllow)
	assert.Contains(t, allow, "Bash(git status *)")
	assert.NotContains(t, allow, "Bash(git -C /* status *)",
		"git -C variant must not be generated anymore")

	assert.Equal(t, "feat(claude,perms): add 1 allow rule (Bash)", gitLogSubject(t, configDir))
	assert.Contains(t, gitLogBody(t, configDir), "+ Bash(git status *)")
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
	assert.Contains(t, err.Error(), "--bash, --read, --fetch, or --skill")
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

// readBashAllow returns the parsed bash-allow-trusted.json next to the
// hook in the test repo.
func readBashAllow(t *testing.T, configDir string) struct {
	Excluded [][]string `json:"excluded"`
	Trusted  [][]string `json:"trusted"`
} {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(configDir, bashAllowRelPath))
	require.NoError(t, err)
	var parsed struct {
		Excluded [][]string `json:"excluded"`
		Trusted  [][]string `json:"trusted"`
	}
	require.NoError(t, json.Unmarshal(data, &parsed))
	return parsed
}

// seedExcluded rewrites settings.json with the given sandbox.excludedCommands.
func seedExcluded(t *testing.T, configDir string, excl []string) {
	t.Helper()
	s, err := perms.Load(filepath.Join(configDir, settingsRelPath))
	require.NoError(t, err)
	s.SetExcludedCommands(excl)
	require.NoError(t, s.Save(filepath.Join(configDir, settingsRelPath)))
}

// seedAllow rewrites permissions.allow with the given raw rules.
func seedAllow(t *testing.T, configDir string, rules []string) {
	t.Helper()
	s, err := perms.Load(filepath.Join(configDir, settingsRelPath))
	require.NoError(t, err)
	s.SetList(perms.ListAllow, rules)
	require.NoError(t, s.Save(filepath.Join(configDir, settingsRelPath)))
}

func TestPermsAllowAdd_RegeneratesBashAllow(t *testing.T) {
	configDir, cfg, _ := setupPermsTestRepo(t)
	seedExcluded(t, configDir, []string{"gh *"})

	cmd := newPermsCmd(cfg)
	cmd.SetArgs([]string{"allow", "add", "--bash", "gh pr view *"})
	require.NoError(t, cmd.Execute())

	ba := readBashAllow(t, configDir)
	assert.Contains(t, ba.Trusted, []string{"gh", "pr", "view"})
	assert.Contains(t, ba.Excluded, []string{"gh"})

	out, err := exec.CommandContext(t.Context(), "git", "-C", configDir,
		"show", "--name-only", "--pretty=format:", "HEAD").CombinedOutput()
	require.NoError(t, err)
	assert.Contains(t, string(out), "bash-allow-trusted.json")
	assert.Contains(t, string(out), "settings.json")
}

// readExcluded re-reads sandbox.excludedCommands from settings.json.
func readExcluded(t *testing.T, configDir string) []string {
	t.Helper()
	s, err := perms.Load(filepath.Join(configDir, settingsRelPath))
	require.NoError(t, err)
	return s.ExcludedCommands()
}

func TestPermsExcludeAddRemove(t *testing.T) {
	configDir, cfg, _ := setupPermsTestRepo(t)
	seedExcluded(t, configDir, []string{"git *"})
	seedAllow(t, configDir, []string{"Bash(rushx test *)"})

	add := newPermsCmd(cfg)
	add.SetArgs([]string{"exclude", "add", "rushx test *"})
	require.NoError(t, add.Execute())

	assert.Contains(t, readExcluded(t, configDir), "rushx test *")
	// now rushx test is allowed ∩ excluded -> trusted
	assert.Contains(t, readBashAllow(t, configDir).Trusted, []string{"rushx", "test"})

	rm := newPermsCmd(cfg)
	rm.SetArgs([]string{"exclude", "remove", "rushx test *"})
	require.NoError(t, rm.Execute())

	assert.NotContains(t, readExcluded(t, configDir), "rushx test *")
	assert.NotContains(t, readBashAllow(t, configDir).Trusted, []string{"rushx", "test"})
}

func TestPermsRebuild_BackfillsFromSettings(t *testing.T) {
	configDir, cfg, _ := setupPermsTestRepo(t)
	seedExcluded(t, configDir, []string{"rushx test *"})
	seedAllow(t, configDir, []string{"Bash(rushx test *)", "Bash(go build *)"})

	cmd := newPermsCmd(cfg)
	cmd.SetArgs([]string{"rebuild"})
	require.NoError(t, cmd.Execute())

	ba := readBashAllow(t, configDir)
	assert.Equal(t, [][]string{{"rushx", "test"}}, ba.Trusted) // go build dropped
}

// TestCommittedBashAllowInSync guards against drift: the committed
// bash-allow-trusted.json must equal what Derive produces from the committed
// settings.json. A failure means settings.json was edited without running
// `melvin-config claude perms rebuild`.
func TestCommittedBashAllowInSync(t *testing.T) {
	root := filepath.Join("..", "..")
	s, err := perms.Load(filepath.Join(root, settingsRelPath))
	require.NoError(t, err)
	trusted, excluded := perms.Derive(s.List(perms.ListAllow), s.ExcludedCommands())

	expected := filepath.Join(t.TempDir(), "expected.json")
	require.NoError(t, perms.WriteBashAllow(expected, trusted, excluded))
	want, err := os.ReadFile(expected)
	require.NoError(t, err)
	got, err := os.ReadFile(filepath.Join(root, bashAllowRelPath))
	require.NoError(t, err)
	assert.Equal(t, string(want), string(got),
		"bash-allow-trusted.json is stale — run `melvin-config claude perms rebuild`")
}
