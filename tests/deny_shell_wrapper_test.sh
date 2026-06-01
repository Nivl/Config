#!/usr/bin/env bash
# Exercises shared_config/.claude/hooks/deny-shell-wrapper.py: feeds it
# PreToolUse payloads and asserts both the permission decision and a
# distinguishing fragment of the reason for each rule.
#   bash/sh/zsh -c                  -> deny (R0)
#   python/perl/ruby/node/deno code -> deny (R1)
#   inline function def              -> deny (R2)
#   for/while/until loop             -> deny (R3)
#   heredoc into an interpreter      -> deny (R4)
#   multi-line ANSI-C $'...\n...'    -> deny (R5)
#   everything else                  -> silent (falls through)
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOOK="$SCRIPT_DIR/shared_config/.claude/hooks/deny-shell-wrapper.py"

. "$(cd "$(dirname "$0")" && pwd)/test_helpers.sh"

# decision <command> -> permissionDecision string, or "silent" when the hook
# emits nothing.
decision() {
  local payload out
  payload="$(jq -nc --arg c "$1" '{tool_input: {command: $c}}')"
  out="$(printf '%s' "$payload" | python3 "$HOOK")"
  if [[ -z "$out" ]]; then
    echo "silent"
  else
    jq -r '.hookSpecificOutput.permissionDecision' <<<"$out"
  fi
}

# reason <command> -> permissionDecisionReason string, or empty on silent.
reason() {
  local payload out
  payload="$(jq -nc --arg c "$1" '{tool_input: {command: $c}}')"
  out="$(printf '%s' "$payload" | python3 "$HOOK")"
  if [[ -z "$out" ]]; then
    echo ""
  else
    jq -r '.hookSpecificOutput.permissionDecisionReason' <<<"$out"
  fi
}

# ---- R0: shell -c wrappers (existing rule, preserved verbatim) ----
assert_eq "R0_bash_c" "deny" "$(decision "bash -c 'echo hi'")"
assert_eq "R0_sh_c" "deny" "$(decision "sh -c 'echo hi'")"
assert_eq "R0_zsh_c" "deny" "$(decision "zsh -c 'echo hi'")"
assert_eq "R0_zsh_lc" "deny" "$(decision "zsh -lc 'echo hi'")"
assert_eq "R0_bash_ec" "deny" "$(decision "bash -ec 'echo hi'")"
assert_eq "R0_dash_c" "deny" "$(decision "dash -c 'echo hi'")"
assert_eq "R0_ksh_c" "deny" "$(decision "ksh -c 'echo hi'")"
assert_eq "R0_mksh_c" "deny" "$(decision "mksh -c 'echo hi'")"
assert_eq "R0_ash_c" "deny" "$(decision "ash -c 'echo hi'")"
assert_eq "R0_pwsh_c" "deny" "$(decision "pwsh -c 'Get-Process'")"
assert_eq "R0_abs_bash_c" "deny" "$(decision "/bin/bash -c 'echo hi'")"
assert_eq "R0_script" "silent" "$(decision "bash deploy.sh")"
assert_eq "R0_interactive" "silent" "$(decision "bash -i")"
assert_eq "R0_lone_bash" "silent" "$(decision "bash")"
# Embedded usage is a deliberate carve-out: only the leading token is checked.
assert_eq "R0_embedded" "silent" "$(decision "xargs bash -c 'echo hi'")"
assert_contains "R0_reason" "bash -c" "$(reason "bash -c 'echo hi'")"

# ---- R1: interpreter code-string flags ----
# Python -c, including version-suffixed names, bundled clusters, packed code,
# and pypy. The packed-form `-cCODE` must still match.
assert_eq "R1_python_c" "deny" "$(decision "python -c 'print(1)'")"
assert_eq "R1_python3_c" "deny" "$(decision "python3 -c 'print(1)'")"
assert_eq "R1_python311_Bc" "deny" "$(decision "python3.11 -Bc 'print(1)'")"
assert_eq "R1_python_ic" "deny" "$(decision "python -ic 'print(1)'")"
assert_eq "R1_python_packed" "deny" "$(decision "python -cprint(1)")"
assert_eq "R1_pypy_c" "deny" "$(decision "pypy -c 'print(1)'")"
assert_eq "R1_pypy3_c" "deny" "$(decision "pypy3 -c 'print(1)'")"
assert_eq "R1_python_m" "silent" "$(decision "python3 -m pip install foo")"
assert_eq "R1_python_script" "silent" "$(decision "python3 app.py")"
# Trailing-dot variants like `python3.` are NOT real interpreters.
assert_eq "R1_python_dot" "silent" "$(decision "python3. -c 'x'")"

