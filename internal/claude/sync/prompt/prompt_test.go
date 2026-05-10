package prompt

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Nivl/config/internal/claude/sync/state"
	"github.com/Nivl/config/internal/dryrun"
)

// stubRenderer is a minimal Renderer for prompt tests. Summary and Diff
// produce fixed strings so the test can assert exact output bytes.
type stubRenderer struct {
	summary string
	diff    string
}

func (s *stubRenderer) Summary(w io.Writer) { _, _ = io.WriteString(w, s.summary) }
func (s *stubRenderer) Diff(w io.Writer)    { _, _ = io.WriteString(w, s.diff) }

// newPromptTestEnv builds a Paths in a tempdir with EnsureStateDir already run.
func newPromptTestEnv(t *testing.T) state.Paths {
	t.Helper()
	tmp := t.TempDir()
	p := state.NewPaths(tmp, tmp)
	require.NoError(t, os.MkdirAll(p.RepoDir, 0o755))
	require.NoError(t, state.EnsureStateDir(p))
	return p
}

// TestPrompter_MenuByteIdentical asserts the exact text of the
// 6-option menu.
func TestPrompter_MenuByteIdentical(t *testing.T) {
	p := newPromptTestEnv(t)
	// Stdin: pick option 1 (Keep local).
	in := strings.NewReader("1\n")
	var out bytes.Buffer

	prompter := NewPrompter(p, in, &out, "", false, dryrun.NewNullReporter())
	choice, err := prompter.Resolve(context.Background(), Request{
		Kind:   KindSettings,
		Key:    `["model"]`,
		Header: "Conflict in settings.json at .model",
		Renderer: &stubRenderer{
			summary: "  base   (sha none): {\"model\":\"opus\"}\n  local                : {\"model\":\"haiku\"}\n  remote               : {\"model\":\"sonnet\"}\n",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, ChoiceKeepLocal, choice)

	want := strings.Join([]string{
		"",
		"Conflict in settings.json at .model",
		"  base   (sha none): {\"model\":\"opus\"}",
		"  local                : {\"model\":\"haiku\"}",
		"  remote               : {\"model\":\"sonnet\"}",
		"",
		"  1) Keep local",
		"  2) Take remote",
		"  3) View diff",
		"  4) Keep local AND remember",
		"  5) Take remote AND remember",
		"  6) Skip (do not advance last-sync-commit)",
		"  (clear remembered choices: rm " + p.DecisionsFile + ")",
		"",
	}, "\n")
	assert.Equal(t, want, out.String())
}

// TestPrompter_OverrideShortCircuits verifies tier 1: a non-empty
// mergeResolution arg short-circuits the prompt and prints the
// "-> X (override)" line. The cmd layer resolves the value from the
// --merge-resolution flag or CLAUDE_MERGE_RESOLUTION env; the prompt
// package itself only sees the resolved string.
func TestPrompter_OverrideShortCircuits(t *testing.T) {
	p := newPromptTestEnv(t)

	var out bytes.Buffer
	prompter := NewPrompter(p, strings.NewReader(""), &out, "take-remote", false, dryrun.NewNullReporter())
	choice, err := prompter.Resolve(context.Background(), Request{
		Kind: KindSettings, Key: `["model"]`,
		Header:   "Conflict in settings.json at .model",
		Renderer: &stubRenderer{},
	})
	require.NoError(t, err)
	assert.Equal(t, ChoiceTakeRemote, choice)
	assert.Equal(t, "Conflict in settings.json at .model -> take-remote (override)\n", out.String())
}

// TestPrompter_CachedDecisionShortCircuits verifies tier 2: a remembered
// "local" choice in decisions.json short-circuits the prompt and prints
// "-> keep-local (remembered)".
func TestPrompter_CachedDecisionShortCircuits(t *testing.T) {
	p := newPromptTestEnv(t)
	d, err := state.LoadDecisions(p)
	require.NoError(t, err)
	d.Version = 1
	d.Settings[`["model"]`] = state.ChoiceLocal
	require.NoError(t, state.SaveDecisions(p, d))

	var out bytes.Buffer
	prompter := NewPrompter(p, strings.NewReader(""), &out, "", false, dryrun.NewNullReporter())
	choice, err := prompter.Resolve(context.Background(), Request{
		Kind: KindSettings, Key: `["model"]`,
		Header:   "Conflict in settings.json at .model",
		Renderer: &stubRenderer{},
	})
	require.NoError(t, err)
	assert.Equal(t, ChoiceKeepLocal, choice)
	assert.Equal(t, "Conflict in settings.json at .model -> keep-local (remembered)\n", out.String())
}

// TestPrompter_Remember persists a choice to decisions.json via
// option 4 (Keep local AND remember).
func TestPrompter_Remember(t *testing.T) {
	p := newPromptTestEnv(t)
	in := strings.NewReader("4\n")
	var out bytes.Buffer

	prompter := NewPrompter(p, in, &out, "", false, dryrun.NewNullReporter())
	choice, err := prompter.Resolve(context.Background(), Request{
		Kind: KindSettings, Key: `["model"]`,
		Header:   "Conflict in settings.json at .model",
		Renderer: &stubRenderer{},
	})
	require.NoError(t, err)
	assert.Equal(t, ChoiceKeepLocal, choice)

	d, err := state.LoadDecisions(p)
	require.NoError(t, err)
	assert.Equal(t, state.ChoiceLocal, d.Settings[`["model"]`])
}

// TestPrompter_Skip option 6 returns ChoiceSkip (caller MUST NOT
// advance last-sync-commit).
func TestPrompter_Skip(t *testing.T) {
	p := newPromptTestEnv(t)
	in := strings.NewReader("6\n")
	var out bytes.Buffer

	prompter := NewPrompter(p, in, &out, "", false, dryrun.NewNullReporter())
	choice, err := prompter.Resolve(context.Background(), Request{
		Kind: KindSettings, Key: `["model"]`,
		Header:   "Conflict in settings.json at .model",
		Renderer: &stubRenderer{},
	})
	require.NoError(t, err)
	assert.Equal(t, ChoiceSkip, choice)

	// Skip is NOT cached.
	d, _ := state.LoadDecisions(p)
	assert.Empty(t, d.Settings)
}

// TestPrompter_ViewDiffRePrompts option 3 invokes Renderer.Diff and
// re-prompts. Second input is option 2 (Take remote).
func TestPrompter_ViewDiffRePrompts(t *testing.T) {
	p := newPromptTestEnv(t)
	in := strings.NewReader("3\n2\n")
	var out bytes.Buffer
	renderer := &stubRenderer{diff: "DIFF_OUTPUT\n"}

	prompter := NewPrompter(p, in, &out, "", false, dryrun.NewNullReporter())
	choice, err := prompter.Resolve(context.Background(), Request{
		Kind: KindSettings, Key: `["model"]`,
		Header: "Conflict", Renderer: renderer,
	})
	require.NoError(t, err)
	assert.Equal(t, ChoiceTakeRemote, choice)
	assert.Contains(t, out.String(), "DIFF_OUTPUT\n")
}

// TestPrompter_InvalidInputReprints rejects "x" then accepts "1".
func TestPrompter_InvalidInputReprints(t *testing.T) {
	p := newPromptTestEnv(t)
	in := strings.NewReader("x\n1\n")
	var out bytes.Buffer

	prompter := NewPrompter(p, in, &out, "", false, dryrun.NewNullReporter())
	choice, err := prompter.Resolve(context.Background(), Request{
		Kind: KindSettings, Key: `["model"]`,
		Header: "Conflict", Renderer: &stubRenderer{},
	})
	require.NoError(t, err)
	assert.Equal(t, ChoiceKeepLocal, choice)
	assert.Contains(t, out.String(), "Invalid value\n")
}

// TestPrompter_EOFReturnsError — Resolve returns an error on EOF
// instead of spinning forever on closed stdin.
func TestPrompter_EOFReturnsError(t *testing.T) {
	p := newPromptTestEnv(t)
	in := strings.NewReader("") // EOF immediately
	var out bytes.Buffer

	prompter := NewPrompter(p, in, &out, "", false, dryrun.NewNullReporter())
	_, err := prompter.Resolve(context.Background(), Request{
		Kind: KindSettings, Key: `["model"]`,
		Header: "Conflict", Renderer: &stubRenderer{},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, errNoInput)
}

// Helper: confirm the decisions.json path is what tests think it is.
func TestPrompter_DecisionsPathIsExpected(t *testing.T) {
	p := newPromptTestEnv(t)
	assert.Equal(t, filepath.Join(p.StateDir, "decisions.json"), p.DecisionsFile)
}

// fakeReporter records FileChange calls for assertion. Embeds
// NullReporter so methods we don't override stay silent.
type fakeReporter struct {
	dryrun.Reporter

	fileChangeCalls []fakeFileChange
}

// fakeFileChange holds the arguments of one FileChange call.
type fakeFileChange struct {
	target  string
	before  []byte
	after   []byte
	summary string
}

// FileChange records the call.
func (f *fakeReporter) FileChange(target string, before, after []byte, summary string) {
	f.fileChangeCalls = append(f.fileChangeCalls,
		fakeFileChange{target: target, before: before, after: after, summary: summary})
}

// TestPrompter_DryRunRememberSkipsPersistence — when dryRun=true and
// the user picks option 4 (Keep local AND remember), Remember calls
// reporter.FileChange and does NOT persist the new choice to
// decisions.json (file content unchanged).
func TestPrompter_DryRunRememberSkipsPersistence(t *testing.T) {
	p := newPromptTestEnv(t)
	// Record the content of decisions.json before the call (seeded by EnsureStateDir).
	before, err := os.ReadFile(p.DecisionsFile)
	require.NoError(t, err)

	in := strings.NewReader("4\n")
	rep := &fakeReporter{Reporter: dryrun.NewNullReporter()}

	var out bytes.Buffer
	prompter := NewPrompter(p, in, &out, "", true, rep)
	choice, resolveErr := prompter.Resolve(context.Background(), Request{
		Kind: KindSettings, Key: `["model"]`,
		Header:   "Conflict in settings.json at .model",
		Renderer: &stubRenderer{},
	})
	require.NoError(t, resolveErr)
	assert.Equal(t, ChoiceKeepLocal, choice)

	// decisions.json content must be unchanged.
	after, _ := os.ReadFile(p.DecisionsFile)
	assert.Equal(t, before, after, "decisions.json must not be written in dry-run")

	// Reporter received one FileChange call.
	require.Len(t, rep.fileChangeCalls, 1)
	assert.Equal(t, p.DecisionsFile, rep.fileChangeCalls[0].target)
}
