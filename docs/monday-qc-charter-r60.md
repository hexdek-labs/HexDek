# The Dreaded Quality Control and Hopefully Quality of Life Magically Hunt

*presented by Hex and sponsored by RedBull*

> A structured audit of the r60 codebase, focused on the InstanceID substrate
> as a lens for evaluating card-handler engineering debt. Output: an
> economic ledger of "what fits the substrate, what doesn't, what's been
> shipped anyway, and what to do next."

---

## §0 — Preamble + Goals

The night-feyd swarm shipped ~222 PRs over ~12 hours. The engine reached 0/0/0/0
invariants at 5k Loki baseline. Phase 1 200k stress-verification surfaced a
~99.95%-clean residual at 50k seed-dispersion (paradigm-family + exile-until-leaves
+ Hostage Taker LTBReturn signatures). This charter audits the codebase **with
the InstanceID system as the evaluation lens** — asking, for every card-handler
and every issue, whether the substrate is the right canonical solution.

**Goals:**

1. Map every card-issue (open + recently-resolved) against InstanceID applicability
2. Identify refactor candidates where ad-hoc per_card code shipped despite
   the substrate being the right answer (the "shipped before checking" smell)
3. Catalogue the toss-backs — cards/issues where InstanceID genuinely doesn't fit
   — with reasoning, so human eyes can second-guess and so re-audit cycles
   benefit from new primitives
4. Inventory every accepted per_card handler ("the CVS Receipt") to make
   the per_card economic tax visible rather than invisible
5. Produce a Phase H / I / J priority ordering for substrate work and per_card
   refactor batches

**Non-goals:**

- Not a behavior regression hunt — that's QC's Test surface, separate scope
- Not an LOC accounting sweep — that's the LOC QC pass, separate scope
- Not a CR compliance audit per se — CR compliance is implicit in the
  "is this CR-expressible via InstanceID" question on each card

---

## §1 — Substrate Baseline

Inventory of InstanceID + sibling primitives currently available, as of
session end 2026-05-31. Audit decisions in later sections reference this
baseline; a re-audit after substrate evolution (new primitives shipped)
re-checks toss-backs against the updated baseline.

### 1.1 InstanceID format (Phase A)

`<prefix><seat><provenance><visibility><color><cmc><seq5>`, e.g. `h2OGVR100042`.

- **prefix** — zone class
- **seat** — owner seat (0-3)
- **provenance** — OG / TK / CP / AB (original / token / copy / ability-derived)
- **visibility** — visible / hidden
- **color** — color identity bits
- **cmc** — CMC at mint time
- **seq5** — 5-digit monotonic sequence per (game, seat) pair

### 1.2 Mint chokepoints (Phases A → G)

Four canonical mint paths, every per_card / engine site producing a `*Card`
or wrapping one as a Permanent / StackItem MUST route through these:

| Primitive | Use |
|---|---|
| `MintTokenAsCopyOf` | per_card token-copy from another card (e.g. Drafna, Sai, etc.) |
| `MintSpellCopy` | engine §707.10 spell-copy on the stack (Aziza, paradigm, etc.) |
| `MintPermanentCopy` | engine §707.2 permanent-copy on the battlefield (clones) |
| `MintMutateMeld` | mutate / meld merge — uses `MergedCardPtrs` + `MergeKind` |

### 1.3 Cessation primitives (Phase D + F)

- `MarkInstanceIDCeased(gs, iid)` — flag at §707.10 / §704.5d boundaries
- `SweepOrphanedInstanceIDs` — §514.2 cleanup-step sweep, hooked into CheckEnd
- `CleanupHandSize` LeftGame skip — Phase F §400.7c carveout

### 1.4 Lineage edges (Phases A + Huginn 2.0)

- `Permanent.ExiledByMe` — source-permanent lineage edge (vs global table)
- `LinkedExile` — exile-with-source-tracking (Banisher Priest / Prison Barricade / Knowledge Pool family)
- Huginn 2.0 reverse index — `OracleID → []InstanceID` + per-edge provenance lineage
- Engine `no_op_loop` hook → Huginn `infinite_cycle` event

### 1.5 Companion subsystems

- **Counter DB (Phases 1-8):** 252 counter types catalogued, §122 §706 §704.5q
  compliant, doubling support, energy/XP/seat-carveout
