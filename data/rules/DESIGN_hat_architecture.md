# Hat Architecture Design Notes

**Status:** Logged (per 7174n1c 2026-04-16 10:38 UTC). Not yet implemented. Build when ready.

## The "hat" vocabulary

Every AI decision-maker in the mtgsquad engine is a **hat** — a pluggable policy that implements the `PlayerPolicy` protocol. The engine wears a hat per seat. Seats can swap hats freely; hats never modify the engine.

Current hat inventory:
- **GreedyHat** (`scripts/extensions/policies/greedy.py`) — baseline. Attack if legal, cast biggest spell, pick highest-threat target. Stateless, no mode tracking. Current implementation class is called `GreedyPolicy`; rename to `GreedyHat` when convenient.
- **PokerHat** (`scripts/extensions/policies/poker.py`) — adaptive HOLD/CALL/RAISE mode state + 7-dimensional threat scoring + archetype detection + RAISE cascade. Current implementation class is called `PokerPolicy`; rename to `PokerHat` when convenient.

Future hats (sketched, not built):
- **LLMHat** — BYOK, each seat plugs in their own AI (Claude/GPT/local). Decisions routed to prompt + response.
- **NeuralHat** — trained policy network. Weights loaded at setup, decisions via forward pass.
- **HumanHat** — UI-driven. Engine pauses on decision points, surfaces state, waits for human choice.
- **ScriptedHat** — follows a predetermined sequence. For reproducible test cases.
- **ChaosHat** — random legal move. For variance baselines.
- **RulesHat** (proposed) — plays the tournament-legal correct move when one exists, otherwise defers to a base hat. For judge-grade simulation.

## The Directive Slot

**7174n1c's new design idea (2026-04-16):** every hat can carry a **directive** — a specific play-style override that influences decisions without changing the hat's base logic.

Directives are orthogonal to the hat's internal state (mode, threat score, etc.). They modify HOW the base logic plays, not WHAT base logic is being used.

### Example directives (strawman list)

```python
class Directive(Enum):
    NEUTRAL = "neutral"                  # no override, base logic
    ALL_IN_COMBO = "all_in_combo"        # prioritize combo assembly over board presence
    DEFEND_ALLY = "defend_ally:idx"      # protect a specific seat (for ally roles in 4p)
    HATE_GRAVEYARD = "hate_graveyard"    # prioritize gy hate cards (bojuka, leyline, rest in peace)
    SAVE_COUNTERS = "save_counters"      # only counterspell wraths / wincons, not value
    RISK_AVERSE = "risk_averse"          # minimize variance, stable plays
    CHAOS = "chaos"                      # random legal move (overrides all else)
    ARCHENEMY = "archenemy"              # force everyone to target this seat (for 1v3 scenarios)
    TUTOR_HEAVY = "tutor_heavy"          # prioritize tutors over draw/ramp
    RUSH = "rush"                        # attack on curve regardless of threat assessment
    STALL = "stall"                      # stay in HOLD mode, defensive posture only
    PROTECT_COMMANDER = "protect_commander"  # counterspell anything targeting commander
    MILL_HATE = "mill_hate"              # protect library, don't self-mill
    LIBRARY_DRY = "library_dry"          # comfortable with low-library; play for deck-out win
```

### Implementation sketch

Add a `directive: Directive = Directive.NEUTRAL` field to `PlayerPolicy` Protocol (or base class).

Each hat checks its directive at decision-making time:
```python
def choose_cast_from_hand(self, game, seat, castable):
    if self.directive == Directive.ALL_IN_COMBO:
        # Override: prioritize combo pieces
        ...
    if self.directive == Directive.HATE_GRAVEYARD:
        # Override: prefer gy hate if available
        ...
    # Fall through to base hat logic
    return self._base_choose(game, seat, castable)
```

Directives can be **static** (set at setup) or **dynamic** (change during game via `observe_event`). A hat could decide to adopt HATE_GRAVEYARD when a opp's gy exceeds N cards.

### Multi-directive composition

Directives should be composable (multiple active at once). Implementation: `self.directives: list[Directive]` or `self.directive_set: frozenset[Directive]`. Resolve via priority order at decision sites.

### Why this matters

- **Scenario testing**: "what if everyone played aggro?" → set RUSH on all seats. "what if one deck is archenemy?" → ARCHENEMY on seat 3.
- **Deck-specific hats**: a Ragost-treasure-focused directive would play differently from a generic CALL/RAISE hat on the same deck.
- **Meta simulation**: run tournaments with varying directives to see meta dynamics.
- **Research**: ablation studies — "how much winrate comes from directive X vs base hat?"

### What to log vs build

Per 7174n1c: **log for now, build when ready.** The architecture supports it — add a Directive enum + an optional field on hats — but no need to ship today. Mission 6 showed the current policy is working at the 14-36pp band without directives. Directives are a power-user feature for future experiments, not a must-have.

## Renaming to "Hat" — future task

Find/replace `PlayerPolicy` → `Hat`, `GreedyPolicy` → `GreedyHat`, `PokerPolicy` → `PokerHat` across the codebase:
- `scripts/extensions/policies/__init__.py` — Protocol + re-exports
- `scripts/extensions/policies/greedy.py` — class rename
- `scripts/extensions/policies/poker.py` — class rename + `PokerHat(GreedyHat)` inheritance
- `scripts/playloop.py` — type hints
- `scripts/test_policy_interface.py` — test names + imports
- `scripts/gauntlet_poker.py` — imports
- `scripts/extensions/policies/README.md` — docs

This is cosmetic — keep Protocol + baseline + file structure unchanged, just rename classes. 1-2 hour task. Not blocking anything.

## Hat collection roadmap (per 7174n1c: "we can hat collect over time")

Build order when ready:
1. Rename existing classes (1-2 hr, no functional change)
2. Directive slot infrastructure (2-4 hr)
3. HumanHat (4-8 hr, needs UI)
4. ScriptedHat (2 hr, useful for regression testing specific scenarios)
5. LLMHat with BYOK (8-16 hr, prompts + response parsing)
6. ChaosHat (1 hr, useful for variance baselines)
7. NeuralHat (week+ project, needs training data pipeline)

Each hat expands the research surface without engine changes.
