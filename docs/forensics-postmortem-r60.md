# Forensics Postmortem (r60)

## Summary

Post-mortem of the InstanceID forensics workflow as it stands at the
end of r60. Catalogs what the `hexdek-forensics --patterns` pipeline
plus the `scripts/run-forensics.sh` CI guard can actually detect,
where the blind spots are, and how an on-call engineer should drive a
fresh Loki violation to either a fix or a baseline update. Audience:
anyone landing engine PRs after 2026-05-30 who hits a red CI run or a
new Loki cluster.

## What the forensics workflow catches

Mint-bypass classes, grouped by the r60 phase that closed them. Each
phase corresponds to a stage in the InstanceID cleanup arc; D and E
predate Phase F's explicit labeling and are reconstructed from PR
history.

### Phase D — initial mint coverage

Early r60 work wiring `MintOGInstanceID` into deck-load and basic
spell-cast paths. The bug class: a permanent enters the battlefield
with a zero-valued or shared InstanceID because the cast path forgot
to call the mint helper at all.

- Exemplar: Cerulean Sphinx r41 follow-up (PR #169) — 1,622 of 1,652
  r41 hits, Activated-AST body cast resolved into owner's library
  before ETB minted a fresh ID.
- `--patterns` surface: provenance decodes as `OG` but
  `FirstEventKind` lands on `cast` or `<not_found>`, `MatchKind` is
  `card_name` because the event log doesn't carry the instance ID.
- CI catch: yes on the first Loki run that loads the affected card —
  Phase D bugs cluster heavily (hundreds of hits per game) and surface
  in seed=42 / 50-game CI immediately.

### Phase E — residual tail edges

LTB / control-trade / reanimate paths that re-add a stale `*Card`
reference back to a zone without re-minting or routing through the
canonical exit API.

- Exemplar: Adric (PR #178) — 18 of 410 r43 CardIdentity hits via raw
  `removePermanent` + `moveCardBetweenZones` bypassing
  `ExilePermanent`.
- Exemplar: God-Eternal Oketra (r48 commit `7e782cf`) — tuck arm of
  `resolveModificationEffect` skipping graveyard/exile fall-through.
- `--patterns` surface: provenance is the original `OG`/`TK`/`CP`
  marker of the resurrected card; `FirstEventKind` is the LTB event
  (`destroy`, `exile`, `sacrifice`); `MatchKind` is `instance_id` when
  the LTB event logged the ID directly.
- CI catch: yes when the affected card is exercised. Phase E hits are
  lower-frequency (single-digit per game) but cluster across multiple
  games on the same key, so 50-game CI surfaces them.

### Phase F — spell-copy canonical chokepoint

Per-card handlers fabricating spell copies by aliasing the source
`*Card` pointer directly into a new `StackItem` instead of routing
through `MintSpellCopy`. Phase F closed 10 sites: Alania, Zada, Krark,
Mica, Mendicant, Rootha, Kalamax, Ivy, Fire Lord Azula, Ulalek (5K
verify at commit `07252406`).

- `--patterns` surface: provenance decodes as `OG` (because the copy
  inherited the original's ID rather than getting a `CP` mint);
  `FirstEventKind` is `stack_push` or `cast`; the rogue ID appears in
  multiple stack slots in the same turn.
- CI catch: yes once the handler triggers. Phase F sites are
  trigger-gated (need a spell cast of the right color / type), so seed
  variance matters — seed=42 covers most but not all.

### Phase G — DFC and alias-on-copy

The current frontier. Two known open clusters at 2026-05-30:

- Aziza spell-copy fabrication (game 2762, seed 42) — 34
  ZoneConservation hits. Handler aliased `*Card` directly into
  `StackItem` without `MintSpellCopy`. Phase G closure routed through
  the canonical chokepoint.
- Silk, Web Weaver + Opaline Bracers (game 4557, seed 43) — 38 hits.
  DFC + equipment-cycling lineage gap, queued as Phase H.
- `--patterns` surface: same shape as Phase F (provenance `OG`,
  `FirstEventKind` ~ `stack_push`), but only one or two cards trigger
  the bypass, so cluster row count is low.
- CI catch: partial. Phase G clusters surface only on the seed that
  exercises the affected card; the Silk hit is seed=43, not the
  default seed=42 CI run.

## What the workflow does NOT catch

### Low-frequency tail edges

The `RankedKeys` aggregate sorts by hit count descending. A single-hit
fabrication in a game that doesn't run in CI never surfaces. Closing
this needs either a wider seed sweep in CI (cost) or per-key always-
report regardless of count (noise). Deferred — see Future Work for the
sweep-multiplier proposal.

### Cross-seat interactions

`PatternKey` intentionally excludes the seat number so the same code
path collapses across all four seats. The side effect: inter-seat
relationship breaks (e.g. a control-trade leaking an InstanceID
between seats) cluster the same way as a single-seat bypass and the
relational signal is lost. Closing it needs a second pattern dimension
keyed on `(source_seat, target_seat)` deltas.

### Non-ZoneConservation invariants

`ExileLinkageIntegrity`, `CardIdentity`, token mint/cease imbalance,
and replacement-completeness violations have different message shapes
and aren't extracted by `ExtractFabricatedInstanceID`. Each needs a
sibling extractor + classifier. The ExileLinkage variant is the
highest-leverage next target — see Future Work.

### Synthesis lossiness

`run-forensics.sh` currently synthesizes a Replay JSON from Loki's
`--violations-dump` TSV because Loki has no `--replay-stream-out` yet.
Synthesis carries the violation message and decoded provenance, but
the event log is empty. Consequences:

- `FirstEventKind` degrades to `<not_found>` for every cluster.
- `MatchKind` degrades to `<none>`.
- Two different code paths that mint the same provenance type collapse
  into the same cluster key.

Provenance still classifies correctly (decoded from the InstanceID
directly), so a NEW provenance surface still produces a NEW key and
CI still fires. But within-provenance discrimination is lost until
producer-side replay streaming lands.

### Race conditions

Loki replays a recorded random seed deterministically. Genuine
multi-goroutine race conditions in the engine that surface only under
real wall-clock scheduling won't replay reliably. The forensics
pipeline assumes deterministic replay throughout.

### Test-scaffold paths

A `*Card` struct-literal built inside a test file emits no production
event. `TraceFirstAppearance` returns `NotFound`, which is
indistinguishable from a production bypass that also fails to log.
The current workaround: if `FirstEventKind=<not_found>` and a manual
event-log walk finds nothing around the violation turn, suspect a test
fixture leaking into the engine path.

## Operational runbook — Loki hit a new violation, now what?

1. **Triage: fabrication or disappearance?** The CI script prints the
   raw violation message. Fabrication messages match `present in a
   zone but not in (Minted - Ceased)`; disappearance is the inverse.
   The forensics CLI only classifies fabrications today. Disappearance
   needs the per-violation `--replay` path.

2. **Capture the replay.** If `--replay-stream-out` is wired (see
   Future Work), grab the per-game JSON directly. Otherwise rerun
   Loki with `--violations-dump` and let `run-forensics.sh`
   synthesize:

   ```
   hexdek-loki --games 50 --seed 42 \
     --violations-dump /tmp/v.tsv --report /tmp/r.md
   scripts/run-forensics.sh --use-stored-replay /tmp/replay.json
   ```

3. **Cluster.** Read the `--patterns` output. Top row by hit count is
   the most likely systemic bypass site:

   ```
   prov=OG first=stack_push match=instance_id   hits=34
     example: h1OGVR200096  game=2762 turn=14
     first_event: stack_push src=Aziza, ...  seat=1  turn=14
   ```

4. **Bisect.** The `first_event` line names the closest engine event
   to the bypass. Grep production code for that event emit
   (`LogEvent("stack_push", ...)`) and walk callers until you find the
   handler that built the `StackItem` without minting.

5. **Decide: fix or baseline.** Default is fix. Update the baseline
   only when:
   - A deliberate refactor moves an existing fabrication to a new
     provenance helper (key changes; count doesn't).
   - The new cluster is a known engineering carve-out with its own
     tracking issue, called out in the PR body.

   Never update the baseline to silence flapping. Find the root
   cause.

6. **Fall back when the first event is `<not_found>`.** This is the
   synthesis-lossiness case until streaming lands. Rerun forensics in
   per-violation mode against a single replay:

   ```
   hexdek-forensics --replay /tmp/replay.json --violation 0
   ```

   Read the event log by hand around the violation turn — usually the
   2-3 events preceding the SBA pass name the perm and seat. If the
   manual walk also finds nothing, suspect a test fixture or
   struct-literal bypass that emits no event.

7. **Escalate.** If the cluster falls into a known blind spot
   (cross-seat, non-ZoneConservation, race), the forensics CLI cannot
   close it. File a TODO referencing the Future Work entry and gate
   the PR on a deferred-work decision rather than a baseline update.

## Trajectory

| PR | Date | Violations | Notes |
|----|------|-----------|-------|
| #781 | 2026-05-25 | 268 | Loki r60 canonical-final baseline; strict-census on |
| #819 | 2026-05-26 | 224 | LinkedExile rebase, removed dominant linkage cluster |
| #839 | 2026-05-27 | n/a | Original `hexdek-forensics` CLI — one-shot per-violation tracing |
| #857 | 2026-05-28 | n/a | `hexdek-forensics` v2 with `--patterns` clustering and `--watch` stream |
| #880 | 2026-05-29 | 0 | CI integration: `run-forensics.sh` + empty baseline, fails on new cluster |

The 50K deep sweep (2,808 violations across 50 games on extended
seeds) is excluded from the trajectory because it surfaces the same
two Etali clusters as 5K runs without introducing new pattern keys.

## Future work

- Loki producer-side `--replay-stream-out` flag so `run-forensics.sh`
  reads real event logs instead of synthesizing from TSV.
- Disappearance-arm extractor + classifier (sibling to
  `ExtractFabricatedInstanceID`).
- `ExileLinkageIntegrity` pattern variant — highest-leverage non-ZC
  invariant.
- Cross-seat `PatternKey` extension: add a `(src_seat, dst_seat)`
  dimension behind an opt-in flag.
- Sweep-multiplier in CI: 3-5 seeds at the cost of CI minutes to catch
  Phase G-class seed-bound clusters (e.g. Silk on seed=43).
- Causal-chain tracing in `TraceFirstAppearance` — walk dependency
  chain rather than stopping at first match.
- Token mint/cease imbalance extractor.
- Auto-baseline regeneration tooling once streaming lands (one-time
  cluster-key shift expected).