# Perl -e / -E, bare and bundled.
assert_eq "R1_perl_e" "deny" "$(decision "perl -e 'print 1'")"
assert_eq "R1_perl_E" "deny" "$(decision "perl -E 'say 1'")"
assert_eq "R1_perl_pe" "deny" "$(decision "perl -pe 's/x/y/'")"
assert_eq "R1_perl_nE" "deny" "$(decision "perl -nE 'say'")"
assert_eq "R1_perl_p_script" "silent" "$(decision "perl -p script.pl")"
assert_eq "R1_perl_script" "silent" "$(decision "perl app.pl")"

# Ruby -e (and bundled). The trailing-only rule lets `-ractive_support` (the
# library-load flag with packed arg) through while still catching `-e`/`-ne`.
assert_eq "R1_ruby_e" "deny" "$(decision "ruby -e 'puts 1'")"
assert_eq "R1_ruby_ne" "deny" "$(decision "ruby -ne 'puts'")"
assert_eq "R1_ruby_pe" "deny" "$(decision "ruby -pe 'puts \$_'")"
assert_eq "R1_ruby_script" "silent" "$(decision "ruby app.rb")"
assert_eq "R1_ruby_r_lib" "silent" "$(decision "ruby -ractive_support app.rb")"
assert_eq "R1_ruby_r_erb" "silent" "$(decision "ruby -rerb app.rb")"
# Perl -i.bak (in-place edit with packed extension) is not a code-string.
assert_eq "R1_perl_inplace" "silent" "$(decision "perl -i.bak -p script.pl file")"
# Packed forms (no space between flag and code) must match.
assert_eq "R1_perl_packed" "deny" "$(decision "perl -eprint")"
assert_eq "R1_ruby_packed" "deny" "$(decision "ruby -eputs")"
assert_eq "R1_node_packed" "deny" "$(decision "node -eclass")"
# Bun (Node-alternative runtime).
assert_eq "R1_bun_e" "deny" "$(decision "bun -e 'console.log(1)'")"
assert_eq "R1_bun_eval_long" "deny" "$(decision "bun --eval '1+1'")"
assert_eq "R1_bun_script" "silent" "$(decision "bun app.ts")"

# Node / nodejs: -e, -p, --eval, --print, --eval=...
assert_eq "R1_node_e" "deny" "$(decision "node -e 'console.log(1)'")"
assert_eq "R1_node_p" "deny" "$(decision "node -p '1+1'")"
assert_eq "R1_node_eval_long" "deny" "$(decision "node --eval '1+1'")"
assert_eq "R1_node_print_long" "deny" "$(decision "node --print '1+1'")"
assert_eq "R1_node_eval_eq" "deny" "$(decision "node --eval=1+1")"
assert_eq "R1_nodejs_e" "deny" "$(decision "nodejs -e 'x'")"
assert_eq "R1_node_script" "silent" "$(decision "node app.js")"

# Deno: eval is a subcommand, must dodge leading options.
assert_eq "R1_deno_eval" "deny" "$(decision "deno eval '1+1'")"
assert_eq "R1_deno_eval_with_opts" "deny" "$(decision "deno --quiet eval '1+1'")"
assert_eq "R1_deno_run" "silent" "$(decision "deno run app.ts")"
assert_eq "R1_deno_help" "silent" "$(decision "deno --help")"

# awk/gawk are excluded by design (first positional arg is always a program).
assert_eq "R1_awk" "silent" "$(decision "awk '{print \$1}' file")"
assert_eq "R1_gawk" "silent" "$(decision "gawk '{print \$1}' file")"

# Embedded interpreter -c is allowed (matches R0's leading-token carve-out).
assert_eq "R1_embedded_python_c" "silent" "$(decision "timeout 5 python3 -c 'print(1)'")"

assert_contains "R1_reason" "code strings" "$(reason "python -c 'x'")"

