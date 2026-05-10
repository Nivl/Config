package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Nivl/config/internal/errutil"
)

// Choice is the persisted form of a "remembered" decision. "local" →
// keep-local, "remote" → take-remote. "skip" is intentionally not
// persistable — skipping a conflict is a per-run choice, not a
// remembered preference.
type Choice string

const (
	// ChoiceLocal means "keep the local copy unchanged".
	ChoiceLocal Choice = "local"
	// ChoiceRemote means "apply the remote value".
	ChoiceRemote Choice = "remote"
)

// Decisions is the typed shape of decisions.json.
//
// Settings keys are the compact-JSON encoding of the merge-unit path
// array. Examples: `["model"]`, `["permissions","allow"]`. Keys must
// be byte-stable so the cache survives across versions.
//
// Files keys are relative paths under .claude/, e.g. "CLAUDE.md".
//
//nolint:govet // fieldalignment: JSON schema order is non-negotiable
type Decisions struct {
	Version  int               `json:"version"`
	Settings map[string]Choice `json:"settings"`
	Files    map[string]Choice `json:"files"`
}

// LoadDecisions reads p.DecisionsFile. Returns an empty (but non-nil)
// Decisions struct if the file is missing. Returns a wrapped error on
// I/O failure or JSON parse failure.
func LoadDecisions(p Paths) (Decisions, error) {
	d := Decisions{
		Settings: map[string]Choice{},
		Files:    map[string]Choice{},
	}
	data, err := os.ReadFile(p.DecisionsFile)
	if errors.Is(err, os.ErrNotExist) {
		return d, nil
	}
	if err != nil {
		return d, fmt.Errorf("read decisions.json: %w", err)
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return d, fmt.Errorf("parse decisions.json: %w", err)
	}
	// Defensive: ensure maps are non-nil even if the file had explicit nulls.
	if d.Settings == nil {
		d.Settings = map[string]Choice{}
	}
	if d.Files == nil {
		d.Files = map[string]Choice{}
	}
	return d, nil
}

// SaveDecisions writes d to p.DecisionsFile atomically (tempfile +
// rename). Uses MarshalJq so cache keys containing HTML-special chars
// stay byte-stable.
func SaveDecisions(p Paths, d Decisions) (err error) {
	data, err := MarshalJq(d)
	if err != nil {
		return fmt.Errorf("marshal decisions: %w", err)
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(p.DecisionsFile), "decisions-*.tmp")
	if err != nil {
		return fmt.Errorf("create tempfile: %w", err)
	}
	defer errutil.RunAndSetError(
		func() error { return CleanupTempFile(tmp) },
		&err, "cleanup tempfile",
	)
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write decisions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close tempfile: %w", err)
	}
	if err := os.Rename(tmp.Name(), p.DecisionsFile); err != nil {
		return fmt.Errorf("rename to decisions.json: %w", err)
	}
	return nil
}
