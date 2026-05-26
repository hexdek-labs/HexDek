# Per-Card Registry Audit — R60 (Versailles Phase 1B)

**Date:** 2026-05-25
**Branch:** `dev/audit-percard-registry-r60` (built from `origin/main`)
**Scope:** `internal/gameengine/per_card/` end-to-end vs engine call sites

> **Findings only — no code changes shipped under this PR.** All
> corrective work is enumerated at the bottom as "Recommended
> followups" so the gaps stay visible and can be triaged before any
> handler is deleted, renamed, or rewired.

## Headline

| Surface                                                    |  Count |
| ---------------------------------------------------------- | -----: |
| `internal/gameengine/per_card/` non-test `.go` files       |    947 |
| Total `OnTrigger` call sites (non-comment, non-test)       |  1,028 |
| Total `OnETB` call sites                                   |    505 |
| Total `OnResolve` / `OnActivated` / `OnCast` call sites    | 77 / 253 / 11 |
| Distinct `(card, event)` `OnTrigger` pairs                 |  1,022 |
| Distinct cards with `OnETB`                                |    494 |
| Distinct registered trigger events (canonical, post-alias) |     55 |
| Distinct engine-emitted trigger events                     |     85 |
| **Genuine orphan trigger events** (registered, never fired) |   **3** |
| **Genuine event-name mismatches** (broken alias chain)     |   **1 cluster, 5 cards** |
| **Duplicate registrations** across files                   |    **17** |
| **`Register*` 404 risk** (called but undefined)            |   **0** |
| **Orphan `Register*` defs** (defined but never called)     |   **0** (excl. `registerDefaults` entry point) |

## 1. Event-name mismatches (404 risk by alias miscarriage)

The registry normalizes event names at both **registration time** (registry.go:307
`gameengine.NormalizeEventSingle`) and **fire time** (registry.go:470). As long as
the alias map in `internal/gameengine/event_aliases.go` resolves both sides to
the same canonical, the trigger fires.

The May-6 `begin_combat` / `combat_begin` issue from the issue log was exactly
this kind of mismatch — caught by the alias-table audit done that day.
**Sweep of all aliased-but-broken targets found one similar cluster still live:**

### ETB alias mismatch — 5 cards affected

`event_aliases.go:142-144` maps three friendly aliases to `etb`:

```go
"creature_enters_battlefield":   {"etb"},
"land_entered_battlefield":      {"etb"},
"permanent_entered_battlefield": {"etb"},
```

But the engine **never fires `etb` via `FireCardTrigger`**. Its only
`FireCardTrigger` ETB site emits `permanent_etb`:

```
internal/gameengine/etb_dispatch.go:94:  FireCardTrigger(gs, "permanent_etb", ...)
internal/gameengine/stack.go:1535:        FireCardTrigger(gs, "permanent_etb", ...)
```

The alias resolves both registration and fire to **different canonical strings**,
so handlers registered under the three friendly names silently never fire.

**Affected handler registrations:**

| Card | File | Registered event |
|------|------|------------------|
| Aura Shards | `internal/gameengine/per_card/game_changers.go:1120` | `creature_enters_battlefield` |
| Aura Shards | `internal/gameengine/per_card/game_changers.go:1121` | `permanent_entered_battlefield` |
| Sokka and Suki | `internal/gameengine/per_card/sokka_and_suki.go:25` | `creature_enters_battlefield` |
| Field of the Dead | `internal/gameengine/per_card/game_changers.go:1296` | `permanent_entered_battlefield` |
| Field of the Dead | `internal/gameengine/per_card/game_changers.go:1297` | `land_entered_battlefield` |

Note that the Aura Shards and Field of the Dead handlers each register
the same trigger under MULTIPLE friendly names — both paths normalize
to `etb` and both miss the engine's `permanent_etb`. The double-
registration is itself benign (both end up at the same canonical
storage key) but neither fires.

**Same shape as the May-6 `begin_combat` / `combat_begin` bug.** Two fix paths
(do not ship under this PR):

1. **Fix the alias map** — repoint the three entries from `etb` to `permanent_etb`:
   ```go
   "creature_enters_battlefield":   {"permanent_etb"},
   "land_entered_battlefield":      {"permanent_etb"},
   "permanent_entered_battlefield": {"permanent_etb"},
   ```
   Lowest-risk fix; preserves every existing handler registration.

