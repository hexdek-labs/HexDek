# InstanceID property fuzz — r60

## What it covers

`internal/gameengine/instanceid_property_fuzz_r60_test.go` drives the raw
InstanceID primitives (mint, mark-ceased, record-merge, zone moves) in
random sequences and asserts the five InstanceID invariants after every
step. It **bypasses the engine entirely** — no spell resolution, no
trigger cascades, no phase machinery. Just the lifecycle primitives that
real per-card handlers and engine LTB paths call.

The reasoning: the existing `instanceid_phase*_test.go` files cover
specific scenarios (Mutate stack, Meld unmerge, Phase 5 token-as-copy,
Phase F spell-copy). Loki covers full-game integration. Neither one
probes the primitive space DENSELY. The property fuzz fills that gap:
~60 random ops per sequence × 10,000 sequences = ~600k op applications
× 5 invariants per op = ~3M invariant evaluations per run.

## The five invariants

After every op, the fuzz runs the following 5-invariant battery:

- **(A) Uniqueness across zones** — `checkCardIdentity(gs)`: no
  non-empty InstanceID appears in two zones simultaneously. CR §400.7
  one-object-per-ID semantics.
- **(B) Strict census** — `checkZoneConservationByInstanceID(gs)` with
  `gs.Flags["instanceid_strict_census"]=1`: present set (every InstanceID
  in every zone, including MergedCardPtrs) must equal Minted − Ceased
  for OG/TK/CP provenance (AB excluded). Fabrications and disappearances
  both fire here.
- **(C) Lineage consistency** — every Card with non-empty
  `SourceInstanceID` and provenance ∈ {CP, TK} must have that source ID
  recorded in `gs.MintedInstanceIDs`. Catches forged lineage.
- **(D) ExiledByMe pointer integrity** — every Permanent's `ExiledByMe`
  entry must correspond to a Card in some seat's Exile zone OR appear in
  `gs.CeasedInstanceIDs`. Catches stale source-held linkage references
  (a leak shape similar to the §406.7 LinkedExile bugs Loki used to
  surface pre-Phase-3).
- **(E) Enabler history monotonic** — for every Card with non-empty
  `EnablerHistory`, the last entry must equal `EnablerInstanceID`.
  Catches malformed Phase 6 lineage where the history append got out of
  sync with the current enabler field.

## Op space

Weighted-random selection over 10 op kinds:

| Kind | Weight | What it exercises |
|------|-------:|-------------------|
| `mint_og` | 30 | `MintOGInstanceID` → library |
| `mint_token` | 15 | `MintTokenInstanceID` → battlefield |
| `mint_token_copy` | 15 | `MintTokenAsCopyOf` (Phase 5 chokepoint) |
| `mint_spell_copy` | 10 | `MintSpellCopy` (Phase F chokepoint) → §707.10 cease |
| `move` | 20 | All 5-zone × 5-zone transitions (with bf wrapping/unwrapping) |
| `exile_with_linkage` | 5 | Source-held `ExiledByMe` stamp (Phase 3) |
| `cease_battlefield_leave` | 2 | LTB cessation for tokens + Phase 8 unmerge |
| `cease_explicit` | 1 | `MarkInstanceIDCeased` from any zone, scrubs MergedCardPtrs |
| `mutate_merge` | 1 | `RecordMutateMerge` two BF perms |
| `meld_merge` | 1 | `RecordMeldMergeWithCards` two BF perms |

Each sequence runs 20-100 ops. Ops with unmet preconditions (e.g. a
mutate_merge when fewer than 2 perms exist) generate a different op kind
instead of crashing.

## Running it

```bash
# Full run (10k sequences, ~8s on a modern laptop):
go test ./internal/gameengine/ -run TestInstanceID_PropertyFuzz_R60 -count=1

# Quick dev iteration (100 sequences):
FUZZ_SEQUENCES=100 go test ./internal/gameengine/ -run TestInstanceID_PropertyFuzz_R60 -count=1 -v

# Short-mode (auto-caps at 1000 sequences):
go test ./internal/gameengine/ -run TestInstanceID_PropertyFuzz_R60 -short -count=1

# Cross-seed sweep — a different base seed gives a fully different
# sequence space:
FUZZ_BASE_SEED=42 go test ./internal/gameengine/ -run TestInstanceID_PropertyFuzz_R60 -count=1

# Reproduce a specific failed sequence (the failure dump prints the
# exact command):
FUZZ_REPRO_SEED=<seed> go test ./internal/gameengine/ -run TestInstanceID_PropertyFuzz_R60_Repro -count=1 -v
```

## Reading a failure

When an invariant fires, the fuzz dumps a structured failure block to
stderr. Example:

```
=== INSTANCEID PROPERTY FUZZ FAILURE ===
  base_seed=1  sequence=207  op_index=19  derived_seed=1000207
  invariant=B
  error=[B] ZoneConservation: InstanceID "h2TKVR200001" (Fuzz_TK_3) present in a zone but not in (Minted - Ceased) — fabrication or stale ceased entry
  op_history=
    [0] mint_og seat=1 name=Fuzz_OG_0
    [1] mint_og seat=2 name=Fuzz_OG_1
    ...
    [19] cease_explicit card_idx=7
  state_summary=
    seats=4  total_indexed=15  minted=15  ceased=5
    seat0={library:0 hand:0 gy:0 exile:0 bf:1 cmd:0}
    ...
    offending_id=h2TKVR200001
      minted=true ceased=true
      tracker_zone="ceased" owner=2 types=[token creature]
    phantom_bf id=h2TKVR200001 name="Fuzz_TK_3" owner=2
=== END FAILURE ===
  reproduce: FUZZ_REPRO_SEED=1000207 go test ./internal/gameengine/ -run TestInstanceID_PropertyFuzz_R60_Repro -count=1 -v
```

Fields:
- `invariant=<A..E>` — which of the 5 invariants fired (see "The five
  invariants" above).
- `op_history` — every op applied up to and including the failing op,
  in order. Replay-able via the repro seed.
- `state_summary` — per-seat zone counts at failure time.
- `offending_id` — when the error message contains an InstanceID, the
  dump cross-references it against the fuzz tracker, `MintedInstanceIDs`,
  and `CeasedInstanceIDs`.
- `phantom_bf` — IDs the fuzz tracker thinks are on the battlefield but
  which aren't actually in any seat's `Battlefield` slice OR any perm's
  `MergedCardPtrs`. Common signal of a missing cease or unmerge in a
  primitive.

## Out of scope

The fuzz deliberately does NOT exercise:
- Spell resolution, trigger cascades, or phase ticking — those are
  covered by the existing engine integration tests (`instanceid_phase*_test.go`,
  `instanceid_orphan_sweep_test.go`, `instanceid_gap_walk_test.go`) and by
  the golden-path Loki probe in `docs/loki-golden-path-r60.md`.
- Real card oracle parsing — there is no AST execution.
- Combat, lifecycle hooks, or SBAs.
- Multi-game ELO/tournament concerns.

The fuzz is a TARGETED probe of the InstanceID primitives. When it fires,
the bug is in `instanceid_mint.go`, `instanceid_phase5.go`,
`instanceid_phase8.go`, or `invariants.go` — not in spell handlers.
