// Package brew provides primitives for invoking homebrew. It is the only
// package in this module that calls exec.Command("brew", ...) — all other
// packages talk to brew through the Runner interface.
package brew

import (
	"context"
	"fmt"

	"github.com/Nivl/config/internal/dryrun"
	"github.com/Nivl/config/internal/iox"
)

// Runner is the interface every brew operation goes through. The production
// implementation (NewRunner) shells out to the brew CLI; tests inject a
// FakeRunner backed by testify/mock.
type Runner interface {
	// Upgrade runs `brew upgrade` to bring installed packages current.
	// With no arguments it upgrades all outdated formulae (the blanket
	// pass is scoped to --formula so it never quits/upgrades a running
	// cask app); with names it upgrades just those packages (used to
	// retry a failed subset). Casks are upgraded by InstallCask, which
	// skips any whose app is running.
	Upgrade(ctx context.Context, packages ...string) error
	// Outdated runs `brew outdated --quiet` (optionally scoped, e.g.
	// "--formula") and returns the names of packages that still have a
	// newer version available. Used after a failed upgrade (bulk or
	// scoped retry) to approximate the failed subset — brew limps
	// through every package, so the leftovers are treated as the
	// failures. See packages.outdatedAfterFailedUpgrade for why the
	// list is a proxy, not ground truth.
	Outdated(ctx context.Context, scopes ...string) ([]string, error)
	// OutdatedCasks runs `brew outdated --cask --quiet` and returns the
	// tokens of installed casks with a newer version available
	// (non-greedy, so self-updating casks are excluded). The packages
	// layer feeds these through InstallCask so every outdated cask —
	// managed or not — is upgraded behind the running-app skip, never by
	// a bare `brew upgrade` that would quit the app.
	OutdatedCasks(ctx context.Context) ([]string, error)
	// Install installs one or more formulae in a single `brew install` call.
	Install(ctx context.Context, formulae ...string) error
	// InstallCask installs a single cask with limp-and-report semantics:
	// it skips if the cask is already installed and its app is running,
	// captures the last non-empty stderr line as the failure reason on
	// error, and never returns a non-nil error for those soft outcomes.
	InstallCask(ctx context.Context, cask string) (CaskOutcome, error)
	// IsCaskInstalled reports whether brew already has the cask registered.
	IsCaskInstalled(ctx context.Context, cask string) (bool, error)
	// IsAppRunning reports whether the named macOS application is currently
	// running, using both `pgrep -ix` and `osascript tell application`.
	IsAppRunning(ctx context.Context, app string) (bool, error)
	// ListCaskApps parses `brew info --cask --json=v2` and returns the
	// .app artifact names with `.app` stripped and any path prefix removed.
	ListCaskApps(ctx context.Context, cask string) ([]string, error)
}

// CaskStatus encodes the three terminal outcomes of an InstallCask call.
type CaskStatus int

const (
	// StatusInstalled means brew completed the cask install successfully.
	StatusInstalled CaskStatus = iota
	// StatusSkipped means the cask was already installed AND its app was
	// running, so we declined to upgrade rather than risk killing the app.
	StatusSkipped
	// StatusFailed means brew returned a non-zero exit. The CaskOutcome's
	// Reason field carries the trimmed last non-empty line of stderr.
	StatusFailed
)

// CaskOutcome carries per-cask "limp and report" data from InstallCask.
// Reason is set only when Status == StatusFailed; it carries the
// trimmed last non-empty stderr line from the failing brew call.
type CaskOutcome struct {
	// Reason is the failure reason; empty unless Status == StatusFailed.
	Reason string
	// Status is one of the three CaskStatus values.
	Status CaskStatus
}

// NewRunner returns the production Runner implementation that shells
// to the brew CLI. The streams bundle is wired into each brew
// subprocess: Out receives brew's stdout, Err receives its stderr.
// In is unused (brew commands aren't interactive). Performs no I/O at
// construction time.
func NewRunner(streams iox.Streams) Runner {
	return &runner{streams: streams}
}

// runner is the production Runner. It is unexported so consumers always
// go through the Runner interface — this keeps the package's call sites
// trivially mockable.
type runner struct {
	// streams is the I/O bundle threaded into each brew subprocess so
	// callers (tests, non-TTY modes) can redirect output.
	streams iox.Streams
}

