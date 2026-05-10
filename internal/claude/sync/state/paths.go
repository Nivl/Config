// Package state owns the .claude/.sync-state/ filesystem: paths, the
// last-sync-commit anchor SHA, the decisions cache, backups, and the
// git interactions that resolve "what was in .claude/<rel> at the last
// successful sync." All I/O lives here.
package state

import "path/filepath"

// RepoSubdir is the path of the curated .claude directory inside the
// config repo, relative to CONFIG_DIR. The Go code and the git
// wrapper use this when constructing absolute paths and `<sha>:<path>`
// git refs.
const RepoSubdir = "shared_config/.claude"

// Paths bundles the locations the sync engine reads/writes.
// Constructed once by NewPaths from CONFIG_DIR + HOME.
type Paths struct {
	// ConfigDir is the root of the config repo (typically ~/.melvin/config).
	// It's the git repo's top-level directory; precommit-hook install
	// and `git -C` calls work off it.
	ConfigDir string
	// RepoDir is <ConfigDir>/shared_config/.claude — source of truth (the "remote" side).
	RepoDir string
	// HomeDir is $HOME/.claude — what Claude Code actually reads (the "local" side).
	HomeDir string
	// StateDir is <RepoDir>/.sync-state — gitignored bookkeeping dir.
	StateDir string
	// LastSyncFile is <StateDir>/last-sync-commit — anchor SHA for 3-way merge base.
	LastSyncFile string
	// DecisionsFile is <StateDir>/decisions.json — remembered choices.
	DecisionsFile string
}

// NewPaths builds a Paths from the config dir (typically ~/.melvin/config)
// and the home dir (typically $HOME). All six paths are derived; no
// directories are created — that's EnsureStateDir's job.
func NewPaths(configDir, homeDir string) Paths {
	repoDir := filepath.Join(configDir, RepoSubdir)
	homeClaudeDir := filepath.Join(homeDir, ".claude")
	stateDir := filepath.Join(repoDir, ".sync-state")
	return Paths{
		ConfigDir:     configDir,
		RepoDir:       repoDir,
		HomeDir:       homeClaudeDir,
		StateDir:      stateDir,
		LastSyncFile:  filepath.Join(stateDir, "last-sync-commit"),
		DecisionsFile: filepath.Join(stateDir, "decisions.json"),
	}
}
