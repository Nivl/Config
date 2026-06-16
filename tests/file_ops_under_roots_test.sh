#!/usr/bin/env bash
# Exercises shared_config/.claude/hooks/file-ops-under-roots.py: feeds it
# PreToolUse Bash payloads and asserts the permission decision.
#   rm/rmdir/unlink/cp/mv with every path under an allowed root -> allow
#   any path escapes the roots (lexical or symlink)             -> ask
#   path traverses .git, or rm targets a repo root              -> ask
#   rm under a dev root but not inside a git repo               -> ask
#   cp/mv hide the destination in -t/--target-directory/-S      -> ask
#   command uses shell expansion (cmd sub etc.)                 -> ask
#   leading command isn't a handled file op                     -> silent
#
# The env roots are overridden per hook invocation: REPOS root points at
# this checkout (a real git repo, so the repo-root, inside-a-repo, and
# .git carve-outs are exercised against real paths), WORKTREES root at a
# path that does not exist — containment is pure path math, so a missing
# root still proves the env plumbing works for cp and mv destinations,
# while rm targets and mv sources there ask (nothing under a
# nonexistent root can be inside a repo).
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOOK="$SCRIPT_DIR/shared_config/.claude/hooks/file-ops-under-roots.py"

. "$(cd "$(dirname "$0")" && pwd)/test_helpers.sh"

WT_ROOT="$HOME/nonexistent-worktrees-root"
RP_ROOT="$SCRIPT_DIR"

