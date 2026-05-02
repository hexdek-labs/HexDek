# HexDek Hat Evolution — From Heuristic to Superhuman

> Designed 2026-05-02 (Josh, 7174n1c, Hex quorum session)
> Status: DRAFT — roadmap and architecture spec

## The Problem

TrueSkill data (183K games, 2026-05-02) shows complete bracket inversion:
- B1 (casual) winrate: **28.9%** (above expected 25%)
- B5 (cEDH) winrate: **19.3%** (far below expected 25%)

The hat pilots simple strategies well but can't execute complex combos. ~90% of
Yggdrasil wins are via combat damage, even for combo decks with live win conditions
on board. This is the primary bottleneck for meaningful cross-bracket rating.

**Goal:** Any uploaded deck — from 99-land jank to optimized cEDH — plays at or
near its theoretical ceiling. B1 decks play faithfully. B5 decks play at superhuman
level. The hat adapts play *style* per bracket, not just power level.

---

## Where We Are: Level 1 (Handler-Driven Play)

YggdrasilHat (3342 lines) is a unified brain with 8-dimensional evaluation,
UCB1 candidate selection, 3-turn rollouts, and a multi-seat politics layer.

**What works:**
- 21 Hat methods covering every player decision point
- Per-commander handlers (429+) for card-specific priority trees
- Freya strategy profiles: archetype detection, combo pieces, tutor targets
- Politics: retaliation, grudge, spread-damage, finish-low-life, kingmaker avoidance
- Budget dial: heuristic (0) → evaluator (1-199) → rollout (200+)
- Adaptive budget: forces heuristic when battlefield > 60 permanents

