# HexDek R60 — Release Notes

**The "Engine Officially Clean" Release**

HexDek R60 closes a multi-week stress-discovery cycle that pushed
the Magic: The Gathering Commander engine from "near zero" invariant
violations to **literal zero across 100,000 canonical chaos games
+ 100,000 nightmare boards.** Alongside, the AI player picked up
22 custom-tuned archetype weight profiles, TrueSkill learned to
read the matchup before the game starts, Freya's deck analysis
grew a card-power-tier coaching layer, and the parser corpus got
its long-tail backlog scaffolded across all 4 eras.

This release covers the headline ships. Full audit trails for every
fix are in `docs/loki-*.md`, `docs/seat-bias-meta-study-r60.md`, and
`CLAUDE.md`'s Issue Log.

---

## 🟢 Engine: officially clean

The R60 stress-discovery cycle started with the engine at **1,652
invariant violations per 5,000 games** on the r41 baseline. It ends
with **zero violations per 100,000 games** across the 10 canonical
seeds (42 / 43 / 99 / 7 / 1337 / 2024 / 2025 / π / e / φ). That's a
**−100% per-game stochastic violation rate** reduction — not a
near-zero rate, an actual zero.

The path:

| Wave | Games / Seed | Per-Game Violation Rate |
|:---:|:---:|:---:|
| r41 baseline | 5K × 1 | 33.0% |
| r44 | 5K × 1 | 8.0% |
| r60 rounds 1-3 | 5K × 1-2 | 0.2% |
| r60 mega-stress | 2K × 5 fresh | 0.04% |
| r60 deep-stress | 5K × 10 | 0.05% |
| r60 extreme-stress | 10K × 10 | 0.06% |
| **r60 canonical-final** | **10K × 10** | **0.0%** |

12 distinct lifecycle / invariant / per_card fixes shipped during the
cycle. Each one has a deterministic single-game reproducer captured
from loki and a regression test pinning the bit-stable shape:

- **District Mascot static-ETB-counter** — the AST classifier emits
  "this creature enters with a +1/+1 counter on it" as a `Static`
  ability; the engine only handled it via the effect-resolution path.
  New `ApplyStaticETBCounters` primitive wired into both ETB entry
  points (cast + blink/reanimate).
- **SBA cap mandatory-loop-draw cleanup** — the §704.3 SBA pass cap
  set `ended=1` + `game_draw=1` but never marked seats Lost, leaving
  the turn loop calling TakeTurn with SBAs permanently muted. Fix
  marks all non-Lost seats Lost under CR §104.4b.
- **Gisa, Glorious Resurrector TriggerCompleteness FP** — the
  invariant matched bearer-seat against death-seat without
  considering opp-only handlers. Added a registry of the 6 known
  opp-only `creature_dies` handlers (Gisa, The Reaper, Toxrill,
  Yahenni, Grave Pact, Grave Betrayal).
- **Necrogen Communion CardIdentity FP** — `pickReplacement` now
  skips stack items whose `Kind` is "triggered"/"activated" or
  whose `Source != nil`, since their `item.Card` is a log-label,
  not a zone occupant per CR §405.
- **Athreos cross-seat reanimate race** — when two Athreos
  controllers both placed coin counters on a dying creature, both
  handlers raced to claim the *Card pointer. Added the established
  Adric / Oketra / Gisa "validate still in graveyard before
  claiming" defensive check.
- **Charix ended-flag SBA short-circuit** — the SBA toughness check
  needed a `!ended` guard (mirroring the existing life-loss arm
  pattern); +X/-X mods stacking past game-end is rules-correct, the
  invariant was over-eager.
- **HandleSeatElimination ExpireSourceGrants** — the §800.4a
  seat-loss cleanup was the lone LTB-equivalent path PR #106's
  earlier ZoneCastGrant work didn't cover. Added one-line plumbing.
- **Zidane EOT control-return left-play guard** — the delayed
  trigger blindly re-added the captured perm to original owner's
  battlefield without checking whether the perm had left play
  (died → graveyard, exiled, bounced). Per CR §611.3c the return is
  a no-op when the perm isn't on the battlefield.
