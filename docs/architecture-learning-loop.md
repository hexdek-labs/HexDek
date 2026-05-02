# HexDek Learning Loop — Architecture Spec

> Designed 2026-05-02 (Josh, 7174n1c, Hex quorum session)
> Status: DRAFT — ready for implementation review

## Overview

Wire observation, memory, and evolution into all game paths so the engine
learns from every game it plays. Currently only ELO updates flow from the
grinder; all other data pipelines (Huginn, Muninn, Analytics) are dark.

## Dependency Order

```
Phase 1: Heimdall package + seed capture        (1 week)
Phase 2: Muninn + Huginn wiring                  (3-4 days)
Phase 3: Huginn → Freya pipe                     (2-3 days)
Phase 4: Hat state machine                       (2-3 weeks)
Phase 5: Genetic Amiibo prototype                (2-3 weeks)
Phase 6: GA4 telemetry bus                       (2-3 days)
```

---

## Phase 1: Heimdall Package + Seed Capture

### New package: `internal/heimdall`

```go
package heimdall

import "github.com/hexdek/hexdek/internal/gameengine"

// GameSeed stores the minimum data needed to deterministically replay a game.
// 28 bytes per game. At 50M games/day = 1.4 GB/day.
type GameSeed struct {
    RNGSeed    int64    `json:"rng_seed"`
    DeckKeys   [4]string `json:"deck_keys"`
    Winner     int      `json:"winner"`
    Turns      int      `json:"turns"`
    KillMethod string   `json:"kill_method"` // combat, commander, combo, mill, poison, timeout
}

// Observation is the lightweight per-game extraction. ~200 bytes.
// Only populated during batch replay or live observation mode.
type Observation struct {
    Seed           GameSeed
    ParserGaps     []string          // card names that hit unhandled abilities
    DeadTriggers   []DeadTrigger     // registered but never fired
    ComboAttempted bool              // did the deck attempt its Freya combo?
    ComboSucceeded bool              // did the combo resolve to a win?
    ComboMissed    bool              // were combo pieces available but not used?
    CoTriggers     []CoTriggerPair   // Huginn food: cards that synergized
    CausalPivot    *PivotEvent       // Tesla: the turn/action that decided the game
}

type DeadTrigger struct {
    CardName    string
    TriggerType string // etb, ltb, dies, cast, etc.
}

type CoTriggerPair struct {
    CardA       string
    CardB       string
    ImpactScore float64
    TurnWindow  int
}

type PivotEvent struct {
    Turn   int
    Action string
    Seat   int
}

// Observer is the singleton that receives game results from all paths.
type Observer struct {
    seedBuf    []GameSeed         // ring buffer, flush every 1000 games
    obsBuf     []Observation      // only populated in observation mode
    huginn     HuginnSink
    muninn     MuninnSink
    telemetry  TelemetrySink
    mu         sync.Mutex
}

// HuginnSink receives co-trigger observations.
type HuginnSink interface {
    IngestCoTriggers(pairs []CoTriggerPair, deckNames []string)
}

// MuninnSink receives bug signals.
type MuninnSink interface {
    RecordParserGaps(gaps []string, gameID string)
    RecordDeadTriggers(triggers []DeadTrigger, gameID string)
    RecordCrash(panicMsg string, stackTrace string, deckKeys []string)
}

// TelemetrySink sends health pulses (GA4).
type TelemetrySink interface {
    Pulse(stats HealthPulse)
}

type HealthPulse struct {
    GamesPlayed   int
    ParserGaps    int
    Crashes       int
    DeadTriggers  int
    TopGapCards   []string
    EngineVersion string
}

// RecordSeed is called after EVERY game (grinder, fishtank, gauntlet).
// Zero allocation fast path — just appends to ring buffer.
func (o *Observer) RecordSeed(seed GameSeed) {
    // append to ring buffer, flush to disk when full
}

// RecordObservation is called during batch replay or live observation mode.
// Extracts full observation data and routes to downstream sinks.
func (o *Observer) RecordObservation(obs Observation) {
    // route to Huginn, Muninn, Analytics
}

// RecordCrash is called from panic recovery in all game paths.
func (o *Observer) RecordCrash(panicMsg string, stack []byte, deckKeys []string) {
    // route to Muninn
}
```

### Hook points in showmatch.go

