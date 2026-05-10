package state

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeGit writes an executable shell script named "git" into a tempdir
// and returns the dir. Caller prepends it to PATH via t.Setenv.
// Mirrors the fakeBrew helper in internal/brew/runner_test.go.
func fakeGit(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "git"),
		[]byte("#!/bin/sh\n"+body), 0o755))
	return dir
}

// withLastSyncSHA writes the given SHA to a fresh Paths' LastSyncFile
// and returns the Paths.
func withLastSyncSHA(t *testing.T, sha string) Paths {
	t.Helper()
	tmp := t.TempDir()
	p := NewPaths(tmp, tmp)
	require.NoError(t, os.MkdirAll(p.StateDir, 0o755))
	if sha != "" {
		require.NoError(t, os.WriteFile(p.LastSyncFile, []byte(sha+"\n"), 0o644))
	}
	return p
}

// TestGit_ShowBase_NoAnchor returns ErrNoBase silently when LastSyncFile is empty.
func TestGit_ShowBase_NoAnchor(t *testing.T) {
	p := withLastSyncSHA(t, "")
	g := NewGit(p, "/dummy/repo")
	_, err := g.ShowBase(context.Background(), "settings.json")
	assert.ErrorIs(t, err, ErrNoBase)
}

// TestGit_ShowBase_HappyPath_FakeBinarySubprocess verifies the real
// exec.Command path: git is invoked with the right args; stdout is returned.
func TestGit_ShowBase_HappyPath_FakeBinarySubprocess(t *testing.T) {
	body := `case "$*" in
"-C /repo show abcdef:shared_config/.claude/settings.json") echo '{"model":"opus"}'; exit 0 ;;
*) echo "unexpected: $*" >&2; exit 99 ;;
esac`
	dir := fakeGit(t, body)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	p := withLastSyncSHA(t, "abcdef")
	g := NewGit(p, "/repo")
	out, err := g.ShowBase(context.Background(), "settings.json")
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"opus"}`, string(out))
}

// TestGit_ShowBase_UnreachableSHA returns ErrNoBase when git exits non-zero.
func TestGit_ShowBase_UnreachableSHA(t *testing.T) {
	body := `exit 128`
	dir := fakeGit(t, body)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	p := withLastSyncSHA(t, "abcdef")
	g := NewGit(p, "/repo")
	_, err := g.ShowBase(context.Background(), "settings.json")
	assert.ErrorIs(t, err, ErrNoBase)
}

// TestGit_BaseHas_HappyPath returns true when git cat-file exits 0.
func TestGit_BaseHas_HappyPath(t *testing.T) {
	body := `case "$*" in
"-C /repo cat-file -e abcdef:shared_config/.claude/settings.json") exit 0 ;;
*) exit 1 ;;
esac`
	dir := fakeGit(t, body)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	p := withLastSyncSHA(t, "abcdef")
	g := NewGit(p, "/repo")
	ok, err := g.BaseHas(context.Background(), "settings.json")
	require.NoError(t, err)
	assert.True(t, ok)
}

// TestGit_BaseHas_NoAnchor returns false without error or invoking git.
func TestGit_BaseHas_NoAnchor(t *testing.T) {
	p := withLastSyncSHA(t, "")
	g := NewGit(p, "/dummy/repo")
	ok, err := g.BaseHas(context.Background(), "settings.json")
	require.NoError(t, err)
	assert.False(t, ok)
}

// TestGit_ListTree_NoAnchor returns (nil, nil) silently when LastSyncFile is empty.
func TestGit_ListTree_NoAnchor(t *testing.T) {
	p := withLastSyncSHA(t, "")
	g := NewGit(p, "/dummy/repo")
	out, err := g.ListTree(context.Background(), "skills")
	require.NoError(t, err)
	assert.Nil(t, out)
}

// TestGit_ListTree_HappyPath_FakeBinarySubprocess verifies the real
// exec.Command path: git ls-tree is invoked with the right args; the
// .claude/<dir>/ prefix is stripped from each line.
func TestGit_ListTree_HappyPath_FakeBinarySubprocess(t *testing.T) {
	body := `case "$*" in
"-C /repo ls-tree -r --name-only abcdef -- shared_config/.claude/skills")
  printf 'shared_config/.claude/skills/a.md\nshared_config/.claude/skills/sub/b.md\nshared_config/.claude/skills/.gitkeep\n'
  exit 0 ;;
*) echo "unexpected: $*" >&2; exit 99 ;;
esac`
	dir := fakeGit(t, body)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	p := withLastSyncSHA(t, "abcdef")
	g := NewGit(p, "/repo")
	out, err := g.ListTree(context.Background(), "skills")
	require.NoError(t, err)
	assert.Equal(t, []string{"a.md", "sub/b.md", ".gitkeep"}, out)
}

// TestGit_ListTree_UnreachableSHA returns (nil, nil) when git exits
// non-zero — a missing or stale SHA yields no entries, not an error.
func TestGit_ListTree_UnreachableSHA(t *testing.T) {
	body := `exit 128`
	dir := fakeGit(t, body)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	p := withLastSyncSHA(t, "abcdef")
	g := NewGit(p, "/repo")
	out, err := g.ListTree(context.Background(), "skills")
	require.NoError(t, err)
	assert.Nil(t, out)
}

// TestGit_ListTree_EmptySubtree returns nil when the SHA exists but the
// dir has no files (git ls-tree exits 0 with no output).
func TestGit_ListTree_EmptySubtree(t *testing.T) {
	body := `exit 0`
	dir := fakeGit(t, body)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	p := withLastSyncSHA(t, "abcdef")
	g := NewGit(p, "/repo")
	out, err := g.ListTree(context.Background(), "agents")
	require.NoError(t, err)
	assert.Nil(t, out)
}

// TestGit_HeadSHA_HappyPath_FakeBinarySubprocess verifies the real
// exec.Command path: git rev-parse HEAD is invoked with the right
// args; the trailing newline is trimmed.
func TestGit_HeadSHA_HappyPath_FakeBinarySubprocess(t *testing.T) {
	body := `case "$*" in
"-C /repo rev-parse HEAD")
  printf 'abcdef1234567890abcdef1234567890abcdef12\n'
  exit 0 ;;
*) echo "unexpected: $*" >&2; exit 99 ;;
esac`
	dir := fakeGit(t, body)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	p := withLastSyncSHA(t, "")
	g := NewGit(p, "/repo")
	sha, err := g.HeadSHA(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "abcdef1234567890abcdef1234567890abcdef12", sha)
}

// TestGit_HeadSHA_NonGitDir wraps the git error when run outside a
// git repo. The error MUST propagate (unlike ListTree's exit-error
// silent fallback): callers want the full SHA on success or a real
// error on failure — no silent empty return.
func TestGit_HeadSHA_NonGitDir(t *testing.T) {
	body := `exit 128`
	dir := fakeGit(t, body)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	p := withLastSyncSHA(t, "")
	g := NewGit(p, "/nowhere")
	_, err := g.HeadSHA(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git rev-parse HEAD")
}

// TestGit_HeadSHA_CancelledContext returns context.Canceled when ctx
// is cancelled before c.Output returns. Mirrors the pattern in
// ListTree/ShowBase.
func TestGit_HeadSHA_CancelledContext(t *testing.T) {
	body := `sleep 5`
	dir := fakeGit(t, body)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	p := withLastSyncSHA(t, "")
	g := NewGit(p, "/repo")
	_, err := g.HeadSHA(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}
