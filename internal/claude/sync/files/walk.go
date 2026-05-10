package files

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Nivl/config/internal/claude/sync/state"
)

// DirResult aggregates per-file results from a directory walk.
type DirResult struct {
	// Files is the list of per-file FileResults, in sort-ascending
	// order of relative path.
	Files []FileResult
	// HadSkips is true if any file's MergeFile ended in HadSkip.
	HadSkips bool
}

// MergeDir mkdir-p's the local dir, builds the union of L/R/B file
// lists with .gitkeep filtered, sorts ascending, and calls MergeFile
// per entry.
//
// dirName is relative to .claude/ (e.g. "skills"). An empty union
// list returns clean (Files == nil, HadSkips == false).
//
// Pipeline:
//   - mkdir -p the local dir before scanning.
//   - Union of remote walk + local walk + git ls-tree of the base SHA.
//   - Filter .gitkeep at every step.
//   - Sort the union; empty union short-circuits.
//   - Call MergeFile per entry in sort order.
func MergeDir(ctx context.Context, paths state.Paths, git state.Git,
	dirName string, opts Options) (DirResult, error) {
	localDir := filepath.Join(paths.HomeDir, dirName)
	remoteDir := filepath.Join(paths.RepoDir, dirName)

	if opts.DryRun {
		// MkdirAll is a no-op on existing dirs; report accordingly so
		// the dry-run preview doesn't claim already-present
		// ~/.claude/<subdir>s "would change".
		if info, err := os.Stat(localDir); err == nil && info.IsDir() {
			opts.Reporter.FileNoOp(localDir, "dir already exists")
		} else {
			opts.Reporter.FileChange(localDir, nil, nil, "would mkdir 0o755")
		}
	} else {
		if err := os.MkdirAll(localDir, 0o755); err != nil {
			return DirResult{}, fmt.Errorf("mkdir local %s: %w", localDir, err)
		}
	}

	seen := map[string]struct{}{}
	if err := collectDirFiles(remoteDir, seen); err != nil {
		return DirResult{}, fmt.Errorf("scan remote %s: %w", remoteDir, err)
	}
	if err := collectDirFiles(localDir, seen); err != nil {
		return DirResult{}, fmt.Errorf("scan local %s: %w", localDir, err)
	}
	baseEntries, err := git.ListTree(ctx, dirName)
	if err != nil {
		return DirResult{}, fmt.Errorf("list base subtree %s: %w", dirName, err)
	}
	// .gitkeep filter is duplicated here (also in collectDirFiles)
	// because the base source comes from git.ListTree, not through
	// collectDirFiles. Every source must filter.
	for _, rel := range baseEntries {
		if rel == "" || rel == ".gitkeep" || strings.HasSuffix(rel, "/.gitkeep") {
			continue
		}
		seen[rel] = struct{}{}
	}

	if len(seen) == 0 {
		return DirResult{}, nil
	}
	rels := make([]string, 0, len(seen))
	for k := range seen {
		rels = append(rels, k)
	}
	sort.Strings(rels)

	result := DirResult{Files: make([]FileResult, 0, len(rels))}
	for _, rel := range rels {
		r, err := MergeFile(ctx, paths, git, dirName+"/"+rel, opts)
		if err != nil {
			return result, fmt.Errorf("merge file %s/%s: %w", dirName, rel, err)
		}
		result.Files = append(result.Files, r)
		if r.HadSkip {
			result.HadSkips = true
		}
	}
	return result, nil
}

// collectDirFiles walks root and adds every REGULAR file (relative
// to root, with .gitkeep filtered) into seen. Non-regular entries
// (directories, symlinks, FIFOs) are silently skipped. Missing root
// is a no-op.
func collectDirFiles(root string, seen map[string]struct{}) error {
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return fs.SkipAll
			}
			return walkErr
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("relative path: %w", err)
		}
		if rel == ".gitkeep" || strings.HasSuffix(rel, "/.gitkeep") {
			return nil
		}
		// Normalize to forward slashes.
		seen[filepath.ToSlash(rel)] = struct{}{}
		return nil
	})
	if errors.Is(err, fs.SkipAll) {
		return nil
	}
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("walk dir: %w", err)
	}
	return nil
}
