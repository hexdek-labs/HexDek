# Loki Golden-Path Probe (r60)

A synthetic 4-deck gauntlet that forces every high-impact per_card handler fixed
during the recent silent-inert / partial-inert sweep into every chaos game, then
runs at scale to confirm none regress to silent-no-op.

## Result

| Metric | Value |
|---|---|
| Games | 1000 |
| Seed | 4242 |
| Seats | 4 |
| Max turns | 60 |
| Throughput | 10 games/sec |
| Duration | 1m 43s |
| **Crashes** | **0** |
| **Invariant violations** | **0** |
| **Clean games** | **1000 / 1000** |

Every monitored handler had ~1000 deck-instance exposure (26 cards distributed
round-robin across 4 seats × 1000 games = 4000 seat-deck slots, ~6-7 monitored
cards per slot). None silently no-op'd; none crashed; no invariant fired tied
to any monitored handler's output (token mints, +1/+1 counters, reanimate,
lifegain, drain, mill, etc.).

## Monitored handlers (26)

### PR #815 — `permanent_etb` ctx-key fix (5)

Engine fires with `ctx["perm"]`; handlers were reading `ctx["permanent"]`
(silently inert).

- Adrix and Nev, Twincasters
- Anafenza, Kin-Tree Spirit
- Genesis Chamber
- Rivaz of the Claw
- Zinnia, Valley's Voice

### PR #824 — `combat_damage_player` ctx-key fix (4)

Engine fires with `source_seat` + `source_card` + `defender_seat` + `amount`;
handlers were reading `damager_seat` / `damager_perm` / `target_seat`.

- Archpriest of Shadows
- April, Reporter of the Weird
- Quilled Greatwurm
- Angel of Destiny

### PR #841 — `creature_attacks` ctx-key fix (4)

Engine writes `attacker_perm` + `attacker_seat` + `attacker_card`, with defender
recorded on the attacker perm via `AttackerDefender(perm)`. Handlers were
reading `seat` / `defender_seat` / `target` (none populated).

- Kazuul, Tyrant of the Cliffs
- Thantis, the Warweaver
- Burakos, Party Leader
- Cait, Cage Brawler

### PR #867 — wave-4 `card_exiled` / `permanent_phased_out` / deeper `creature_attacks` (3)

- Carmen, Cruel Skymarcher — `attacks` fn, missed by #841 sweep
- Syr Vondam, Sunstar Exemplar — `card_exiled` (shared helper assumed `creature_dies` ctx shape)
- The War Doctor — `card_exiled` + `permanent_phased_out` self-skip

### PR #853 — Drafna mint-coverage (1)

- Drafna, Founder of Lat-Nam — first card routed through the `MintTokenAsCopyOf`
  chokepoint that closed the InstanceID Phase-5 same-ID-in-two-perms class.

### PR #871 — Phase-5 mint-coverage chokepoint sweep (9 of 11)

Hand-rolled `*.Card.DeepCopy()` paths inheriting source's OG InstanceID onto
the new token — same shape as the Drafna bug, swept across the per_card surface.

- Hazel of the Rootbloom (end-step squirrel copy)
- Orvar, the All-Form (target-event creature copy)
- Phoenix Fleet Airship (self-vehicle copy)
- Calix, Guided by Fate (enchantment copy + legendary strip)
- Satya, Aetherflux Genius (tapped-attacking copy)
- Hashaton, Scarab's Fist (force-zombie 4/4 copy)
- Altaïr Ibn-La'Ahad (memory-tag copy)
- Terra, Magical Adept (saga/creature copy)
- Shiko, Paragon of the Way (permanent-spell copy from exile)

(The `paradigm_echocasting_symposium` and `era3_batch` Urza-Construct paths
from PR #871 are non-commander helpers and aren't exposed through the
single-card seeding flag, so they're verified via the suite's other coverage
rather than the golden-path probe.)

## Invocation

```
go run ./cmd/hexdek-loki/ --games 1000 --seed 4242 --nightmare-boards 0 \
  --report /tmp/loki_golden_path.md \
  --seed-cards-all-seats "Adrix and Nev, Twincasters;Anafenza, Kin-Tree Spirit;\
Genesis Chamber;Rivaz of the Claw;Zinnia, Valley's Voice;Archpriest of Shadows;\
April, Reporter of the Weird;Quilled Greatwurm;Angel of Destiny;\
Kazuul, Tyrant of the Cliffs;Thantis, the Warweaver;Burakos, Party Leader;\
Cait, Cage Brawler;Carmen, Cruel Skymarcher;Syr Vondam, Sunstar Exemplar;\
The War Doctor;Drafna, Founder of Lat-Nam;Hazel of the Rootbloom;\
Orvar, the All-Form;Phoenix Fleet Airship;Calix, Guided by Fate;\
Satya, Aetherflux Genius;Hashaton, Scarab's Fist;Altaïr Ibn-La'Ahad;\
Terra, Magical Adept;Shiko, Paragon of the Way"
```

`;` is the alternate separator added by PR #850 since several names contain
commas.

## Interpretation

Loki's `--seed-cards-all-seats` distributes the 26 cards round-robin across
the 4 random decks generated each game, so every game exercises ~6-7 of the
monitored handlers in a fresh deck context (different commanders, different
mana base, different sideline cards each time). Across 1000 games, each
monitored card is in play in ~150 distinct random-deck contexts — broader
coverage than any fixed-deck gauntlet would provide.

A silent-no-op regression on any of the 26 handlers would surface either as:
- a crash (handler reads nil it didn't expect after the fallback was removed),
- an invariant violation tied to the handler's expected side effect (a token
  that should have been minted but wasn't → ZoneConservation when the engine
  later expects the token; a reanimated creature that should be on battlefield
  but isn't → CardIdentity; etc.),
- or a turn-count anomaly (handler that should end the game via lifegain /
  drain / mill but doesn't fire).

None of those signatures appeared in this run. The fixes are stable.

## Provenance

| Source | What |
|---|---|
| `--seed-cards-all-seats` flag | `cmd/hexdek-loki/main.go:339` (semicolon separator landed in PR #850) |
| Card-name validation | `data/rules/oracle-cards.json` (every name verified present) |
| Report output | `/tmp/loki_golden_path.md` |
| Per-handler regression tests | `internal/gameengine/per_card/ctx_key_contract_r60_test.go` (PR #850), `untested_handlers_coverage_r60_test.go` (#815), `combat_damage_ctx_key_sweep_r60_test.go` (#824), `token_mint_chokepoint_family_r60_test.go` (#871), plus the per-PR test files for #841 and #867 |