# ---- W: invocation-wrapper prefixes (env / sudo / VAR=val / etc.) ----
# These wrappers are peeled before R0/R1 fire on the real leading command.
# `xargs` and `timeout` are intentionally not peeled — see R0/R1 carve-outs.
assert_eq "W_env_bash_c" "deny" "$(decision "env bash -c 'echo hi'")"
assert_eq "W_env_FOO_python" "deny" "$(decision "env FOO=bar python3 -c 'print(1)'")"
assert_eq "W_env_PATH_python" "deny" "$(decision "env PYTHONIOENCODING=utf-8 python3 -c 'print(1)'")"
assert_eq "W_VAR_python" "deny" "$(decision "FOO=bar python3 -c 'print(1)'")"
assert_eq "W_two_VAR_python" "deny" "$(decision "FOO=bar BAZ=qux python3 -c 'print(1)'")"
assert_eq "W_sudo_bash_c" "deny" "$(decision "sudo bash -c 'echo hi'")"
assert_eq "W_sudo_u_bash_c" "deny" "$(decision "sudo -u melvin bash -c 'echo hi'")"
assert_eq "W_sudo_E_python" "deny" "$(decision "sudo -E python3 -c 'print(1)'")"
assert_eq "W_command_python" "deny" "$(decision "command python3 -c 'print(1)'")"
assert_eq "W_exec_bash_c" "deny" "$(decision "exec bash -c 'echo hi'")"
assert_eq "W_nice_python" "deny" "$(decision "nice python3 -c 'print(1)'")"
assert_eq "W_nohup_bash_c" "deny" "$(decision "nohup bash -c 'echo hi'")"
assert_eq "W_time_bash_c" "deny" "$(decision "time bash -c 'echo hi'")"
assert_eq "W_busybox_sh_c" "deny" "$(decision "busybox sh -c 'echo hi'")"
assert_eq "W_env_dash_dash" "deny" "$(decision "env -- bash -c 'echo hi'")"
# Wrapper followed by a benign command stays silent.
assert_eq "W_env_benign" "silent" "$(decision "env FOO=bar python3 app.py")"
assert_eq "W_sudo_benign" "silent" "$(decision "sudo cat /etc/hosts")"
# Composition wrappers stay allowed (deliberate carve-out).
assert_eq "W_xargs_bash_c" "silent" "$(decision "xargs bash -c 'echo'")"
assert_eq "W_timeout_bash_c" "silent" "$(decision "timeout 5 bash -c 'echo hi'")"

# ---- R2: inline function definitions ----
assert_eq "R2_posix" "deny" "$(decision "run() { echo hi; }; run a")"
assert_eq "R2_bash" "deny" "$(decision "function deploy { echo hi; }; deploy")"
assert_eq "R2_bash_parens" "deny" "$(decision "function deploy() { echo hi; }; deploy")"
# A quoted occurrence collapses into a single shlex token and must not fire.
assert_eq "R2_quoted" "silent" "$(decision "echo 'run() { echo hi; }'")"
assert_eq "R2_normal_call" "silent" "$(decision "echo hi")"
assert_contains "R2_reason" "shell functions inline" "$(reason "run() { echo hi; }")"

# ---- R3: for/while/until loops with a do...done body ----
assert_eq "R3_for" "deny" "$(decision "for f in *.txt; do echo \$f; done")"
assert_eq "R3_while" "deny" "$(decision "while read l; do echo \$l; done < file")"
assert_eq "R3_until" "deny" "$(decision "until false; do echo hi; done")"
assert_eq "R3_select" "deny" "$(decision "select x in a b c; do echo \$x; done")"
assert_eq "R3_for_pipeline" "deny" "$(decision "for i in 1 2 3; do echo \$i; done | wc -l")"
assert_eq "R3_quoted_for" "silent" "$(decision "echo 'for x in y; do z; done'")"
assert_eq "R3_find_done" "silent" "$(decision "find . -name done")"
# Keyword tokens present but no do/done body -> not a loop.
assert_eq "R3_keywords_no_body" "silent" "$(decision "echo for while until")"
# Ordered structural check: words appearing as separate args must not trigger
# the loop rule.
assert_eq "R3_split_keywords" "silent" "$(decision "echo for; echo do; echo done")"
assert_eq "R3_pipe_keywords" "silent" "$(decision "echo for | cat; echo do | cat; echo done | cat")"
# Heredoc body containing a loop must not trip R3 when the leading command is
# not an interpreter (R4's deliberate carve-out for cat/tee).
r3_cat_loop=$'cat <<EOF > /tmp/script.sh\nfor x in *.txt; do echo $x; done\nEOF'
assert_eq "R3_cat_heredoc_loop" "silent" "$(decision "$r3_cat_loop")"
assert_contains "R3_reason" "loops inline" "$(reason "for f in x; do echo \$f; done")"

