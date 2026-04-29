# Pluggable AI Hats

> *"D shouldn't be anything more than a hat that gets swapped out as we
> change AI pilot policy."* — 7174n1c

All AI decision-making in the mtgsquad engine lives behind the
`Hat` Protocol. The engine calls `seat.policy.method(...)` at
every decision site and never inspects which concrete class is attached.
Swapping AI pilots is a **one-line change**:

```python
seats[0].policy = GreedyHat()      # heuristic baseline (default)
seats[1].policy = PokerHat()       # HOLD/CALL/RAISE adaptive
seats[2].policy = LLMHat(api_key=...)    # future
seats[3].policy = HumanUIHat(ws=...)     # future
```

Zero engine changes required to drop in a new hat — just implement
the Protocol.

> Backward compat: the previous names `PlayerPolicy`, `GreedyPolicy`, and
> `PokerPolicy` are still exported as aliases, so any external code that
> imported them keeps working.

---

## The `Hat` Protocol

Full interface (see [`__init__.py`](./__init__.py) for docstrings):

| Method | When called | Return |
|---|---|---|
| `choose_mulligan(game, seat, hand)` | Opening hand (CR 103.4) | `bool` — True to mulligan |
| `choose_land_to_play(game, seat, lands_in_hand)` | Main phase | `CardEntry` or `None` |
| `choose_cast_from_hand(game, seat, castable_cards)` | Main-phase cast loop | `CardEntry` or `None` |
| `choose_activation(game, seat, activatable)` | Activated abilities | `(perm, ability_idx)` or `None` |
| `declare_attackers(game, seat, legal_attackers)` | Combat §508 | `list[Permanent]` |
| `declare_attack_target(game, seat, attacker, legal_defenders)` | Multi-opp combat | `int` (seat idx) |
| `declare_blockers(game, seat, attackers)` | Combat §509 | `{id(atk): [blockers]}` |
| `choose_target(game, seat, filter_spec, legal_targets)` | Target selection | `(kind, target)` |
| `respond_to_stack_item(game, seat, stack_item)` | Priority (CR §117) | `CardEntry` or `None` |
| `choose_mode(game, seat, spell, modal_choices)` | Modal spells (CR §700.2d) | `list[int]` |
| `order_replacements(game, seat, candidates)` | Replacements (CR §616.1) | ordered `list` |
| `choose_discard(game, seat, hand, n)` | Forced discard / cleanup | `list[CardEntry]` |
| `choose_distribution(game, seat, n, targets)` | Damage/counter split | `{target: n}` |
| `observe_event(game, seat, event)` | Every `game.ev()` fire | `None` (side-effect only) |

The engine guarantees:
1. **Hats never mutate game state directly.** They return their
   decision; the engine applies it. This is so a neural-hat or LLM
   reasoning step can't accidentally produce illegal state — the engine
   validates every hat output against the rules.
2. **`observe_event` fires on EVERY seat's hat.** A defender's
   hat learns about the active player's plays through the event
   stream, the same way a replay viewer would.
3. **Protocol conformance is duck-typed.** You don't have to inherit
   from `GreedyHat`. Anything that implements these methods works.
   Subclassing `GreedyHat` just gives you a sane default for the
   methods you don't want to override.

---

## Current implementations

### `GreedyHat` — heuristic baseline

Ports the decision logic that used to live inline in `playloop.py`.
Every method here matches the original behavior 1:1, so regression
tests / gauntlet baselines stay unaffected by the refactor.

- **Cast**: biggest affordable non-counterspell first
- **Attack**: every creature that can legally attack
- **Block**: deadliest attacker first, smallest survivor, chump iff
  lethal
- **Target**: threat-ranked opponent via `threat_score`
- **Counter**: threshold 3 on stack-item threat score
- **Discard**: highest-CMC first

Stateless — `observe_event` is a no-op.

### `PokerHat` — adaptive HOLD/CALL/RAISE

Internal mode that the engine knows nothing about. Transitions on
events with hysteresis to avoid chatter:

```
HOLD  ── self_threat ≥ 12 ──→  CALL  ── combo_ready + mana ──→  RAISE
HOLD  ←── self_threat ≤ 8 ──   CALL  ←─── life > 10 ─────────   RAISE
                               ↑           ↑
                          hysteresis   emergency gates:
                          (not 12!)    life ≤ 10, opp 1 from win
```

