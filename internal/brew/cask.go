package brew

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path"
	"strings"
)

// extractCaskFailureReason returns the trimmed last non-empty line of
// stderr. If stderr contained no non-empty lines, returns a fixed
// default string.
func extractCaskFailureReason(stderr []byte) string {
	var last string
	for line := range bytes.SplitSeq(stderr, []byte{'\n'}) {
		trimmed := strings.TrimSpace(string(line))
		if trimmed != "" {
			last = trimmed
		}
	}
	if last == "" {
		return "Homebrew cask command failed without an error message"
	}
	return last
}

// brewInfoCaskV2 is the partial shape of `brew info --cask --json=v2`
// output we care about — just enough to find .app artifacts.
type brewInfoCaskV2 struct {
	Casks []struct {
		Artifacts []map[string]any `json:"artifacts"`
	} `json:"casks"`
}

// parseCaskApps extracts .app artifact names from `brew info --cask
// --json=v2` output. Path prefixes are stripped (Applications/Foo.app
// → Foo) and the .app suffix is removed.
func parseCaskApps(data []byte) ([]string, error) {
	var info brewInfoCaskV2
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("parse brew info JSON: %w", err)
	}
	if len(info.Casks) == 0 {
		return nil, nil
	}
	var apps []string
	for _, art := range info.Casks[0].Artifacts {
		raw, ok := art["app"]
		if !ok {
			continue
		}
		list, ok := raw.([]any)
		if !ok {
			continue
		}
		for _, v := range list {
			s, ok := v.(string)
			if !ok {
				continue
			}
			base := path.Base(s)
			base = strings.TrimSuffix(base, ".app")
			apps = append(apps, base)
		}
	}
	return apps, nil
}

// IsCaskInstalled reports whether brew already has the cask registered,
// i.e. `brew list --cask <cask>` exits zero. A cancelled context
// propagates as an error (not as "not installed") so the orchestrator
// can stop the run instead of pressing on to brew install.
func (r *runner) IsCaskInstalled(ctx context.Context, cask string) (bool, error) {
	c := exec.CommandContext(ctx, "brew", "list", "--cask", cask)
	err := c.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return false, fmt.Errorf("brew list --cask %s: %w", cask, ctxErr)
	}
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false, nil
	}
	return false, fmt.Errorf("brew list --cask %s: %w", cask, err)
}

// ListCaskApps shells to `brew info --cask --json=v2 <cask>` and parses
// the result via parseCaskApps. A non-zero exit from brew (transient
// flake) AND a malformed JSON body (brew bug, partial output) are
// both treated as "no apps known" so the orchestrator falls through
// to attempting the install. Only catastrophic errors (binary
// missing, ctx cancelled) propagate.
func (r *runner) ListCaskApps(ctx context.Context, cask string) ([]string, error) {
	c := exec.CommandContext(ctx, "brew", "info", "--cask", "--json=v2", cask)
	out, err := c.Output()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, fmt.Errorf("brew info --cask %s: %w", cask, ctxErr)
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, nil
		}
		return nil, fmt.Errorf("brew info --cask %s: %w", cask, err)
	}
	apps, parseErr := parseCaskApps(out)
	if parseErr != nil {
		// An unparseable info payload is treated as "no apps known" so
		// a brew bug or partial output doesn't abort the cask loop.
		return nil, nil //nolint:nilerr // parse error reported via empty list, not error
	}
	return apps, nil
}

// InstallCask installs a single cask. If the cask is already installed and
// its app is running, returns Status=Skipped with no error. If the install
// fails, returns Status=Failed with the trimmed last non-empty stderr line
// as Reason and no error. Hard failures (the brew call itself errored)
// return a non-nil error for the caller to triage: the packages layer
// aborts the whole run only when the context was cancelled or brew
// could not run at all (any *exec.Error — missing or unrunnable
// binary; see packages.catastrophic), and records other hard errors
// as a per-cask failure gated by the failed-packages prompt.
func (r *runner) InstallCask(ctx context.Context, cask string) (CaskOutcome, error) {
	installed, err := r.IsCaskInstalled(ctx, cask)
	if err != nil {
		return CaskOutcome{}, fmt.Errorf("is cask installed %s: %w", cask, err)
	}
	if installed {
		apps, err := r.ListCaskApps(ctx, cask)
		if err != nil {
			return CaskOutcome{}, fmt.Errorf("list cask apps %s: %w", cask, err)
		}
		for _, app := range apps {
			running, err := r.IsAppRunning(ctx, app)
			if err != nil {
				return CaskOutcome{}, fmt.Errorf("is app running %s: %w", app, err)
			}
			if running {
				return CaskOutcome{Status: StatusSkipped}, nil
			}
		}
	}

	var stderr bytes.Buffer
	c := exec.CommandContext(ctx, "brew", "install", "--cask", cask)
	// User sees brew's live progress; we also capture stderr into a
	// buffer so we can extract the trimmed failure reason if the
	// install exits non-zero.
	c.Stdout = r.streams.Out
	c.Stderr = io.MultiWriter(&stderr, r.streams.Err)
	runErr := c.Run()
	// A cancelled context is "catastrophic" per the docstring, not a
	// limp-and-report failure — surface it so the orchestrator can
	// stop the run instead of marking the cask Failed and moving on.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return CaskOutcome{}, fmt.Errorf("brew install --cask %s: %w", cask, ctxErr)
	}
	if runErr != nil {
		// Per the limp-and-report contract: a non-zero `brew install`
		// surfaces as Status=Failed with the stderr summary, not as a
		// Go error, so the caller keeps installing the rest of the
		// list. A failure of the brew call itself (e.g. a missing
		// binary — typically caught upstream by IsCaskInstalled, but
		// possible if brew is removed mid-loop) returns *exec.Error,
		// not *exec.ExitError; return it as a hard error so the caller
		// can triage it (packages.catastrophic aborts the run for it)
		// instead of it masquerading as a stderr-style cask failure.
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) {
			return CaskOutcome{}, fmt.Errorf("brew install --cask %s: %w", cask, runErr)
		}
		return CaskOutcome{
			Status: StatusFailed,
			Reason: extractCaskFailureReason(stderr.Bytes()),
		}, nil
	}
	return CaskOutcome{Status: StatusInstalled}, nil
}
