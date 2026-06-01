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

Invocation wrappers (`env`, `sudo`, `command`, `exec`, `nice`, `nohup`, `time`, `FOO=bar` assignments, etc.) are peeled before the check — they do not provide an escape. Composition wrappers `xargs` and `timeout` are deliberately left alone.

If you need to repeat logic, run the command N times directly. The Bash tool's working directory persists across calls, so each call can share state with the previous one. If you genuinely need a multi-step script, write a real script file via the Write tool first and then execute it as one command.

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