#### `runOneGameFast` (grinder) — line 882, after winner determined:

```go
// EXISTING (line 882):
// sm.mu.Lock()
// sm.stats.gamesPlayed++
// sm.updateELO(...)
// sm.mu.Unlock()

// ADD after ELO update (outside lock):
if sm.heimdall != nil {
    sm.heimdall.RecordSeed(heimdall.GameSeed{
        RNGSeed:    gameSeed,       // already captured at line 822
        DeckKeys:   [4]string{deckKeys[0], deckKeys[1], deckKeys[2], deckKeys[3]},
        Winner:     winner,
        Turns:      gs.Turn,
        KillMethod: classifyKill(gs, winner),
    })
}
```

#### `runOneGameFast` crash recovery — line 773:

```go
// EXISTING:
// log.Printf("grinder: game crashed: %v\n%s", r, buf[:n])

// ADD:
if sm.heimdall != nil {
    sm.heimdall.RecordCrash(fmt.Sprintf("%v", r), buf[:n], deckKeys)
}
```

#### Same pattern for `runOneGame` (fishtank) and `RunGauntlet`.

---

## Phase 2: Muninn + Huginn Wiring

### Muninn receives from Heimdall:

```go
// internal/muninn — extend existing package

// RecordParserGaps appends gap card names to data/muninn/parser_gaps.json
// Batched writes — accumulates in memory, flushes every 60 seconds.
func RecordParserGaps(gaps []string, gameID string)

// RecordDeadTriggers appends dead trigger info.
func RecordDeadTriggers(triggers []DeadTrigger, gameID string)

// RecordCrash appends crash with full stack trace.
// Writes IMMEDIATELY (crashes are rare and critical).
func RecordCrash(panicMsg string, stackTrace string, deckKeys []string)
```

### Huginn receives from Heimdall:

```go
// internal/huginn — extend existing package

// IngestCoTriggers accepts co-trigger pairs from Heimdall observation.
// Batched — accumulates, then calls Ingest() on flush.
func IngestCoTriggers(pairs []CoTriggerPair, deckNames []string)
```

### Parser gap detection in game engine:

```go
// internal/gameengine — add to card resolution

// When a card ability hits an unhandled path, set a flag:
card.Flags["parser_gap"] = true

// Heimdall reads this flag at game end for gap extraction.
```

---

## Phase 3: Huginn → Freya Pipe

When Huginn promotes a pattern to Tier 3 (confirmed), write it to a
shared location that Freya reads on next analysis pass:

```go
// data/huginn/tier3_for_freya.json
// Huginn writes Tier 3 promotions here.
// Freya reads this file at startup and incorporates confirmed
// interactions into combo package detection.

type FreyaInteraction struct {
    CardA       string   `json:"card_a"`
    CardB       string   `json:"card_b"`
    Pattern     string   `json:"pattern"`
    AvgImpact   float64  `json:"avg_impact"`
    Confidence  int      `json:"observation_count"`
}
```

Freya's combo detection checks this file for interactions the parser
didn't catch. If CardA and CardB appear in the same deck, Freya adds
the interaction to that deck's combo packages.

---

## Phase 4: Hat State Machine

### States:

```go
package hat

type GamePlan int

const (
    PlanDevelop   GamePlan = iota // default: play lands, cast spells, build board
    PlanAssemble                  // combo pieces tracked, prioritize tutors + protection
    PlanExecute                   // combo ready, go for the win NOW
    PlanDisrupt                   // opponent threatening win, hold interaction
    PlanPivot                     // primary plan failed, switch to plan B (beatdown/grind)
    PlanDefend                    // archenemy, survive first, win later
)

type PlanState struct {
    Current       GamePlan
    ComboReady    int       // count of combo pieces in hand+battlefield
    ComboTotal    int       // total pieces needed (from Freya)
    ThreatLevel   float64   // highest opponent threat score
    TurnsSincePlan int      // turns in current plan (for timeout transitions)
}
```

### Transitions:

```go
func (ps *PlanState) Evaluate(gs *GameState, seat int, freya *FreyaProfile) GamePlan {
    // ComboReady >= ComboTotal && window open → Execute
    // ComboReady >= ComboTotal-1 && have tutor → Assemble
    // Opponent combo pieces visible → Disrupt
    // Key combo piece exiled/lost → Pivot
    // 2+ opponents targeting us → Defend
    // Default → Develop
}
```