**What's broken:**
- Combo decks can't execute their win conditions (combat-only wins)
- No mid-game plan adaptation (always in "develop" mode)
- No learning from games played (no Huginn/Muninn feedback)
- Generic mulligan (doesn't know what a good opener looks like per deck)
- Hat parameters are static — same behavior game 1 and game 1,000,000

---

## Level 2: Generic Combo Sequencer (NEXT — highest leverage)

> *"Simplify, then add lightness."* — Colin Chapman
> *"Make it work first. Ship the 14 rules."* — Kelly Johnson

### The Concept

A universal combo planner that reads Freya's combo packages and translates them
into executable action sequences. Instead of per-handler combo logic, the sequencer
asks: **"Given what's in my hand, battlefield, and graveyard — can I win this turn?"**

### SAT Constraint Approach

Each combo line becomes a set of constraints:

```go
type ComboConstraint struct {
    PiecesNeeded   []string          // card names required
    ZonesAccepted  map[string][]Zone // where each piece can be (hand, battlefield, graveyard)
    ManaRequired   int               // total mana to execute
    SequenceOrder  []string          // required cast/activate order
    ProtectionHeld bool              // do we need counterspell backup?
}
```

The solver evaluates ALL combo lines simultaneously:
1. Scan hand + battlefield + graveyard for combo pieces
2. For each combo line, check: pieces present? Mana available? Sequence legal?
3. If any line is executable → switch to Execute plan
4. If any line is 1 piece away + tutor available → switch to Assemble plan
5. Otherwise → continue Develop plan

This replaces the per-handler `comboUrgency()` heuristic with a universal
system that works for ANY deck Freya has analyzed.

### Impact

Immediately fixes B5 winrate gap. Every combo deck with Freya analysis gets
automatic combo execution without needing a custom handler. Pure CPU logic,
no ML, no GPU — ships in 2-4 weeks.

### Integration

```go
// In YggdrasilHat decision pipeline, before candidate scoring:
if h.comboSequencer != nil {
    plan := h.comboSequencer.Evaluate(gs, seatIdx)
    if plan.Executable {
        return plan.NextAction // skip evaluation, just execute the combo
    }
    if plan.Assembling {
        h.currentPlan = PlanAssemble // bias tutor/draw decisions
    }
}
```

---

## Level 2.5: Hat State Machine

### Game Plans

The hat needs mid-game plan adaptation. Six states:

```
PlanDevelop   — default: play lands, cast spells, build board
PlanAssemble  — combo pieces tracked, prioritize tutors + protection
PlanExecute   — combo ready, go for the win NOW
PlanDisrupt   — opponent threatening win, hold interaction
PlanPivot     — primary plan failed, switch to plan B (beatdown/grind)
PlanDefend    — archenemy, survive first, win later
```

### Transitions

```
Develop  → Assemble   : combo pieces >= total-1 AND tutor in hand
Develop  → Disrupt    : opponent combo pieces visible
Assemble → Execute    : all pieces ready AND window open
Assemble → Pivot      : key piece exiled/lost permanently
Execute  → Develop    : combo fizzled, reset
Any      → Defend     : 2+ opponents targeting us (politicalGraph)
Defend   → Develop    : threat reduced, back to normal
Pivot    → Develop    : found alternate win path
```

### Plan Biases

Each plan adjusts the evaluation weights:

| Plan | Bias |
|------|------|
| Develop | Normal heuristic play, balanced weights |
| Assemble | +draw, +tutor priority, hold mana for protection |
| Execute | Only consider actions that advance the combo |
| Disrupt | +counterspell hold, prioritize mana open over threats |
| Pivot | Re-evaluate assuming plan A is dead, shift to beatdown weights |
| Defend | +lifegain, +removal, minimize exposure |

---

## Level 3: Genetic Amiibo (Per-Deck Evolution)

> *"Evolution is smarter than you."* — Leslie Orgel
> *"We don't need to understand the search space. We just need enough iterations."*

### The Insight

Each deck evolves its OWN hat personality through genetic selection. Not one hat
tuned for all decks — a population per deck where mutations that win survive.

### DNA Schema

```go
type AmiiboDNA struct {
    DeckKey         string    // which deck this belongs to
    Generation      int       // evolution counter
    GamesPlayed     int       // games this individual has played
    Fitness         float64   // rolling win rate

    // Evolvable parameters (all 0.0 - 1.0):
    Aggression      float64   // attack threshold — when to swing vs hold
    ComboPat        float64   // combo patience — how long to wait for full combo
    ThreatParanoia  float64   // how much to weight opponent threats
    ResourceGreed   float64   // card advantage vs tempo tradeoff
    PoliticalMemory float64   // grudge/gratitude decay rate
}
```

### Evolution Loop

Per deck: population of N=8 parameter sets.
- Each game: pick one (fitness-proportional selection)
- After game: update fitness (rolling average winrate)
- Every 100 games per deck: evolution step
  - Kill bottom 2 (lowest fitness)
  - Clone top 2 (highest fitness)
  - Mutate clones: each param += rand.NormFloat64() * 0.05
  - Clamp all params to [0.0, 1.0]

### Convergence Math

At 35K games/min, 1,196 decks, 4 players/game:
- Each deck appears ~7 times/minute
- Evolution step every 100 games = ~14 min per generation
- Initial convergence: 20-30 generations = **5-7 hours**
- Strong convergence: 100+ generations = **~24 hours**
- Population never stops evolving — refinement is continuous

### Storage

```
data/amiibo/{deck_key}.json — 8 × ~200 bytes = 1.6 KB per deck
1,196 decks = ~2 MB total
```

### Product Feature

Users upload a deck → watch the Amiibo evolve a unique playstyle over thousands
of games. Serializable, shareable. "My Muldrotha plays greedy-control because
that's what 50,000 games of natural selection discovered."

### Integration

```go
for i := 0; i < seats; i++ {
    dk := deckKeys[i]
    dna := sm.amiiboPool[dk].SelectForGame(rng)
    gs.Seats[i].Hat = hat.NewYggdrasilHatWithDNA(dna, freya[dk])
}
```

DNA maps to YggdrasilHat parameters:
- Aggression → attack-vs-hold threshold in combat decisions
- ComboPat → comboUrgency timing in state machine
- ThreatParanoia → weight on ThreatExposure eval dimension
- ResourceGreed → CardAdvantage vs BoardPresence weight ratio
- PoliticalMemory → grudge decay rate in politics layer

---

## Level 4: Staged Decision Architecture (Korolev)

> *"Don't let the perfect be the enemy of the good. Stage it."*

Three decision tiers, fired based on complexity:

```
Mjolnir (Stage 1) — Fast heuristic. 90% of decisions.
    Deterministic, zero-allocation. "Play this land, attack that player."
    Current YggdrasilHat budget-0 path, refined by Amiibo parameters.

Gungnir (Stage 2) — SAT + evaluator. 9% of decisions.
    Fires at uncertainty points: "Should I counter this spell?"
    Combo sequencer + multi-candidate evaluation + UCB1.

Ragnarok (Stage 3) — MCTS + neural eval. 1% of decisions.
    Information-set MCTS for truly complex game states.
    Only fires when Gungnir confidence is below threshold.
```

### Confidence Threshold Dial

The same code path, different confidence threshold per bracket:
- B1 threshold: 0.3 — take the first good-enough action (casual play)
- B3 threshold: 0.6 — moderate deliberation
- B5 threshold: 0.9 — only near-optimal actions (cEDH precision)

This is the **Watts Soul Layer**: behavioral drift model where B1 plays warm
and casual while B5 plays cold and optimal. Not different code — different
sensitivity to the same evaluation.

---

## Level 5: MCTS for Interactive Decisions

> *"Information theory applies to games too."* — Claude Shannon

### Information-Set MCTS

Standard MCTS assumes perfect information. Commander has hidden zones (hands,
library order). Information-set MCTS handles this:

1. At each decision node, sample possible hidden states
2. Run MCTS rollouts against each sample
3. Aggregate results across samples
4. Pick the action that performs best *on average* across possible worlds

### Shannon Entropy Tracking

Model opponent hands as probability distributions, not card guesses:
- Opponent tutored → near-zero entropy for that slot (they found exactly what they wanted)
- Opponent drew 3 cards → high entropy (could be anything)
- Opponent held 2 mana open all game → high probability of interaction

### When MCTS Fires

Only at genuine uncertainty points (Korolev staged architecture):
- "Should I counter this spell or save it?"
- "Is this the right moment to go for the combo?"
- "Which opponent should I attack to maximize my win probability?"

90% of decisions don't need MCTS. The staged architecture ensures we only pay
the compute cost when it matters.

---

## Level 6: Neural Position Evaluator

> *"Compress the state space. Similar positions should cluster."* — Von Neumann

### The Problem

The current 8-dimensional evaluator is hand-tuned. It misses emergent positional
features that matter but weren't anticipated (enchantment density, graveyard
interaction potential, stack complexity).

