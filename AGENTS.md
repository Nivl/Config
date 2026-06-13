# Repository instructions

This is a personal macOS bootstrap and shell-config repo deployed to `~/.melvin/config`. Read this file before making non-trivial changes.

## Keep this file fresh

When you make a change that matches a row below, update the named section in the same change. Out-of-date instructions are worse than missing ones — future agents will trust this file, so don't let it lie. If your change doesn't fit any row but future-you would still want to know about it, add a row.

| If you change… | Update this section |
|---|---|
| Top-level layout (new/removed dir or top-level dotfile) | [Layout](#layout) |
| `melvin-config setup` order, a subsystem package, or an env-var contract | [Bootstrap flow](#bootstrap-flow) + [Env vars + flags](#env-vars--flags) |
| Claude config sync semantics (base SHA, decisions cache, merge strategy, sync-state files) | [Claude Code config sync](#claude-code-config-sync) |
| `tests/*.sh` added/removed/renamed, or `.github/workflows/*.yml` jobs | [Tests & CI](#tests--ci) |
| Top-level helper in `config.zshrc` (the `run`/`add`/`install`/`lint`/`wt`/`cl` family) | [Shell config](#shell-config) |
| Symlinked dotfile set in `dotfiles.CopyConfigFiles` (`dotfileItems` constant in `internal/dotfiles/dotfiles.go`) | [Layout](#layout) + [Bootstrap flow](#bootstrap-flow) |
| Go package added/renamed/removed under `internal/` | [Go packages](#go-packages) + [Bootstrap flow](#bootstrap-flow) + [Env vars + flags](#env-vars--flags) |
| Top-level `melvin-config` subcommand added/renamed/removed | [Go packages](#go-packages) (`internal/cmd` row) |
| Convention/idiom established or changed (error wrapping, doc comments, resolve helpers, test sub-packages, etc.) | [Conventions](#conventions) |

## What this repo is

A personal dotfiles + macOS bootstrap repository. The source of truth is this repo at `~/.melvin/config`; `$HOME` gets thin symlinks back into it, plus a small set of materialized config files (`~/.zshrc`, `~/.gitconfig`, `~/.zprofile`) whose repo-relative content is managed via a sentinel-fenced block that gets re-checked on every setup run.

The runtime is a Go binary (`cmd/melvin-config`) backed by the packages under `internal/`. The user-facing entrypoint is `install.bootstrap.sh`, which resolves prereqs (brew, git, go), clones/pulls the repo, builds the `melvin-config` binary, and execs `melvin-config setup`. See [Go packages](#go-packages) for what each package owns.

## Layout

| Path | What it is |
|---|---|
| `install.bootstrap.sh` | User-facing entrypoint. Resolves prereqs (brew, git, go), clones/pulls the repo, builds the `melvin-config` binary, then execs `melvin-config setup`. The only bash file in the runtime — the chicken-and-egg shim for installing Go itself. |
| `shared_config/` | The shared user-facing config payload — files that get installed into `$HOME` (symlinked or `[include]`-d) or injected in the environment. Add new user-visible files here, not at the repo root. Subdirs called out below. |
| `shared_config/base.zshrc` | Stable shell entrypoint sourced from generated `~/.zshrc`. Oh My Zsh + theme + `PATH` baseline + `melvin-config check-update` startup hook. Sources `config.zshrc` at the end. |
| `shared_config/config.zshrc` | Day-to-day aliases and helper functions. Sourced from `shared_config/base.zshrc`. |
| `shared_config/.gitconfig` + `shared_config/.gitmessage` | Canonical git config. `~/.gitconfig` is materialized to `[include]` this file. |
| `shared_config/.golangci.yml` + `shared_config/lsd.yaml` | Tool configs. `.golangci.yml` is symlinked into `$HOME`; `lsd.yaml` is passed to `lsd` via the `lsd` alias in `shared_config/config.zshrc`. |
| `shared_config/.oh-my-zsh/` | Git submodule pointing at `ohmyzsh/ohmyzsh` (see `.gitmodules`). `$ZSH` env var points here. `install.bootstrap.sh` auto-bumps the submodule to upstream HEAD on `PERSONAL_COMPUTER=true` machines after the parent-repo pull, then auto-commits + pushes the bump so other personal machines pick it up on their next pull. |
| `shared_config/.oh-my-zsh-custom/` | User customizations kept *outside* the OMZ submodule (because upstream OMZ's `.gitignore` excludes `custom/`). `$ZSH_CUSTOM` env var points here. Currently holds `themes/nivl.zsh-theme`. |
| `shared_config/.emacs.d/` | Tracked emacs config, symlinked to `~/.emacs.d`. |
| `shared_config/.bin-remote/` | Build output for the `melvin-config` binary. Gitignored except the `.keep` sentinel that tracks the empty dir. Prepended to `PATH` by `base.zshrc`. |
| `shared_config/.claude/` | Curated Claude Code config: `settings.json` + `skills/` + `agents/` + `commands/` + `hooks/`. Synced into `~/.claude/` per the [Claude Code config sync](#claude-code-config-sync) flow. Runtime subdirs (`projects/`, `sessions/`, `plugins/`, …) are gitignored. Permissions are mutated via `melvin-config claude perms {allow\|ask\|deny} {add\|remove} [--bash X] [--read X] [--fetch X] [--skill X] [--force] [--dry-run]`, which writes `settings.json` (each `--bash` value gets a raw + PATH-resolved variant). `hooks/` holds PreToolUse guards: `git-deny-dash-c.py` denies `git -C <path>` (cd into the repo first, then plain git), `cd-under-roots.py` allows `cd` into /tmp/$WORKTREES_ROOT/$REPOS_ROOT and denies chained `cd`, `write-under-roots.py` auto-allows writes under those roots (plus $HOME/.melvin/config) but downgrades writes traversing a `.git` segment (case-insensitive) to ASK, `read-guard.py` auto-allows reads under those same roots but ASKs for sensitive files (`.env`, private keys, credential stores) anywhere, `gh-api-write-guard.py` auto-allows read-only `gh api`, `rm-under-tmp.py` auto-allows `rm`/`rmdir`/`unlink` whose every target stays inside `/tmp` (lexical + realpath checked) and ASKs otherwise, `deny-json-tool.py` denies `python -m json.tool` at any command-position and steers the agent at `jq` instead, `deny-awk.py` denies `awk`/`gawk`/`mawk`/`nawk` at any command-position and steers at `cut`/`sed`/`grep`/`head`/`tail`/`jq` (awk is a full programming language with `system()` and file redirects, too much surface to allow-list), `deny-multi-command.py` denies Bash invocations that bundle multiple statements via `;` or a bare newline (forces one-command-per-call so each invocation is vetted independently; `&&`/`||`/`|` are still allowed because they express semantics one-call-at-a-time can't replicate), `deny-shell-wrapper.py` denies inline-scripting shapes (shell `-c`, interpreter `-c`/`-e`/`--eval`, function defs, `for`/`while`/`until`/`select` loops, `eval`/`source`/`.`, interpreter heredocs, multi-line ANSI-C strings), `deny-command-substitution.py` denies command/process substitution (`$(...)`, backticks, `<(...)` / `>(...)`) wherever the shell would actually expand it — tracking quote state so single-quoted literals, arithmetic `$((...))`, and `${VAR}` expansion stay allowed — closing the same leading-token bypass as `deny-shell-wrapper.py`, `deny-env-vars.py` denies Bash commands that would expand a banned env var (the `BANNED_VARS` list, currently just `$TMPDIR`, whose macOS `/var/folders` target the sandbox write-denies) using the same quote-state tracking — single-quoted/escaped literals and `TMPDIR=/x cmd` assignments stay allowed, `bash-allow-trusted.py` auto-allows a lone allow-listed sandbox-excluded command (git/gh/docker/go — which run unsandboxed where `permissions.allow` can't suppress prompts; the trusted set is exactly allow ∩ excluded, so it includes mutating-but-allow-listed subcommands like `git add`/`gh run rerun`, not only read-only ones) and denies excluded commands that are piped or chained, matching token-tuple prefix lists derived from `settings.json` into `bash-allow-trusted.json` by `melvin-config claude perms` — see `shared_config/.claude/AGENTS.md`. Each resolves a relative `file_path`/target against the hook input's `cwd`, so a subagent reading/writing from another directory still matches. |
| `cmd/melvin-config/` | Cobra CLI binary entrypoint. `main.go` is a thin shell around `internal/cmd`. |
| `internal/` | Go runtime packages. See [Go packages](#go-packages) for what each owns. |
| `go.mod` / `go.sum` | Go module manifest pinning the dep graph that builds `melvin-config`. CI lookup via `go-version-file: go.mod`. |
| `.golangci.yml` (repo root) | The project lint config picked up automatically by `golangci-lint` when invoked at the repo root. Mirrors the content of `shared_config/.golangci.yml` (which is the file symlinked into `$HOME` for user-level use). |
| `.githooks/` | Versioned git hooks installed by `internal/claude/sync/precommit.InstallPrecommitHook` into `.git/hooks/` as a relative symlink. Currently holds `pre-commit` (canonicalizes staged JSON files under `.claude/`) and `canonicalize.jq` (the shared jq filter that sorts+dedupes any array of primitives). |
| `.github/workflows/` | CI definitions. |
| `tests/` | Bash test scripts run by CI and locally. |
| `.vscode/` | Editor settings. |
| `README.md` | The curl-bootstrap one-liner and its `--dry-run` sibling. |

## Bootstrap flow

Two layers: `install.bootstrap.sh` (bash; the chicken-and-egg shim) then `melvin-config setup` (Go; the runtime).

**`install.bootstrap.sh`** runs in this order:

1. **`PERSONAL_COMPUTER` prompt (interactive):** if the env var is unset, prompt the user (y/n) and persist the answer to `~/.zprofile`. Must happen here — *before* the Go handoff — because `packages.Install` reads the env var to decide whether to install `PersonalCasks`. `userinput.Personal` is the in-process fallback when the bootstrap is skipped.
2. **Brew + Go bootstrap (shell-only):** install Homebrew if missing, then `brew install git go`. Must stay in shell because the Go binary doesn't yet exist.
3. **Clone or update `$CONFIG_DIR`:** `git clone` if absent; `git pull` otherwise. If new commits were fetched, `exec` into the freshly-pulled `install.bootstrap.sh`, propagating `PERSONAL_COMPUTER`, `CLAUDE_MERGE_RESOLUTION`, `INSTALL_FAILURE_RESOLUTION`, `MELVIN_DRY_RUN`, and the internal `_MELVIN_REEXECED=1` guard.
4. **Build the Go binary:** `go build -o $CONFIG_DIR/shared_config/.bin-remote/melvin-config ./cmd/melvin-config`. Go's per-package cache makes incremental builds sub-100ms. The output dir is added to `PATH` by `shared_config/base.zshrc` so the binary is reachable without the user having `~/.local/bin` on `PATH`.
5. **Exec `melvin-config setup`:** the Go binary owns the rest.

**`melvin-config setup`** (Go) runs, in order:

- **Resolve user inputs** (`internal/userinput`) — `Personal`, `DevRoot`, `GitOrg`, `GitHost`. Each takes a pre-resolved `prearg` (flag → env wins handled by the cmd layer); if `prearg` is empty, the prompt fires.
- **Install packages** (`internal/packages` + `internal/brew`) — brew upgrade + formula + cask install with cask-running guard; every step limps past per-package failures, then `InstallWithRetry` prompts abort / retry-failed-only / ignore over whatever is left (pre-answerable via `INSTALL_FAILURE_RESOLUTION` / `--install-failure-resolution`).
- **Claude Code config sync** (`internal/claude/sync`) — symlink mode (`PERSONAL_COMPUTER=true`) or copy mode (3-way merge). Precommit hook install runs in both modes.
- **Dotfile copy + config generation** (`internal/dotfiles` + `internal/configgen` + `internal/managedblock`) — symlinks 2 curated dotfiles (via `internal/symlinkfs.Install`; existing targets that aren't already the right symlink get auto-backed-up as `<name>.<YYYYMMDDHHMMSS>.bkp` then replaced) + creates `~/.emacs-saves`; materializes `~/.zshrc`, `~/.gitconfig`, and `~/.gnupg/gpg-agent.conf`. The repo-relative `source` / `[include]` / `pinentry-program` lines inside those materialized files live in a sentinel-fenced managed block that gets re-checked and rewritten on every `setup` run — user customizations outside the block are preserved.
- **Final application setup** (`internal/appsetup`) — `SetupSSH` (ed25519 keypair if missing) + `SetupGitHub` (gh auth status probe + optional `gh auth login -w`).
- **Success stamp write** — `<configDir>/.last_update_check` gets the current Unix timestamp. Consumed by `melvin-config check-update`, invoked from `base.zshrc` on shell startup to gate the 14-day update prompt.
- **Post-install reminder + zshrc-reload hint** — `PrintRemainingTasks` lists optional follow-ups (SSH-key upload if not authenticated, EasyRes, PGP import); the stderr hint tells the user to `source ~/.zshrc`.

**Dry-run mode** (`--dry-run` / `MELVIN_DRY_RUN=true`): the `melvin-config setup` phase emits per-file unified diffs to stderr describing every change it *would* make, then exits without writing anything. Bootstrap prereq steps (brew install, go install, repo clone, binary build) run for real regardless — they're user-invisible scaffolding, not config decisions. Dry-run also covers `claude perms add`/`remove` (settings.json mutation + git commit are previewed but never executed).

**`melvin-config update`** is the post-install entrypoint into the same flow. It `syscall.Exec`s into `install.bootstrap.sh` so the running binary's process image is gone before `go build -o $BIN` rewrites the file on disk — no risk of the old binary clinging to anything. Args after `update` (e.g. `--dry-run`, `--personal`) forward verbatim through to `setup`. `melvin-config check-update` is the shell-startup hook called from `base.zshrc`; it stamps `.last_update_check`, prompts only on a real TTY, then hands off to the same `syscall.Exec` path.

### Go packages

Each package has a single clear responsibility. The cmd layer (`internal/cmd`) owns config resolution + cobra wiring; the leaf packages own behavior and never read env or flags directly.

| Package | Owns |
|---|---|
| `internal/cmd` | Cobra subcommand graph: `setup`, `update`, `check-update`, `claude sync`, `claude perms {allow\|ask\|deny} {add\|remove}`, `install packages`, `version`. `appConfig` holds shared deps + `iox.Streams`. Hosts `resolveBool` / `resolveString` / `resolveBoolAsString` — the single boundary that reads `os.Getenv` for config knobs. `update` re-execs into `install.bootstrap.sh` via `syscall.Exec` (no race with in-place binary rewrite); `check-update` reads `.last_update_check`, no-ops if <14 days, otherwise stamps + prompts (only on a real TTY — `term.IsTerminal`, not `ModeCharDevice`, so `/dev/null` isn't treated as interactive). |
| `internal/iox` | `Streams{In, Out, Err}` value type + `System()` factory. The only sanctioned spot to touch process-global `os.Stdin/Stdout/Stderr` outside `main`. |
| `internal/appsetup` | SSH key gen, GitHub auth, post-install reminders. `CmdRunner` inherits stdio so interactive subprocess flows (ssh-keygen passphrase, gh browser login) work. |
| `internal/brew` | Sole owner of `exec.Command("brew", …)`. Real `Runner` shells out; fakes for tests live in `internal/brew/brewtest`. |
| `internal/claude/sync` | Top-level `Sync` + symlink mode + precommit hook install + last-sync-commit advance. Sub-packages: `state` (paths, git wrapper, decisions cache, atomic file writes), `prompt` (3-tier conflict resolver), `settings` (settings.json merge with set semantics for `permissions.*`), `files` (per-file + per-dir 3-way merge). |
| `internal/claude/perms` | Mutates `permissions.{allow,ask,deny}` in `shared_config/.claude/settings.json` for the `melvin-config claude perms` command. `Variants(kind, value)` fans a value out into rule strings (a `--bash` value → raw `Bash(value)` + PATH-resolved twin; git is NOT special-cased, since `git -C` is denied at the hook layer), `Settings` JSON round-trip preserves unknown top-level + `permissions.*` keys via `json.RawMessage` (with `json.Indent` normalization pass), `Add`/`Remove` with cross-list conflict detection (errors without `--force`; `--force` removes from every other list). Personal-computer-only — `cmd` enforces. |
| `internal/configgen` | Materializes `~/.zshrc`, `~/.gitconfig`, `~/.gnupg/gpg-agent.conf`; re-checks and rewrites the sentinel-fenced managed block (via `internal/managedblock`) on every setup run. `~/.gnupg` uses `0o700` (gpg's expectation); `killall gpg-agent` exit code 1 ("no such process") is ignored rather than treated as an error. |
| `internal/dotfiles` | `CopyConfigFiles` symlinks `.emacs.d`, `.golangci.yml` into `$HOME` and creates `~/.emacs-saves`. Collisions are handled by `internal/symlinkfs` (no prompt). |
| `internal/dryrun` | `Reporter` interface for dry-run output. `NullReporter` for production (silent), `NewReporter(out io.Writer)` for `--dry-run` mode (writes unified diffs via go-udiff). Consumed at every side-effecting chokepoint. |
| `internal/errutil` | `RunAndSetError` cleanup-helper. Keeps cleanup-error handling out of the main flow. |
| `internal/managedblock` | `Upsert(path, payload, priorLine *regexp.Regexp) error` — installs / refreshes / migrates a sentinel-fenced managed block inside an existing text file. Idempotent (mtime preserved on no-op), atomic (tempfile + rename), fails loud on malformed marker state. Consumed only by `internal/configgen`. |
| `internal/packages` | `packages.InstallWithRetry` (the entrypoint both callers use) runs `Install` — brew upgrade + per-group formula install + per-cask install with limp-and-report semantics across all three steps — then loops an abort/retry/ignore prompt over the collected failures. Retries are scoped to just the failed items; abort surfaces the `ErrAborted` sentinel; ignore returns the failures with a nil error. The curated lists (`Formulae`, `CommonCasks`, `BetaCasks`, `PersonalCasks`, `Fonts`, `DevTools`, `AI`) live here. |
| `internal/symlinkfs` | `Install(source, target, now)` — symlink-aware idempotent installer. Existing target that's already a symlink to `source` is a no-op; anything else gets renamed to `<target>.<YYYYMMDDHHMMSS>.bkp` and replaced. Shared by `dotfiles` and `claude/sync` symlink mode. |
| `internal/userinput` | Five interactive prompts (Personal, DevRoot, GitHost, GitOrg, InstallRetryChoice). Each prompt takes a `prearg string` first arg; non-empty means "skip the prompt" (InstallRetryChoice parses its prearg into the abort/retry/ignore enum instead of returning it verbatim). This package never reads env directly. |

Most packages with a public interface also expose a test sub-package (`<pkg>test`) holding a testify-mock-backed fake. Production builds never transitively pull in testify — that's the whole point of the split. See [Conventions](#conventions).

### Claude Code config sync

`internal/claude/sync` (entrypoint: `Sync` in `sync.go`) syncs the six curated items (`settings.json`, `CLAUDE.md`, `AGENTS.md`, `skills/`, `agents/`, `commands/`) from the repo's `shared_config/.claude/` into `~/.claude/`. The in-repo path is hardcoded as `state.RepoSubdir`. Two modes:

- **Personal computer (`PERSONAL_COMPUTER=true`)**: `internal/claude/sync/symlink.go` makes `~/.claude/<item>` a symlink to the repo copy. `~/.claude/` itself is a real directory because Claude Code writes runtime state there — only the six curated items become symlinks.
- **Otherwise**: `internal/claude/sync/files` copies + 3-way merges. Base = repo content at the SHA in `<RepoDir>/.sync-state/last-sync-commit`. Local = `~/.claude/<file>`. Remote = current repo HEAD. Only true conflicts (both sides diverged differently from base) prompt the user.

In **both** modes, `Sync` first calls `internal/claude/sync/precommit.InstallPrecommitHook` to symlink `.git/hooks/pre-commit` → `../../.githooks/pre-commit`. The hook canonicalizes staged JSON files under `shared_config/.claude/` (and the transitional `.claude/` at the repo root) at commit time so the repo copy stays in stable form (sorted+deduped primitive arrays). Idempotent and refuses to clobber a pre-existing regular file or foreign symlink.

Inside the merge engine, `internal/claude/sync/settings` treats merge units where every present value is a primitive array (e.g. `permissions.allow|ask|deny`) as **sets** rather than ordered sequences. Equality is set-equality, so reorders alone never trigger conflicts; adds and removes from each side are 3-way set-merged silently with `(B ∪ L_adds ∪ R_adds) − L_removes − R_removes`. Decisions emitted from this branch carry `kind: "set"` plus `adds`/`removes`/`total` counts, which `settings.Merge` renders as a one-line stderr summary on real changes (`settings.json: <path> merged: +N -M (now K items)`). Pure-reorder noops are silent — the local file is not touched. Arrays containing objects fall back to the existing equality-based merge.

Three persistent state files live in `shared_config/.claude/.sync-state/` (gitignored):

- `last-sync-commit` — anchor SHA for the merge base. Only advanced after a fully-clean sync (no skipped conflicts); see `internal/claude/sync/advance.go` (`AdvanceLastSyncCommit`).
- `decisions.json` — remembered "always keep local" / "always take remote" choices per conflict path.
- `README.md` — explains the directory to humans who poke at it.

Non-interactive override: `CLAUDE_MERGE_RESOLUTION=keep-local|take-remote|skip` (decoded by `parseMergeResolution` in `internal/claude/sync/prompt/envoverride.go`), or the equivalent `--merge-resolution` flag on `claude sync` / `setup`. The `skip` value flips the in-process `HadSkips` flag, which gates `AdvanceLastSyncCommit` so `last-sync-commit` is held in place and unresolved conflicts re-surface on the next run.

### Env vars + flags

Every gating env var has a matching flag on the subcommand(s) that read it. Resolution chain in `internal/cmd/root.go` (`resolveBool` / `resolveString` / `resolveBoolAsString`):

> **flag (when `.Changed` is true) → env var → fallback / prompt (setup only) / zero value**

The cmd layer resolves the value once in each subcommand's `RunE`, then threads it down through a `*Params` struct (`installPackagesParams`, `claudeSyncParams`, `setupParams`). Business-logic packages (`internal/userinput`, `internal/dotfiles`, `internal/claude/sync`, `internal/claude/sync/prompt`, `internal/configgen`) **never read env or cobra directly** — they take pre-resolved values as parameters. For bool flags, `.Changed` is required so `--personal=false` can win over `PERSONAL_COMPUTER=true`. For string flags, an empty value is treated as "unset" to prevent cobra defaults from clobbering env values. Fallback closures (e.g., `brewPrefixFallback`) run only when neither flag nor env is set, so expensive operations like a `$(brew --prefix)` shellout never run unnecessarily.

| Var | Flag (on commands) | Effect |
|---|---|---|
| `PERSONAL_COMPUTER` | `--personal` (install packages, claude sync, setup) | Gates personal casks, gpg signing, default git org, and Claude sync mode. `install.bootstrap.sh` prompts first if unset and persists to `~/.zprofile`; `userinput.Personal` is the in-process fallback when no prearg is supplied. |
| `DEV_ROOT` | `--dev-root` (setup) | `melvin-config setup` prompts via `userinput.DevRoot` (default `$HOME/dev`). Threaded into `configgen.SetupZshrc` → written to `~/.zshrc`. |
| `GIT_HOST` | `--git-host` (setup) | `melvin-config setup` prompts via `userinput.GitHost` (4-option menu). Threaded into `configgen.SetupZshrc` → written to `~/.zshrc`. |
| `GIT_CLONE_USER_NAME` | `--git-org` (setup) | `melvin-config setup` prompts via `userinput.GitOrg` (returns `Nivl` when `PERSONAL_COMPUTER=true`, else free-text). Threaded into `configgen.SetupZshrc` → written to `~/.zshrc`. |
| `CLAUDE_MERGE_RESOLUTION` | `--merge-resolution` (claude sync, setup) | `keep-local` / `take-remote` / `skip` — pre-resolves merge conflicts. Resolved value flows into `prompt.NewPrompter(..., mergeResolution)` and is decoded by `parseMergeResolution`. Validated against the closed set in `validateMergeResolution` before the business logic runs. |
| `INSTALL_FAILURE_RESOLUTION` | `--install-failure-resolution` (install packages, setup) | `abort` / `retry` / `ignore` — pre-answers the failed-packages prompt shown when brew packages fail to install or upgrade. Applies to the first ask only (a pre-resolved retry that leaves failures behind re-prompts). Threaded into `packages.InstallWithRetry` via `Opts.FailureResolution`; validated by `validateInstallFailureResolution`. |
| `HOMEBREW_PREFIX` | `--homebrew-prefix` (setup) | Determines the path to `pinentry-mac` written into `~/.gnupg/gpg-agent.conf`. Lazy fallback in `brewPrefixFallback` shells `$(brew --prefix)` when neither flag nor env is set; if brew itself errors, setup surfaces a wrapped error pointing at the flag + env. |
| `MELVIN_DRY_RUN` | `--dry-run` (setup) | Preview every file write, symlink decision, and side-effecting shellout via unified diffs to stderr without performing any of them. Bootstrap's own prereq install/clone/build is unaffected. |
| `WORKTREES_ROOT` / `REPOS_ROOT` / `SDKS_ROOT` | — | shell + `wt` helpers. Written to `~/.zshrc` by `configgen.SetupZshrc`; default to `$DEV_ROOT/{worktrees,repos,sdks}`. No flag — set in `~/.zshrc` only. |
| `_MELVIN_REEXECED` | — | `install.bootstrap.sh` internal guard set by the re-exec-after-pull branch; prevents infinite loop if `git pull` runs twice. |

When you add a new env-backed config knob: define the flag in the subcommand's `newXxxCmd`, resolve it in `RunE` via `resolveBool` / `resolveString` (passing a fallback closure if a lazy default makes sense), and thread the resolved value down via the `*Params` struct. Don't read env or call `os.Setenv` in business-logic packages — the cmd layer is the only owner of that boundary. If the env var also flows through `install.bootstrap.sh`'s re-exec branch, add it to the propagation site.

## Shell config

- `base.zshrc` — Oh My Zsh setup, theme `nivl`, plugin list, `PATH` baseline, `zsh-syntax-highlighting`, and a one-line `melvin-config check-update` hook on shell startup (the 14-day stamp/prompt/exec logic lives in `internal/cmd/update.go`). Sources `config.zshrc` at the end.
- `config.zshrc` — marker-based dispatch helpers + utility functions. When adding a new "this means different things in different repo types" helper, follow the existing style (test for marker files, then dispatch):
  - **Project dispatchers**: `run`, `add`, `install`, `lint` — pick behavior from markers (`yarn.lock`, `pnpm-lock.yaml`, `go.mod`, `manage.py`, …).
  - **`cl`** — clone any repo into `$REPOS_ROOT/<host>/<org>/<repo>` regardless of where you are.
  - **Worktree family**: `wt <branch>` creates `$WORKTREES_ROOT/<repo>/<branch>` and copies a curated set of gitignored files (env files, SDK caches, …) into it; `wt_done` cleans up; `wt_exit` returns to the main checkout. Internal helpers: `_should_copy_wt_ignored_path`, `_confirm_wt_done_delete`, `_cleanup_wt_worktree_dirs`.
  - **Other utilities**: `rec`, `findport`, `erase`, `code`, `is-go-repo`, `keygen`.

## Tests & CI

CI workflow at `.github/workflows/tests.yml` runs on `pull_request` and pushes to `main`. Most jobs run on `macos-latest`; `go-unit` runs on `ubuntu-latest` inside a `ghcr.io/nivl/service-images-go-dev` container:

| Job | Command |
|---|---|
| `shell-syntax` | three separate `zsh -n` invocations against `install.bootstrap.sh`, `shared_config/base.zshrc`, `shared_config/config.zshrc` (one script per `zsh -n` so missing files / syntax errors don't slip past as ignored positional args). |
| `worktrees` | `bash tests/wt_ignored_copy_test.sh` |
| `canonicalize` | `bash tests/canonicalize_test.sh` |
| `gh-api-hook` | `bash tests/gh_api_write_guard_test.sh` |
| `deny-shell-wrapper-hook` | `bash tests/deny_shell_wrapper_test.sh` |
| `write-under-roots-hook` | `bash tests/write_under_roots_test.sh` |
| `file-ops-under-roots-hook` | `bash tests/file_ops_under_roots_test.sh` |
| `deny-json-tool-hook` | `bash tests/deny_json_tool_test.sh` |
| `deny-awk-hook` | `bash tests/deny_awk_test.sh` |
| `deny-multi-command-hook` | `bash tests/deny_multi_command_test.sh` |
| `deny-command-substitution-hook` | `bash tests/deny_command_substitution_test.sh` |
| `deny-env-vars-hook` | `bash tests/deny_env_vars_test.sh` |
| `bash-allow-trusted-hook` | `bash tests/bash_allow_trusted_test.sh` |
| `go-unit` | `go test ./internal/... ./cmd/...` + `golangci-lint run` (in service-images-go-dev container on ubuntu-latest) |
| `go-darwin` | `go test -tags darwin ./internal/brew/...` (macos-latest, Go version from `go.mod`) |
| `go-integration` | `go test -run 'FakeBinarySubprocess' ./internal/brew/...` (macos-latest, Go version from `go.mod`) |
| `go-integration-configgen` | `go test -tags integration ./internal/configgen/...` (macos-latest, Go version from `go.mod`) |

Test scripts use `set -euo pipefail`. Several test files share assertion helpers via `tests/test_helpers.sh` (sourced — no shebang, no `set` directive). Run any of them locally with `bash tests/<script>.sh`. The runtime is Go; test coverage for every subsystem lives under `internal/`.

## Conventions

### Runtime + UX

- **Idempotent**: `melvin-config setup` must handle first install and repeated updates. Every destination check and idempotency guard (e.g., `managedblock.Upsert`'s mtime-preserving no-op, `symlinkfs.Install`'s existing-symlink short-circuit) exists for a reason.
- **Interactive but scriptable**: prompts loop until valid input; every gating decision has both a flag and an env-var fallback (see *Env vars + flags*). New gates should follow the same shape: flag (`.Changed` wins) → env → prompt fallback.
- **Symlink, don't copy**: shared assets stay as the source of truth in this repo. The materialized exceptions are files that need machine-specific values (`~/.zshrc`, `~/.gitconfig`, `~/.zprofile`, `~/.gnupg/gpg-agent.conf`).
- **Layered runtime**: bootstrap shim (`install.bootstrap.sh`), main runtime (Go: `melvin-config setup`), shell startup (`base.zshrc`), day-to-day helpers (`config.zshrc`). Don't blur the layers.

### Go: I/O + config

- **I/O streams flow through `iox.Streams`**, not `os.Stdin/Stdout/Stderr`. `iox.System()` (and `cmd/melvin-config/main.go`) is the only place that touches the process-global streams. Every subsystem takes the bundle (or just the writer it needs) as a parameter.
- **`os.Getenv` lives only in `internal/cmd/root.go`** (the `resolve*` helpers). Business-logic packages take pre-resolved values via their `*Params` struct (`installPackagesParams`, `claudeSyncParams`, `setupParams`) or constructor args. `os.Setenv` is not called anywhere in production code.
- **Subcommand wiring shape**: `newXxxCmd` declares flags, `RunE` resolves them to a `*Params` struct, then calls the body function which takes `(ctx, cfg, params)`. The body is decoupled from cobra and env.

### Go: errors

- **Wrap every error return with `fmt.Errorf("<callee action>: %w", err)`**. The wrap message describes the function being called, not the function doing the wrapping — so the chain reads top-to-bottom (e.g. `setup: skip existing: yes-no prompt: read input: <cause>`). Don't re-wrap with the current function's own identity; that's redundant.
- **Don't double-wrap.** If the inner already self-describes the failed operation, propagate it unchanged.
- **`ctxErr`** branches wrap with the same prefix as the parallel non-ctx branch (e.g. both `brew upgrade: <cause>`), so `errors.Is(err, context.Canceled)` still works through the chain.

### Go: testing + packaging

- **Doc comment on every declaration.** Exported and unexported. Types, functions, methods, constants. The repo lints for this implicitly via review; keep new code consistent.
- **Test-only deps stay out of production builds.** A package's testify-mock-backed fake lives in a sibling `<pkg>test/` sub-package (`brewtest`, `appsetuptest`, `configgentest`, `userinputtest`, `statetest`, `prompttest`). Production files do not import `github.com/stretchr/testify/...`. Confirm with `go list -deps ./cmd/melvin-config | grep testify` — should be empty.
- **Comments describe current behavior, not history.** No references to retired bash code, migration step numbers, or spec citations. If a behavior is non-obvious, explain *why* it's that way; don't anchor it to historical context.

### Shell

- **`set -e` in `install.bootstrap.sh`**: the script deliberately uses `set -e` (not `-u`). If you reference an env var that may be unset, default it (`${VAR:-}`).
- **Format shell files with `shfmt`**: after modifying any shell file (`*.sh`, `*.zsh`, `*.zshrc`, `.gitmessage`-style snippets, etc.), run `shfmt -w <file>` to normalize formatting. `shfmt` honors the repo's `.editorconfig` (2-space indent, LF, trailing newline), so no extra flags are needed. `shfmt` is installed via Homebrew by the `packages` Go subsystem.
- **`.emacs.d/`**: tracked payloads. Targeted edits are fine; broad refactors there are risky and rarely worth it.
