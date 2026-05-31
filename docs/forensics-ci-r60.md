# Forensics CI Integration (r60)

`scripts/run-forensics.sh` is the CI guard around the InstanceID
mint-bypass forensics pipeline. It runs the engine under Loki, feeds
the resulting ZoneConservation violations through `hexdek-forensics
--patterns`, and fails the build if a NEW cluster key appears that
isn't in `scripts/forensics-baseline.txt`.

## What it catches

Any new code path that mints (or, more precisely, fails to mint) a
`*Card` InstanceID and lands the card on a battlefield / hand /
graveyard / library / stack. Concrete failure modes:

- A struct-literal `Card{...}` somewhere in a new per_card handler
  that bypasses `MintOGInstanceID` / `MintCopyInstanceID`.
- A clone helper or `deepCopy` that copies the `*Card` body but
  doesn't allocate a fresh instance ID.
- A token-mint variant that forgot to call the TK provenance helper.
- A reanimate / blink / control-trade path that re-adds a stale
  `*Card` reference (already-ceased instance ID) back to a zone.

The dominant historical exemplars:

- Adric (PR #178 line): hand+battlefield duplication via raw
  `removePermanent + moveCardBetweenZones` instead of the canonical
  `ExilePermanent` API. 18 of 410 r43 CardIdentity hits.
- Cerulean Sphinx (r41 follow-up, PR #169): 1,622 of 1,652 r41
  hits — Activated-AST cast resolution mis-routing the body to the
  owner's library before the permanent ETB.
- God-Eternal Oketra zone-leak (r48, commit `7e782cf`): tuck arm of
  `resolveModificationEffect` not falling through to graveyard/exile.

Each of those shows up in `hexdek-forensics --patterns` as a single
`PatternKey` — a `{Provenance, FirstEventKind, MatchKind}` triple
that's stable across every game where the same code path is the
bypass site. One row = one bisect target.

## Why this matters

The r60 era closed with **0 violations / 0 crashes / 0 panics** across
10K games x 10 canonical seeds (2026-05-25 canonical-final, CLAUDE.md
kanban). That clean surface is fragile: any future PR that adds a
hand-rolled `*Card` somewhere can re-open the cluster, and the only
signal today is a 5K-game Loki run that takes minutes and produces a
markdown report a human has to skim. This CI integration:

1. Runs Loki at a CI-tractable game count (default 50, configurable).
2. Auto-classifies whatever fabrications surface into stable cluster
   keys.
3. Fails the run if the cluster set grew vs. the baseline.

A green CI run after a PR means "no new mint-bypass code paths."

## How it runs

```
hexdek-loki --games N --seed S --violations-dump T.tsv --report R.md
   ↓ (synthesize a Replay JSON — see TODO below)
hexdek-forensics --replay replay.json --patterns
   ↓ (grep "prov=X first=Y match=Z" lines)
diff vs. scripts/forensics-baseline.txt
   ↓
EXIT 0 (clean) | EXIT 1 (new cluster)
```

The script supports four flags:

- `--games N` — override Loki game count (default 50).
- `--seed N` — override Loki seed (default 42).
- `--update-baseline` — overwrite the baseline file with the current
  cluster set. Use sparingly — see "Updating the baseline" below.
- `--use-stored-replay <path>` — skip the Loki step entirely and feed
  a pre-existing Replay JSON. Used by `run-forensics_test.sh` for
  deterministic test runs against the bundled testdata fixtures.

## Updating the baseline

The shipped baseline (`scripts/forensics-baseline.txt`) is the
**empty set**. On a healthy engine, a 50-game / seed=42 Loki run
surfaces no ZoneConservation fabrications, so any cluster appearing
in CI is a regression.

When you see a CI failure under this script:

1. **Default response: fix the bug.** The `--patterns` output names
   the first event the rogue InstanceID appeared in. Bisect from
   there — that's the closest event boundary to the bypass site.
2. **Only update the baseline when the new cluster is genuinely
   intentional.** That's rare. Examples that DO warrant an update:
   - A deliberate refactor that re-routes an existing fabrication
     through a different provenance helper (cluster key changes;
     count doesn't).
   - A known engineering carve-out that's already a separately
     tracked issue.
3. **Never update the baseline as a way to silence noise during
   debugging.** If something flapped, find the root cause first.

To update: review the new entries by hand, then run

```
scripts/run-forensics.sh --update-baseline
```

The script overwrites `scripts/forensics-baseline.txt` with the
current cluster set. Commit the diff along with the engine change
that introduced it and call out the rationale in the PR body.

## Loki `--replay-stream-out` TODO

Today the script SYNTHESIZES a Replay JSON from Loki's `--violations-dump`
TSV (turn / invariant / message columns) plus an empty event log and
empty CardIndex. That's lossy on two axes:

1. The `FirstEventKind` field of every PatternKey degrades to
   `<not_found>` because there's no event log to walk.
2. The `MatchKind` field degrades to `<none>` for the same reason.

The provenance (`OG` / `TK` / `CP`) still classifies correctly because
it's decoded directly from the InstanceID in the violation message via
the `internal/instanceid` layout, so the baseline-diff signal still
fires reliably — a NEW provenance surface still produces a new key.
But the cluster keys are less specific than they could be.

The producer-side fix lives in `cmd/hexdek-loki`: add a
`--replay-stream-out <path>` flag that opens the file for append and
writes one full `Replay` JSON line per finished game. The watch-mode
consumer in `cmd/hexdek-forensics/watch.go` is already shaped for it
(see the package comment in `watch.go`).

Once that flag lands, swap the synthesis block in `run-forensics.sh`
for a direct file read of Loki's output. The baseline file likely
needs a one-time regeneration because the cluster keys will shift from
`first=<not_found>` to real event kinds — same provenance breakdown,
finer-grained event resolution.

## References

- PR #781 — Loki r60 baseline canonical-final closure (0 violations
  at 10K games x 10 seeds), the engine surface this CI defends.
- PR #819 — LinkedExile rebase, the most recent surface where a
  potential new mint-bypass had to be cleared by hand.
- PR #839 — original `hexdek-forensics` CLI (one-shot tracing).
- PR #857 — `hexdek-forensics` v2 with `--patterns` clustering and
  `--watch` streaming aggregate (the CLI surface this CI consumes).
- `cmd/hexdek-forensics/watch.go` package comment — the producer-side
  TODO for `--replay-stream-out`.
- `cmd/hexdek-forensics/replay.go` — the on-disk Replay schema.
- `CLAUDE.md` kanban → "Loki r60 canonical-final" — the broader r60
  cleanup history this guard locks in.
