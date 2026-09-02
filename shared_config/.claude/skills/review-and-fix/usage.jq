# Per-agent token usage from a set of transcripts. See USAGE.md beside this file.
# Run as: jq -r -n -f usage.jq <session>/subagents/agent-*.jsonl <session>/subagents/workflows/*/agent-*.jsonl
# One "usage ..." line per transcript that has at least one assistant turn.

def rates: {
  "claude-opus-5":   {in: 5.00, cr: 0.50, cw: 6.25, out: 25.00},
  "claude-sonnet-5": {in: 2.00, cr: 0.20, cw: 2.50, out: 10.00},
  "claude-haiku-4-5":{in: 1.00, cr: 0.10, cw: 1.25, out: 5.00}
};
def est($m; $in; $cr; $cw; $out):
  (rates[$m]) as $r
  | if $r == null then "?" else
      ((($in*$r.in + $cr*$r.cr + $cw*$r.cw + $out*$r.out) / 1e6 * 100 | round) / 100) end;
def M: (. / 1e6 * 10 | round) / 10;
def K: (. / 1e3 | round);
[inputs | {f: input_filename, r: .}]
| group_by(.f)
| map(
    (map(.r)) as $rows
    | ($rows[0].message.content // "" | tostring) as $p
    | ([$rows[] | select(.type=="assistant") | .message]) as $m
    | select(($m|length) > 0)
    | ($p | capture("<!-- (?<kind>[a-z-]+)(?<rest>[^>]*) -->")? // {kind:"unstamped", rest:""}) as $s
    | ($m[0].model // "?") as $model
    | ([$m[].usage.input_tokens]|add // 0) as $in
    | ([$m[].usage.cache_read_input_tokens]|add // 0) as $cr
    | ([$m[].usage.cache_creation_input_tokens]|add // 0) as $cw
    | ([$m[].usage.output_tokens]|add // 0) as $out
    | "usage kind=\($s.kind)\($s.rest) id=\(.[0].f | sub("^.*agent-";"") | .[0:8]) model=\($model|sub("^claude-";"")) turns=\($m|length) cache_read=\($cr|M)M cache_write=\($cw|M)M in=\($in|K)K out=\($out|K)K est_usd=\(est($model;$in;$cr;$cw;$out))"
  )
| .[]
