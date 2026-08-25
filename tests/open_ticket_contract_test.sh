#!/usr/bin/env bash
# Asserts the load-bearing rules are actually stated in the open-ticket skill
# files. Every rule checked here is one where a silent omission produces a
# wrong Jira write or a duplicate ticket, so a later edit that drops one must
# fail CI rather than pass quietly.
#
# Prose in these files is hand-wrapped, so a needle holding a literal space can
# straddle a newline. A plain grep for it would return zero and the assertion
# would pass without ever being able to fail. flatten() collapses the file to
# one space-normalized line first, which removes the whole class of problem.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SKILL_DIR="$SCRIPT_DIR/shared_config/.claude/skills/open-ticket"

SKILL_MD="$SKILL_DIR/SKILL.md"
TEMPLATES_MD="$SKILL_DIR/TEMPLATES.md"
CREATE_FIELDS_MD="$SKILL_DIR/CREATE-FIELDS.md"
DEDUP_MD="$SKILL_DIR/DEDUP-QUERIES.md"

# Lives under work-on and is shared. open-ticket's Step 4 routes the Stats block's
# impact count through it, so this suite asserts the half of that file open-ticket
# depends on. work-on owns everything else in there.
DB_QUERIES_MD="$SCRIPT_DIR/shared_config/.claude/skills/work-on/DB-QUERIES.md"

. "$(cd "$(dirname "$0")" && pwd)/test_helpers.sh"

flatten() { tr '\n' ' ' < "$1" | tr -s ' '; }

require_file() {
  if [[ ! -s "$1" ]]; then
    echo "missing or empty: $1" >&2
    exit 1
  fi
}

# assert_contains does a substring match, and a heading needle absorbs into a
# deeper heading of the same text: "# X" matches inside "## X". A heading
# check needs a fully anchored line pattern against the raw file instead, so
# a demoted heading actually fails the count.
assert_line_count() {
  local label="$1" pattern="$2" expected="$3" file="$4"
  local actual
  actual="$(grep -c "$pattern" "$file" || true)"
  assert_eq "$label" "$expected" "$actual"
}

# ---- TEMPLATES.md: all five templates present ----
require_file "$TEMPLATES_MD"

TEMPLATES_FLAT="$(flatten "$TEMPLATES_MD")"

# The house convention: every reference file opens with an H1 title, then the
# back-pointer. A missing title should report as a missing title, not get
# mistaken for a missing template further down.
assert_line_count "tpl_has_title" '^# Ticket templates$' 1 "$TEMPLATES_MD"

assert_line_count "tpl_has_feature_story" '^## Feature Story$' 1 "$TEMPLATES_MD"
assert_line_count "tpl_has_bug" '^## Bug Fix and Security$' 1 "$TEMPLATES_MD"
assert_line_count "tpl_has_tech_debt" '^## Technical Debt$' 1 "$TEMPLATES_MD"
assert_line_count "tpl_has_epic" '^## Epic$' 1 "$TEMPLATES_MD"
assert_line_count "tpl_has_bug_subtask" '^## Bug Subtask$' 1 "$TEMPLATES_MD"

# ELI5 is the section a reader with no project context uses, and it is the
# first one a drafter drops. Feature Story, Bug Fix and Security, Technical
# Debt, and Epic all carry it. Bug Subtask does not carry it, because that
# piece of work runs hours to a day, so a full template on it is noise, and
# ELI5 is part of that noise.
assert_line_count "tpl_eli5_section_count" '^## ELI5$' 4 "$TEMPLATES_MD"

# The bug template's Stats block is what makes a bug ticket triageable. The
# 1-to-5 likelihood rating is the part most likely to be quietly dropped.
assert_line_count "tpl_bug_has_stats" '^## Stats$' 1 "$TEMPLATES_MD"
assert_contains "tpl_bug_has_rating" "sets the ends of the scale at 1 and 5" "$TEMPLATES_FLAT"

# Each question is pinned on its own, the same way the rubric rungs below are. A
# partial block is the failure mode here rather than a missing one, and a reader
# cannot tell a question nobody answered from one nobody asked.
assert_contains "tpl_stats_impact_count" "- **How many people are impacted**: [number]" "$TEMPLATES_FLAT"
assert_contains "tpl_stats_locked_state" "- **Are impacted users in a locked state**: [yes/no]" "$TEMPLATES_FLAT"
assert_contains "tpl_stats_backfill" "- **Is a backfill required to unblock users**: [yes/no]" "$TEMPLATES_FLAT"
assert_contains "tpl_stats_legacy_route" "- **Is this in a legacy route**: [yes/no]" "$TEMPLATES_FLAT"
assert_contains "tpl_stats_live_codepath" "- **Is this in a live codepath**: [yes/no]" "$TEMPLATES_FLAT"