2. **Rename handler registrations** to use `permanent_etb` directly. Larger
   diff (5 call sites) but matches the canonical convention.

Recommendation: fix path 1.

## 2. Orphan trigger events (registered, never fired)

Events whose canonical (post-`NormalizeEventSingle`) name does not match
any engine `FireCardTrigger` site. The May-6 `begin_combat` was an alias
fix and is no longer in this list; current standing orphans:

| Canonical event              | Raw registered as                | Cards affected |
|------------------------------|----------------------------------|---------------|
| `postcombat_main_controller` | `postcombat_main_controller`     | Mostly `gen_*` files — likely intended to fire from a phase hook the engine does not emit yet |
| `untap_step`                 | `untap_step`                     | 2 registrations |
| `upkeep_opponent`            | `upkeep_opponent`                | 2 registrations |

The ETB cluster above (5 cards) overlaps the canonical `etb` orphan
row — listed there separately because the root cause is the broken
alias, not a missing engine event.

Cards using these orphan events:

```
$ grep -rln 'OnTrigger.*"postcombat_main_controller"' internal/gameengine/per_card/
$ grep -rln 'OnTrigger.*"untap_step"' internal/gameengine/per_card/
$ grep -rln 'OnTrigger.*"upkeep_opponent"' internal/gameengine/per_card/
```

Run those commands locally to enumerate the affected handlers before any
remediation — they were not exhaustively listed in this report because
the engine-side fix (add the missing `FireCardTrigger` site OR alias the
event to an existing canonical like `end_step_controller` /
`untap_event` / `upkeep_controller`) is the actual lever, not the
per_card side.

## 3. Engine-emitted events with NO per_card consumer

Not strictly a bug — these are engine hooks emitted via `FireCardTrigger`
that simply have no per_card listeners yet. Recorded for product
prioritization (the events are surface area open for new card support).

29 events fall in this bucket. Highlights of mechanics with significant
card pools that could be wired:

| Event                  | Mechanic              | Notable cards lacking handler |
|------------------------|-----------------------|-------------------------------|
| `magecraft`            | STX magecraft trigger | Lorehold Apprentice, Quandrix Pledgemage, ~30+ cards |
| `mount_saddled`        | OTJ saddle keyword    | Slickshot Show-Off, ~10 mount payoffs |
| `investigate`          | Clue creator trigger  | Lonis, Cryptozoologist, Tireless Tracker |
| `inspired`             | Untap-during-untap-step trigger | KTK Inspired cycle |
| `eerie`                | DSK enchantment-ETB trigger | DSK enchantments cycle |
| `bargain_paid`         | WOE bargain | Up the Beanstalk, etc. |
| `class_level_up`       | AFR Class subtype | All Classes |
| `creature_mutated`     | IKO Mutate | Mutate decks |
| `pack_tactics_triggered` | AFR pack-tactics | AFR creatures |
| `gift_promised`/`gift_delivered` | LCI gift | Gift cycle |
| `expend`               | TFT expend | TFT cycle |
| `tribute_resolved`     | BNG tribute | BNG creatures |
| `outlaw_etb`           | OTJ outlaw | OTJ cycle |
| `beheld`               | DMU behold | Phyrexian Phyrexia cards |
| `manifest_flipped`     | manifest face-up | Manifest cards |
| `unsuspected`/`suspected` | MKM suspect | MKM cycle |
| `card_cycled`          | cycling trigger | Astral Slide, etc. |
| `visited`              | LCI room/dungeon | Dungeon Crawler |
| `creature_sacrificed`  | aristocrats finer-grained | Distinct from `permanent_sacrificed` |
| `became_solved`        | MKM case solved | Case cards |
| `becomes_defeated`     | LCI defeat | Defeat creatures |
| `becomes_renowned`     | OGW renown | Renown cycle |
| `battalion_triggered`  | GTC battalion | Boros battalion cycle |
| `untap`                | once-per-untap | Untap-step triggers |
| `draw_step`            | draw-step phase | Phase-step triggers |
| `channel_activated`    | NEO channel | Channel cycle |
| `first_crime`          | MKM first crime per turn | Crime-once payoffs |

Full list of 29 in `/tmp/normalize.py` output (regen via
`python3 /tmp/normalize.py` from this branch).