# ---- R6: eval / source / . at command-position ----
assert_eq "R6_eval" "deny" "$(decision "eval 'echo hi'")"
assert_eq "R6_eval_unquoted" "deny" "$(decision "eval echo hi")"
assert_eq "R6_source" "deny" "$(decision "source /tmp/x.sh")"
assert_eq "R6_dot_source" "deny" "$(decision ". /tmp/x.sh")"
assert_eq "R6_eval_after_semi" "deny" "$(decision "echo start; eval 'rm tmp'")"
assert_eq "R6_eval_after_and" "deny" "$(decision "test -f x && source ~/.bashrc")"
assert_eq "R6_env_eval" "deny" "$(decision "env FOO=1 eval 'echo hi'")"
# `.` and `eval` as plain args (not at command-position) must not trigger.
assert_eq "R6_find_dot" "silent" "$(decision "find . -type d")"
assert_eq "R6_cd_dot" "silent" "$(decision "cd .")"
assert_eq "R6_grep_eval" "silent" "$(decision "grep -r eval src/")"
assert_eq "R6_printf_eval" "silent" "$(decision "printf eval")"
# Words containing the builtin name as a substring don't match.
assert_eq "R6_evalstuff" "silent" "$(decision "evaluate-script foo.sh")"
assert_contains "R6_reason" "eval" "$(reason "eval 'x'")"

# ---- B: structural bypass shapes (compound statements + groupings) ----
# All previously bypassed because CMD_SEPARATORS missed then/else/do/)/!/grouping.
assert_eq "B_if_then_eval" "deny" "$(decision "if true; then eval rm; fi")"
assert_eq "B_if_then_source" "deny" "$(decision "if true; then source /tmp/x; fi")"
assert_eq "B_if_then_bash_c" "deny" "$(decision "if true; then bash -c 'rm /'; fi")"
assert_eq "B_if_then_python_c" "deny" "$(decision "if true; then python3 -c 'x'; fi")"
assert_eq "B_case_eval" "deny" "$(decision "case x in a) eval rm;; esac")"
assert_eq "B_case_python_c" "deny" "$(decision "case x in a) python3 -c 'x';; esac")"
assert_eq "B_bang_loop" "deny" "$(decision "! for i in 1; do :; done")"
assert_eq "B_bang_bash_c" "deny" "$(decision "! bash -c 'echo'")"
assert_eq "B_subshell_bash_c" "deny" "$(decision "(bash -c 'echo')")"
assert_eq "B_subshell_python_c" "deny" "$(decision "(python3 -c 'print(1)')")"
assert_eq "B_brace_group_bash_c" "deny" "$(decision "{ bash -c 'echo'; }")"
assert_eq "B_chain_bash_c" "deny" "$(decision "echo start && bash -c 'rm /'")"
assert_eq "B_semi_python_c" "deny" "$(decision "echo start; python3 -c 'print(1)'")"
assert_eq "B_pipe_python_c" "deny" "$(decision "echo start | python3 -c 'print(1)'")"
# Wrapper-prefixed loop (no head wrapper-peel handles mid-stream wrappers, so
# this exercises the WRAPPER_PREFIXES tolerance in has_loop).
assert_eq "B_time_loop" "deny" "$(decision "time for i in 1; do :; done")"
assert_eq "B_nice_loop" "deny" "$(decision "nice for i in 1; do :; done")"

# ---- C: shell option-arg pairs preceding -c ----
# `bash -o pipefail -c CMD` previously bypassed because uses_dash_c bailed on
# the first non-flag token (`pipefail`).
assert_eq "C_bash_o_pipefail_c" "deny" "$(decision "bash -o pipefail -c 'echo hi'")"
assert_eq "C_bash_eO_extglob_c" "deny" "$(decision "bash -O extglob -c 'echo hi'")"
assert_eq "C_zsh_o_y_c" "deny" "$(decision "zsh -o errexit -c 'echo hi'")"

