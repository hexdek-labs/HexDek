# Parser Coverage Report

Pool: **31,963 real cards**.

## Headline

- 🟢 GREEN (every ability parsed cleanly): **31,963** (100.00%)
- 🟡 PARTIAL (some abilities parsed, others left as raw text): **0** (0.00%)
- 🔴 UNPARSED (parser couldn't recognize any abilities): **0** (0.00%)

**Goal: 100% GREEN.** Every PARTIAL/UNPARSED entry corresponds to a specific
unhandled grammar production — fixable by adding an effect rule, a trigger pattern,
or a keyword. No heuristic catch-alls.

## Top unparsed fragments — the work queue

Each row is an unparsed clause prefix. The count is how many cards' parse failed
at this prefix. Tackling the highest-count entries first shrinks the queue fastest.

| Count | Fragment prefix |
|---:|---|