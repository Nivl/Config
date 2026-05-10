package files

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Nivl/config/internal/claude/sync/prompt"
	"github.com/Nivl/config/internal/claude/sync/prompt/prompttest"
	"github.com/Nivl/config/internal/claude/sync/state"
	"github.com/Nivl/config/internal/claude/sync/state/statetest"
	"github.com/Nivl/config/internal/dryrun"
)

// TestMergeDir_EmptyUnionReturnsCleanNoCalls — empty union
// short-circuits without calling MergeFile.
func TestMergeDir_EmptyUnionReturnsCleanNoCalls(t *testing.T) {
	p := newMergeEnv(t)
	// No files in either tree, no base entries.
	fakeGit := statetest.NewFakeGit()
	fakeGit.On("ListTree", mock.Anything, "skills").Return([]string(nil), nil)

	r, err := MergeDir(context.Background(), p, fakeGit, "skills", Options{
		Prompter: prompttest.NewFakePrompter(),
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	assert.Empty(t, r.Files)
	assert.False(t, r.HadSkips)
	// Local dir was created even though the union was empty.
	info, err := os.Stat(filepath.Join(p.HomeDir, "skills"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

// TestMergeDir_FiltersGitkeepEverywhere — .gitkeep entries from all
// three sources are filtered out.
func TestMergeDir_FiltersGitkeepEverywhere(t *testing.T) {
	p := newMergeEnv(t)
	require.NoError(t, os.MkdirAll(filepath.Join(p.RepoDir, "agents"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "agents", ".gitkeep"), []byte(""), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(p.HomeDir, "agents"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "agents", ".gitkeep"), []byte(""), 0o644))
	fakeGit := statetest.NewFakeGit()
	fakeGit.On("ListTree", mock.Anything, "agents").Return([]string{".gitkeep"}, nil)

	r, err := MergeDir(context.Background(), p, fakeGit, "agents", Options{
		Prompter: prompttest.NewFakePrompter(),
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	assert.Empty(t, r.Files, "all three .gitkeep sources must be filtered")
}

// TestMergeDir_UnionSortedAscendingIteration — files from multiple
// sources, in mixed order, must merge in sort-ascending order.
func TestMergeDir_UnionSortedAscendingIteration(t *testing.T) {
	p := newMergeEnv(t)
	// Repo: c.md, a.md, sub/d.md
	require.NoError(t, os.MkdirAll(filepath.Join(p.RepoDir, "skills", "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "skills", "c.md"), []byte("c"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "skills", "a.md"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "skills", "sub", "d.md"), []byte("d"), 0o644))
	// Local: b.md (unique to local — local-add, row 10)
	require.NoError(t, os.MkdirAll(filepath.Join(p.HomeDir, "skills"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "skills", "b.md"), []byte("b"), 0o644))
	// Base: e.md (unique to base — clean local-delete that's still
	// reported in the walk, row 11 or 12).
	fakeGit := statetest.NewFakeGit()
	fakeGit.On("ListTree", mock.Anything, "skills").Return([]string{"e.md"}, nil)
	// Per-file ShowBase calls: a/c/sub-d not in base (ErrNoBase); b not
	// in base; e IS in base (return same as remote so we hit row 11 noop).
	fakeGit.On("ShowBase", mock.Anything, "skills/a.md").Return([]byte(nil), state.ErrNoBase)
	fakeGit.On("ShowBase", mock.Anything, "skills/b.md").Return([]byte(nil), state.ErrNoBase)
	fakeGit.On("ShowBase", mock.Anything, "skills/c.md").Return([]byte(nil), state.ErrNoBase)
	fakeGit.On("ShowBase", mock.Anything, "skills/sub/d.md").Return([]byte(nil), state.ErrNoBase)
	fakeGit.On("ShowBase", mock.Anything, "skills/e.md").Return([]byte("e-base"), nil)

	r, err := MergeDir(context.Background(), p, fakeGit, "skills", Options{
		Prompter: prompttest.NewFakePrompter(),
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	// Expected entries in sort order: a.md, b.md, c.md, e.md, sub/d.md.
	require.Len(t, r.Files, 5)
	// We don't assert per-entry rels (FileResult doesn't carry rel),
	// but the per-Action sequence proves the iteration order:
	// a.md  → row 13 remote-add → CopyRemoteToLocal
	// b.md  → row 10 local-add  → Noop
	// c.md  → row 13 remote-add → CopyRemoteToLocal
	// e.md  → row 11 clean local-delete (has_L=0, has_B=1, R≠B since
	//                                    remote is absent → has_R=0 too,
	//                                    actually base-only is unreachable
	//                                    pattern → defaults to Noop)
	// sub/d.md → row 13 remote-add → CopyRemoteToLocal
	assert.Equal(t, ActionCopyRemoteToLocal, r.Files[0].Decision.Action)
	assert.Equal(t, ActionNoop, r.Files[1].Decision.Action)
	assert.Equal(t, ActionCopyRemoteToLocal, r.Files[2].Decision.Action)
	assert.Equal(t, ActionNoop, r.Files[3].Decision.Action)
	assert.Equal(t, ActionCopyRemoteToLocal, r.Files[4].Decision.Action)
}

// TestMergeDir_MkdirpCreatesMissingLocalDir — local dir created on
// demand even when no entries exist.
func TestMergeDir_MkdirpCreatesMissingLocalDir(t *testing.T) {
	p := newMergeEnv(t)
	// Deliberately do NOT create $HOME/.claude/commands.
	fakeGit := statetest.NewFakeGit()
	fakeGit.On("ListTree", mock.Anything, "commands").Return([]string(nil), nil)

	_, err := MergeDir(context.Background(), p, fakeGit, "commands", Options{
		Prompter: prompttest.NewFakePrompter(),
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	info, err := os.Stat(filepath.Join(p.HomeDir, "commands"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

// TestMergeDir_HadSkipsBubbles — any per-file Skip bubbles up.
func TestMergeDir_HadSkipsBubbles(t *testing.T) {
	p := newMergeEnv(t)
	require.NoError(t, os.MkdirAll(filepath.Join(p.RepoDir, "agents"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "agents", "x.md"), []byte("remote"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(p.HomeDir, "agents"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "agents", "x.md"), []byte("local"), 0o644))
	fakeGit := statetest.NewFakeGit()
	fakeGit.On("ListTree", mock.Anything, "agents").Return([]string(nil), nil)
	fakeGit.On("ShowBase", mock.Anything, "agents/x.md").Return([]byte("base"), nil)
	fakeP := prompttest.NewFakePrompter()
	fakeP.On("Resolve", mock.Anything, mock.Anything).Return(prompt.ChoiceSkip, nil)

	r, err := MergeDir(context.Background(), p, fakeGit, "agents", Options{
		Prompter: fakeP,
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	assert.True(t, r.HadSkips)
}

// TestMergeDir_ListTreeErrorPropagates — when git.ListTree fails,
// MergeDir wraps the error and aborts before iterating MergeFile.
func TestMergeDir_ListTreeErrorPropagates(t *testing.T) {
	p := newMergeEnv(t)
	fakeGit := statetest.NewFakeGit()
	fakeGit.On("ListTree", mock.Anything, "skills").
		Return([]string(nil), errors.New("rpc error"))

	_, err := MergeDir(context.Background(), p, fakeGit, "skills", Options{
		Prompter: prompttest.NewFakePrompter(),
		Reporter: dryrun.NewNullReporter(),
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "list base subtree")
}
