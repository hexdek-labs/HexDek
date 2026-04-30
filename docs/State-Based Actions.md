# State-Based Actions

> Last updated: 2026-04-29
> Source: `internal/gameengine/sba.go`
> CR refs: §704

§704.3 says: whenever a player would get priority, perform all applicable SBAs simultaneously, then repeat until none fire. Engine implements this as a fixed-point loop, capped at 40 passes.

## SBA Loop

```mermaid
flowchart TD
    Entry[StateBasedActions called<br/>before priority opens] --> Pass[Run all helpers]
    Pass --> ChangeQ{Any helper<br/>mutated state?}
    ChangeQ -- yes --> Pass
    ChangeQ -- no --> Done[Return anyChange flag]
    Pass -.cap 40 passes.-> Done
```

## Helpers (per CR §704.5 / §704.6)

```mermaid
flowchart LR
    subgraph PlayerLoss [Player loss §704.5a-c]
        A[5a life ≤ 0]
        B[5b empty library draw]
        C[5c poison ≥ 10]
    end
    subgraph Zone [Zone / copy §704.5d-e]
        D[5d token in non-bf]
        E[5e copy spell]
    end
    subgraph Death [Creature death §704.5f-i]
        F[5f toughness ≤ 0]
        G[5g lethal damage]
        H[5h deathtouch — stub]
        I[5i pw loyalty ≤ 0]
    end
    subgraph Dup [Duplicates §704.5j-k]
        J[5j legend rule]
        K[5k world rule]
    end
    subgraph Att [Attachment §704.5m-p]
        M[5m aura attach legality]
        N[5n equipment attach legality]
        P[5p bestow detach]
    end
    subgraph Counter [Counters §704.5q-r]
        Q[5q +1/-1 annihilate]
        R[5r counter cap — stub]
    end
    subgraph Special [Special §704.5s-z]
        S[5s saga final chapter]
        Z[battle / role / speed]
    end
    subgraph Cmdr [Commander §704.6]
        CD[6c 21 cmdr damage]
        CG[6d cmdr in gy / exile]
    end
```

## Wired Helpers

Implemented mutating: 5a, 5b, 5c, 5d, 5f, 5g, 5i, 5j, 5k, 5m, 5n, 5p, 5q, 5s, 6c. Stubs: 5e, 5h (deathtouch tracked elsewhere via [[Combat Phases]]), 5r.

## Interaction with Other Systems

- Calls [[Replacement Effects|FireEvent]] for `would_die`, `would_be_put_into_graveyard`, `would_lose_game` — Platinum Angel, Rest in Peace, Anafenza all hook here.
- Death triggers fan out via [[Trigger Dispatch]] after destroy event resolves.
- [[Layer System]] is re-applied each pass so toughness math stays current (§704.5f).
- Loop invocations logged to [[Tool - Stack Trace]] as `sba_check`.

## Cap Rationale

40 passes matches the Python reference. Pathological loops (e.g. mutual indestructible legend rule weirdness) are caught here rather than spinning forever. None observed in 50K production games.

## Related

- [[Stack and Priority]]
- [[Invariants Odin]]
- [[Replacement Effects]]