# One bullet per question, answered in a number or a yes or a no. Lose these two
# sentences and the block grows back into the paragraph the request asked it as,
# which is the shape a stakeholder has to hunt through for each answer.
assert_contains "tpl_stats_answers_short" "Every Stats answer is short." "$TEMPLATES_FLAT"
assert_contains "tpl_stats_no_paragraph" "Do not put that paragraph back." "$TEMPLATES_FLAT"

# The request's own wording defines 1 and 5 and nothing between them. Without the
# four boundaries below, two runs rate the same defect differently and the board
# cannot sort on the number at all. Each boundary is pinned on its own, because a
# partial rubric is the failure mode here rather than a missing one.
assert_line_count "tpl_has_odds_rubric" '^### The odds rating$' 1 "$TEMPLATES_MD"
assert_contains "tpl_odds_1" "**1.** Needs a combination of conditions no known user hits." "$TEMPLATES_FLAT"
assert_contains "tpl_odds_2" "**2.** Needs a non-default config or an unusual input." "$TEMPLATES_FLAT"
assert_contains "tpl_odds_3" "**3.** On a normal path, but behind a condition only some users meet." "$TEMPLATES_FLAT"
assert_contains "tpl_odds_4" "**4.** Fires on the default path for anyone who takes it." "$TEMPLATES_FLAT"
assert_contains "tpl_odds_5" "**5.** Measured, and actively firing in production now." "$TEMPLATES_FLAT"

# Preconditions and not population share. Lose this sentence and the next reader
# reads the scale as a percentage of users, which no code read can answer, so
# every unmeasured defect starts arriving with no rating at all.
assert_contains "tpl_odds_is_preconditions" "Rungs 1 through 4 measure preconditions" "$TEMPLATES_FLAT"

# Rung 5 is measurement-defined while 1 through 4 are not, so a code read tops out
# at 4. Saying the whole scale measures preconditions contradicts its own top rung
# and tells a reader a guard read can reach 5, which is the rating inflation the
# evidence requirement exists to stop.
assert_contains "tpl_odds_five_needs_measurement" "a read of the guards tops out at 4" "$TEMPLATES_FLAT"

# Amplitude measures the one axis the scale does not use. Without this the probe's
# number looks like it belongs in the rating, and a big reach count silently
# inflates a defect that fires only behind a rare precondition.
assert_contains "tpl_odds_excludes_population" "Population share never enters the scale." "$TEMPLATES_FLAT"

# The rubric starts at 1, so a defect that cannot fire has no rung. Filed as a 1 it
# reads as work worth doing, which is the opposite of what a dead path deserves.
assert_contains "tpl_odds_dead_path_has_no_rung" "A defect that cannot fire at HEAD has no rung here." "$TEMPLATES_FLAT"

# The template has to admit the fallback SKILL.md mandates, or a drafter with no
# derived number has only a digit to write and writes one.
assert_contains "tpl_odds_todo_rendering" "the bullet carries a \`TODO(user):\` where the digit would go" "$TEMPLATES_FLAT"

# The templates admit the TODO(user): shape, so they are where a drafter decides to use
# one. Deciding it here skips the ask, and the line then asserts a human was asked and
# declined when nobody was asked. The verdict belongs to Step 8 and the rendering
# belongs here.
assert_contains "tpl_todo_not_decided_in_template" "Never resolve a number to that line from inside a template." "$TEMPLATES_FLAT"

# The impact count is the one Stats bullet with no telemetry probe behind it on most
# runs, so it is the one that decays into an unrun TODO(user):. The template says what
# it is owed first.
assert_contains "tpl_impact_owed_a_query_and_an_ask" "owed a warehouse query and an ask before that line is its answer" "$TEMPLATES_FLAT"

# The rendered bullet in the template, and the ban on a digit with nothing behind
# it. A rating a triager cannot check is the thing this whole block exists to
# stop, and the bare digit is what it decays into.
assert_contains "tpl_odds_rendered_line" "**Odds of triggering**: [1-5]/5" "$TEMPLATES_FLAT"
assert_contains "tpl_odds_no_bare_digit" "A bare digit is not a rating." "$TEMPLATES_FLAT"

# Never a table. A ticket arrives mangled more often from a table than from
# anything else, and a triage assessment is the natural table.
assert_contains "tpl_bans_tables" "Never a table" "$TEMPLATES_FLAT"

# The back-pointer convention every reference file in this repo follows.
assert_contains "tpl_back_pointer" "Step 8 of [SKILL.md](SKILL.md)" "$TEMPLATES_FLAT"

# ---- CREATE-FIELDS.md: the ids and shapes a create call cannot get wrong ----
require_file "$CREATE_FIELDS_MD"

CREATE_FIELDS_FLAT="$(flatten "$CREATE_FIELDS_MD")"

# The house convention again. A missing title here should report as a
# missing title, not get mistaken for a missing field id further down.
assert_line_count "cf_has_title" '^# Create fields$' 1 "$CREATE_FIELDS_MD"

assert_contains "cf_back_pointer" "Step 10 of [SKILL.md](SKILL.md)" "$CREATE_FIELDS_FLAT"