### Integration with Hat:

YggdrasilHat checks `PlanState.Current` before making decisions.
Each plan biases the evaluation differently:
- **Execute**: only consider actions that advance the combo
- **Disrupt**: prioritize holding mana for interaction
- **Develop**: normal heuristic play
- **Pivot**: re-evaluate threats assuming plan A is dead

---

## Phase 5: Genetic Amiibo

### Parameter Schema:

```go
package hat

type AmiiboDNA struct {
    DeckKey         string    `json:"deck_key"`
    Generation      int       `json:"generation"`
    GamesPlayed     int       `json:"games_played"`
    Fitness         float64   `json:"fitness"` // rolling win rate
    
    // Evolvable parameters (all 0.0 - 1.0):
    Aggression      float64   `json:"aggression"`       // attack threshold
    ComboPat        float64   `json:"combo_patience"`   // how long to wait for full combo
    ThreatParanoia  float64   `json:"threat_paranoia"`   // how much to weight opponent threats
    ResourceGreed   float64   `json:"resource_greed"`    // card advantage vs tempo
    PoliticalMemory float64   `json:"political_memory"`  // grudge/gratitude decay rate
}
```

### Evolution loop (runs inside grinder):

```go
// Per deck: maintain a population of N=8 parameter sets.
// Each game, pick one parameter set (weighted by fitness).
// After the game, update fitness (rolling average win rate).
// Every M=100 games per deck:
//   - Kill bottom 2 (lowest fitness)
//   - Clone top 2 (highest fitness)
//   - Mutate clones: each param += rand.NormFloat64() * 0.05
//   - Clamp all params to [0.0, 1.0]

type AmbiboPool struct {
    DeckKey    string
    Population [8]AmiiboDNA
    GameCount  int
}

func (pool *AmbiboPool) SelectForGame(rng *rand.Rand) *AmiiboDNA {
    // Fitness-proportional selection
}

func (pool *AmbiboPool) RecordResult(idx int, won bool) {
    // Update rolling fitness, maybe trigger evolution step
}
```

### Storage:

```
data/amiibo/{deck_key}.json — population state per deck
```

Small: 8 × ~200 bytes = 1.6 KB per deck. 1196 decks = ~2 MB total.

### Integration:

In `runOneGameFast`, after picking decks but before creating hats:

```go
for i := 0; i < showmatchSeats; i++ {
    dk := deckKeys[i]
    dna := sm.amiiboPool[dk].SelectForGame(rng)
    gs.Seats[i].Hat = hat.NewYggdrasilHatWithDNA(dna, freya[dk])
}
```

---

## Phase 6: GA4 Telemetry

### Event bus:

```go
package telemetry

type GA4Client struct {
    MeasurementID string
    APISecret     string
}

func (c *GA4Client) SendEvent(category, name string, params map[string]interface{}) {
    // POST to https://www.google-analytics.com/mp/collect
}
```

### Health pulse (every 60 seconds from Heimdall):

```go
c.SendEvent("engine", "health_pulse", map[string]interface{}{
    "games_played":  count,
    "parser_gaps":   gapCount,
    "crashes":       crashCount,
    "dead_triggers": triggerCount,
    "gap_cards":     strings.Join(topGaps, ","),
    "engine_version": version,
    "throughput_gpm": gamesPerMinute,
})
```

---

## Data Flow Summary

```
GRINDER (runOneGameFast)
    │
    ├─ RecordSeed() ──→ data/heimdall/seeds.bin (28 bytes/game)
    ├─ RecordCrash() ──→ Muninn (immediate write)
    │
    └─ Every 10K games: batch replay ──→ RecordObservation()
        ├─ CoTriggers ──→ Huginn ──→ tier graduation
        │                              └─→ Tier 3 ──→ Freya
        ├─ ParserGaps ──→ Muninn
        ├─ DeadTriggers ──→ Muninn
        └─ ComboHit/Miss ──→ Analytics ──→ API endpoints

GENETIC LOOP (every 100 games per deck):
    AmiiboDNA selected ──→ Hat plays game ──→ win/loss ──→ fitness update
    Every 100 games: evolve population (kill worst, clone best, mutate)

GA4 PULSE (every 60 seconds):
    Heimdall aggregates ──→ Measurement Protocol POST
```