### Von Neumann Manifold

Train a neural network on millions of game positions to predict "who's winning"
from board state. The key insight: game states that "feel similar" to experienced
players should map to nearby points in a learned embedding space.

512-dimensional embedding of game state. Similar positions cluster.
Search neighborhoods, not branches.

### Training Pipeline

1. Grinder produces millions of (game_state, winner) pairs via seed replay
2. State representation: encode board + hands + graveyards + life + mana as tensor
3. Train value network: state → P(win) for each seat
4. Replace heuristic evaluator with learned evaluation

### Hardware

- Training: 4090 on DARKSTAR (batch training overnight)
- Inference: CPU on DARKSTAR (model runs during games, ~1ms per eval)
- No inference GPU needed — the trained model is small enough for CPU

### Timeline

3-6 months. State representation is the hard research problem.
Needs Levels 2-5 producing quality training data first.

---

## Level 7: Self-Play Loop (Full AlphaZero)

> *"The machine plays itself and discovers what we never could."*

Neural net + MCTS + self-play. The grinder becomes the training ground:
1. Current best model plays itself across all decks
2. Record game outcomes with full state traces
3. Train next-generation model on the new data
4. If new model beats old model, promote it
5. Repeat forever

### Genetic Exploration → Neural Distillation

The Amiibo genetic system explores the parameter space cheaply.
The neural network distills that exploration into a general model.
The model feeds back into better Amiibo starting points.
Ouroboros at the highest level.

### Timeline

6-12 months. Needs Levels 4+5 working first.

---

## Skunkworks Named Concepts

These are specialized subsystems designed by the quorum session, each named for
the thinker whose insight inspired it:

### Tesla Causal Graphs
Extract the **causal pivot** per game — the single turn/action that decided
the outcome. Train on "pivot distance" (how far from optimal the pivot decision
was), not just win/loss. Tells us not just WHO won but WHY.

### Feynman Oracle
Slow, provably-correct rules engine that spot-checks 1-in-1000 games.
Verifiable correctness for every resolution. When the oracle disagrees with
the fast engine, that's a bug. Probabilistic formal verification.