# Exact field ids. A paraphrase here becomes a failed create.
assert_contains "cf_story_points_id" "customfield_10028" "$CREATE_FIELDS_FLAT"
assert_contains "cf_sprint_id" "customfield_10021" "$CREATE_FIELDS_FLAT"
assert_contains "cf_eng_weeks_id" "customfield_10503" "$CREATE_FIELDS_FLAT"

# The two never-write ids, each named with its ban. Writing either one produces
# a ticket that looks right and carries the wrong linkage or the wrong estimate.
assert_contains "cf_bans_epic_link" "customfield_10014" "$CREATE_FIELDS_FLAT"
assert_contains "cf_bans_point_estimate" "customfield_10016" "$CREATE_FIELDS_FLAT"
assert_contains "cf_bans_components" "Never send components or fixVersions" "$CREATE_FIELDS_FLAT"

# The three two-call types, collapsed into one assertion. A bare "Bug" or
# "Epic" needle matches any file that mentions a bug or an epic in passing,
# so on its own it could never fail.
assert_contains "cf_two_call_types" "Bug, Epic and Bug Subtask" "$CREATE_FIELDS_FLAT"
assert_contains "cf_names_two_calls" "two calls" "$CREATE_FIELDS_FLAT"

# The second call is an editJiraIssue, which has no additional_fields parameter
# at all. Carrying the create call's shape over to the edit fails it after the
# create has landed, leaving an issue with no description and no points. The
# pattern anchors on a line that opens with `fields:` and mentions
# additional_fields later, which is only ever an edit recipe. A create line
# carries additional_fields alone and does not match. This catches the one-line
# recipe shape, not a multi-line rewrite of the same mistake.
assert_line_count "cf_edit_takes_no_additional_fields" '^ *fields: .*additional_fields' 0 "$CREATE_FIELDS_MD"
assert_contains "cf_edit_param_list" 'There is no `additional_fields` on it, and no `assignee_account_id` either.' "$CREATE_FIELDS_FLAT"

# markdown works on create. Stating contentFormat explicitly costs nothing and
# the shared enum text hedges about defaults.
assert_contains "cf_content_format" "contentFormat" "$CREATE_FIELDS_FLAT"

# The parameter table has 11 rows because the schema has 11 parameters. A
# table missing one of these two lets a reader miss a real parameter.
# responseContentFormat appears once in the whole file, in its own row, so
# the flattened needle is already pinned to that row. "transition" is a
# common word and the prose under the table uses it again, so a bare needle
# stays green after the row is deleted. The anchored pattern matches the row
# and nothing else.
assert_contains "cf_lists_response_format" "responseContentFormat" "$CREATE_FIELDS_FLAT"
assert_line_count "cf_lists_transition" '^| `transition` | ' 1 "$CREATE_FIELDS_MD"

assert_contains "cf_markdown_on_create" "Markdown works on create" "$CREATE_FIELDS_FLAT"

# Parent for every level, including Story to Epic. GRO is company-managed and
# still uses the unified parent field.
assert_contains "cf_parent_every_level" "parent for every level" "$CREATE_FIELDS_FLAT"

# The subtask type this skill emits, spelled exactly.
assert_contains "cf_subtask_one_word" "Bug Subtask" "$CREATE_FIELDS_FLAT"

# The description cap. A longer body fails the call.
assert_contains "cf_description_cap" "32000" "$CREATE_FIELDS_FLAT"

# WMP does not exist on this site. An example using it teaches a wrong key.
WMP_HITS="$(grep -c 'WMP' "$CREATE_FIELDS_MD" || true)"
assert_eq "cf_no_wmp_examples" "0" "$WMP_HITS"

# ---- DEDUP-QUERIES.md: the six queries and the facts that back them ----
require_file "$DEDUP_MD"

DEDUP_FLAT="$(flatten "$DEDUP_MD")"

# The house convention again. A missing title here should report as a
# missing title, not get mistaken for a missing query further down.
assert_line_count "dq_has_title" '^# Duplicate queries$' 1 "$DEDUP_MD"

assert_contains "dq_back_pointer" "Step 5 of [SKILL.md](SKILL.md)" "$DEDUP_FLAT"

# Q0 is the positive control and the single most important rule in this
# file. An unknown JQL field name returns totalCount 0 with no error, so a
# typo here makes the sweep report no duplicates and the skill files the
# duplicate it exists to prevent.
assert_contains "dq_q0_mandatory" "positive control" "$DEDUP_FLAT"
assert_contains "dq_silent_zero" "totalCount" "$DEDUP_FLAT"

