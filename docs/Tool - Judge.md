# Tool - Judge

> Last updated: 2026-04-29
> Source: `cmd/mtgsquad-judge/`

Interactive REPL for adversarial rules-engine testing. Construct board states by hand, fire actions, query state, run invariants. The full card corpus is loaded so permanents have real AST data + [[Layer System|layer calculations]].

## REPL Loop

```mermaid
flowchart LR
    Start[judge start] --> Load[Load AST + oracle corpus]
    Load --> Prompt[> prompt]
    Prompt --> Cmd{command}
    Cmd -- create game --> Setup[fresh GameState]
    Cmd -- seat N add_permanent --> Build[place permanent]
    Cmd -- seat N cast --> Cast[run [[Stack and Priority|CastSpell]]]
    Cmd -- resolve --> Pop[ResolveStackTop]
    Cmd -- sba --> SBA[StateBasedActions]
    Cmd -- query --> Inspect[print state slice]
    Cmd -- invariants --> RunInv[run all 20 [[Invariants Odin]]]
    Cmd -- assert --> Verify[PASS / FAIL]
    Cmd -- step --> Step[advance one game step]
    Setup --> Prompt
    Build --> Prompt
    Cast --> Prompt
    Pop --> Prompt
    SBA --> Prompt
    Inspect --> Prompt
    RunInv --> Prompt
    Verify --> Prompt
    Step --> Prompt
```

## Commands

| Command | Purpose |
|---|---|
| `create game --seats N` | fresh GameState |
| `seat N add_permanent "Card"` | place permanent on battlefield |
| `seat N add_to_hand "Card"` | put card in hand |
| `seat N set_life N` | set life total |
| `seat N cast "Card" [targeting ...]` | cast a spell |
| `resolve` | resolve top of stack |
| `sba` | run state-based actions |
| `query seat N permanent "Name"` | characteristics of a permanent |
| `query layers` | show all continuous effects |
| `assert <condition>` | PASS / FAIL on engine state |
| `step` | advance one game step |
| `events` | show recent event log |
| `invariants` | run all 20 |
| `state` | dump full game state |

## Why It Exists

When Thor or Loki finds a bug, Judge is where you reproduce it. Hand-build the exact prerequisite board, fire the action, watch what happens. Designed for rules-lawyer flow: "I claim X. Engine, prove me wrong."

## Usage

```bash
go run ./cmd/mtgsquad-judge \
  --ast data/rules/ast_dataset.jsonl \
  --oracle data/rules/oracle-cards.json
```

## Related

- [[Tool - Thor]]
- [[Invariants Odin]]
- [[Layer System]]
