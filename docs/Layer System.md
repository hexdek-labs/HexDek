# Layer System

> Last updated: 2026-04-29
> Source: `internal/gameengine/layers.go`
> CR ref: §613

Continuous-effect layer application. `GetEffectiveCharacteristics(perm)` is the canonical accessor — never read raw `perm.Card` characteristics for combat / SBA decisions.

## Layer Stack

```mermaid
flowchart TD
    Base[Card baseline<br/>name, types, P/T,<br/>colors, abilities] --> L1[Layer 1<br/>copy effects 613.1a]
    L1 --> L2[Layer 2<br/>control changes 613.1b]
    L2 --> L3[Layer 3<br/>text changes 613.1c]
    L3 --> L4[Layer 4<br/>type/subtype/super 613.1d<br/>Bestow lives here]
    L4 --> L5[Layer 5<br/>color changes 613.1e]
    L5 --> L6[Layer 6<br/>ability add/remove 613.1f]
    L6 --> L7a[Layer 7a<br/>P/T CDA 613.4a]
    L7a --> L7b[Layer 7b<br/>set P/T 613.4b]
    L7b --> L7c[Layer 7c<br/>modify P/T<br/>+1/+1 / -1/-1 counters]
    L7c --> L7d[Layer 7d<br/>switch P/T 613.4d]
    L7d --> Eff[Effective characteristics]
    Eff -.face-down §707.2.-> Override[2/2 colorless creature]
```

## Within-Layer Ordering

- §613.7 timestamp ascending (default)
- §613.3 CDAs first inside layers 2-6 (MVP: timestamp wins)
- §613.8 dependency: implemented via `DependencyOrder` building a graph + topological sort. **Cycle break falls to timestamp** (§613.8b)

## Counter Application (§613.4c)

Post-layer pass reads `perm.Counters["+1/+1"]` and `["-1/-1"]` plus `perm.Modifications` (until-EOT buffs). Per-counter timestamps not yet tracked → matches Python reference behavior.

## Idempotency Invariant

Calling `GetEffectiveCharacteristics(perm)` twice MUST return identical results. Enforced by the `LayerIdempotency` invariant — see [[Invariants Odin]].

## Known Gap

§613.8 dependency graph handles most cases (Blood Moon + Urborg, Conspiracy + Volrath's Shapeshifter). True dependency cycles (Humility + Opalescence) fall back to timestamp; not all aspirational outputs match printed-FAQ rulings. Acceptable for v1.

## Related

- [[State-Based Actions]]
- [[Combat Phases]]
- [[Card AST and Parser]]
- [[Invariants Odin]]