# Step 5's queries take a set of placeholders and nothing else in the
# pipeline says what goes in them. The heading alone does not pin the table
# under it, so each of the five rows below gets its own anchored pattern.
# Delete one row and only its own assertion drops to zero, while the heading
# and the other four rows stay green. Without these, a dropped row leaves a
# query running against undefined input, which reads exactly like a clean
# sweep.
assert_line_count "dq_has_placeholder_legend" '^## The placeholders$' 1 "$DEDUP_MD"
assert_line_count "dq_legend_projects" "^| \`<PROJECTS>\` | A comma list of project keys\. Step 2's majority project plus the runner-up it reports\. |\$" 1 "$DEDUP_MD"
assert_line_count "dq_legend_path_n" '^| `<PATH_N>` | A repo-relative path from Step 4\. Never with `:LINE` appended\. |$' 1 "$DEDUP_MD"
assert_line_count "dq_legend_symbol_1" '^| `<SYMBOL_1>` | A distinctive symbol from Step 4, a function or type name\. |$' 1 "$DEDUP_MD"
assert_line_count "dq_legend_noun_n" '^| `<NOUN_N>` | A distinctive domain noun from the request\. |$' 1 "$DEDUP_MD"
assert_line_count "dq_legend_key_1" "^| \`<KEY_1>\` | An epic, parent or sibling key already known from the request or from an earlier query's hits\. |\$" 1 "$DEDUP_MD"
assert_contains "dq_projects_is_a_list" "comma list and not a single key" "$DEDUP_FLAT"

# All six queries, each pinned to its own label line. A flattened "Q0" or
# "Q1" needle matches the back-pointer at the top of the file and the
# cross-references inside the other glosses, so deleting a whole query left
# those assertions green. The label lines are written "**Q<n>, <gloss>.**",
# and anchoring to that prefix is what makes the assertion fail when the
# query it labels is gone.
for q in Q0 Q1 Q2 Q3 Q4 Q5; do
  assert_line_count "dq_has_$q" "^\*\*$q, " 1 "$DEDUP_MD"
done

# The six JQL strings are the content this file exists to carry, and
# nothing above pins any of them. `project in (<PROJECTS>) AND ` is a
# start anchor, and the operator assertions for Q0, Q1, Q2 and Q4 below
# land on that same spot, not a second one. Q1, Q2 and Q4 also carry an
# end anchor, the `ORDER BY updated DESC$` that dq_jql_order_by asserts.
# Q3 carries those same start and end anchors, but its own operator,
# `text ~`, has no separate assertion, so swapping it for `summary ~`
# leaves every assertion here green. Q0 has no end anchor at all, so
# appending text to its tail leaves all seven JQL assertions here green.
# Only Q5, in dq_jql_q5_unscoped below, is pinned whole. Text inserted in
# the middle of Q1, Q2, Q3 or Q4, after the start anchor and before the
# end anchor, still leaves every assertion here green.
#
# Q0 through Q4 are scoped to a project list. Q5 deliberately is not, which
# is why this count is five and not six.
assert_line_count "dq_jql_project_scope" '^project in (<PROJECTS>) AND ' 5 "$DEDUP_MD"

# One assertion per distinct operator. summary ~ carries both Q0 and Q2, so
# its count is two.
assert_line_count "dq_jql_summary_operator" '^project in (<PROJECTS>) AND summary ~ ' 2 "$DEDUP_MD"
assert_line_count "dq_jql_text_operator" '^project in (<PROJECTS>) AND text ~ ' 1 "$DEDUP_MD"
assert_line_count "dq_jql_comment_operator" '^project in (<PROJECTS>) AND comment ~ ' 1 "$DEDUP_MD"

# Q3's already-done pass. Done is the majority of the corpus, so dropping
# this clause loses the closed duplicate the query exists to un-bury.
assert_line_count "dq_jql_done_pass" 'AND statusCategory = Done AND ' 1 "$DEDUP_MD"

# Five of the six queries rank by recency. The prose mention in Filters ends
# in a backtick and a period, so the end anchor counts query lines alone.
assert_line_count "dq_jql_order_by" ' ORDER BY updated DESC$' 5 "$DEDUP_MD"

# Q5 whole. It is the one query that is neither project-scoped nor quoted,
# and both of those are deliberate, so both belong in the pattern.
assert_line_count "dq_jql_q5_unscoped" '^text ~ "<NOUN_1> <NOUN_2>" ORDER BY updated DESC$' 1 "$DEDUP_MD"

# Bare ~ is an order-independent AND over stemmed tokens. A single absent
# term zeroes the whole query.
assert_contains "dq_escaped_quotes" "escaped inner quotes" "$DEDUP_FLAT"

# statusCategory = Done was 65% of matches for the tested term. A 180-day
# window cut 29 hits to 8.
assert_contains "dq_no_date_filter" "no date filter" "$DEDUP_FLAT"

# A bare :LINE needle also matches the legend row at line 30, so deleting
# this rule alone left that assertion green. Pin the rule's own wording,
# which appears nowhere else in the file.
assert_contains "dq_no_line_numbers" "Never append \`:LINE\` to a path" "$DEDUP_FLAT"