# Several probes are cwd-sensitive (link-to-cwd) or assume the fake
# worktrees root is absent — fail loudly up front instead of producing
# confusing assertion mismatches later.
case "$PWD" in
/tmp/* | /private/tmp/*)
  echo "precondition failed: run this suite from outside /tmp" >&2
  exit 1
  ;;
esac
if [[ -e "$WT_ROOT" ]]; then
  echo "precondition failed: $WT_ROOT exists" >&2
  exit 1
fi

# decision_with_roots <command> <worktrees_root> <repos_root> [cwd] ->
# permissionDecision string, "silent" when the hook emits nothing, or
# "error:<status>" when it exits non-zero. Without the status check a
# crashed hook (empty stdout) would satisfy every silent_* assertion.
decision_with_roots() {
  local payload out status=0
  payload="$(jq -nc --arg c "$1" --arg cwd "${4:-/tmp}" \
    '{cwd: $cwd, tool_input: {command: $c}}')"
  out="$(printf '%s' "$payload" |
    WORKTREES_ROOT="$2" REPOS_ROOT="$3" python3 "$HOOK")" || status=$?
  if ((status != 0)); then
    echo "error:$status"
  elif [[ -z "$out" ]]; then
    echo "silent"
  else
    jq -r '.hookSpecificOutput.permissionDecision' <<<"$out"
  fi
}

# decision <command> [cwd] -> shorthand using the file-level env roots.
decision() {
  decision_with_roots "$1" "$WT_ROOT" "$RP_ROOT" "${2:-/tmp}"
}

# reason <command> [cwd] -> the permissionDecisionReason string, for
# pinning message-only behavior the decision alone can't discriminate.
reason() {
  local payload
  payload="$(jq -nc --arg c "$1" --arg cwd "${2:-/tmp}" \
    '{cwd: $cwd, tool_input: {command: $c}}')"
  printf '%s' "$payload" |
    WORKTREES_ROOT="$WT_ROOT" REPOS_ROOT="$RP_ROOT" python3 "$HOOK" |
    jq -r '.hookSpecificOutput.permissionDecisionReason'
}

# ---- Allow: rm family under /tmp ----
assert_eq "allow_tmp_file" "allow" "$(decision "rm /tmp/foo.txt")"
assert_eq "allow_tmp_rf" "allow" "$(decision "rm -rf /tmp/sub/dir")"
assert_eq "allow_tmp_multi" "allow" "$(decision "rm /tmp/a /tmp/b /tmp/c")"
assert_eq "allow_private_tmp" "allow" "$(decision "rm /private/tmp/foo")"
assert_eq "allow_rmdir" "allow" "$(decision "rmdir /tmp/empty")"
assert_eq "allow_unlink" "allow" "$(decision "unlink /tmp/foo")"
assert_eq "allow_bin_rm" "allow" "$(decision "/bin/rm /tmp/foo")"
assert_eq "allow_long_flag" "allow" "$(decision "rm --recursive --force /tmp/sub")"
assert_eq "allow_double_dash" "allow" "$(decision "rm -- /tmp/-weird-file")"
# Dash-leading positionals after `--` exercise the boundary for real:
# without the in_options flip rm sees no target (ask), and without
# has_destination_option's `--` stop the -S lookalike reads as a
# hidden-suffix option (ask).
assert_eq "allow_double_dash_dashfile" "allow" "$(decision "rm -- -weird-file" "/tmp")"
assert_eq "allow_mv_dashdash_lookalike" "allow" "$(decision "mv -- -S.bak /tmp/b" "/tmp")"
assert_eq "allow_relative_cwd" "allow" "$(decision "rm foo.txt" "/tmp")"
assert_eq "allow_usr_bin_rm" "allow" "$(decision "/usr/bin/rm /tmp/foo")"
# A brace group without a `,` or `..` never expands — it's a filename.
assert_eq "allow_literal_brace" "allow" "$(decision "rm /tmp/{a}")"

# ---- rm under the env roots: must be inside a repo ----
# RP_ROOT is this checkout — a real git repo — so deletions inside it
# are git-recoverable and allow. WT_ROOT doesn't exist, so nothing
# under it can be inside a repo: rm there asks (cp, and mv
# destinations, stay path-math only and still allow — see the
# cross-roots cases below).
assert_eq "ask_rm_bare_worktrees_root" "ask" "$(decision "rm $WT_ROOT/proj/file.txt")"
assert_eq "allow_repos_root" "allow" "$(decision "rm $RP_ROOT/sub/file.txt")"
assert_eq "allow_relative_repos" "allow" "$(decision "rm foo.txt" "$RP_ROOT")"
# Deleting a plain subdirectory of a repo is fine — only the repo root
# itself (the dir holding .git) is carved out.
assert_eq "allow_repo_subdir" "allow" "$(decision "rm -rf $RP_ROOT/tests")"

# ---- Allow: cp and mv with every path under the roots ----
assert_eq "allow_cp_tmp" "allow" "$(decision "cp /tmp/a /tmp/b")"
assert_eq "allow_cp_r" "allow" "$(decision "cp -r /tmp/dir /tmp/dir2")"
assert_eq "allow_mv_tmp" "allow" "$(decision "mv /tmp/a /tmp/b")"
assert_eq "allow_bin_cp" "allow" "$(decision "/bin/cp /tmp/a /tmp/b")"
assert_eq "allow_bin_mv" "allow" "$(decision "/bin/mv /tmp/a /tmp/b")"
assert_eq "allow_cp_cross_roots" "allow" "$(decision "cp /tmp/file $RP_ROOT/file")"
assert_eq "allow_mv_cross_roots" "allow" "$(decision "mv $RP_ROOT/a $WT_ROOT/b")"
assert_eq "allow_mv_double_dash" "allow" "$(decision "mv -- /tmp/-weird /tmp/normal")"
assert_eq "allow_cp_relative" "allow" "$(decision "cp rel.txt dest.txt" "/tmp")"
# Safe GNU long options that take non-path values must not over-trigger
# the destination-option ask.
assert_eq "allow_cp_sparse" "allow" "$(decision "cp --sparse=auto /tmp/a /tmp/b")"
assert_eq "allow_cp_capital_t" "allow" "$(decision "cp -T /tmp/a /tmp/b")"
# RP_ROOT doubles as the env root in this fixture, and relocating an
# allowed root itself always asks — even though a plain repo root may
# move (pinned by allow_mv_tmp_repo_root_source below).
assert_eq "ask_mv_env_root_source" "ask" "$(decision "mv $RP_ROOT /tmp/relocated-repo")"

# ---- Ask: path escapes the roots ----
assert_eq "ask_etc_hosts" "ask" "$(decision "rm /etc/hosts")"
assert_eq "ask_dotdot" "ask" "$(decision "rm -rf /tmp/..")"
assert_eq "ask_dotdot_etc" "ask" "$(decision "rm /tmp/../etc/hosts")"
assert_eq "ask_mixed_targets" "ask" "$(decision "rm /tmp/foo /etc/bar")"
assert_eq "ask_var_dir" "ask" "$(decision "rm /var/db/something")"
assert_eq "ask_relative_outside" "ask" "$(decision "rm foo.txt" "/etc")"
# A lone '-' is a positional (a file named "-") — it must be vetted
# like any target, not skipped as an option.
assert_eq "ask_dash_operand" "ask" "$(decision "rm /tmp/x -" "/etc")"
assert_eq "allow_dash_operand_tmp" "allow" "$(decision "rm -" "/tmp")"
# Literal tildes reach the hook unexpanded (bash leaves ~ alone inside
# double quotes). Without expanduser they would join under cwd (/tmp
# here) and auto-allow while the real shell touches $HOME.
assert_eq "ask_tilde_home" "ask" "$(decision "rm ~/somefile")"
assert_eq "ask_mv_tilde_dest" "ask" "$(decision "mv /tmp/a ~/b")"
# Bash dirstack/OLDPWD tilde forms (~-, ~+, ~N) and unknown ~user are
# left unexpanded by expanduser. Joined under cwd they would look
# contained while bash sends them elsewhere (e.g. ~- -> $OLDPWD), so
# they must ask, not allow.
assert_eq "ask_tilde_oldpwd" "ask" "$(decision "rm ~-/passwd")"
assert_eq "ask_tilde_pwd" "ask" "$(decision "rm ~+/passwd")"
assert_eq "ask_tilde_dirstack" "ask" "$(decision "rm ~1/passwd")"
assert_eq "ask_tilde_unknown_user" "ask" "$(decision "rm ~nosuchuser/x")"
assert_eq "ask_mv_tilde_oldpwd_dest" "ask" "$(decision "mv /tmp/a ~-/b")"
# Deleting an allowed root wholesale always asks, in both spellings —
# and so does relocating one via mv.
assert_eq "ask_rm_tmp_root" "ask" "$(decision "rm -rf /tmp")"
assert_eq "ask_rm_private_tmp_root" "ask" "$(decision "rm -rf /private/tmp")"
assert_eq "ask_mv_tmp_root_source" "ask" "$(decision "mv /private/tmp /tmp/stash")"
# Prefix lookalikes of a root are NOT inside it — guards against a
# future commonpath -> startswith regression in under_any_root.
assert_eq "ask_tmp_prefix_sibling" "ask" "$(decision "rm /tmpfoo/file")"
assert_eq "ask_private_tmp_prefix" "ask" "$(decision "rm /private/tmpfoo/x")"
assert_eq "ask_repos_prefix_sibling" "ask" "$(decision "rm ${RP_ROOT}-sibling/file")"
assert_eq "ask_cp_source_outside" "ask" "$(decision "cp /etc/passwd /tmp/x")"
assert_eq "ask_cp_dest_outside" "ask" "$(decision "cp /tmp/x /etc/y")"
assert_eq "ask_mv_dest_outside" "ask" "$(decision "mv /tmp/x /var/y")"

# ---- Ask: cp/mv destination hidden inside an option ----
# The -t values point INSIDE the roots on purpose: an outside value
# like /etc would be caught by the containment check as a positional,
# letting these pass even with has_destination_option deleted.
assert_eq "ask_cp_t" "ask" "$(decision "cp -t /tmp/dest /tmp/a")"
assert_eq "ask_cp_target_dir" "ask" "$(decision "cp --target-directory=/tmp/dest /tmp/a")"
assert_eq "ask_cp_target_abbrev" "ask" "$(decision "cp --target=/tmp/dest /tmp/a")"
assert_eq "ask_cp_clustered_t" "ask" "$(decision "cp -rt /tmp/dest /tmp/a")"
assert_eq "ask_cp_t_attached" "ask" "$(decision "cp -t/tmp/dest /tmp/a")"
assert_eq "ask_mv_suffix_short" "ask" "$(decision "mv -S .bak /tmp/a /tmp/b")"
assert_eq "ask_mv_suffix_long" "ask" "$(decision "mv --suffix=.bak /tmp/a /tmp/b")"
assert_eq "ask_mv_suffix_attached" "ask" "$(decision "mv -S.bak /tmp/a /tmp/b")"

# ---- Ask: .git carve-out and rm of a repo root ----
assert_eq "ask_rm_repo_root" "ask" "$(decision "rm -rf $RP_ROOT")"
assert_eq "ask_rm_git_segment" "ask" "$(decision "rm $RP_ROOT/.git/index")"
assert_eq "ask_rm_git_upper" "ask" "$(decision "rm $RP_ROOT/.GIT/index")"
assert_eq "ask_cp_into_git" "ask" "$(decision "cp /tmp/hook $RP_ROOT/.git/hooks/pre-commit")"
assert_eq "ask_mv_onto_git" "ask" "$(decision "mv /tmp/cfg $RP_ROOT/.git/config")"
tmpdir="$(mktemp -d /tmp/file_ops_under_roots_test.XXXXXX)"
trap 'rm -rf "$tmpdir"' EXIT
mkdir -p "$tmpdir/repo/.git"
assert_eq "ask_rm_tmp_repo_root" "ask" "$(decision "rm -rf $tmpdir/repo")"
assert_eq "ask_rm_tmp_git_file" "ask" "$(decision "rm $tmpdir/repo/.git/config")"
# A linked worktree's .git is a FILE — under /tmp, repo_root_state's
# lstat is the only guard, so an isdir-style regression would silently
# auto-allow deleting a worktree root.
mkdir "$tmpdir/wt"
echo "gitdir: elsewhere" >"$tmpdir/wt/.git"
assert_eq "ask_rm_tmp_worktree_root" "ask" "$(decision "rm -rf $tmpdir/wt")"
# The realpath half of the carve-out: a symlink with no lexical .git
# that resolves into a real .git must still ask.
ln -s "$tmpdir/repo/.git" "$tmpdir/sneaky"
assert_eq "ask_rm_symlink_into_git" "ask" "$(decision "rm $tmpdir/sneaky/config")"
assert_eq "ask_cp_symlink_into_git" "ask" "$(decision "cp /tmp/hook $tmpdir/sneaky/hooks/pre-commit")"
# Lookalike names that aren't a .git path segment must not trigger it.
assert_eq "allow_rm_gitignore" "allow" "$(decision "rm $tmpdir/repo/.gitignore")"
assert_eq "allow_cp_github" "allow" "$(decision "cp /tmp/a $RP_ROOT/.github/workflows/ci.yml")"
assert_eq "allow_rm_bare_git_suffix" "allow" "$(decision "rm -rf /tmp/foo.git")"

# ---- rm under the dev roots is auto-allowed only INSIDE a repo ----
# Fixture: a fake dev-root layout. Repos hold a .git dir. Linked
# worktrees hold a .git file. Both count as recoverable-by-git.
DEV_RP="$tmpdir/devroots/repos"
DEV_WT="$tmpdir/devroots/worktrees"
mkdir -p "$DEV_RP/github.com/org/repo1/.git" "$DEV_RP/github.com/org/repo1/src"
mkdir -p "$DEV_WT/wt1/sub"
echo "gitdir: elsewhere" >"$DEV_WT/wt1/.git"
assert_eq "ask_rm_dev_root_itself" "ask" "$(decision_with_roots "rm -rf $DEV_RP" "$DEV_WT" "$DEV_RP")"
assert_eq "ask_rm_org_dir" "ask" "$(decision_with_roots "rm -rf $DEV_RP/github.com" "$DEV_WT" "$DEV_RP")"
assert_eq "ask_rm_loose_file" "ask" "$(decision_with_roots "rm $DEV_RP/loose.txt" "$DEV_WT" "$DEV_RP")"
assert_eq "allow_rm_inside_repo" "allow" "$(decision_with_roots "rm $DEV_RP/github.com/org/repo1/src/file.c" "$DEV_WT" "$DEV_RP")"
assert_eq "allow_rm_repo_subdir_rf" "allow" "$(decision_with_roots "rm -rf $DEV_RP/github.com/org/repo1/src" "$DEV_WT" "$DEV_RP")"
assert_eq "ask_rm_repo1_root" "ask" "$(decision_with_roots "rm -rf $DEV_RP/github.com/org/repo1" "$DEV_WT" "$DEV_RP")"
assert_eq "allow_rm_inside_worktree" "allow" "$(decision_with_roots "rm $DEV_WT/wt1/sub/x" "$DEV_WT" "$DEV_RP")"
assert_eq "ask_rm_worktrees_root" "ask" "$(decision_with_roots "rm -rf $DEV_WT" "$DEV_WT" "$DEV_RP")"
# mv SOURCES share rm's inside-a-repo guard: staging through /tmp
# ('mv <org dir> /tmp/x' then 'rm -rf /tmp/x') must not dodge the rm
# guard. A repo root itself can move (rm of the moved repo still asks),
# and destinations under the dev roots are unguarded writes.
assert_eq "ask_mv_org_dir_source" "ask" "$(decision_with_roots "mv $DEV_RP/github.com /tmp/elsewhere" "$DEV_WT" "$DEV_RP")"
assert_eq "ask_mv_loose_source" "ask" "$(decision_with_roots "mv $DEV_RP/loose.txt /tmp/x" "$DEV_WT" "$DEV_RP")"
assert_eq "allow_mv_inside_repo_source" "allow" "$(decision_with_roots "mv $DEV_RP/github.com/org/repo1/src/a /tmp/b" "$DEV_WT" "$DEV_RP")"
assert_eq "allow_mv_tmp_repo_root_source" "allow" "$(decision_with_roots "mv $DEV_RP/github.com/org/repo1 /tmp/staged" "$DEV_WT" "$DEV_RP")"
assert_eq "allow_mv_into_devroot" "allow" "$(decision_with_roots "mv /tmp/a $DEV_RP/incoming.txt" "$DEV_WT" "$DEV_RP")"
# A symlink operand is judged by where the LINK lives, not its target:
# a link in an org dir must not borrow the repo's recoverability (mv
# moves the link itself), and rm of a link to a repo only unlinks it.
ln -s "$DEV_RP/github.com/org/repo1/src" "$DEV_RP/github.com/link-into-repo"
assert_eq "ask_mv_symlink_source_org" "ask" "$(decision_with_roots "mv $DEV_RP/github.com/link-into-repo /tmp/x" "$DEV_WT" "$DEV_RP")"
# EVERY mv source is guarded, not just the first — and cp sources are
# deliberately exempt (a copy removes nothing).
assert_eq "ask_mv_multi_source_loose" "ask" "$(decision_with_roots "mv $DEV_RP/github.com/org/repo1/src/a $DEV_RP/loose.txt /tmp/dest" "$DEV_WT" "$DEV_RP")"
assert_eq "allow_mv_multi_source_inside" "allow" "$(decision_with_roots "mv $DEV_RP/github.com/org/repo1/src/a $DEV_RP/github.com/org/repo1/src/b /tmp/dest" "$DEV_WT" "$DEV_RP")"
assert_eq "allow_cp_loose_source" "allow" "$(decision_with_roots "cp $DEV_RP/loose.txt /tmp/x" "$DEV_WT" "$DEV_RP")"
# The everyday case: rm an EXISTING regular file inside a repo. Other
# dev-root fixtures use non-existent targets, so this is the only one
# that drives a real file (not a dir, not a missing path) through
# repo_root_state's S_ISDIR-false branch.
echo data >"$DEV_RP/github.com/org/repo1/src/real.c"
assert_eq "allow_rm_existing_file_inside_repo" "allow" "$(decision_with_roots "rm $DEV_RP/github.com/org/repo1/src/real.c" "$DEV_WT" "$DEV_RP")"

# ---- Blank env roots degrade to /tmp-only ----
# allowed_roots() must skip unset/blank env values. A regression that
# appends them anyway turns normpath("") into "." whose REALPATH is the
# hook process's cwd — the lexical "." root is inert (commonpath raises
# on mixed relative/absolute), so the fail-open only shows through the
# resolved-root check. The link-to-cwd case below is the discriminating
# probe: its realpath lands in the cwd, which must still ask.
assert_eq "allow_noenv_tmp" "allow" "$(decision_with_roots "rm /tmp/x" "" "")"
assert_eq "ask_noenv_repos" "ask" "$(decision_with_roots "rm $RP_ROOT/sub/file.txt" "" "")"
assert_eq "ask_noenv_cp" "ask" "$(decision_with_roots "cp /tmp/a $RP_ROOT/b" "" "")"
ln -s "$PWD" "$tmpdir/link-to-cwd"
assert_eq "ask_noenv_symlink_to_cwd" "ask" "$(decision_with_roots "rm $tmpdir/link-to-cwd/x" "" "")"

# Stat errors during the repo-root check must map to ask, not allow. A
# self-referencing symlink makes os.stat raise ELOOP deterministically.
ln -s "$tmpdir/self" "$tmpdir/self"
assert_eq "ask_rm_stat_error" "ask" "$(decision "rm $tmpdir/self/x")"
assert_contains "reason_stat_error" "could not inspect" "$(reason "rm $tmpdir/self/x")"
# The second error branch (lstat of <dir>/.git) needs a searchable-but
# -unreadable dir. Restore the mode afterwards so the EXIT trap's
# rm -rf can clean up.
mkdir "$tmpdir/noperm"
chmod 000 "$tmpdir/noperm"
assert_eq "ask_rm_noperm" "ask" "$(decision "rm -rf $tmpdir/noperm")"
assert_contains "reason_rm_noperm" "could not inspect" "$(reason "rm -rf $tmpdir/noperm")"
chmod 755 "$tmpdir/noperm"

# ---- Ask: symlink under /tmp escapes via realpath ----
ln -s /etc "$tmpdir/link-to-etc"
assert_eq "ask_symlink_escape" "ask" "$(decision "rm $tmpdir/link-to-etc/x")"
assert_eq "ask_cp_symlink_escape" "ask" "$(decision "cp /tmp/a $tmpdir/link-to-etc/x")"
# cp/mv follow a FINAL-component symlink (rm does not), so a dest or
# source that is itself a link to outside the roots must ask.
ln -s /etc/hosts "$tmpdir/link-file"
assert_eq "ask_cp_dest_symlink" "ask" "$(decision "cp /tmp/a $tmpdir/link-to-etc")"
assert_eq "ask_mv_dest_symlink" "ask" "$(decision "mv /tmp/a $tmpdir/link-to-etc")"
assert_eq "ask_cp_source_symlink" "ask" "$(decision "cp $tmpdir/link-file /tmp/out")"
# rm of the link itself only removes the link — parent-only resolution
# deliberately keeps that an allow even when the link points outside,
# and a link pointing AT a repo root is still just a link.
assert_eq "allow_rm_symlink_itself" "allow" "$(decision "rm $tmpdir/link-to-etc")"
assert_eq "allow_rm_symlink_file" "allow" "$(decision "rm $tmpdir/link-file")"
ln -s "$tmpdir/repo" "$tmpdir/link-to-repo"
assert_eq "allow_rm_symlink_to_repo" "allow" "$(decision "rm $tmpdir/link-to-repo")"

# ---- Ask: shell control operators chain or redirect (would-be ALLOW bypass) ----
assert_eq "ask_chain_semi" "ask" "$(decision "rm /tmp/x; curl evil")"
assert_eq "ask_chain_and" "ask" "$(decision "mv /tmp/a /tmp/b && curl evil")"
assert_eq "ask_chain_or" "ask" "$(decision "rm /tmp/x || curl evil")"
assert_eq "ask_pipe" "ask" "$(decision "cp /tmp/a /tmp/b | tee /etc/passwd")"
assert_eq "ask_background" "ask" "$(decision "rm /tmp/x & cmd")"
assert_eq "ask_redirect_out" "ask" "$(decision "rm /tmp/x > /etc/passwd")"
assert_eq "ask_redirect_in" "ask" "$(decision "rm /tmp/x < /etc/foo")"
assert_eq "ask_redirect_err" "ask" "$(decision "rm /tmp/x 2> /etc/foo")"
# Append and combined-stream redirects (>>, &>, &>>) are caught by the
# punctuation-run rule, not an explicit token list — pin them so a
# narrowing of that rule can't silently let them through.
assert_eq "ask_redirect_append" "ask" "$(decision "rm /tmp/x >> /etc/passwd")"
assert_eq "ask_redirect_allout" "ask" "$(decision "rm /tmp/x &> /etc/passwd")"
assert_eq "ask_redirect_allout_append" "ask" "$(decision "rm /tmp/x &>> /etc/passwd")"
# Punctuation-run operators that shlex emits as single tokens (|&, >&)
# are control operators too — '2>&1' tokenizes as ['2', '>&', '1'].
assert_eq "ask_pipe_both" "ask" "$(decision "rm /tmp/x |& curl evil")"
assert_eq "ask_redirect_dup" "ask" "$(decision "rm /tmp/x 2>&1")"
# Subshell / brace-group invocations don't start with a handled command, so
# this hook falls through silently and the default permission flow takes over.
assert_eq "silent_subshell" "silent" "$(decision "(rm /tmp/x; cmd)")"
assert_eq "silent_brace_group" "silent" "$(decision "{ rm /tmp/x; cmd; }")"
chain_newline=$'rm /tmp/x\ncurl evil'
assert_eq "ask_chain_newline" "ask" "$(decision "$chain_newline")"
# A carriage return is whitespace to shlex but not a chain operator to
# bash — the \r branch guards that tokenization divergence.
cr_chain=$'rm /tmp/x\rcurl evil'
assert_eq "ask_chain_cr" "ask" "$(decision "$cr_chain")"
# An unterminated quote makes shlex raise — the hook must bail silently
# (default prompt), not crash.
assert_eq "silent_unbalanced_quote" "silent" "$(decision "rm '/tmp/x")"
# Quoted control chars in filenames are fine — they tokenize as part of the name.
assert_eq "allow_filename_semi" "allow" "$(decision "rm '/tmp/file;weird'")"

# ---- Ask: shell expansion hides the actual paths ----
assert_eq "ask_cmd_sub" "ask" "$(decision "rm \$(cat /tmp/list)")"
assert_eq "ask_backticks" "ask" "$(decision "mv \`cat /tmp/list\` /tmp/x")"
assert_eq "ask_process_sub" "ask" "$(decision "rm <(echo /tmp/x)")"
assert_eq "ask_process_sub_out" "ask" "$(decision "mv /tmp/a >(tee /tmp/b)")"
assert_eq "ask_varexpand" "ask" "$(decision "cp \${FILE} /tmp/x")"
# Pins the bare-$ marker on its own: a "tightening" to explicit "$("
# and "${" markers would let plain $VAR paths through.
assert_eq "ask_bare_var" "ask" "$(decision "rm /tmp/\$FILE")"
assert_eq "ask_no_targets" "ask" "$(decision "rm")"
assert_eq "ask_only_flags" "ask" "$(decision "cp -r")"
assert_eq "ask_empty_arg" "ask" "$(decision "rm /tmp/x ''")"
# An empty arg and a control operator both yield 'ask', so pin the
# reason to lock onto the empty-arg path specifically.
assert_contains "reason_empty_arg" "empty path argument" "$(reason "rm /tmp/x ''")"
# Brace groups and globs expand AFTER the static check: the `..` in
# `/tmp/{a,../etc/passwd}` hides inside one path component where
# normpath can't collapse it, and `.g*` can match a .git entry. The
# commands go through variables because bash brace-expands the whole
# quoted "$(decision "...{a,b}...")" word otherwise (brace expansion is
# a textual pre-pass, and expansion results are never re-scanned).
brace_basic='rm /tmp/{a,b}'
brace_dotdot='rm /tmp/{a,../etc/passwd}'
brace_git='cp /tmp/x /tmp/repo/{.git,y}/z'
assert_eq "ask_brace_expansion" "ask" "$(decision "$brace_basic")"
assert_eq "ask_brace_dotdot" "ask" "$(decision "$brace_dotdot")"
assert_eq "ask_brace_git" "ask" "$(decision "$brace_git")"
assert_eq "ask_glob_star" "ask" "$(decision "rm -rf /tmp/build-*")"
assert_eq "ask_glob_dotgit" "ask" "$(decision "rm -rf $RP_ROOT/.g*")"
assert_eq "ask_glob_question" "ask" "$(decision "rm /tmp/file?")"
# '[' is the glob form that can spell .git without * or ? — and a
# comma-free {a..z} range exercises the '..' half of the brace check,
# which no comma case can.
assert_eq "ask_glob_bracket" "ask" "$(decision "rm /tmp/file[ab]")"
assert_eq "ask_glob_bracket_git" "ask" "$(decision "rm -rf $tmpdir/repo/.gi[t]")"
brace_range='rm -rf /tmp/x{1..3}'
assert_eq "ask_brace_range" "ask" "$(decision "$brace_range")"
# A literal first group must not stop the scan finding a later expanding
# one (exercises the find("{", j+1) loop continuation).
brace_second='rm /tmp/{a}/{b,c}'
assert_eq "ask_brace_second_group" "ask" "$(decision "$brace_second")"
# An unterminated brace never expands — it's a literal filename.
brace_open='rm /tmp/{a,b'
assert_eq "allow_brace_unterminated" "allow" "$(decision "$brace_open")"

# ---- Silent: not a handled file-op command ----
assert_eq "silent_echo" "silent" "$(decision "echo hi")"
assert_eq "silent_find_delete" "silent" "$(decision "find /tmp -delete")"
assert_eq "silent_git_rm" "silent" "$(decision "git rm /tmp/foo")"
assert_eq "silent_git_mv" "silent" "$(decision "git mv /tmp/a /tmp/b")"
assert_eq "silent_sudo_rm" "silent" "$(decision "sudo rm /tmp/foo")"
assert_eq "silent_xargs_rm" "silent" "$(decision "xargs rm")"
assert_eq "silent_install" "silent" "$(decision "install /tmp/a /tmp/b")"
assert_eq "silent_empty_cmd" "silent" "$(decision "")"
# Arbitrary binaries that merely share a file-op basename are not ours
# to vouch for — only system bin dirs count as the real commands.
assert_eq "silent_pathy_rm" "silent" "$(decision "/tmp/evil/rm /tmp/x")"
assert_eq "silent_relative_rm" "silent" "$(decision "./rm /tmp/x")"

# ---- A relative target with no payload cwd asks, never guesses ----
# Guessing from the hook process's getcwd() would usually land inside a
# repo under REPOS_ROOT and fail open.
nocwd_payload="$(jq -nc '{tool_input: {command: "rm foo.txt"}}')"
nocwd_out="$(printf '%s' "$nocwd_payload" |
  WORKTREES_ROOT="$WT_ROOT" REPOS_ROOT="$RP_ROOT" python3 "$HOOK")"
assert_eq "ask_relative_no_cwd" "ask" \
  "$(jq -r '.hookSpecificOutput.permissionDecision' <<<"$nocwd_out")"
# Absolute targets stay decidable without a cwd.
noabs_payload="$(jq -nc '{tool_input: {command: "rm /tmp/x"}}')"
noabs_out="$(printf '%s' "$noabs_payload" |
  WORKTREES_ROOT="$WT_ROOT" REPOS_ROOT="$RP_ROOT" python3 "$HOOK")"
assert_eq "allow_absolute_no_cwd" "allow" \
  "$(jq -r '.hookSpecificOutput.permissionDecision' <<<"$noabs_out")"

# ---- Malformed stdin bails silently (exit 0, no output), not a crash ----
# Non-object JSON (null, arrays) must take the same silent path as
# unparseable input instead of crashing on the .get calls.
# A well-formed dict whose tool_input is absent or not a dict must take
# the same silent path (the isinstance(tool_input, dict) guard), not
# crash on .get.
for bad_payload in 'not-json' 'null' '[]' '"str"' '{}' '{"tool_input":null}' '{"tool_input":"x"}' '{"tool_input":["x"]}'; do
  raw_status=0
  raw_out="$(printf '%s' "$bad_payload" |
    WORKTREES_ROOT="$WT_ROOT" REPOS_ROOT="$RP_ROOT" python3 "$HOOK")" || raw_status=$?
  assert_eq "bad_payload_silent($bad_payload)" "0|" "$raw_status|$raw_out"
done

# ---- sed: in-place (-i) is gated like rm; read-only sed is untouched ----
# Read-only sed writes nothing -> silent (the allow rule covers it). The
# $-anchor case is the regression guard: a `$` in the SCRIPT must not be
# read as shell expansion and is never seen because read-only sed returns
# before any check.
assert_eq "silent_sed_readonly" "silent" "$(decision "sed 's/a/b/' /tmp/f")"
assert_eq "silent_sed_n_print" "silent" "$(decision "sed -n 1,5p /tmp/f")"
sed_ro_dollar='sed s/$/x/ /tmp/f'
assert_eq "silent_sed_ro_dollar" "silent" "$(decision "$sed_ro_dollar")"

# In-place under a safe root with the recoverability guards satisfied.
assert_eq "allow_sed_tmp" "allow" "$(decision "sed -i '' 's/a/b/' /tmp/f.txt")"
assert_eq "allow_sed_tmp_multi" "allow" "$(decision "sed -i '' 's/a/b/' /tmp/a /tmp/b")"
assert_eq "allow_sed_e_flags" "allow" "$(decision "sed -i '' -e 's/a/b/' -e 's/c/d/' /tmp/f")"
assert_eq "allow_sed_attached_suffix" "allow" "$(decision "sed -i.bak 's/a/b/' /tmp/f")"
assert_eq "allow_sed_repo_file" "allow" "$(decision "sed -i '' 's/a/b/' $RP_ROOT/sub/file.txt")"
assert_eq "allow_sed_abs_bin" "allow" "$(decision "/usr/bin/sed -i '' 's/a/b/' /tmp/f")"
# A `$` end-anchor lives in the script, not a path, so it must not ask.
sed_ip_dollar="sed -i '' 's/\$/x/' /tmp/f"
assert_eq "allow_sed_dollar_anchor" "allow" "$(decision "$sed_ip_dollar")"

# In-place that must ask: outside roots, a .git path, a repo root, no file
# target, expansion in a path, a chained command, or under a dev root but
# not inside a repo (not git-recoverable).
assert_eq "ask_sed_outside" "ask" "$(decision "sed -i '' 's/a/b/' /etc/hosts")"
assert_eq "ask_sed_git_segment" "ask" "$(decision "sed -i '' 's/a/b/' $RP_ROOT/.git/config")"
assert_eq "ask_sed_repo_root" "ask" "$(decision "sed -i '' 's/a/b/' $tmpdir/repo")"
assert_eq "ask_sed_no_target" "ask" "$(decision "sed -i '' 's/a/b/'")"
sed_ip_var="sed -i '' 's/a/b/' /tmp/\${VAR}/f"
assert_eq "ask_sed_var_in_path" "ask" "$(decision "$sed_ip_var")"
sed_ip_chain="sed -i '' 's/a/b/' /tmp/f; rm -rf /tmp/x"
assert_eq "ask_sed_chain" "ask" "$(decision "$sed_ip_chain")"
assert_eq "ask_sed_loose_devroot" "ask" \
  "$(decision_with_roots "sed -i '' 's/a/b/' $DEV_RP/loose.txt" "$DEV_WT" "$DEV_RP")"

echo "file-ops-under-roots.py: all tests passed"
