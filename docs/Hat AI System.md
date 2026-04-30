# Hat AI System

> Last updated: 2026-04-29
> Source: `internal/hat/`, `internal/gameengine/hat.go`

A "Hat" is the pluggable player-decision protocol. Every choice the rules leave to a player goes through the Hat interface. Engine NEVER inspects hat type — load-bearing architectural directive from 7174n1c.

## Decision Flow

```mermaid
flowchart TD
    Engine[Engine reaches a player choice<br/>cast / target / mulligan / block /<br/>response / mode / discard] --> Method{Which Hat<br/>method?}
    Method --> M1[ChooseMulligan]
    Method --> M2[ChooseLandToPlay]
    Method --> M3[ChooseCastFromHand]
    Method --> M4[ChooseResponse]
    Method --> M5[ChooseAttackers]
    Method --> M6[AssignBlockers]
    Method --> M7[ChooseMode]
    Method --> M8[ChooseTarget]
    Method --> M9[OrderReplacements]
    Method --> M10[OrderTriggers]
    Method --> M11[ChooseDiscard]
    Method --> M12[ShouldCastCommander]
    Method --> M13[ShouldRedirectCommanderZone]
    M1 --> Hat[Seat.Hat]
    M2 --> Hat
    M3 --> Hat
    M4 --> Hat
    M5 --> Hat
    M6 --> Hat
    M7 --> Hat
    M8 --> Hat
    M9 --> Hat
    M10 --> Hat
    M11 --> Hat
    M12 --> Hat
    M13 --> Hat
    Hat --> Decision[Returns choice]
    Decision --> Engine2[Engine applies + continues]
    Engine -.broadcast.-> Observe[ObserveEvent<br/>every Hat hears all events]
```

## Implementations

| Hat | Status | Purpose |
|---|---|---|
| [[YggdrasilHat]] | **CURRENT** | Unified brain, budget dial 0-200+, native multi-seat |
| [[Greedy Hat]] | Deprecated | Baseline heuristic — kept for parity tests |
| [[Poker Hat]] | Deprecated | HOLD/CALL/RAISE adaptive — superseded |
| MCTSHat | Deprecated | Was wrapping inner hat — superseded |
| OctoHat | Test-only | Says yes to everything — engine stress only |

## Why Pluggable

```
gs.Seats[0].Hat = &hat.GreedyHat{}
gs.Seats[1].Hat = hat.NewYggdrasilHat(...)
gs.Seats[2].Hat = hat.NewYggdrasilHat(WithArchetype("combo"))
gs.Seats[3].Hat = hat.NewPokerHat()
```

Mixed pods. Hot-swap mid-game. Engine tests run with deterministic GreedyHat; tournaments use Yggdrasil.

## Interface Lives in Engine

`gameengine.Hat` declared in `gameengine/hat.go` (not `internal/hat/`) so `Seat.Hat` references it without an import cycle. Implementations live in `hat/`.

## Eval Pipeline (Yggdrasil)

```mermaid
flowchart LR
    State[GameState + seatIdx] --> Eval[GameStateEvaluator]
    Eval -.tanh-normalize.-> Score[score in -1..+1]
    Eval --> Dim1[BoardPresence]
    Eval --> Dim2[CardAdvantage]
    Eval --> Dim3[ManaAdvantage]
    Eval --> Dim4[LifeResource]
    Eval --> Dim5[ComboProximity]
    Eval --> Dim6[ThreatExposure]
    Eval --> Dim7[CommanderProgress]
    Eval --> Dim8[GraveyardValue]
    Dim1 --> Weights[archetype weights]
    Dim2 --> Weights
    Weights --> Score
```

See [[Eval Weights and Archetypes]] for weights and [[MCTS and Yggdrasil]] for budget mechanics.

## Strategy Profile

`StrategyProfile` carries deck-specific intelligence from [[Freya Strategy Analyzer]]: archetype, combo pieces, tutor priorities, value-engine keys, card roles, win lines. Loaded by tournament runner via `LoadStrategyFromFreya`.

## Related

- [[YggdrasilHat]]
- [[Eval Weights and Archetypes]]
- [[MCTS and Yggdrasil]]
- [[Greedy Hat]]
- [[Poker Hat]]
- [[Freya Strategy Analyzer]]