# -S only fires when the occurrence count changes. On the same string -S
# found 4 commits and -G found 10.
assert_contains "dq_git_uses_dash_g" "log -G" "$DEDUP_FLAT"
assert_contains "dq_git_needs_all" "--all" "$DEDUP_FLAT"

# Three unguarded queries returned close to a million characters each,
# because description sits in a mandatory field floor no query can exclude.
assert_contains "dq_count_mode" "searchResultMode" "$DEDUP_FLAT"

# A field named "sprint" is silently dropped with no error. This is a
# second silent-failure mode, distinct from the silent JQL zero Q0 catches.
assert_contains "dq_sprint_name_dropped" "customfield_10021" "$DEDUP_FLAT"

DEDUP_WMP="$(grep -c 'WMP' "$DEDUP_MD" || true)"
assert_eq "dq_no_wmp_examples" "0" "$DEDUP_WMP"

# ---- SKILL.md: the pipeline, the one gate and the five abort codes ----
require_file "$SKILL_MD"

SKILL_FLAT="$(flatten "$SKILL_MD")"

# Frontmatter. The name must match the directory, or the skill does not load.
assert_contains "sk_frontmatter_name" "name: open-ticket" "$SKILL_FLAT"
assert_contains "sk_frontmatter_desc" "description:" "$SKILL_FLAT"

# The description says what NOT to use this skill for, following the house
# convention. A bare "work-on" needle matches any mention of the sibling, so
# the needle is the phrase that carries the rule.
assert_contains "sk_says_not_work_on" "that is work-on" "$SKILL_FLAT"

# All five abort codes, as literal tokens. The token is the wire contract, and
# a paraphrased reason is not an abort a caller can recognize.
assert_contains "sk_abort_no_tooling" "JIRA_UNAVAILABLE_NO_TOOLING" "$SKILL_FLAT"
assert_contains "sk_abort_write_denied" "JIRA_WRITE_DENIED" "$SKILL_FLAT"
assert_contains "sk_abort_duplicate" "DUPLICATE_FOUND" "$SKILL_FLAT"
assert_contains "sk_abort_no_human" "GATE_UNREACHABLE_NO_HUMAN" "$SKILL_FLAT"

# The delegated abort. A delegated call files one issue of the type its caller stated, so a sizing
# over 5 is reported and never split. Drop this rule and an oversized delegated scope becomes a tree
# of issues no human approved, which is the one outcome no Jira create can be rolled back out of.
assert_contains "sk_abort_delegated_too_large" "DELEGATED_TOO_LARGE" "$SKILL_FLAT"

# acli is not a fallback. Even authenticated it cannot set sprint, story
# points, or any custom field, so a tree is not expressible through it.
assert_contains "sk_acli_not_a_fallback" "acli cannot substitute" "$SKILL_FLAT"

# The sprint filter. The array is a history in no meaningful order, and the
# active entry was last on 65 of 69 issues and first on 40 of 69, so both [0]
# and [-1] are wrong.
assert_contains "sk_sprint_active_filter" 'state == "active"' "$SKILL_FLAT"

# The one gate. Nothing is created before it, and the plan file it writes is
# keyed by slug so two runs cannot read each other's tree.
assert_contains "sk_plan_gate_path" "/tmp/claude/open-ticket-" "$SKILL_FLAT"

# The points rule. The epic boundary is stated as a number, so the plan file
# carries a number the user can argue with.
assert_contains "sk_points_13" "over 13" "$SKILL_FLAT"

# Three verification outcomes, never two. An unverified write is the thing the
# user most needs told.
assert_contains "sk_three_outcomes" "Unverified is its own outcome" "$SKILL_FLAT"

# The publish override must name the create command specifically. The
# enumerated list in writing-work-docs names create separately from edit, and
# an override that misses one copy gets blocked by the copy still standing.
assert_contains "sk_publish_override" "publish override" "$SKILL_FLAT"
assert_contains "sk_override_names_create" "workitem create" "$SKILL_FLAT"

# Every reference file is linked from the step that reads it.
assert_contains "sk_links_templates" "TEMPLATES.md" "$SKILL_FLAT"
assert_contains "sk_links_create_fields" "CREATE-FIELDS.md" "$SKILL_FLAT"
assert_contains "sk_links_dedup" "DEDUP-QUERIES.md" "$SKILL_FLAT"

# All thirteen step headings, each anchored to its own line. A flattened
# substring check cannot tell "## Step 1:" from "## Step 12:", and it cannot
# tell an H2 from a demoted H3 either. The trailing space in the pattern is
# what separates step 1 from step 12.
for n in 0 1 2 3 4 5 6 7 8 9 10 11 12; do
  assert_line_count "sk_has_step_$n" "^## Step $n: " 1 "$SKILL_MD"
done

# The loop above pins the numbers and not the titles. A comment in
# shared_config/.claude/hooks/deny-review-in-workflow.py names Step 4 and Step 9 by number, so a
# retitle or a swap of either one leaves that comment asserting something false about this file
# while every assertion above still passes. These two pin what each of those steps actually is.
assert_line_count "sk_step_4_is_exploration" '^## Step 4: Explore the codebase$' 1 "$SKILL_MD"
assert_line_count "sk_step_9_is_the_gate" '^## Step 9: The plan gate$' 1 "$SKILL_MD"