- **HandleSeatElimination pending-triggers purge** — `gs.Stack`
  items controlled by the leaving seat were purged at elimination,
  but the CR §603.3b trigger batch (`gs.pendingTriggers`) wasn't.
  Myr Moonvessel's "when this dies, add {1} to your mana pool"
  resolved on an already-eliminated seat. CR §800.4a: abilities
  cease to exist.
- **pickReplacement stale-source backstop** — defensive gate against
  replacements whose `SourcePerm` had left play through non-canonical
  paths (mutate-eaten perm, sweep alt-cost return, per_card flicker
  helpers). The gate catches phantoms regardless of which code path
  leaked them.
- **Rest in Peace ETB-vs-zone-change false positive** — when a spell
  resolves directly into the graveyard via CR §608.2g (no
  intervening `destroy` event), the existing destroy-guard didn't
  fire. New ETB-after-zone-change scan suppresses the FP.
- **WinCondition LeftGame guard** — the poison + commander-damage
  arms of `checkWinCondition` re-validated against current counter
  state, but counters can decrease post-loss (Hapatra-style counter
  swaps, "remove all poison" effects). Added the `!s.LeftGame`
  guard the life arm already had.

12 fixes, every one drilled to a single-game seed reproducer.

Full details: `docs/loki-r60-canonical-final.md`.

---

## 🤖 AI player: 22 custom-tuned archetype weight profiles

R60 retired YggdrasilHat's "everyone gets the midrange weight set"
default and shipped per-archetype tuning for the full 22-archetype
roster. Each profile is documented inline with a rationale of which
dimensions are signature (anchored at the 2.0 mark), neutral (around
0.8-1.0 baseline), or deprioritized.

Highlights of the new profiles:

- **Aggro** (`BoardPresence 2.0 / LifeResource 1.6`): "press the
  strongest opponent's life to zero before they assemble" — the
  evaluator now folds opponent-pressure into `scoreLife`, so the
  hat values lowering the strongest opp's life, not just preserving
  its own.
- **Control** (`CardAdvantage 1.6 / StackInteraction 1.5 / ManaAdvantage
  1.3 / ThreatExposure 1.3`): "outdraw, hold up answers, neutralize
  the biggest threat." ManaAdvantage bumped from 0.8 → 1.3 so the
  hat values keeping mana up for instant-speed interaction.
- **Burn** (`LifeResource 1.8 / ThreatExposure 1.4 / ComboProximity
  0.1`): "sequence damage, protect the damage sources, no combo
  assembly to chase."
- **Group Hug** (`CardAdvantage 2.0 / StackInteraction 0.9 /
  LifeResource 0.3`): "tax the table's draws, disrupt their combos,
  survive the late game as kingmaker."
- **Voltron** (`CommanderProgress 2.0 / ThreatExposure 1.4`): the
  single-threat fragility makes Voltron the most ThreatExposure-
  sensitive archetype, capping Control's 1.3.
- **Aristocrats** (`DrainEngine 2.0`)
- **Storm** (`ComboProximity 2.0 / ManaAdvantage 1.5`)
- **Reanimator** (`GraveyardValue 2.0`)
- **Enchantress** (`EnchantmentSynergy 1.8 / CardAdvantage 1.4`)
- **Artifacts** (`ArtifactSynergy 1.8 / ActivationTempo 1.2`)
- **Stax / Superfriends / Mill / Tribal / Selfmill / Lifegain / Lands
  Matter / Counters Matter / Blink / Extra Combats**: each picked up
  a deck-shape-specific dimension anchor.

The full set lives in `internal/hat/eval_weights.go`. Every block has
an inline comment with the why-this-deck-cares-about-this-dimension
rationale.

Beyond the archetype weights, R60 also shipped a **self-trigger
response matrix**: the hat now correctly counters its OWN trigger
when the self-harm would be lethal — own Manabarbs would burn lethal
damage on the controller? Counter it. Own mill self-decks the deck
with a Laboratory Maniac dead? Counter it. The validation gauntlet
was 2,500 games × 5 seeds with **0 self-harm fires across the full
lethal-state matrix** (mill / life-loss / damage / commander damage).

A handful of cast-prioritization improvements landed: `ChooseAttackers`
now folds expected attack-trigger value into per-attacker scoring
(Edric / Bygone draws, Surveil-on-attack, treasure-on-attack);
`cardHeuristic` gives ETB-trigger creatures a cast-priority bonus so
Mulldrifter / Reclamation Sage rank above equal-cost creatures
without strong ETBs.