These are not bugs — they are opportunities. The engine is paying the
cost of emitting these triggers; per_card handlers can light up the
mechanic without further engine work.

### 3a. Correction (2026-05-26) — Notable-cards / mechanic-cycle false positives

The §3 table above is **strictly accurate at the EVENT level**: every
event in the "Event" column had zero per_card `OnTrigger` registrations
under that canonical name at audit commit `d54a118`. That part of the
claim was verified post-hoc by snapshotting `internal/gameengine/per_card/`
at the audit commit and grepping for `OnTrigger("...", "<event>", ...)`
— 0 hits across all 29 rows.

BUT the table's **"Notable cards"** and **"Mechanic"** columns
implicitly suggested that named cards / whole cycles were unwired,
which is **wrong** for the 6 cards below. Each is wired pre-audit via
a DIFFERENT dispatch event or surface — usually because the card is
the cycle's *causer* (fires the keyword action from a different
trigger event) rather than its *consumer*. PR #515 (outlaw payoff
wiring) tripped on this when it tried to wire Olivia under
`combat_damage_player` and discovered the handler already existed.

This sub-section enumerates the genuine false positives so future
wiring PRs don't duplicate work. Sweep methodology at the bottom.

#### Verified-wired (pre-audit) — but listed/implied as unwired

| Audit row | Card | Pre-existing handler | Why the audit missed it |
|---|---|---|---|
| `investigate` (Notable: "Lonis, Cryptozoologist, Tireless Tracker") | **Lonis, Cryptozoologist** | `OnTrigger("Lonis, Cryptozoologist", "nonland_permanent_etb", ...)` + `OnActivated(...)` in `lonis_cryptozoologist.go` | Lonis is an investigate *causer* — her trigger fires the investigate action, she doesn't *listen* on the `investigate` event. The audit row is correct that no card listens on `investigate`, but listing Lonis as the example card is misleading. |
| `outlaw_etb` (Mechanic: "OTJ outlaw" / cycle) | **Olivia, Opulent Outlaw** | `OnTrigger("Olivia, Opulent Outlaw", "combat_damage_player", ...)` in `olivia_opulent_outlaw.go` | Olivia's payoff fires from combat damage, not outlaw ETB. Her oracle isn't an `outlaw_etb` listener at all — she's an OTJ outlaw payoff via a different event. |
| `outlaw_etb` (Mechanic: "OTJ outlaw" / cycle) | **Vihaan, Goldwaker** | `OnTrigger("Vihaan, Goldwaker", "combat_begin", ...)` in `vihaan_goldwaker.go` | Vihaan's payoff fires at beginning of combat. Same shape as Olivia — an OTJ outlaw card whose handler dispatches off a non-outlaw-ETB event. |
| `outlaw_etb` (Mechanic: "OTJ outlaw" / cycle) | **Laughing Jasper Flint** | `OnTrigger("Laughing Jasper Flint", "upkeep_controller", ...)` in `laughing_jasper_flint.go` | Upkeep-triggered card, not an ETB listener. |
| `magecraft` (Mechanic: "STX magecraft trigger") | **Veyran, Voice of Duality** | `OnTrigger("Veyran, Voice of Duality", "instant_or_sorcery_cast", ...)` in `custom_veyran_voice_of_duality.go` | Veyran is a magecraft *causer* / amplifier on i/s casts; her handler observes the cast directly rather than via the canonical `magecraft` event. |
| `creature_mutated` (Mechanic: "IKO Mutate" / Mutate decks) | **Illuna, Apex of Wishes** | `OnETB("Illuna, Apex of Wishes", ...)` in `illuna_apex_of_wishes.go` | Illuna dispatches via an `OnETB` snowflake hook (not `OnTrigger("...", "creature_mutated", ...)`) — the engine treats her mutate-trigger as part of the ETB cascade. |

**Recommended phrasing for future audit revisions:** keep the §3 table
strictly event-keyed and stop putting card names in the "Notable
cards" column. The CARD-level question ("is the famous card from this
cycle wired?") needs a separate sweep that checks ALL dispatch
surfaces (`OnTrigger` on any event, `OnETB`, `OnResolve`, `OnCast`,
`OnActivated`) rather than just the named event. The grep that
produced the §3 list only looked at one of those five surfaces.

