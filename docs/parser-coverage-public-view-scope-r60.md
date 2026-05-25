# Parser Coverage — Public-Facing View (Scope)

This document scopes a public, read-only view of the parser-coverage
data — a "click any card to see if HexDek parses it correctly" surface
for visitors, deck builders, and curious users who want to know
whether their pet card works in the engine.

This is a **scoping document**, not an implementation. It defines the
audience, UX, integration points, plain-language status mapping, and
phased rollout. Concrete code/PRs follow the design choices here.

## Summary

A visitor to `hexdek.dev/cards/<name>` sees a new **HexDek Parser
Status** panel on the existing CardPage. The panel answers, in plain
language, one question: "does HexDek's engine play this card
correctly?" — with one of four obvious answers (fully supported /
mostly supported / partial / not yet supported) and, for the
not-fully-supported cases, a brief honest explanation of what's
missing.

A landing page at `hexdek.dev/parser-coverage` gives the overall
state of coverage (the "we're at 89%" headline), a search box for
jumping to any card's status page, and the deck-weighted set-priority
worklist (existing `--set-priority` data, lightly re-skinned for
public consumption).

## Goals

- **Set expectations honestly.** Visitors trying out the engine
  should know up front whether their commander or pet card is
  supported. No surprises mid-game.
- **Drive contribution interest.** Contributors looking for "what
  should I help with?" can see the prioritized worklist for the
  cards real players actually run.
- **Build trust in the engine.** Coverage data is verifiable — the
  same audit tool that generates the public view also drives the
  internal dev report. The public number isn't marketing.
- **Be discoverable.** Card pages are SEO-friendly URLs (one card →
  one stable URL), so search-engine traffic for "MTG parser <card
  name>" lands directly on an authoritative answer.

## Non-goals

- **Not** an admin tool — no edit, no flag-as-fixed, no comments.
  Read-only.
- **Not** the developer UI — the existing `--html`/`--serve` interactive
  audit stays as-is for engine devs. The public view is a separate
  consumer with simpler messaging.
- **Not** real-time. The audit runs at release boundaries (via
  `--history`); the public view reads from a snapshot.
- **Not** scoped here: badges/integrations (Scryfall-side, Moxfield-
  side, etc.). Those are future work once the page exists.

## Audience & use cases

| Persona | Question they're asking |
|---|---|
| Deck builder evaluating HexDek for their commander | "Will my deck play correctly?" |
| Visitor who saw a HexDek mention on Reddit | "Is this a serious engine or a toy?" |
| Would-be contributor | "Where can I help that matters?" |
| Existing user who hit a bug | "Is this a known parser gap or a real bug?" |

## UX shape

### Card page (existing route, new panel)