// dryRunRunner wraps a real Runner: write methods (Upgrade, Install,
// InstallCask) no-op + report via the configured Reporter. Read
// methods (Outdated, IsCaskInstalled, IsAppRunning, ListCaskApps)
// delegate to the wrapped runner so dry-run output reflects the actual
// state of the system.
type dryRunRunner struct {
	wrapped  Runner
	reporter dryrun.Reporter
}

// NewDryRunWrapper returns a Runner that no-ops + reports every
// side-effecting method (Upgrade/Install/InstallCask) and delegates
// every read method (Outdated/IsCaskInstalled/IsAppRunning/
// ListCaskApps) to the wrapped runner. Use this in conjunction with a
// real NewRunner(streams) under --dry-run.
func NewDryRunWrapper(wrapped Runner, reporter dryrun.Reporter) Runner {
	return &dryRunRunner{wrapped: wrapped, reporter: reporter}
}

// Upgrade reports the brew upgrade shellout and returns nil
// without invoking the wrapped runner.
func (r *dryRunRunner) Upgrade(_ context.Context, packages ...string) error {
	args := []string{"upgrade"}
	desc := "upgrade all outdated formulae"
	if len(packages) == 0 {
		args = append(args, "--formula")
	} else {
		desc = fmt.Sprintf("upgrade %d package(s)", len(packages))
	}
	args = append(args, packages...)
	r.reporter.Shellout("brew", args, desc)
	return nil
}

// Outdated delegates to the wrapped runner — reads run normally.
func (r *dryRunRunner) Outdated(ctx context.Context, scopes ...string) ([]string, error) {
	return r.wrapped.Outdated(ctx, scopes...)
}

// OutdatedCasks delegates to the wrapped runner — reads run normally.
func (r *dryRunRunner) OutdatedCasks(ctx context.Context) ([]string, error) {
	return r.wrapped.OutdatedCasks(ctx)
}

// Install reports a brew install of the given formulae and returns
// nil without invoking the wrapped runner.
func (r *dryRunRunner) Install(_ context.Context, formulae ...string) error {
	r.reporter.Shellout("brew", append([]string{"install"}, formulae...),
		fmt.Sprintf("install %d formula(e)", len(formulae)))
	return nil
}

// InstallCask mirrors the production decision tree so the dry-run
// preview matches what the real run would do: an absent cask reports a
// would-`install --cask`; an installed cask whose apps are all quiescent
// reports a would-`upgrade --cask`; an installed cask with a running app
// returns StatusSkipped without reporting anything.
//
// The read methods used here (IsCaskInstalled, ListCaskApps,
// IsAppRunning) are pure observations and safe to invoke under
// --dry-run.
func (r *dryRunRunner) InstallCask(ctx context.Context, cask string) (CaskOutcome, error) {
	installed, err := r.wrapped.IsCaskInstalled(ctx, cask)
	if err != nil {
		return CaskOutcome{}, fmt.Errorf("is cask installed %s: %w", cask, err)
	}
	if !installed {
		r.reporter.Shellout("brew", []string{"install", "--cask", cask},
			"install cask")
		return CaskOutcome{}, nil
	}
	apps, err := r.wrapped.ListCaskApps(ctx, cask)
	if err != nil {
		return CaskOutcome{}, fmt.Errorf("list cask apps %s: %w", cask, err)
	}
	for _, app := range apps {
		running, err := r.wrapped.IsAppRunning(ctx, app)
		if err != nil {
			return CaskOutcome{}, fmt.Errorf("is app running %s: %w", app, err)
		}
		if running {
			return CaskOutcome{Status: StatusSkipped}, nil
		}
	}
	r.reporter.Shellout("brew", []string{"upgrade", "--cask", cask},
		"upgrade cask")
	return CaskOutcome{}, nil
}

// IsCaskInstalled delegates to the wrapped runner — reads run normally.
func (r *dryRunRunner) IsCaskInstalled(ctx context.Context, cask string) (bool, error) {
	return r.wrapped.IsCaskInstalled(ctx, cask)
}

// IsAppRunning delegates to the wrapped runner — reads run normally.
func (r *dryRunRunner) IsAppRunning(ctx context.Context, app string) (bool, error) {
	return r.wrapped.IsAppRunning(ctx, app)
}

// ListCaskApps delegates to the wrapped runner — reads run normally.
func (r *dryRunRunner) ListCaskApps(ctx context.Context, cask string) ([]string, error) {
	return r.wrapped.ListCaskApps(ctx, cask)
}
