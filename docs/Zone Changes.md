# Zone Changes

> Last updated: 2026-04-29
> Source: `internal/gameengine/zone_move.go`, `zone_change.go`, `commander.go`
> CR refs: §400, §603.10, §614, §700.4, §701.7, §701.17, §903.9

Universal `MoveCard` entry point — every non-battlefield-to-battlefield zone transition flows through here. Migration completed 2026 Q1 after audit found 200+ raw zone-move sites bypassing replacements + triggers.

## Universal MoveCard Flow

```mermaid
flowchart TD
    Caller[Mill / Discard / Surveil /<br/>Dredge / Bounce / Tutor /<br/>Reanimate / Library Tuck] --> MC[MoveCard]
    MC --> Remove[Remove card from<br/>source zone by pointer]
    Remove --> FZC[FireZoneChange]
    FZC --> Repl[[[Replacement Effects|§614 replacement chain]]]
    Repl --> CmdrCheck{Commander<br/>redirect?<br/>§903.9b}
    CmdrCheck -- yes --> Cmd[Place in command zone]
    CmdrCheck -- no --> Dest[Place in toZone]
    Cmd --> FZT[FireZoneChangeTriggers]
    Dest --> FZT
    FZT --> SelfTrig[Self-triggers<br/>§603.10 look-back]
    FZT --> ObsTrig[Observer triggers<br/>APNAP-ordered]
    FZT --> PerCard[Per-card hooks<br/>creature_dies, ltb, etc.]
    FZT --> Descend[Set DescendedThisTurn<br/>if perm → graveyard]
    FZT --> Done[Return final zone]
```

## Zone Change Operations

| Function | CR | Notes |
|---|---|---|
| `DestroyPermanent` | §701.7 | indestructible blocks; shield-counter check |
| `ExilePermanent` | §406.3 | bypasses indestructible |
| `sacrificePermanentImpl` | §701.17 | bypasses indestructible; emits typed sacrifice events |
| `BouncePermanent` | §701.10 | return to hand |
| `MoveCard` | §400 | universal entry for non-battlefield sources |

## Why MoveCard Exists

Before this file: mill, discard, surveil, dredge, exile-from-non-battlefield, bounce-to-hand, library-tuck were SILENT — no replacements, no commander redirect, no triggers. Affected:
- **Sidisi, Brood Tyrant** — mill triggers ignored
- **Gitrog Monster** — land-mill triggers ignored
- **Narcomoeba** — mill→battlefield missed
- **Bone Miser** — discard triggers missed
- **Dimir Spybug** — surveil triggers missed
- **Opposition Agent** — opponent search triggers missed
- Every "descend" card

## Battlefield Exits

Battlefield-to-X exits do NOT use `MoveCard` — they have their own lifecycle (remove from `[]*Permanent`, detach auras, drop counters, fire LTB) handled in `stack.go` / `combat.go` / `sba.go`. The wrapper would lose those steps.

## Commander Redirect (§903.9a/b)

When commander would change zones, owner may redirect to command zone. Choice via [[Hat AI System|Hat]] `ShouldRedirectCommanderZone`. Applied inside `FireZoneChange` after replacements run.

## Related

- [[Replacement Effects]]
- [[Trigger Dispatch]]
- [[State-Based Actions]]
- [[Per-Card Handlers]]