```
┌──────────────────────────────────────────────────────────────────┐
│ Sol Ring                                          {1}            │
│ Artifact                                                         │
│                                                                  │
│ [card art]                                                       │
│                                                                  │
│ {T}: Add {C}{C}.                                                 │
│                                                                  │
│ ┌─ HexDek Parser Status ──────────────────────────────────────┐  │
│ │  ✓ Fully supported                                          │  │
│ │  HexDek plays this card without any known gaps.             │  │
│ │  Audited 2026-05-24 against Scryfall oracle data.           │  │
│ └─────────────────────────────────────────────────────────────┘  │
│                                                                  │
│ ┌─ Used in 247 indexed decks ─────────────────────────────────┐  │
│ │ (existing panel — unchanged)                                │  │
│ └─────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

For a not-yet-supported card:

```
┌─ HexDek Parser Status ──────────────────────────────────────┐
│  ⚠ Not yet supported                                        │
│                                                              │
│  HexDek's card parser hasn't ingested "Ormacar, Relic       │
│  Wraith" yet. This is likely because the card was printed   │
│  in a set that landed after our last bulk-data refresh.     │
│                                                              │
│  Workaround: the card resolves as a 0/0 vanilla creature    │
│  in-engine until parser support lands.                       │
│                                                              │
│  Audited 2026-05-24 · Class: MISSING · From: 2023 Heroes   │
│  of the Realm                                                │
└──────────────────────────────────────────────────────────────┘
```

For an EMPTY_AST card (parser ran but produced no abilities):

```
┌─ HexDek Parser Status ──────────────────────────────────────┐
│  ◯ Partial support                                          │
│                                                              │
│  HexDek recognizes "Underground Sea" but doesn't yet parse  │
│  every ability. The engine treats the card as a basic-      │
│  typed land (Island + Swamp), which produces the right      │
│  mana most of the time but may miss niche interactions.     │
│                                                              │
│  Tracking: this is on the parser worklist as part of the   │
│  ABU dual-lands group (10 cards).                           │
│                                                              │
│  Audited 2026-05-24 · Class: EMPTY_AST · From: Vintage     │
│  Masters                                                     │
└──────────────────────────────────────────────────────────────┘
```

### Landing page (`/parser-coverage`)

```
┌──────────────────────────────────────────────────────────────────┐
│  HexDek Parser Coverage                                          │
│                                                                  │
│  Current state: 89.5% of all Magic cards play correctly.         │
│  Coverage moved from 76.8% → 89.5% over the last 24 days        │
│  (+12.7 percentage points).                                      │
│                                                                  │
│  ┌─ Look up a card ──────────────────────────────────────┐       │
│  │  [ Search card name…                          🔍 ]    │       │
│  └────────────────────────────────────────────────────────┘       │
│                                                                  │
│  ## What this means                                              │
│  HexDek is an open-source MTG engine. Like any rules engine, it │
│  doesn't yet implement every card in Magic's 30-year history.    │
│  This page tracks honest progress…                               │
│                                                                  │
│  ## Set priority worklist                                        │
│  Sets contributors should tackle next, weighted by how many       │
│  player decks they unblock:                                       │
│                                                                  │
│  | Rank | Set                              | Decks unblocked |   │
│  |   1  | Vintage Masters                  |  ~740           |   │
│  |   2  | Duskmourn Commander              |   ~43           |   │
│  | …    | …                                | …               |   │
└──────────────────────────────────────────────────────────────────┘
```

## Plain-language status mapping

The four classifier buckets map to user-facing labels with stable
copy. This mapping is the **contract**: changing classifier names
must not silently change the visitor-facing message.

| Classifier class | Public label | Icon | Tone |
|---|---|---|---|
| `OK` | Fully supported | ✓ | Green |
| `OK_VANILLA` | Fully supported | ✓ | Green |
| `PARTIAL` | Mostly supported | ◐ | Amber |
| `EMPTY_AST` | Partial support | ◯ | Amber |
| `MISSING` | Not yet supported | ⚠ | Red |

`OK` and `OK_VANILLA` collapse to the same public label — visitors
don't need to know the difference between "has parsed abilities" and
"is a basic land". Both work.

`PARTIAL` and `EMPTY_AST` are distinct internally (parser tried-but-
failed vs parser-found-nothing) but the visitor messaging for both is
"some abilities aren't wired up yet". The distinction is exposed only
in the small-print line at the bottom of the panel.

`MISSING` gets the strongest warning because the engine literally has
no information about the card — it falls back to a vanilla creature/
land/etc. of the right type.

## Information shown per card

For each card, the panel surfaces:

1. **Status pill** — one of the four labels above.
2. **One-sentence explanation** — plain-language, no jargon.
3. **What the engine does instead** (for non-OK cards) — so the
   visitor knows whether to expect "the card does nothing" vs "the
   card does mana but no ETB trigger" vs "the card is missing
   entirely". This is the most useful field; it should be tuned
   based on class and per-card data.
4. **Worklist tracking line** (for known parser-priority cards) —
   "this is on the worklist as part of group X".
5. **Audit metadata** (small print) — audit date, classifier class,
   set name. Useful for power users; ignorable by everyone else.

The "what the engine does instead" copy is generated by mapping the
card's type line and class:

| Class × Type | Engine behavior copy |
|---|---|
| MISSING × Creature | "resolves as a vanilla N/N creature (no abilities)" |
| MISSING × Instant/Sorcery | "the spell has no effect in-engine" |
| MISSING × Land | "behaves as a basic land of its type(s)" |
| EMPTY_AST × Land | "produces mana from its basic land types but no other abilities" |
| EMPTY_AST × Creature | "resolves as a vanilla N/N — keywords and triggered abilities not applied" |
| EMPTY_AST × Instant/Sorcery | "the spell has no effect — its abilities haven't been wired" |
| PARTIAL × any | "parsed but the engine flagged unresolved clauses (some abilities may not apply)" |

These are seeds — the per-type messaging is finite; tighten the copy
during implementation.

## Integration

The audit data already exists. This view layers UI + a tiny API on
top of primitives that have shipped:

| Layer | Source | Status |
|---|---|---|
| Classifier (per-card class) | `cmd/parser-coverage/classify()` | shipped |
| Audit timestamp / metadata | `cmd/parser-coverage/history.go` | shipped |
| Set-priority worklist | `cmd/parser-coverage/set_priority.go` | shipped |
| Headline coverage rate | `cmd/parser-coverage/history.go` | shipped |
| **NEW**: per-card lookup API | `internal/hexapi/parser_coverage.go` | to build |
| **NEW**: CardPage panel | `hexdek/src/screens/CardPage.jsx` | to build |
| **NEW**: Landing page | `hexdek/src/screens/ParserCoverage.jsx` | to build |

### API shape

```
GET /api/parser-coverage/{card-name}

