# Stub Hunt — per_card (R51 batch J, cheap CMC 1-2)

Scope: per_card stub ports filtered to CMC 1-2 ("cheap" / lower-CMC)
commanders and key spells. Continuation of the per_card stub-hunt
deep-dive series (batches A through G shipped on main; batch J is
focused on the cheapest cards that prior batches didn't touch).

## Method

1. **Build the candidate pool**: enumerate every `gen_*.go` in
   `internal/gameengine/per_card/` whose card has no `custom_*.go`
   override (the registry would route to the custom and the gen
   breadcrumb is dead). 124 candidates.
2. **Query CMC** for each candidate via the `hexdek.dev` oracle
   endpoint (`/api/oracle/card/{name}`). Distribution skewed toward
   3-5 CMC (commanders); only 15 candidates land at CMC ≤ 2.
3. **Drop already-real ports**: 6 of those 15 are already shipped in
   prior batches (June Bounty Hunter, Karlov, Kwain, Rat King, The
   Wandering Minstrel, Wilson) or were ported in earlier R-batches
   (Anafenza Kin-Tree, Bristly Bill, Ivy, Kudo, Rosnakht, Skullbriar).
4. **Survey remaining candidates** for tractable `emitPartial`
   breadcrumbs and pure stubs.

## The 3 ports — cheap CMC 1-2

Initial scope was 10. After filtering out cards covered by batches A-G
and ones whose remaining gaps are engine-deep, the pool collapsed to 4.
During merge, **Kolodin, Triumph Caster** was discovered to already
have a `custom_kolodin_triumph_caster.go` shipped by a concurrent
batch H — its kw:haste anthem is already wired via trigger-driven
flag stamping. Batch J's layer-6 CE port was dropped on merge to
avoid duplication. Final scope: 3 ports.

| # | Card | CMC | Category | What ships | Precedent |
|---|------|----:|----------|------------|-----------|
| 1 | **Akiri, Line-Slinger** | 2 (RW) | Static buff | Promote ETB-snapshot + per-artifact-ETB recount to a real layer-7c continuous effect. ApplyFn counts the controller's artifacts at evaluation time and writes +N to chars.Power, so artifact LTBs (which the prior implementation didn't react to) and any newly-entering artifact pick up the buff immediately. SourcePerm = Akiri, so the grant tears down via `UnregisterContinuousEffectsForPermanent` on LTB. | Kudo's layer-7b base-PT SET |
| 2 | **Aziza, Mage Tower Captain** | 2 (UW) | Spell-copy | Pure-stub port. OnTrigger("instant_or_sorcery_cast") gated on `caster_seat == perm.Controller`; if 3 untapped friendly creatures (excluding Aziza) are available, tap them and push a `StackItem` copy with `IsCopy = true` (CR §707.2 / §707.10). Targets inherited from the originating stack item; "choose new targets for copy" stays partial (engine doesn't dispatch target-reselection for copies). | Ivy, Gleeful Spellthief (R46) |
| 3 | **Arguel's Blood Fast** | 2 (1B) | DFC transform | Wire the low-life upkeep transform. The R46 implementation emitted a "transform_to_temple_of_aclazotz_engine_call_unimplemented" partial; the engine actually exposes `gameengine.TransformPermanent` (Cecil uses it for the darkness threshold), so we just call it when life ≤ 5 and `!perm.Transformed`. Idempotent on already-transformed permanents. | Cecil, Dark Knight (darkness threshold) |

## Tests

`internal/gameengine/per_card/percard_stub_batch_r51_j_test.go` — 10
new tests pin happy paths and negative-case guards. Full
`go test ./...` stays green across cmd/, internal/gameengine
(+ per_card), internal/hat, internal/heimdall, internal/hexapi,
internal/muninn, internal/tournament, etc.

## Out-of-scope (intentionally skipped)

- **Cards already real-ported** — June Bounty Hunter (B), Kwain (B),
  Rat King (B), The Wandering Minstrel (B), Wilson (E), Karlov,
  Skullbriar, Tajic, Anafenza Kin-Tree, Bristly Bill, Ivy (R46), Kudo
  (R46), Rosnakht (R42).
- **The Reality Chip** — remaining gap (cast spells from top of library
  while attached) needs a cast-legality scanner integration that lives
  engine-side, not per_card.
- **Cecil, Dark Knight** — already substantively ported; no tractable
  gap remaining at the per_card layer.
- **Cloud, Midgar Mercenary** — trigger-doubling-when-equipped requires
  a trigger-dispatch hook in the engine; per_card breadcrumb correctly
  identifies the engine-side surface.
- **Angel's Grace** — full `would_lose_game` + `would_take_damage`-to-1
  replacement pipeline is more substantial than a batch-J one-shot;
  the Ad Nauseam integration (the primary cEDH use case) is already
  wired via the `angels_grace_eot_seat_N` flag.
- **Bloodchief Ascension** — substantively ported; remaining
  `emitPartial` breadcrumbs are about "may" choice surfaces, not
  missing logic.
- **Bruce Banner** — fully implemented.
- **Aphelia, Viper Whisperer** — attacks-trigger snake token works; the
  unregistered `{4}{B}` tribal-damage-halving activation is delayed-
  continuous-trigger territory and explicitly out of scope per the
  existing file comment.