#### Mechanics confirmed STILL unwired pre-audit (sample)

For each row in §3, I also sampled the broader cycle to confirm the
mechanic was genuinely unwired beyond the named example. Pre-audit
sweep (per `git show d54a118:<file>` × all 947 per_card files in the
audit tree) found ZERO pre-existing handlers for:

- AFR Class cycle (sampled 19: Wizard / Warlock / Artificer / Monk /
  Sorcerer / Barbarian / Bard / Rogue / Fighter Classes + Talents +
  Cool but Rude, Ninja Teen, Intermediate Chirography, Party Dude)
- OTJ saddle cycle (sampled 7: Slickshot Show-Off, Calamity, Stubborn
  Burrowfiend, Fortune, Lagorin, Akul, Goldvein Pick)
- DSK eerie cycle (sampled 7: Victor's Seneschal, Fear of Infinity /
  Sleep Paralysis, Ghostly Dancers, Mothlight Processionist,
  Unwilling Vessel, Cult Healer)
- WOE bargain cycle (sampled 6: Beseech the Mirror, Talion's
  Throneguard, Hamlet Glutton, Realm-Scorcher Hellkite, Agatha's
  Champion, Dunbarrow Revivalist)
- LCI gift cycle (sampled 5: Octomancer, Starforged Sword, Coiling
  Rebirth, Cruelclaw's Heist, Perch Protection)
- IKO mutate beyond Illuna (sampled 13)
- STX magecraft beyond Veyran (sampled 17)

These cycles ARE genuine wiring opportunities and have been
progressively wired by PRs #483 (magecraft), #488 (mount/saddle),
#490 (investigate), #493 (eerie), #496 (bargain), #503 (class level-
up), #507 (creature_mutated), #510 (gift), #515 (outlaw payoffs).

#### Sweep methodology (reproducer)

```bash
# 1. Enumerate non-test files that existed at audit commit.
git ls-tree -r --name-only d54a118 internal/gameengine/per_card/ \
  | grep -v _test.go > /tmp/audit_files.txt

# 2. Dump full pre-audit per_card source into one searchable blob.
while read -r f; do
  git show "d54a118:$f" 2>/dev/null
done < /tmp/audit_files.txt > /tmp/audit_snapshot.txt

# 3. For each card in question, check ALL FIVE dispatch surfaces.
grep -E "(OnTrigger|OnETB|OnResolve|OnCast|OnActivated)\(\"<card>\"" \
  /tmp/audit_snapshot.txt
```

Empty grep result = genuinely unwired at audit time. Any hit = the
audit's claim is wrong (or at least misleading) for that card.

## 4. Duplicate registrations

Same `(card, event)` tuple registered twice. In the registry's slice-
of-handlers model, both fire on the event — sometimes intentional
(multi-handler decomposition, e.g. Urabrask's `urabraskUpkeepImpulse`
+ `urabraskRegisterOppDrawReplacement`), sometimes accidental (the
common `custom_*.go` + `gen_*.go` shadow-pair pattern from the gen-to-
custom upgrade pipeline that left both files registering).

**6 duplicate `OnTrigger` pairs:**

| Card / Event | Files | Likely cause |
|---|---|---|
| Urabrask, Heretic Praetor / `upkeep` | `gen_urabrask_heretic_praetor.go:31,37` | **Intentional** — two distinct handlers documented in source |
| Toxrill, the Corrosive / `end_step` | `custom_toxrill_the_corrosive.go`, `gen_toxrill_the_corrosive.go` | custom+gen shadow pair |
| Tovolar, Dire Overlord / `upkeep_controller` | `custom_tovolar_dire_overlord.go`, `tovolar_dire_overlord.go` | custom+gen shadow pair |
| Tovolar (DFC) / `upkeep_controller` | same two files | DFC sibling of above |
| Mendicant Core, Guidelight / `spell_cast` | `custom_mendicant_core_guidelight.go`, `gen_mendicant_core_guidelight.go` | custom+gen shadow pair |
| Kolodin, Triumph Caster / `permanent_etb` | `custom_kolodin_triumph_caster.go`, `gen_kolodin_triumph_caster.go` | custom+gen shadow pair |

**11 duplicate `OnETB` registrations:**

