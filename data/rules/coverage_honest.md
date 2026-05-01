# Honest Coverage Report

**Parser status: 100% GREEN** (every card returns an AST without parse errors).

But GREEN is two things, and the distinction matters for what the runtime engine
will actually be able to execute. This report splits them.

## Three honest numbers

| Category | Cards | % | What it means |
|---|---:|---:|---|
| **Structural** | 27,272 | 85.32% | Every ability maps to a typed AST node (Damage, Buff, Tutor, Destroy, etc.) that the engine can execute directly. |
| **Mixed** | 2,392 | 7.48% | Some abilities are typed, others are stubs waiting for engine-side custom resolvers. Playable but incomplete. |
| **Stub** | 1,938 | 6.06% | AST contains only stub Modifications (`custom(slug)` or similar placeholders). Card is recognized; engine needs a hand-coded resolver. |
| **Vanilla** | 361 | 1.13% | No oracle text (vanilla creatures, tokens with no abilities). Trivially executable. |

## Per-card handler stats

- Per-card handlers in `per_card.py`: **1,079** named cards
- Of those, cards that actually hit the handler (i.e., are in the oracle dump): **1,079**

Per-card handlers are intentionally emitting stub placeholders for snowflake
cards. They are NOT the same as structural coverage — they're a work queue
for the runtime engine's custom-resolver dispatch.

## The honest framing

- **"100% GREEN" = 100% of cards parse without error.** This is real.
- **"Engine-executable today" = Structural + Vanilla = 27,633 (86.45%).**
  For these cards, the AST is fully typed and a runtime interpreter can execute
  them based on the node types alone.
- **"Engine work owed" = Stub + Mixed = 4,330 (13.55%).**
  These cards parse, but the runtime engine would need custom-resolver code
  keyed by slug or by card name to actually play them.

## What to show externally

When describing this project honestly:

> "The parser reaches syntactic coverage of every printed Magic card (31,963 cards,
> 100%). Of those, 86.45% produce a fully-typed AST that a runtime
> engine can execute from the node types alone. The remaining 13.55%
> are recognized but carry stub modifications that will need hand-coded resolvers
> in the engine layer. This is the parser — the runtime engine is the next build."

That framing is both impressive and accurate. "Parsed every magic card" is
legitimately a thing no public FOSS project has cleanly accomplished. But
"can play every magic card" is not yet true, and this report preserves the
distinction.