- **Forensics CLI:** `cmd/hexdek-forensics/` — mint-bypass tracer, pattern clustering,
  CI integration with baseline-diff guard
- **Muninn 2.0:** `--silent-inert-audit` automates Genesis-Chamber-style ctx-key
  bug detection
- **Heimdall:** DeckFingerprint + ClassifyArchetype + InteractionMatrix + ThreatResolution
  + game-replay narrative summarization
- **Heimdall → Hat closed self-tune loop:** observation → attribution → feedback → applier

### 1.6 Known substrate gaps (open primitives queue for Phase H+)

Surfaced by Phase 1 + structural analysis. These are the unfilled slots in the
substrate; toss-backs may convert to refactor candidates if any of these ship.

- `mint_returned_with_provenance_filter` — Sun Titan / Karmic Guide / Reveillark
  return-from-graveyard with CMC + type filter (handlers exist; substrate could canonicalize)
- `phase_scoped_static_with_event_arm` — paradigm-family exile-on-resolve + return-on-event
  with InstanceID-tracked lineage (currently uses sideband `gs.ParadigmExile` slice)
- `attached_aura_provenance_edge` — aura's `AttachedTo` becomes a lineage edge rather
  than a runtime pointer (would close AttachmentConsistency at scale)
- `library_top_provenance` — Yuriko-style "card revealed from top" as a lineage edge
- `paradigm_exile_reflection` (proposed Phase H primary) — explicit primitive for
  paradigm-shape exile-and-return-on-resolve, replacing `gs.ParadigmExile` sideband

---

## §2 — Audited Cards (the 4-Way Classification)

Every card with a per_card handler OR a known issue gets a row. Classifications:

| Symbol | Meaning |
|---|---|
| ✅ | Card-issue is identity/lineage-shaped, current solution is InstanceID-canonical |
| 🔧 | Card-issue is identity/lineage-shaped, current solution is sideband / ad-hoc / non-canonical → **refactor candidate** |
| ⚠ | Card-issue needs engine state beyond InstanceID — substrate gap noted, toss-back to §3 |
| ❌ | Card-issue isn't identity-shaped at all → InstanceID is the wrong lens, toss-back to §3 with that classification |

Each row carries:

| Column | Purpose |
|---|---|
| Card | name |
| Issue source | Loki / Goldilocks / Muninn / per_card test / manual / observed-in-fuzz |
| Classification | ✅ / 🔧 / ⚠ / ❌ |
| Current solution | the live code path (per_card file:line, engine helper, sideband state, etc.) |
| Canonical alternative | if 🔧, the substrate primitive(s) the card SHOULD route through |
| CR citations | which §X.Y rules the canonical solution invokes |
| Tests pinning current behavior | regression baseline that proves the refactor is behavior-preserving |
| Observation trail | Loki signature / Muninn line / forensics trace — concrete evidence |

Estimated rows: ~1200 (matching the per_card handler count from §1.4 of the
engine event registry). Will live as a CSV companion file (`monday-qc-cards.csv`)
referenced from this doc.

---

## §3 — The Toss-Back Catalogue (⚠ + ❌ Crab Pot)

Every ⚠ and ❌ row from §2 expanded with reasoning. Six columns per row:

| Column | Purpose |
|---|---|
| Card / issue | what it is |
| Classification | ⚠ vs ❌ |
| Specific reason | not generic — *which* InstanceID primitive falls short and *why* |
| What it would need | the missing primitive OR alternative framing OR "unknown" |
| Human-eyes line | date checked, signer, "still toss-back" / "wiggled in" |
| Headache score | 1-5, frequency in Loki / per_card / forensics traffic |

**Bonus invariant:** every toss-back must cite a real Loki / forensics / Goldilocks
observation trail. No speculative "I think this is weird" entries — every toss-back
is backed by actual observed engine behavior.

**Re-audit triggers (per §6):** when a new InstanceID primitive ships, the ⚠ rows
re-audit against the updated baseline. Some may flip to 🔧 (substrate now expresses
them); the ❌ rows stay parked unless the lens itself changes.

---

## §4 — Refactor Candidates (the 🔧 Pile)

Every 🔧 row from §2 lifted into a prioritized work queue. Three priority bands:

### Phase H — high-leverage / high-frequency

Cards / families where the refactor materially shrinks the engine surface OR closes
a known Phase 1 residual at the 50k bar.