# ---- The odds of triggering ----
# TEMPLATES.md asks a bug ticket for a 1-to-5 rating. These assertions pin the
# half of the skill that derives it. Drop any one of them and the rating still
# gets asked for, so the run fills it with a number nobody derived, and a guessed
# digit in a Stats block reads exactly like triage somebody performed.
assert_line_count "sk_has_odds_section" '^### The odds of triggering, on a defect report only$' 1 "$SKILL_MD"

# The token alone is satisfied by any of its scattered mentions, so the needle has
# to reach the sentence that says when the reader runs. Otherwise the paragraph
# defining the reader can go and the suite still finds the name somewhere else.
assert_contains "sk_names_trigger_odds_reader" "\`trigger-odds\` runs whenever Step 3 called the request a defect report" "$SKILL_FLAT"

# The one silent-failure guard in the section. An absent or unauthenticated MCP
# read as "measured nothing" sets the rating too low, which is the direction that
# gets a real defect deprioritized rather than the direction that wastes time.
assert_contains "sk_probe_unreachable_is_reported" "A probe that cannot reach its source says so" "$SKILL_FLAT"

# The gate is per node, and Step 3's request-level flag can disagree with Step 7's
# per-node type. Lose this and a Bug leaf on a request nobody flagged as a defect
# report reaches Step 8 with a Stats block and no authorized producer for the number.
assert_contains "sk_odds_gate_is_per_node" "The gate is per node and not per request" "$SKILL_FLAT"

# The inline path collapses the two halves into one context, so the independence
# ban cannot hold there. Stating the ban without stating where it fails is how a
# single judgement gets read as two that agreed.
assert_contains "sk_inline_loses_independence" "the independence is gone" "$SKILL_FLAT"

# Step 3 owns both gates, because Step 7 picks the issue type three steps after
# the exploration that has to act on it. Lose this and the reader has nothing to
# switch it on, so it either never runs or runs on every feature request.
assert_contains "sk_step_3_gates_odds" "names a production symptom" "$SKILL_FLAT"

# The independence ban. A reader that has seen the rate stops reading the guards
# honestly, and the two halves then agree because one copied the other. That
# turns a disagreement worth acting on into a silent consensus.
assert_contains "sk_odds_reader_cannot_query" "may not query Datadog or Amplitude" "$SKILL_FLAT"

# Which half wins. A measured rate beats a read of the guards, and without this
# the two halves have no stated precedence, so a run picks whichever it liked.
assert_contains "sk_telemetry_wins" "Telemetry wins when the two halves disagree" "$SKILL_FLAT"

# That the loser survives is a separate rule from which half wins, and the needle
# above cannot reach it. The losing half is the one worth keeping, because a flag
# defaulting off against a measured twelve thousand a day is a second defect. Both
# places the rule is stated get their own needle, since either one alone leaves
# the other deletable.
assert_contains "sk_losing_half_is_kept" "Record the losing half in the plan file anyway." "$SKILL_FLAT"
assert_contains "sk_plan_file_carries_odds" "the odds-of-triggering rating on every defect node" "$SKILL_FLAT"
assert_contains "sk_plan_file_carries_loser" "whichever half lost the disagreement" "$SKILL_FLAT"

# The probes are gated on a production symptom, so a defect request without one
# produces no digests at all. Demanding them unconditionally would make the bullet
# unsatisfiable on that path, and the reason they were gated off is the thing a
# human at the gate actually needs in its place.
assert_contains "sk_plan_file_says_gated_off" "the reason they were gated off when they did not" "$SKILL_FLAT"

# telemetryRules() in work-on/VALIDATION.md is the single copy of the rules the
# probes follow. Restate them here and the two copies drift, then a run follows
# whichever it read last. The pointer names that function and not the file,
# because the absence-is-never-a-zero rule sits in triageRules() next door and
# this skill states that one itself rather than pointing at it.
assert_contains "sk_points_at_telemetry_rules" "telemetryRules()" "$SKILL_FLAT"
assert_contains "sk_absence_rule_is_local" "triageRules()" "$SKILL_FLAT"

# The five readers stay five. A conditional sixth row in that table falsifies the
# count the inline-path paragraph asserts, on every feature request that never
# switches it on. This is the same trap work-on/VALIDATION.md documents for its
# own LENSES array.
assert_contains "sk_odds_not_a_reader_row" "None of this is a row in the table above." "$SKILL_FLAT"

# Neither half landing a number is a TODO(user): line and never a guess. The
# needle has to run through the token itself. Stopped one word short, at "makes
# the rating a", it matches "makes the rating a reasonable guess" just as
# happily, so the assertion passes on the exact outcome it exists to ban.
assert_contains "sk_odds_falls_back_to_todo" "makes the rating a \`TODO(user):\` line carrying the query" "$SKILL_FLAT"

