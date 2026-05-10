package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewPaths verifies the six derived paths.
func TestNewPaths(t *testing.T) {
	p := NewPaths("/home/u/.melvin/config", "/home/u")
	assert.Equal(t, "/home/u/.melvin/config", p.ConfigDir)
	assert.Equal(t, "/home/u/.melvin/config/shared_config/.claude", p.RepoDir)
	assert.Equal(t, "/home/u/.claude", p.HomeDir)
	assert.Equal(t, "/home/u/.melvin/config/shared_config/.claude/.sync-state", p.StateDir)
	assert.Equal(t, "/home/u/.melvin/config/shared_config/.claude/.sync-state/last-sync-commit", p.LastSyncFile)
	assert.Equal(t, "/home/u/.melvin/config/shared_config/.claude/.sync-state/decisions.json", p.DecisionsFile)
}