# ---- D: wrapper-flag-set hygiene (no-arg flags no longer over-strip) ----
# `sudo -S` and `-n` are no-arg flags; over-strip would have consumed the
# real interpreter as their "arg". `time -p` and `command -p` similarly.
assert_eq "D_sudo_S_python_c" "deny" "$(decision "sudo -S python3 -c 'print(1)'")"
assert_eq "D_sudo_n_python_c" "deny" "$(decision "sudo -n python3 -c 'print(1)'")"
assert_eq "D_time_p_bash_c" "deny" "$(decision "time -p bash -c 'echo hi'")"
assert_eq "D_command_p_bash_c" "deny" "$(decision "command -p bash -c 'echo hi'")"
# Long-flag arg form (`--user melvin`) should still peel correctly.
assert_eq "D_sudo_user_bash_c" "deny" "$(decision "sudo --user melvin bash -c 'echo hi'")"
assert_eq "D_sudo_user_eq_bash_c" "deny" "$(decision "sudo --user=melvin bash -c 'echo hi'")"
# Real arg-flag (nice -n 10) still peels.
assert_eq "D_nice_n_bash_c" "deny" "$(decision "nice -n 10 bash -c 'echo hi'")"

# ---- E: here-strings + heredoc pipelines + space-separated arg-swallow ----
# `<<<` here-strings feed an interpreter the same way `-c` does.
assert_eq "E_python_herestring" "deny" "$(decision "python3 <<<'print(1)'")"
assert_eq "E_bash_herestring" "deny" "$(decision "bash <<<'echo hi'")"
assert_eq "E_node_herestring" "deny" "$(decision "node <<<'console.log(1)'")"
# Heredoc piped into an interpreter — same opacity as `python3 <<EOF`.
r4_pipe_python=$'cat <<EOF | python3\nprint(1)\nEOF'
r4_pipe_bash=$'cat <<EOF | bash\necho hi\nEOF'
r4_pipe_eval=$'cat <<EOF | eval\nrm -rf /\nEOF'
r4_pipe_tee_python=$'cat <<EOF | tee /tmp/x | python3\nprint(1)\nEOF'
assert_eq "E_heredoc_pipe_python" "deny" "$(decision "$r4_pipe_python")"
assert_eq "E_heredoc_pipe_bash" "deny" "$(decision "$r4_pipe_bash")"
assert_eq "E_heredoc_pipe_eval" "deny" "$(decision "$r4_pipe_eval")"
assert_eq "E_heredoc_pipe_tee_python" "deny" "$(decision "$r4_pipe_tee_python")"
# Space-separated arg-swallowing form (the cluster's swallow char ends the
# cluster, and the next positional is the arg).
assert_eq "E_node_r_space_e" "deny" "$(decision "node -r mod -e 'console.log(1)'")"
assert_eq "E_python_W_space_c" "deny" "$(decision "python3 -W default -c 'print(1)'")"
assert_eq "E_ruby_r_space_e" "deny" "$(decision "ruby -r active_support -e 'puts 1'")"
# Same flag without a code-string flag must still be silent.
assert_eq "E_node_r_silent" "silent" "$(decision "node -r mod app.js")"
assert_eq "E_ruby_r_silent" "silent" "$(decision "ruby -r active_support app.rb")"

# ---- F: unclosed heredoc must not swallow the tail (function defs after) ----
# The body-stripper used to drop the rest of the stream when no closing
# delimiter was found, masking any inline-scripting that followed. has_loop
# and has_function_def scan the full stream and don't require command-
# position, so once we stop the swallow they catch what they should. (R6
# eval/source still needs command-position which a mid-stream `eval` in an
# unparseable command doesn't satisfy — fixing that requires newline-as-
# separator handling and is out of scope.)
r4_unclosed_func=$'cat <<EOF\nbody\nrun() { :; }'
r4_unclosed_loop=$'cat <<EOF\nbody\n; for i in 1; do :; done'
assert_eq "F_unclosed_func" "deny" "$(decision "$r4_unclosed_func")"
assert_eq "F_unclosed_loop" "deny" "$(decision "$r4_unclosed_loop")"