# The impact count gets its own subsection because its gating differs from the odds
# work above it. That subsection runs on Step 3's defect-report flag and its Amplitude
# probe needs a production symptom on top, while every node Step 7 types Bug owes the
# count regardless. Fold the count back under the odds heading and a defect nobody
# called a production symptom reaches Step 8 with no source for the number at all,
# which is the shipped-unrun-TODO this whole route exists to stop.
assert_line_count "sk_has_impact_count_section" '^### The impact count, owed on every `Bug` node$' 1 "$SKILL_MD"
assert_contains "sk_impact_not_gated_on_flags" "gated on neither of Step 3's two flags" "$SKILL_FLAT"

# The route to the number. DB-QUERIES.md is the single copy of the skill check, the
# read-only and bound rules, the handoff format and the re-asking loop. Without the
# pointer this skill has Amplitude and nothing else, and an impact count is a row
# count against product tables that no chart holds. The needle is Step 4's own
# sentence about that file and not the link target, because a markdown link spells
# its target twice and this skill now carries three of them. A bare path needle
# would stay green on all six halves while the one link that matters was deleted.
assert_contains "sk_impact_points_at_db_queries" "the single copy of how a skill with no warehouse connection of its own gets a number out of one" "$SKILL_FLAT"

# The handoff file is keyed by slug and not by a Jira key, because on this path the
# ticket does not exist yet. Lose the literal and a run either invents a key or
# collides with another run's file.
assert_contains "sk_impact_handoff_path" "/tmp/claude/open-ticket-<slug>-queries.sql" "$SKILL_FLAT"

# Timing. Steps 5 through 8 all run without the number, so the ask overlaps the sweep,
# the sizing and the drafting. Handed over at the gate instead it has nothing left to
# overlap, and the run then either stalls in front of the gate or reads the gate's yes
# as cover for a number nobody was asked for.
assert_contains "sk_impact_asks_before_sweep" "hand it over before Step 5, not at the gate" "$SKILL_FLAT"

# "Earn every question" bans asking what the repo answers, and a reader applying it
# here skips the one question the repo cannot answer. Naming the doctrine is the point
# of the needle, since that is the rule a run would otherwise cite to stay silent.
assert_contains "sk_impact_ask_survives_doctrine" "\"Earn every question\" does not override it" "$SKILL_FLAT"

# The TODO(user): line is terminal and never a default. Drop this and every number in
# the Stats block has a legal resting place that costs the run nothing to reach, which
# is exactly how an unrun count ships looking like triage somebody attempted.
assert_contains "sk_todo_is_terminal" "never the first place it lands" "$SKILL_FLAT"

# Which endings authorize that line. Silence is not one of them, and it is the ending
# a run will actually encounter, so it gets named rather than left to inference.
assert_contains "sk_todo_two_endings_only" "an explicit decline or an explicit proceed-without, and silence is neither one" "$SKILL_FLAT"

# The gate grows a fourth answer, so a number that arrives late still lands before the
# create. Without it the gate is yes/no/edit and a pasted count has nowhere to go.
assert_contains "sk_gate_fourth_answer" "yes, no, edit, or the result of a query the plan file lists as unrun" "$SKILL_FLAT"
assert_contains "sk_plan_file_carries_open_queries" "every query handed over and not yet run" "$SKILL_FLAT"

# The delegated path skips Step 4 entirely, so nothing there can derive a rating.
# Without this the path either guesses one from files somebody else chose, or the
# gap goes unstated and a delegated Bug arrives with the field silently empty.
assert_contains "sk_delegated_odds_rule" "A delegated \`Bug\` takes its odds rating from the caller." "$SKILL_FLAT"

# The caller needs a row in the supplied-value table, or the sentence above names
# a channel the contract does not have and work-on sends six values that omit it.
assert_contains "sk_delegated_supplies_odds" "Step 4's \`trigger-odds\` reader, which never runs here" "$SKILL_FLAT"

# The file list replaced the exploration, so the reader has nothing this run chose.
# The probes are gated on the requirement text the caller does supply, so they are
# not suppressed by the same reason and the two get stated apart.
assert_contains "sk_delegated_probes_can_run" "the probes can still run and the reader cannot" "$SKILL_FLAT"

# work-on arrives holding the count and the SQL behind it, so the row exists to stop
# this skill putting the same query in front of the same person twice in one run.
assert_contains "sk_delegated_impact_rule" "A delegated \`Bug\` takes its impact count from the caller too." "$SKILL_FLAT"
assert_contains "sk_delegated_supplies_impact" "Step 4's warehouse ask" "$SKILL_FLAT"

# The delegated narrowing drops Step 9's wait on a recorded agreement. That agreement
# is about the issue being filed and cannot double as the proceed-without an unrun
# number needs, so an open query brings the wait back for that one answer. Without
# this the delegated path is the hole the whole change just closed on the normal one.
assert_contains "sk_delegated_open_query_restores_wait" "a still-open query brings the wait back" "$SKILL_FLAT"