| Card | Files | Likely cause |
|---|---|---|
| Ashling, Flame Dancer | `gen_ashling_flame_dancer.go`, `mana_pool_primitive_r57.go` | R57 mana-primitive sweep added handler without removing gen |
| Gornog, the Red Reaper | `custom_gornog_the_red_reaper.go`, `gornog_the_red_reaper.go` | custom+sibling pair |
| Jodah, the Unifier | `custom_jodah_the_unifier.go`, `jodah_the_unifier.go` | custom+sibling pair |
| Kolodin, Triumph Caster | `custom_kolodin_triumph_caster.go`, `gen_kolodin_triumph_caster.go` | custom+gen (also dup in `OnTrigger` above) |
| Lier, Disciple of the Drowned | `custom_lier_disciple_of_the_drowned.go`, `gen_lier_disciple_of_the_drowned.go` | custom+gen |
| Mabel, Heir to Cragflame | `custom_mabel_heir_to_cragflame.go`, `gen_mabel_heir_to_cragflame.go` | custom+gen |
| Morlun, Devourer of Spiders | `custom_morlun_devourer_of_spiders.go`, `gen_morlun_devourer_of_spiders.go` | custom+gen |
| Old One Eye | `custom_old_one_eye.go`, `gen_old_one_eye.go` | custom+gen |
| Rienne, Angel of Rebirth | `custom_rienne_angel_of_rebirth.go`, `gen_rienne_angel_of_rebirth.go` | custom+gen |
| Sandman, Shifting Scoundrel | `custom_sandman_shifting_scoundrel.go`, `gen_sandman_shifting_scoundrel.go` | custom+gen |
| Tiamat | `custom_tiamat.go`, `tiamat.go` | custom+sibling pair |

**Pattern analysis:** 14 of 17 duplicates (11 ETB + 3 OnTrigger excluding
Urabrask's intentional pair, the Tovolar DFC sibling counted twice, and
the Ashling R57 case) are `custom_X.go` + `gen_X.go` shadow pairs from
the gen-to-custom upgrade pipeline. Future workers replacing a gen
handler with a custom one need to either delete the gen file OR delete
the `register*Gen(Global())` call from `registerDefaults()` in
`registry.go`. The gen handler currently still fires alongside the
custom one — usually a no-op or a duplicate emit, but semantically the
custom handler should be the sole truth.

**Remaining 0/77 `OnResolve`, 0/253 `OnActivated`, 0/11 `OnCast`
duplicates.** Clean across all non-trigger surfaces.

## 5. Register* def-vs-call

Cross-reference of `func register*(...)` definitions vs
`register*(Global())` / `register*(r)` call sites across all non-test
files in `internal/gameengine/per_card/`:

- **1,156 distinct `register*` functions defined**
- **1,155 called from registerDefaults / sibling init / ResetHook**
- **0 functions called but not defined** (zero 404 risk)
- **0 functions defined but not called** (the lone exception is
  `registerDefaults` itself, the entry point that wires all the
  others — false positive from the call-shape filter, not a genuine
  orphan)

The registration plumbing is clean. Every `register*` function that
exists in source is wired into the global registry via at least one of:
`registerDefaults()` in registry.go, a sibling `init()` in
`tribal_lords.go` / `obeka_support.go` / `combat_restrictions.go` /
`batch17_sweep.go`, or one of the `zz_*_register.go` files. The
`AddResetHook` pattern (registry.go:114) ensures all sibling init()
registrations survive a test-driven `Reset()`.

## Methodology