Response 200:
{
  "name": "Underground Sea",
  "class": "EMPTY_AST",
  "publicLabel": "Partial support",
  "publicIcon": "◯",
  "publicTone": "amber",
  "engineBehavior": "produces mana from its basic land types but no other abilities",
  "worklistGroup": "ABU Dual Lands",
  "auditedAt": "2026-05-24T12:00:00Z",
  "set": "Vintage Masters"
}

Response 404: card unknown to oracle corpus
```

```
GET /api/parser-coverage/summary

Response 200:
{
  "auditedAt": "2026-05-24T12:00:00Z",
  "successPct": 89.48,
  "totalCards": 35708,
  "classCounts": { "OK": 31601, "OK_VANILLA": 349,
                   "MISSING": 3745, "EMPTY_AST": 12, "PARTIAL": 1 },
  "trend": {
    "since": { "label": "r58", "date": "2026-05-01" },
    "deltaPP": 12.65,
    "cardsGained": 4601
  },
  "setPriority": [
    { "set": "Vintage Masters", "score": 740, "uncovered": 10 },
    { "set": "Duskmourn: House of Horror Commander", "score": 43, "uncovered": 41 },
    …
  ]
}
```

Both endpoints are read-only, cacheable, and backed by audit
artifacts (the classifier output snapshot + history JSONL). The
server pre-loads the audit data at startup; per-request work is just
a map lookup + label translation.

### Static-snapshot variant (deployment option)

For an even lighter footprint, the entire payload can be pre-rendered
to a static JSON file at audit time:

```
data/static/parser-coverage.json
```

The CardPage panel fetches `/static/parser-coverage.json` (cacheable
forever, served by Caddy without hitting the Go server) and looks
the card name up client-side. This trades server-side simplicity for
a 1-2 MB upfront fetch — fine on broadband, slower on mobile.

**Recommendation**: do the dynamic API first (Phase 1) because the
server-side lookup is cheap; revisit static if traffic ever justifies
caching at the CDN edge.

## Deployment options

1. **Inline on hexdek.dev** (recommended) — extend the existing
   CardPage and add a landing route at `hexdek.dev/parser-coverage`.
   Lowest friction; reuses the existing Caddy front, auth-free
   public surface, deploy pipeline.
2. **Subdomain** (e.g., `coverage.hexdek.dev`) — only if we want the
   landing page to feel more like a status page (StatusGator-style)
   than a deeply linked product surface. Adds Caddy config; extra
   moving parts.
3. **Static GitHub Pages mirror** — if traffic ever overflows the
   home server, mirror the static snapshot to Pages. Defer until
   traffic justifies it.

## Phased rollout

**Phase 0 — Scope** (this doc). _Done._

**Phase 1 — API + per-card panel.** Ship `/api/parser-coverage/{name}`
and the HexDek Parser Status panel on the existing CardPage. Zero
new routes; the CardPage change is additive. Smallest possible
public surface, validates the messaging copy with real visitors.

**Phase 2 — Landing page.** Add `/parser-coverage` with the
headline number, search box, and (lightly re-skinned) set-priority
worklist. Adds one route, one screen component, one summary API.

**Phase 3 — SEO + sharing.** OpenGraph/Twitter card meta on
`/cards/:name` so a link preview shows the parser status. Sitemap
entries for every audited card so search engines can index card
status pages directly. Defer until Phases 1-2 prove the surface
is useful.

**Phase 4 — Notifications (optional).** Visitor types a card name,
gets "not yet supported", clicks "notify me when this lands". Stores
email + card name; the next time the card's class flips to `OK`,
sends a one-off email. Real email infra dependency; only worth doing
if visitor demand surfaces in Phase 1-3.

## Out of scope (explicitly)

- Per-card community comments / discussion / votes.
- "Suggest a fix" buttons that file GitHub issues.
- Live engine-test runs ("simulate this card in a game").
- Replays / sample games for each card.
- Multi-language (English only for Phase 1; the audit doesn't have
  localized oracle text).

Each of the above is a defensible follow-up but expands the surface
substantially. Phase 1's value is the simple "click → see status"
loop; everything else compounds risk without compounding the core
value.

## Open questions

These need decisions before Phase 1 implementation lands:

1. **Tone for MISSING cards.** Should the copy say "We haven't gotten
   to this yet" (apologetic) or "Not yet supported — track progress
   on GitHub" (matter-of-fact)? Recommend matter-of-fact; ties into
   contribution-funnel goal.
2. **Vanilla-creature fallback messaging.** When the engine subs in a
   vanilla N/N for a MISSING creature, should we name that as a
   *bug-resistant fallback* or a *limitation*? Recommend "behaves as
   a vanilla N/N until support lands" — neutral framing.
3. **Set-priority worklist visibility.** Phase 2 surfaces the
   deck-weighted set ranking publicly. Is there competitive concern
   about telegraphing "we're working on X next"? Recommend no — the
   data is already in the public Git history, and surfacing it
   recruits contributors.
4. **Refresh cadence.** Audit currently runs ad-hoc. Should the
   public view auto-refresh after every merge to main, or only on
   release? Recommend release (weekly-ish) to keep the surface
   stable; merge-by-merge churn isn't useful to visitors.
5. **Search index for the landing page.** Phase 2's search box needs
   a card-name index (35k entries). Reuse `/api/cards/{name}` lookup
   with substring filter, or pre-build a static name list?
   Recommend pre-built static `card-names.json` — small (~1 MB), CDN-
   cacheable, no server roundtrip per keystroke.

## What "done with this scope" looks like

This document, plus three concrete decisions from §"Open questions",
is enough for Phase 1 implementation to start. The implementer
should:

1. Pick the tone for MISSING cards (§Q1).
2. Decide deploy model (recommendation: inline on hexdek.dev, §1).
3. Decide refresh cadence (recommendation: release, §Q4).

Phase 2 and beyond can defer their open questions until Phase 1
ships and we see visitor behavior on the live surface.
