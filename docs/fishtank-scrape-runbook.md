# Fishtank Live-Issue Scrape — Daily Runbook

**Tool:** `hexdek-muninn --judge-triage` (the Hex Judge triage clerk).
**Purpose:** pull the live grinder's correctness signal off DARKSTAR and
emit a timestamped, deduped, dimension-grouped digest of what's actually
breaking in production — the "firespotting radar."

The triage ingests **two live sources** and merges them into one digest:

1. **The Judge watchdog bucket** — `data/judge/grinder-violations.jsonl`,
   the sampled-Judge stream (conservation / state-integrity / legality /
   outcome / progression / liveness). Written only when the server runs
   with `HEXDEK_JUDGE_SAMPLE` set.
2. **The server.log Feynman lines** — the grinder's post-game Feynman
   oracle prints every bracket-game violation as
   `[sev] feynman/RULE [seat N]: MESSAGE` (e.g. `feynman/704.5f`). These
   land in `server.log`, **not** the jsonl bucket — so when the bucket is
   empty (engine clean on the sampled dimensions) the server.log is where
   the live issues still show up.

Both fold into the same clusters: a cluster dedupes by
`dimension|rule|object` (the **object** is the card name when the message
names one), so the same bug across many games/seats is one cluster with a
true occurrence count, ranked within its dimension.

---

## DARKSTAR access

DARKSTAR is Windows now (Ryzen 9, `192.168.1.207`). Reach it over SSH with
the worker key; the WireGuard VPN must be up when remote
(`sudo wg-quick up ~/.config/wireguard/admin-vpn.conf`).

| | |
|---|---|
| key | `~/.ssh/claude_remote_ed25519` |
| host | `joshu@192.168.1.207` |
| jsonl bucket | `D:/hexdek/data/judge/grinder-violations.jsonl` |
| server log | `D:/hexdek/server.log` |

(Under scp, Windows paths use forward slashes.)

---

## One-shot scrape

```bash
#!/usr/bin/env bash
set -euo pipefail
KEY=~/.ssh/claude_remote_ed25519
HOST=joshu@192.168.1.207
STAMP=$(date -u +%FT%TZ)
WORK=$(mktemp -d /tmp/fishtank-scrape.XXXX)

# 1. Pull both live artifacts. Tolerate an absent/empty bucket.
scp -i "$KEY" "$HOST:D:/hexdek/data/judge/grinder-violations.jsonl" \
      "$WORK/grinder-violations.jsonl" 2>/dev/null || \
  : > "$WORK/grinder-violations.jsonl"
scp -i "$KEY" "$HOST:D:/hexdek/server.log" "$WORK/server.log"

# 2. Triage: jsonl bucket + server.log feynman lines -> one digest.
hexdek-muninn --judge-triage \
  --judge-log "$WORK/grinder-violations.jsonl" \
  --server-log "$WORK/server.log" \
  --server-log-stamp "$STAMP" \
  --dir "$WORK/out"

echo "digest: $WORK/out/judge-triage.md (+ .json)"
```

`server.log` can be large. To pull only the bracket-game lines (the parser
ignores everything else anyway), pre-filter on DARKSTAR:

```bash
ssh -i "$KEY" "$HOST" \
  'findstr /C:"feynman/" /C:"invariants/zone_conservation" D:\hexdek\server.log' \
  > "$WORK/server.log"
```

---

## Daily / only-new scrape

The digest header is timestamped (`Generated:`). To see only issues newer
than yesterday's run, add `--since` — it filters the jsonl rows by their
own `ts` and the server.log lines by their Go-log stamp (or the
`--server-log-stamp` fallback). Excluded rows are counted (`SinceFiltered`).

```bash
hexdek-muninn --judge-triage \
  --judge-log "$WORK/grinder-violations.jsonl" \
  --server-log "$WORK/server.log" --server-log-stamp "$STAMP" \
  --since "$(date -u -d yesterday +%F)" --dir "$WORK/out"
```

`--since` accepts a full RFC3339 instant or a bare `YYYY-MM-DD` (taken as
00:00:00 UTC). Use the previous run's `Generated:` stamp to scrape exactly
the gap. Rotated buckets (`grinder-violations.jsonl.1`, `.2`, …) fold in
automatically.

### Suggested cron (06:00 UTC, only-new)

```cron
0 6 * * *  /usr/local/bin/fishtank-scrape.sh --since "$(date -u -d yesterday +%F)" \
             >> ~/fishtank/triage-cron.log 2>&1
```

---

## Reading the digest

- Clusters group by Judge dimension in canonical order (legality →
  conservation → state_integrity → progression → outcome → liveness),
  ranked by occurrence count within each.
- Each cluster carries: the `dimension|rule|object` fingerprint, count,
  first/last-seen window, worst severity, and a sample message.
- **The object is the card name.** `704.5f` over five different creatures
  is five clusters; the same creature across many games is one cluster
  with a true count — that count tells you which card to prioritize.
- **Repro:** jsonl-sourced clusters carry the repro seed + deck keys
  (replay the same seed with the same decks = same game). server.log-only
  clusters show `seed 0` (the log line carries no seed); when the same bug
  also lands in the jsonl bucket, the merged cluster carries the bucket's
  repro.
- A clean scrape prints `Nothing to triage — no artifacts written` and
  leaves any prior digest untouched (the zero-record writer is a no-op).

---

## Triage → fix → confirm loop

1. Scrape → digest. The top cluster per dimension is the highest-frequency
   live offender.
2. Reproduce (jsonl repro seed + decks) or inspect the named card.
3. **Decide checker-FP vs real bug.** A "violation" can be the *checker*
   diverging from the engine, not an engine fault — e.g. the §704.5f
   layer-aware predicate FP (commit `40477cbf`): the post-game checker read
   the printed creature type while the SBA reads the layer-aware type, so a
   type-stripped permanent (Song of the Dryads → land) at toughness 0 was
   flagged though §704.5f correctly does not apply.
4. Fix the right side (engine or checker), add a guardrail regression test.
5. **Confirm:** a fresh scrape no longer shows the cluster. A clean run
   reports `Nothing to triage`.

---

## CLI reference (the triage flags)

| flag | meaning |
|---|---|
| `--judge-triage` | run the triage clerk |
| `--judge-log path` | jsonl bucket (default `data/judge/grinder-violations.jsonl`); rotated siblings fold in |
| `--server-log path[,path…]` | server.log capture(s) to scrape for `feynman/RULE [seat N]: MESSAGE` lines |
| `--server-log-stamp when` | fallback timestamp for server.log lines with no Go-log stamp (default: run time) |
| `--since when` | only triage records at/after this time (RFC3339 or `YYYY-MM-DD`) |
| `--dir path` | output dir for `judge-triage.md` + `.json` (default `data/muninn`) |