# Step 0 tells the reader the hook is the primary control and this skill's own check is only the
# backstop. Rename that hook, or drop the open-ticket entry from its DENIED_SKILLS tuple, and the
# paragraph goes silently false while the skill keeps claiming a control that is gone. A filename
# and an identifier both survive a reflow and neither can be paraphrased away.
assert_contains "sk_names_the_hook" "deny-review-in-workflow.py" "$SKILL_FLAT"
assert_contains "sk_names_denied_skills" "DENIED_SKILLS" "$SKILL_FLAT"

# WMP does not exist on this site. An example using it teaches a wrong key.
SKILL_WMP="$(grep -c 'WMP' "$SKILL_MD" || true)"
assert_eq "sk_no_wmp_examples" "0" "$SKILL_WMP"

# The delegated entry is what work-on calls. Drop the section and work-on's two
# filing paths have no contract to call, while nothing in this suite notices.
assert_line_count "sk_has_delegated_entry" '^## Delegated entry$' 1 "$SKILL_MD"

# Sibling and never child is a Jira validity rule, not a preference. open-ticket
# parents a Story to an Epic, so a follow-up Story filed under a Story either
# fails the create or forces the follow-up to be a subtask of work it is not
# part of. This is the single most expensive sentence in the section to lose.
assert_contains "sk_followup_is_a_sibling" "sibling of the originating ticket" "$SKILL_FLAT"

# The supplied values. Each one names a step it satisfies, so dropping a row
# silently puts a step back in the delegated path that the caller already answered.
# Each needle is the mapping phrase itself and not the supplied value, because the
# supplied value alone is common prose that deleting a table row would still leave
# sitting elsewhere in the file.
assert_contains "sk_delegated_supplies_project" "Step 2's project inference" "$SKILL_FLAT"
assert_contains "sk_delegated_supplies_type" "Step 7's issue-type-by-intent call" "$SKILL_FLAT"
assert_contains "sk_delegated_supplies_files" "Step 4's exploration" "$SKILL_FLAT"
assert_contains "sk_delegated_supplies_origin" "Step 6's exclusion and the description's context line" "$SKILL_FLAT"
assert_contains "sk_delegated_supplies_parent" "Step 7's parenting" "$SKILL_FLAT"

# Preflight, the sweep and the gate always run. A caller can vouch for its own
# scope decision and cannot vouch for Jira access, so skipping either would file
# a duplicate under a clean verdict.
assert_contains "sk_delegated_still_runs_preflight" "Step 0's preflight always runs" "$SKILL_FLAT"
assert_contains "sk_delegated_still_runs_sweep" "the sweep and the gate always run" "$SKILL_FLAT"

# No sprint in delegated mode. A follow-up lands in the backlog until somebody
# schedules it, and Step 2's open-sprint query is about the caller's current work.
assert_contains "sk_delegated_no_sprint" "no sprint in delegated mode" "$SKILL_FLAT"

# The parent exclusion, inside the Step 6 gate. Without it the sweep finds the
# originating ticket, which is by construction about the same area and often the
# same files, and the gate aborts on the ticket the caller is mid-way through.
assert_contains "sk_origin_not_a_duplicate" "never a credible match" "$SKILL_FLAT"

# A sibling follow-up already filed off the same origin is not excluded from the sweep. It goes
# through the two credibility tests above like any other match. Losing this rule could exempt a
# sibling from the sweep entirely, or promote it past the tests just for being one.
assert_contains "sk_sibling_still_counts" "a sibling follow-up already filed" "$SKILL_FLAT"

# ---- work-on/DB-QUERIES.md, the half open-ticket depends on ----
# Step 4 points at this file for the impact count instead of restating it, so a rename
# or a move breaks the route and nothing else in this suite would notice.
require_file "$DB_QUERIES_MD"

DB_QUERIES_FLAT="$(flatten "$DB_QUERIES_MD")"

# The file has to name open-ticket's handoff path, or a run reading it writes the
# work-on-<KEY> name and has no key to put in it.
assert_contains "dbq_names_open_ticket_handoff" "/tmp/claude/open-ticket-<slug>-queries.sql" "$DB_QUERIES_FLAT"

# Both callers are named in the opening, so a reader arriving from open-ticket is not
# reading a document that says it is about a different skill.
assert_contains "dbq_serves_both_skills" "\`open-ticket\` reads it from Step 4" "$DB_QUERIES_FLAT"

# The loop's endings are what gate the TODO(user): line at the other end. Lose this
# sentence and the two endings read as examples rather than as the only two.
assert_contains "dbq_two_endings_gate_the_todo" "These two endings are the only things that turn a number into a \`TODO(user):\` line." "$DB_QUERIES_FLAT"

echo "open_ticket_contract_test: ok"