```bash
# 1. Census of OnTrigger call sites (non-comment, non-test)
ls internal/gameengine/per_card/*.go | grep -v _test.go | \
  xargs perl -ne 'next if /^\s*\/\//;
    if (/\.OnTrigger\("([^"]+)",\s*"([^"]+)"/) { print "$1|$2|$ARGV\n" }' \
    > /tmp/all_ontrigger.txt

# 2. Distinct registered events
ls internal/gameengine/per_card/*.go | grep -v _test.go | \
  xargs perl -ne 'if (/\.OnTrigger\("[^"]*",\s*"([^"]+)"/) { print "$1\n" }' \
  | sort -u > /tmp/registered_events_clean.txt

# 3. Distinct fired events
grep -rhE 'FireCardTrigger\(gs,\s*"[^"]+"' internal/gameengine/ --include="*.go" | \
  grep -v _test.go | \
  perl -ne 'if (/FireCardTrigger\([^,]+,\s*"([^"]+)"/) { print "$1\n" }' \
  | sort -u > /tmp/fired_events.txt

# 4. Normalize both sets through eventAliasTable and compare
python3 /tmp/normalize.py    # script in commit history

# 5. Duplicate (card, event) pairs
awk -F'|' '{print $1"|"$2}' /tmp/all_ontrigger.txt | \
  sort | uniq -c | sort -rn | awk '$1 > 1 {print}'

# 6. Register* def-vs-call cross-reference
ls internal/gameengine/per_card/*.go | grep -v _test.go | \
  xargs grep -hE '^func register[A-Z]\w+\(' | \
  perl -ne 'if (/^func (register\w+)\(/) { print "$1\n" }' | sort -u \
  > /tmp/register_defined.txt
ls internal/gameengine/per_card/*.go | grep -v _test.go | \
  xargs perl -ne 'if (/(register[A-Z]\w+)\(\s*(Global\(\)|r)\s*\)/) { print "$1\n" }' \
  | sort -u > /tmp/register_called.txt
comm -23 /tmp/register_defined.txt /tmp/register_called.txt   # orphan defs
comm -13 /tmp/register_defined.txt /tmp/register_called.txt   # 404 calls
```

## Recommended followups (do NOT bundle into this PR)

Triage order — smallest blast radius first:

1. **Fix the broken `etb` alias chain** (event_aliases.go:142-144 →
   `permanent_etb`). One-line edit. Unblocks Aura Shards / Sokka and
   Suki / Field of the Dead — three handlers currently inert. Test:
   add `TestEventAlias_ETBFamily_RoutesToPermanentETB` to
   `internal/gameengine/event_aliases_test.go` (if it exists, else
   create) asserting `NormalizeEventSingle("creature_enters_battlefield")
   == "permanent_etb"`.

2. **Decide on the 3 orphan events** (`postcombat_main_controller`,
   `untap_step`, `upkeep_opponent`). For each: either wire a
   `FireCardTrigger` site in the engine at the appropriate phase
   transition, OR alias the event to an existing canonical (e.g.
   `untap_step` → `untap_event`, `upkeep_opponent` →
   `upkeep_controller` with a controller-mismatch guard inside each
   handler).

3. **Resolve the 14 `custom_*.go` + `gen_*.go` shadow-pair
   duplicates**. For each pair: confirm via grep that the custom
   handler subsumes the gen handler's behavior (the standard
   pattern), then delete the gen-side `Register*` call from
   `registry.go::registerDefaults()`. Leave the gen file itself in
   place for diff-history. Affected:
   - Toxrill, Tovolar (front + DFC), Mendicant Core, Kolodin (in
     both OnTrigger and OnETB)
   - Gornog, Jodah, Lier, Mabel, Morlun, Old One Eye, Rienne,
     Sandman, Tiamat
   - Ashling, Flame Dancer (R57 mana-primitive sweep — likely just
     delete the gen-side OnETB)

4. **Discover the 29 engine-emitted events with no per_card consumer**
   as a product-side prioritization exercise. The engine is paying
   the cost; lighting up the mechanic is purely additive. Magecraft
   (~30 cards) and Mount/Saddle (~10) are the largest pools.

5. **No action needed on `Register*` def-vs-call** — the plumbing is
   clean.

## Conclusion

The per_card registry layer is in good shape:

- **0 genuine `Register*` 404s** out of 1,155 call sites.
- **0 orphan `Register*` definitions** out of 1,156 defs.
- **1 alias-chain bug** affecting 5 card handlers (the `etb` → `permanent_etb` cluster). Lowest-risk fix is a 3-line alias-map edit.
- **3 standing orphan events** that genuinely don't fire — needs either an engine-side `FireCardTrigger` site or an alias remap.
- **17 duplicate registrations**, of which 14 are the well-known custom+gen shadow pattern from the upgrade pipeline (semantically fine — both fire — but should be deduplicated to make the truth source unambiguous).
- **29 engine-emitted events with no per_card listener** — discovery opportunity, not a bug.

No code changed under this PR. Followups enumerated above can each ship
as a focused PR with its own regression tests.