Likely Phase H members (pre-audit predictions, will firm up after §2 walk):

- **Paradigm family** (Echocasting Symposium, Decorum Dissertation, Germination Practicum,
  Improvisation Capstone, Restoration Seminar) — refactor `gs.ParadigmExile` sideband
  → InstanceID + lineage edge through `paradigm_exile_reflection` primitive (new in Phase H)
- **Sun Titan / Karmic Guide / Reveillark / Sevinne's Reclamation** — return-from-graveyard
  family with CMC/type filter, refactor to canonical `mint_returned_with_provenance_filter`
- **Hostage Taker / Banisher Priest / Brago / Roon / Conjurer's Closet** — exile-until-leaves
  family, audit which already route through `LinkedExile` (post-#800/#817) vs which still
  have per_card sideband state

### Phase I — moderate-leverage / family-pattern consolidation

Card families that share a refactor pattern but are individually low-frequency. Bundling
them into one primitive PR closes ~5-15 per_card handlers at once.

### Phase J — opportunistic / cleanup

Single-card refactors that pass cost/benefit, can ship in stub-batch follow-ups.

Each candidate carries:
- Estimated LOC reduction (per_card lines removed - substrate lines added)
- Risk class (LOW / MED / HIGH — anchored by test coverage)
- Dependency (which substrate primitive must land first)
- Phase 1 residual closure (does this fix one of the 50k tail signatures? Y/N)

---

## §5 — Headache + Cure Rankings

Two ranked tables.

### 5.1 Headache rankings (the ⚠/❌ scorecard)

Toss-back cards ranked by Headache Score (1-5). Top of list = research priorities
for new InstanceID primitives. Bottom of list = cards we just accept the per_card
handler complexity for, because they're rare-occurrence outliers.

Format:
- Card / pattern
- Headache score
- Why it's a headache (engine bandwidth cost, audit-complexity cost, surprise-bug cost)
- Substrate research direction (what kind of new primitive would help)

### 5.2 Cure clarity rankings (the ✅/🔧 scorecard)

Cards / families ranked by how cleanly the substrate already (or could) solve them.
Top of list = the substrate's "showcase wins" — Genesis Chamber (#815), Prison Barricade
(#800), Knowledge Pool (#817), the paradigm Krark fix (Resolved 2026-05-19), Aziza
(#873/#889 Phase G). These are also the testimony cases for "InstanceID is doing
the job it was designed for."

---

## §6 — Re-Audit Triggers

The charter isn't a one-shot — it's a living catalogue. Re-audit cadence:

- **Every new InstanceID primitive ship** — Phase H/I/J each trigger a re-pass of §3
  (toss-backs may flip to 🔧 against the updated §1 baseline)
- **Every new Loki seed-dispersion finding** — if a card-family signature surfaces in
  a 50k+ sweep that wasn't on the catalogue, add the row
- **Every quarter** — sanity sweep, even if no new primitive shipped, in case substrate
  understanding evolved (the "wiggle the peg" pass)
- **Every CR errata** — if WotC changes a rule we depend on, re-audit affected card rows

Each re-audit produces a delta log appended to the doc (not a rewrite), so the trail
is preserved.

---

## §7 — The CVS Receipt of Cards (Accepted Per-Card Handlers)

The big one. Line per card, no consolidation, every single handler in
`internal/gameengine/per_card/*.go` gets a row.

| Column | Purpose |
|---|---|
| Card | literal handler subject |
| Handler file | `gen_*.go` vs `custom_*.go` vs `batch_*.go` (the latter is itself a smell — batches should be auditable) |
| LOC | actual debt this card costs |
| Substrate fit attempted? | yes/no — was an InstanceID / Counter DB / Replacement primitive route considered before going custom |
| Substrate-fit rejection reason | if substrate was considered + rejected, WHY (per §3 schema) |
| Counter primitive used | Counter DB vs custom sideband — if custom, flag as future-Counter-DB-candidate |
| Tests pinning behavior | count of per_card regression tests, baseline pin |
| Loki / Muninn observation trail | observed firing pattern (always / sometimes / never), silent-no-op check |

**Reading the receipt:**
- ✅ rows — handler is genuinely necessary (Unfinity weird, silver-border one-offs, card-specific mechanics with no substrate generalization)
- 🔄 rows — handler ships today but should be refactored to a substrate primitive as substrate matures (same pool as §4, indexed by card not by issue)
- 📊 family-pattern subsection — cards sharing a pattern (Sun Titan-shape, Hostage Taker-shape, paradigm-shape) get tagged into pattern sub-tables

**Why CVS-receipt-long not summary:** the whole point is to catch the per_card sprawl
tax before it becomes invisible. A summary table makes 1200 OnTrigger registrations
look like "manageable surface." A line-per-card receipt makes it visible: every line
is ongoing maintenance against substrate evolution.

**Cross-reference with §3 (Toss-Back Catalogue):** §3 lists "tried substrate, didn't
fit, here's why." §7 lists "shipped custom handlers, here's the inventory." Cross-ref
surfaces the highest-leverage refactor candidates: rows on §7 that AREN'T on §3 =
"shipped a custom handler without trying substrate" = strongest refactor candidates.

Estimated row count: ~1200. Will live as a CSV companion (`monday-qc-receipt.csv`)
because Discord-paste this section is infeasible.

---

## Scope sizing (per his ask)

Conservative estimate per section, assuming the standing-night swarm pattern:

| Section | Workers | Hours | Notes |
|---|---|---|---|
| §1 Substrate baseline | 1 | 1-2h | Mostly reading + summarizing existing docs (engine event registry, Phase E/F/G PR bodies, Counter DB Phase 1-8 plan) — minimal investigation |
| §2 Audited cards (4-way classify) | 6-8 | 4-6h each = ~30h total | THE BIG ONE — per_card walk-through, 150-200 cards per worker, light triage per card to slot into ✅/🔧/⚠/❌. Cross-worker calibration meeting at hour 2. |
| §3 Toss-back catalogue | 2-3 | 2-3h | Expansion of §2 ⚠/❌ rows — workers do the §3 detail rows for their §2 partition |
| §4 Refactor candidates priority | 2 | 2h | Workers cross-cut §2 🔧 rows by phase, dependency-graph the substrate primitives |
| §5 Headache + Cure rankings | 1 | 1-2h | Score + sort, no new investigation |
| §6 Re-audit trigger doc | 1 | 0.5h | Just write the trigger rules |
| §7 CVS Receipt | 4-5 | 3-4h each = ~15h total | Receipt generation can be partly automated (LOC + counter primitives via grep), but the substrate-fit-attempted / rejection-reason columns need per-card eyeballs |
| Charter doc consolidation | 1 | 1h | Bring it all together |

**Total estimate: ~50-60 worker-hours, ~10-12 hours wall-clock if parallelized
across 8 workers.** Tighter than the typical night swarm because it's mostly
read-and-classify rather than read-and-implement.

**Recommended window:** Sunday late night → Monday morning, off-hours bandwidth.
Or weekday off-hours over 2-3 sessions if you want to spread the read-load.

**Output artifacts when done:**
- This doc (`docs/monday-qc-charter-r60.md`) — the meta-charter
- `monday-qc-cards.csv` — §2 main table (1200 rows × 8 cols)
- `monday-qc-receipt.csv` — §7 receipt (1200 rows × 8 cols, partly overlapping)
- `monday-qc-tossbacks.md` — §3 expanded reasoning (only ⚠/❌ rows, narrative form)
- `monday-qc-refactor-queue.md` — §4 prioritized Phase H/I/J work queue
- `monday-qc-rankings.md` — §5 headache + cure rankings
- `monday-qc-substrate-baseline.md` — §1 lifted into its own ref doc for future re-audits

---

## Charter execution protocol

When you pick the window + say go:

1. **Worker dispatch templates** (one per section) get loaded into the cron's
   dispatch queue
2. **Calibration sync** at hour 2 of §2 — workers cross-check their classifications
   on a shared 20-card sample to align on the ✅/🔧/⚠/❌ boundary
3. **Per-section staging PRs** — each section ships as its own PR for incremental
   review (don't wait until everything's done)
4. **Final charter PR** — pulls all the staging artifacts into the meta-doc + cross-links

When the run completes, you have a navigable, queryable, future-proof catalogue
of the entire per_card handler ecosystem evaluated against the InstanceID substrate,
plus a prioritized work queue for Phase H/I/J substrate consolidation.

🦀🧾☕