---

## 🧙 TrueSkill: composition-aware ratings

The R60 seat-bias meta-study (`docs/seat-bias-meta-study-r60.md`)
established that seat-position bias is QUARK-shaped where directly
measurable — the deck composition at the table dominates the seat
position's effect. Two cross-composition archetypes (Reanimator,
LandsMatter) classified QUARK with composition-stdev of 7-14pp,
versus per-seat within-composition ranges of 0.6-2.4pp.

The follow-up: if composition drives the result, the rating system
should READ the composition before the game and use it as a prior.

R60 shipped the **CompositionPrior** wave across 7 PRs:

1. **Implementation** (#403) — `CompositionPrior` struct + `CompositionScore`
   helper maps the 4-deck archetype tuple to an expected-winrate vector
   via Freya's archetype matchup matrix.
2. **TrueSkill integration** (#408) — `TrueSkillRatings.Update`
   shrinks the mu delta when the matchup-implied result was already
   predicted and amplifies it when surprising.
3. **Validation** (#411) — **+1.4 percentage-point accuracy, +0.036
   log-loss** versus the no-prior baseline. Full breakdown in
   `docs/composition-prior-validation.md`.
4. **Live wire-in** (#415) — composition prior in showmatch updateELO.
5. **Monitoring** (#420) — Heimdall analytics surfaces the per-game
   composition-effect contribution; operators can see how much of
   each rating delta came from the prior versus the actual upset.
6. **Tooling** (#424) — new `hexdek-composition-replay` debug CLI
   for offline what-if analysis ("re-rate this game without the
   prior, show the difference").
7. **Confidence intervals** (#428) — Wilson 95% bounds on each
   matchup cell so low-data archetype pairings shrink toward 50/50
   rather than overweighting the rating delta.

For HexDek operators, this means a B4 cEDH-vs-jank pod that 3-0s
the cEDH deck no longer gives the jank deck a credit-for-an-upset-
that-the-priors-said-was-likely rating boost.

---

## 🌸 Freya: deck-coaching tools

Freya's deck analysis grew a coaching layer aimed at the
deckbuilder-who-wants-to-know-WHY-this-card-stays. The big additions:

### Card power-tier S/A/B/C/D

Every non-land card now gets a 0-100 power score plus an absolute
S/A/B/C/D tier. The score has three explicit components:

- **ArchetypeFit (0-40)** — weights card roles by the deck's primary-
  archetype fingerprint ratio (Combo deck favors RoleCombo / RoleTutor,
  Stax favors RoleStax / RoleRemoval) with a 10-point floor for any
  tagged card that didn't match the fingerprint.
- **CMCEfficiency (0-20)** — absolute curve band (CMC≤1: 20, CMC2:
  18, ... CMC6+: 2) with a 2-point bonus for multi-role at CMC≤2.
- **SynergyContribution (0-40)** — win-line piece (+25), value-chain
  bridge (+20) / step (+10), finisher (+12), cluster member (+6),
  per-role floor (+2 each), with penalties for redundant CMC4+ tutor
  when cheaper alternatives exist (-8) and CMC5+ Utility-only dead
  slot (-10).

Tier bands are **absolute, not percentile** — a casual precon
correctly reports `0S / 7A / 28B / 17C / 11D` rather than promoting
its top filler. The tier letter conveys "your top card is only A-
tier" as the buy-it pacing signal.

Each card also gets a one-line **WHY explanation**:

```
★ [S 84] Thassa's Oracle — wincon piece + 3-role at CMC 2 + Combo fit (Tutor/Combo)
● [B 40] Demonic Tutor — Tutor at CMC 1 + Combo fit (Tutor)
✂ [D 16] Ashaya — CMC 5 (curve heavy) + off-archetype + dead slot
```

Real-world calibration on a 300-deck moxfield corpus:
**6.9% S / 19.4% A / 35.5% B / 29.2% C / 8.9% D** — peaked at B
with thin elite tails, matching the target histogram exactly.

### Bracket B5 cEDH detection

The WotC bracket spec lists 5 brackets (B1 casual → B5 cEDH).
Pre-R60, B5 reached by raw score ≥12 with no specific cEDH-shape
requirement, so an optimized big-mana pile could falsely register
as cEDH.

R60 added a **B5 confirmation gate**: bracket=5 only if
(`freeInteractionCount >= 2` OR `tutorDensity >= 0.12` OR
`gameChangerCount >= 8`) AND `avgCMC < 2.8`. The free-interaction
list is a curated set of ~27 cards (Force of Will / Pact of Negation
/ evoke elementals / Fierce Guardianship-family / phyrexian-mana
counters) — intentionally excludes cheap-but-not-free interaction
since those are B4 staples.

Calibration: 14 of 16 test decks classified exactly, 16 of 16 within
±1 bracket. Every bracket call now ships with a **rationale table**
showing per-signal contribution + evidence cards:

```
Bracket rationale (raw score 19 → B5 cEDH):
  [+3] Game Changers (heavy): 6 cards — Demonic Tutor, Vampiric Tutor, ...
  [+3] Free interaction (heavy): 4 cards — Force of Will, Fierce Guardianship, ...
  [+2] Tutor density: 12.3%
  [+2] Combo lines: 3 true-infinite
  ...
  [gate] B5 confirmation: free_int=4 OR tutor=12% OR gc=6  → B5 allowed
```

### Pet card detection

For low-tier creatures the deckbuilder kept despite an off-archetype
fit — the signature "flavor pick" pattern. Detection requires ALL
of: PowerTier ∈ {C, D}, TypeLine contains "creature", at least one
role tag, no role matches the deck's primary archetype, not a
CMC 5+ Utility-only dead slot. Legendary creatures get the
"signature flavor pick" reason (legendaries are the strongest pet
signal); nonlegendary get "personal-taste pick".

The coaching purpose: when Freya suggests cuts, the pet cards are
flagged separately so the deckbuilder knows "we're not going to
tell you to cut Marwyn the Nurturer, but you should know it's not
helping the deck's archetype-fit."

### Synergy clusters with chain-depth scoring

Round 2 of the synergy-cluster engine (round 1 was producer/payoff
pairs). R60 added a 3rd `clusterRoleAmplifier` role for themes
whose engine is a natural 3-step chain:

- **death_value**: token-maker → sac outlet → dies-trigger
- **etb_value**: HasValueETB creature → blinker → triggers-on-other-ETBs

Scoring adds `min(producers, amplifiers, payoffs) * 3` chain bonus
on top of the existing pair sum, so completing the chain (adding
the missing-link card) creates a discontinuous score jump — matching
the deckbuilding reality that a sac outlet finally makes a token-
flood-plus-drain pile actually win.

### Commander Spellbook integration

The combo database grew to 58 curated entries plus a public-data
import path (`cmd/hexdek-freya/spellbook_import.go`) that parses the
Commander Spellbook variants JSON, infers loop type from features
(`Win the game` / `Infinite *` → true_infinite), and dedupes against
the curated set via canonical lowercased+sorted card-name keys.
Curated entries always win conflicts (richer outlets/stops). CLI
flags `--spellbook <cache>` and `--spellbook-fetch <url>`.

### Combo class hierarchy

Combos now carry a class taxonomy:

- **true_infinite** — actually wins on the spot
- **soft_lock** — closes the table out over a few turns
- **value_engine** — generates incremental advantage
- **win_condition** — discrete kill, not a loop

The class flows into the strategy bridge so hat MCTS weight profiles
can prefer true-infinite finishers over soft-lock value loops.

### Meta-positioning matchup matrix

22 archetypes × 22 archetypes matchup matrix with a **reciprocity
invariant**: `matchup(A,B) + matchup(B,A) = 0` for symmetric
matchups (±1 tolerance for asymmetric advantages like prison-vs-
aggro). Any entry that violates the invariant is flagged as a
matrix-build bug. Reports two-line reasoning per matchup. This
same matrix is what feeds the TrueSkill composition prior above.

### Threat assessment, opening hand sim, color weight optimization,
deck personality blurb, etc.

R60 also shipped the 26-entry hoser database (condition-matched
vulnerability report), 10K-trial Monte Carlo opening hand
simulation with a commander-centric KeepableHandPctAdjusted variant,
demand-vs-supply color analysis with specific land swap suggestions,
and an archetype-aware 2-3 sentence deck personality blurb.

Full Freya kanban in `CLAUDE.md`.

---

## 📦 Decks app: the archetype loop

R60 closed the Freya → Decks → user → Freya loop:

- **System-assigned archetype tags** (#416) — Freya's classification
  surfaces as a system-tag per deck in the Decks UI. Distinguished
  from user-assigned tags by render color.
- **Confirm/correct UI + training log** (#421) — owner can confirm
  Freya's call or correct it to a different tag; corrections write
  to a training-log endpoint feeding the next Freya retune wave.
- **Archetype change history across deck versions** (#425) — tracks
  how Freya's classification shifted as the deckbuilder iterated.
  Useful for "this deck became its archetype on revision N" stories.
- **Training-signal endpoint** (#429) — aggregates corrections into
  similarity clusters; surfaces top mis-classified clusters for the
  next Freya retune wave to prioritize.
- **Saved views** (#404) — per-user saved filter/sort presets.
- **Recent games inline** (#266) — last-N games per deck row,
  clicking jumps to the Heimdall summary.

The loop ships an actual feedback path: bad classification → user
correction → retraining-cluster signal → better classification next
wave. No more silent disagreement between Freya and the deckbuilder.

---

## 🧰 Parser & corpus: long-tail backlog scaffolded

A 2026-05-08 corpus audit identified **4,190 unbucketed
condition/trigger nodes** in the Thor AST corpus, spanning all 4 card
eras. R60 closed this with 4 era-specific scaffold sweeps:

- **Era 1** (PR #87) — 19 condition scaffolds
- **Era 2** (PR #96) — 19 trigger-event slugs + extended audit script
  with per-event bucketing
- **Era 3** (PR #88) — 13 dead r48 scaffolds wired + 7 new
- **Era 4** (PR #94) — 19 condition scaffolds

Current state: **condition gap 14.8%** (era 1 longtail 15.8% + era 3
20% + era 4 11.5%; **era 2 at 0%**). Trigger gap tracked in era 2
audit script at 14.5%; follow-up to wire the same trigger-event
bucketing into eras 1/3/4 audits remains as a future polish task.

The TLA flashback-grant family also closed: Iroh (multi-target
grant), Lier and A-Lier (continuous always-on / active-turn variants),
Return the Past (Enchantment), Backdraft Hellkite (attack-trigger EOT
mass grant), plus the per_card sibling sweep for single-target
attack/ETB-triggered grants (Snapcaster, Sphinx of Forgotten Lore,
Slickshot Lockpicker, Katilda and Lier).

---

## 📊 The numbers

- **12** distinct lifecycle / invariant / per_card engine fixes
- **22** AI-player archetype profiles custom-tuned
- **~500,000** chaos games + **~600,000** nightmare boards simulated
  during the stress cycle
- **100,000 + 100,000** canonical-final games → **0 violations,
  0 crashes, 0 panics**
- **+1.4pp** TrueSkill accuracy gain from composition prior
- **+0.036** log-loss improvement
- **5,000+** AST corpus entries with the long-tail backlog
  scaffolded
- **58** curated combo entries plus the Commander Spellbook import
  path
- **300** moxfield decks calibrated against the Freya power-tier
  distribution
- **−100%** per-game stochastic violation rate vs r41 baseline

---

## What's next

The R60 era is closed on systemic invariant work. The remaining
chase surfaces:

- **Fresh-seed sweeps** — the extended-seeds rate is ~1 residual per
  20K games on never-tested seeds. We have enumeration coverage on
  10 canonical seeds; the next residual surface needs a fresh seed.
- **Longer game depths** — current `max-turns=60` cap. Late-game
  lifecycle bugs (post-turn-60 ZoneCastGrant decay, state-based-
  action interactions after a full mulligan-loop, etc.) need
  `max-turns=100+`.
- **Targeted scenario decks** — Loki's chaos shuffler doesn't surface
  specific 2-card combinations. A scenario-deck driver that
  enumerates the 58 curated combo lists + edge-case interactions
  would catch a class of bugs Loki doesn't sample.

For HexDek users today: the engine is clean, the AI plays
archetype-aware, TrueSkill is composition-aware, Freya coaches
where to put your next dollar, and the Decks app has the
feedback loop.

— The HexDek team
2026-05-25