- **HOLD** — reactive. No attacks, no big casts (cmc > 2 held back),
  self-targeting on any-target effects, greedy counter behavior.
- **CALL** — engaged. Attack the top 60% of creatures by power,
  threat-rank targets, counter only the biggest threats (score ≥ 5).
- **RAISE** — all-in. Attack with everything, don't block unless
  lethal is on the table, don't counter (conserve mana for our plays).

Transition triggers are event-driven (see `observe_event`): every
`damage`, `life_change`, `cast`, `attackers`, `blockers`,
`combat_damage`, `seat_eliminated`, `turn_start`, `draw` fires a
`_re_evaluate()` call. A 3-event cooldown (`_MODE_CHANGE_COOLDOWN_EVENTS`)
prevents ping-pong on the boundary.

Emits a structured `player_mode_change` event (via `game.ev()`) on
every transition so replay tooling / the rule auditor can trace
decisions.

---

## Writing a new hat

Three rules:

1. **Never mutate game state.** If you have to touch game state from a
   hat method, you've put game-flow logic in the wrong place.
2. **Stay fast.** Hats run at every decision site — a 10ms LLM
   call per targeting decision will murder gauntlet throughput.
   Cache aggressively, batch where the Protocol allows.
3. **Keep internal state in `self`.** The engine is ignorant. Fields
   on `Seat`/`Game` are for rules state (life, battlefield, zones,
   etc.), not hat state.

Minimum viable hat (passes on everything):

```python
class MyHat:
    def choose_mulligan(self, g, s, h): return False
    def choose_land_to_play(self, g, s, l): return l[0] if l else None
    def choose_cast_from_hand(self, g, s, c): return None
    def choose_activation(self, g, s, a): return None
    def declare_attackers(self, g, s, a): return []
    def declare_attack_target(self, g, s, a, d): return d[0]
    def declare_blockers(self, g, s, a): return {id(x): [] for x in a}
    def choose_target(self, g, s, f, t): return "none", None
    def respond_to_stack_item(self, g, s, i): return None
    def choose_mode(self, g, s, sp, c): return [0] if c else []
    def order_replacements(self, g, s, c): return list(c)
    def choose_discard(self, g, s, h, n): return h[:n]
    def choose_distribution(self, g, s, n, t): return {t[0]: n} if t else {}
    def observe_event(self, g, s, e): pass
```

If you only need to customize a few decisions, subclass `GreedyHat`
instead — it's Protocol-conformant and provides a sane default for
every method:

```python
from extensions.policies.greedy import GreedyHat

class MyAggroHat(GreedyHat):
    def declare_attackers(self, game, seat, legal_attackers):
        return list(legal_attackers)  # always swing with everything
```

---

## Swap at game setup

```python
from extensions.policies import GreedyHat, PokerHat

# Default: every seat auto-binds GreedyHat via __post_init__.
game = play_game(decks=[d0, d1, d2, d3], commander_format=True)
# All 4 seats using GreedyHat — baseline behavior.

# To use PokerHat instead, pre-build the seats:
seats = [
    Seat(idx=i, library=list(decks[i]), policy=PokerHat())
    for i in range(4)
]
# ... then drive the turn loop yourself, or monkey-assign before the
# first turn:
for s in game.seats:
    s.policy = PokerHat()
```

See [`scripts/test_policy_interface.py`](../../test_policy_interface.py)
for the full verification suite, and
[`scripts/gauntlet_poker.py`](../../gauntlet_poker.py) for the
4-player gauntlet demo that measures the meta shift between policies.

---

## Observed meta shift (20-game 4p EDH gauntlet, seed 42)

| Deck | Greedy win% | Poker win% | Delta |
|---|---:|---:|---:|
| Coram, the Undertaker | 30% | 5% | -25pp |
| Oloro, Ageless Ascetic | 10% | 0% | -10pp |
| Ragost, Deft Gastronaut | 25% | 20% | -5pp |
| Sin, Spira's Punishment | 35% | **75%** | +40pp |

Avg turns: 15.8 (Greedy) → 44.9 (Poker). Poker's HOLD phase delays
early development, stretching games. 1,982 total `player_mode_change`
events across 20 games (~99/game) — policies are actively transitioning
state, and the swap visibly changes which archetype wins.

This is the architectural signal: **swap policy → meta changes**.
Zero delta would mean the policy is being ignored (a spaghetti
regression).