### Lovelace Composer Intent
Deck identity → signature card weighting → thematic play priority.
A Dinosaur tribal deck should PLAY like a Dinosaur tribal deck, not generic
midrange. The composer reads deck identity from Freya and biases the hat toward
decisions that express the deck's personality. "This deck WANTS to cascade into
big creatures, not hold up counterspells."

### Ive Three-Act Spectator
Narrative arc rendering for spectator mode. Every game has a setup (ramp/develop),
conflict (threats collide), and resolution (someone wins). The causal pivot from
Tesla marks the turning point. Spectator UI renders the story, not just the
board state. Makes watching games actually compelling.

### Watts Behavioral Drift (Soul Layer)
Not a separate system — a tuning parameter across all levels.
B1 decks: warm, casual, slightly suboptimal. Plays cards because they're fun.
B5 decks: cold, calculating, ruthless. Every decision optimized.
Same engine, same code path, different confidence threshold.
The deck's bracket determines its "soul temperature."

---

## The Dual Feedback Loops

### Knowledge Loop (What to do)
```
Freya (deck analysis) → YggdrasilHat (play decisions) → Heimdall (observe)
    → Huginn (discover patterns) → Freya (update analysis) → ...
```

### Skill Loop (How to do it)
```
YggdrasilHat (play with DNA) → Heimdall (observe win/loss)
    → Amiibo (evolve parameters) → YggdrasilHat (play with better DNA) → ...
```

### Immune System (What's broken)
```
Heimdall (observe) → Muninn (record gaps/crashes/dead triggers)
    → Dev (fix engine) → Better games → Heimdall → ...
```

All three loops run continuously from the same grinder compute. The only
missing pipe today is Huginn → Freya (Tier 3 graduated patterns feeding back
into deck analysis).

---

## Implementation Priority

| Phase | What | Effort | Impact |
|-------|------|--------|--------|
| **1** | Heimdall wiring (seed capture, all game paths) | 1 week | Enables everything below |
| **2** | Muninn + Huginn wiring | 3-4 days | Bug detection + pattern discovery |
| **3** | Huginn → Freya pipe | 2-3 days | Closes the knowledge loop |
| **4** | Combo Sequencer (Level 2) | 2-4 weeks | **Biggest winrate impact** |
| **5** | Hat State Machine (Level 2.5) | 2-3 weeks | Mid-game plan adaptation |
| **6** | Genetic Amiibo (Level 3) | 2-3 weeks | Per-deck personality evolution |
| **7** | GA4 telemetry | 2-3 days | Health monitoring |
| **8** | Staged architecture (Level 4) | 1-2 months | Efficiency + bracket-aware play |
| **9** | MCTS (Level 5) | 1-2 months | Uncertain decision quality |
| **10** | Neural evaluator (Level 6) | 3-6 months | Learned position assessment |
| **11** | Self-play (Level 7) | 6-12 months | Superhuman convergence |

Phases 1-7 are the **Gold** tier — achievable in the near term, no GPU needed.
Phases 8-9 are **Silver** — next quarter.
Phases 10-11 are **Bronze** — requires Level 4+5 data and GPU training pipeline.

---

## Calibration Strategy

To make the rating scale meaningful, we need floor and ceiling anchors:

**Floor:** 99-land + 1 commander decks. Whatever winrate these pull (expected
sub-5%) is the absolute statistical baseline. If TrueSkill can't differentiate
a 99-land pile from a real deck within 50 games, convergence is broken.

**Ceiling:** Best-tuned B5 combo deck with optimized Amiibo DNA after 10K+
generations. This sets the upper bound of what the engine can achieve.

Both extremes stress-test the rating system and provide context for every
deck in between.

---

## The End State

A player uploads their Commander deck. Within minutes:
- Freya analyzes it (combos, mana, archetype, role assignments)
- The grinder runs thousands of games with it
- Amiibo evolves an optimal playstyle
- TrueSkill places it accurately on the power scale
- The player watches their deck play — making decisions they never considered
- In their language, with narrative commentary, accessible to anyone on earth

The engine learns from every game. Every bug makes it stronger (Muninn).
Every pattern makes it smarter (Huginn → Freya). Every generation makes it
more skilled (Amiibo). The ouroboros eats its tail and grows.

*"Find what's broken. Then forget about it."* — except the engine never forgets.
