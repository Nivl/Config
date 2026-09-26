# Per-agent token usage from a set of transcripts. See USAGE.md beside this file.
# Run as: jq -r -n -f usage.jq <session>/subagents/agent-*.jsonl <session>/subagents/workflows/*/agent-*.jsonl
# One "usage ..." line per transcript that has at least one assistant turn.

def rates: {
  "claude-fable-5-1":{in: 10.00, cr: 0.25, cw: 12.50, out: 50.00},
  "claude-fable-5":  {in: 10.00, cr: 0.25, cw: 12.50, out: 50.00},
  "claude-opus-5-5": {in: 4.00, cr: 0.20, cw: 5.00, out: 20.00},
  "claude-opus-5":   {in: 5.00, cr: 0.50, cw: 6.25, out: 25.00},
  "claude-opus-4-7": {in: 5.00, cr: 0.50, cw: 6.25, out: 25.00},
  "claude-sonnet-5": {in: 2.00, cr: 0.20, cw: 2.50, out: 10.00},
  "claude-haiku-4-5":{in: 1.00, cr: 0.10, cw: 1.25, out: 5.00}
};
# A dated model id is the same model as its undated key, so it prices at that key.
# A bare tier alias names no version, so it stays unpriced per USAGE.md.
# cw is every cache write and cw1h the part of it with a one-hour lifetime,
# which bills at 2x input where a five-minute write bills at 1.25x (cw).
def est($m; $in; $cr; $cw; $cw1h; $out):
  (rates[$m] // rates[$m | sub("-20[0-9]{6}$";"")]) as $r
  | if $r == null then "?" else
      ((($in*$r.in + $cr*$r.cr + ($cw - $cw1h)*$r.cw + $cw1h*$r.in*2 + $out*$r.out) / 1e6 * 100 | round) / 100) end;
def M: (. / 1e6 * 10 | round) / 10;
def K: (. / 1e3 | round);
[inputs | {f: input_filename, r: .}]
| group_by(.f)
| map(
    (map(.r)) as $rows
    # The stamp is in the prompt, which is the first user record unless the
    # Workflow harness relayed the triggering slash command first, in which case
    # it is the second. Read every user record before the first assistant turn.
    | ([$rows[] | select(.type=="user" or .type=="assistant")] | (map(.type) | index("assistant")) // length) as $nu
    | ([$rows[] | select(.type=="user" or .type=="assistant")] | .[0:$nu] | map(.message.content // "" | tostring) | join(" ")) as $p
    # One API response is written as several assistant records, one per content
    # block, all with the same message id. The cache fields repeat, and the
    # output count grows from a partial value on the first record to the final
    # one. So one entry per id, each usage field at its largest, or a response
    # with thinking, text and a tool call is billed three times.
    | ([$rows[] | select(.type=="assistant") | .message]) as $all
    | ($all[0].model // "?") as $model
    | ($all
       | map(select(.id == null)) + (map(select(.id != null)) | group_by(.id) | map(
           {usage: {input_tokens: (map(.usage.input_tokens // 0) | max),
                    cache_read_input_tokens: (map(.usage.cache_read_input_tokens // 0) | max),
                    cache_creation_input_tokens: (map(.usage.cache_creation_input_tokens // 0) | max),
                    cw_1h: (map(.usage.cache_creation.ephemeral_1h_input_tokens // 0) | max),
                    output_tokens: (map(.usage.output_tokens // 0) | max)}}))) as $m
    | select(($m|length) > 0)
    | ($p | capture("<!-- (?<kind>[a-z-]+)(?<rest>[^>]*) -->")? // {kind:"unstamped", rest:""}) as $s
    | ([$m[].usage.input_tokens]|add // 0) as $in
    | ([$m[].usage.cache_read_input_tokens]|add // 0) as $cr
    | ([$m[].usage.cache_creation_input_tokens]|add // 0) as $cw
    | ([$m[].usage.output_tokens]|add // 0) as $out
    | ([$m[].usage.cw_1h // 0]|add // 0) as $cw1h
    | "usage kind=\($s.kind)\($s.rest) id=\(.[0].f | sub("^.*agent-";"") | .[0:8]) model=\($model|sub("^claude-";"")) turns=\($m|length) cache_read=\($cr|M)M cache_write=\($cw|M)M in=\($in|K)K out=\($out|K)K est_usd=\(est($model;$in;$cr;$cw;$cw1h;$out))"
  )
| .[]