# ---- R4: heredocs feeding interpreters ----
r4_python=$'python3 <<EOF\nprint(1)\nEOF'
r4_bash=$'bash <<\'EOF\'\necho hi\nEOF'
r4_dash_eof=$'bash <<-EOF\n\techo hi\nEOF'
r4_perl=$'perl <<EOF\nprint 1;\nEOF'
r4_ruby=$'ruby <<EOF\nputs 1\nEOF'
r4_node=$'node <<EOF\nconsole.log(1)\nEOF'
r4_deno=$'deno <<EOF\nconsole.log(1)\nEOF'
r4_cat=$'cat <<EOF > file\nhello\nEOF'
r4_tee=$'tee file <<EOF\nhello\nEOF'
assert_eq "R4_python" "deny" "$(decision "$r4_python")"
assert_eq "R4_bash" "deny" "$(decision "$r4_bash")"
assert_eq "R4_dash_eof" "deny" "$(decision "$r4_dash_eof")"
assert_eq "R4_perl" "deny" "$(decision "$r4_perl")"
assert_eq "R4_ruby" "deny" "$(decision "$r4_ruby")"
assert_eq "R4_node" "deny" "$(decision "$r4_node")"
assert_eq "R4_deno" "deny" "$(decision "$r4_deno")"
# Heredocs feeding non-interpreters are accepted (out of scope by design).
assert_eq "R4_cat" "silent" "$(decision "$r4_cat")"
assert_eq "R4_tee" "silent" "$(decision "$r4_tee")"
assert_contains "R4_reason" "heredoc" "$(reason "$r4_python")"

# Bypasses the original `^\s*`-anchored regex missed.
r4_chained=$'true && python3 <<EOF\nprint(1)\nEOF'
r4_semi_chained=$'echo start; python3 <<EOF\nprint(1)\nEOF'
r4_abs_path=$'/usr/bin/python3 <<EOF\nprint(1)\nEOF'
r4_env_wrapped=$'env FOO=bar python3 <<EOF\nprint(1)\nEOF'
r4_sudo_wrapped=$'sudo python3 <<EOF\nprint(1)\nEOF'
r4_with_redirect=$'python3 2>/dev/null <<EOF\nprint(1)\nEOF'
r4_hyphen_delim=$'python3 <<\'END-MARK\'\nprint(1)\nEND-MARK'
r4_dash_eof_tabbed=$'python3 <<-EOF\n\tprint(1)\nEOF'
assert_eq "R4_chained" "deny" "$(decision "$r4_chained")"
assert_eq "R4_semi_chained" "deny" "$(decision "$r4_semi_chained")"
assert_eq "R4_abs_path" "deny" "$(decision "$r4_abs_path")"
assert_eq "R4_env_wrapped" "deny" "$(decision "$r4_env_wrapped")"
assert_eq "R4_sudo_wrapped" "deny" "$(decision "$r4_sudo_wrapped")"
assert_eq "R4_with_redirect" "deny" "$(decision "$r4_with_redirect")"
assert_eq "R4_hyphen_delim" "deny" "$(decision "$r4_hyphen_delim")"
assert_eq "R4_dash_eof_tabbed" "deny" "$(decision "$r4_dash_eof_tabbed")"

# ---- R5: multi-line ANSI-C $'...' strings ----
r5_literal_nl=$'printf %s $\'foo\nbar\''
r5_escaped_nl=$'echo $\'one\\ntwo\''
r5_single_line=$'echo $\'tab\\there\''
r5_plain=$'echo \'one line\''
assert_eq "R5_literal_nl" "deny" "$(decision "$r5_literal_nl")"
assert_eq "R5_escaped_nl" "deny" "$(decision "$r5_escaped_nl")"
assert_eq "R5_single_line" "silent" "$(decision "$r5_single_line")"
assert_eq "R5_plain" "silent" "$(decision "$r5_plain")"
assert_contains "R5_reason" "ANSI-C-quoted" "$(reason "$r5_literal_nl")"

# ---- Edge cases ----
assert_eq "edge_empty_cmd" "silent" "$(decision '')"
# Unparseable shlex (unterminated quote): R0-R3 token checks are skipped, but
# R4/R5 raw-text checks still run. Neither matches this input, so silent.
assert_eq "edge_unparseable" "silent" "$(decision 'echo "unterminated')"
# Same unparseable input WITH a heredoc-into-interpreter shape: R4 still fires
# even though tokenization failed.
edge_unparseable_heredoc=$'python3 <<EOF\nprint("unterminated)\nEOF'
assert_eq "edge_unparseable_heredoc" "deny" "$(decision "$edge_unparseable_heredoc")"

echo "deny-shell-wrapper.py: all tests passed"
