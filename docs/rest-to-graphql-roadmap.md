# REST → GraphQL Migration Roadmap

**Status:** scoping document. No code in this PR — this is the plan
of record for the migration if/when the team decides to commit.

**Date:** 2026-05-24

**Background:** PR #321 shipped a minimal GraphQL POC over the game
graph (two queries, one type, ~120 lines + tests). PR #389 grew it
to include `deck` + `decks` + cross-resource `Game.decks`. The
question now is whether that surface should expand to cover the
entire REST API, with REST sunset on a deprecation timeline.

---

## TL;DR — The honest recommendation

**Don't try to deprecate REST entirely.** ~20% of the current
endpoints cannot move to GraphQL without rewriting the consumer or
the transport — WebSockets, SSE, binary media, HTML share pages,
external webhook receivers. Forcing them through GraphQL would
either break the consumers (Discord card unfurls cannot speak
GraphQL) or add a transport layer on top of GraphQL just to
re-do what REST already does well.

**Do migrate the JSON CRUD surface to GraphQL.** That's ~70 of the
~85 distinct endpoints, and it's where the cost/benefit of
GraphQL (field projection, batching, schema documentation) is real.
The result is a hybrid: GraphQL for queries + mutations,
REST for streaming + binary + HTML + webhook intake.

Frame the work as **"shrink REST to its load-bearing core"**, not
"deprecate REST."

---

## Inventory: the current REST surface

104 `mux.HandleFunc` registrations across `internal/hexapi/` as of
2026-05-24. Counting unique routes (a couple are duplicate-mounted
under two paths — `POST /api/decks` and `POST /api/decks/import`
share a handler):

| Category                | Count | Migrate? |
|-------------------------|-------|---------|
| Decks: CRUD + analysis  | ~17   | Yes — flagship migration target |
| Games: list + detail    | ~7    | Mostly yes |
| Card data + art         | ~7    | Mostly yes (art is binary, stays) |
| Card stats              | ~6    | Yes — heavy field-projection win |
| Profiles + owner stats  | ~7    | Yes |
| Gauntlet + tournaments  | ~3    | Mixed (POST yes, SSE stays) |
| Live game state         | ~7    | No — WebSocket-shaped |
| Spectate rooms          | ~5    | Mixed (WS stays, REST surface migrates) |
| Imports (Moxfield)      | ~3    | Yes (mutation) |
| Admin                   | ~5    | Yes (low traffic; can defer) |
| Contrib (WS + REST)     | ~3    | Mixed |
| Webhooks (outbound)     | ~4    | Yes (register/list/delete are CRUD) |
| Events / SSE streams    | ~4    | No — streaming |
| Telemetry intake        | ~2    | Probably stays REST |
| Feedback                | ~1    | Yes (mutation) |
| Webhook receiver (Ko-fi)| ~1    | No — third-party defines body |
| HTML share pages        | ~6    | No — text/html + OG meta |
| Search + meta + tags    | ~5    | Yes |
| CSRF token              | ~1    | Stays REST (cheap, one-shot) |
| Misc (leaderboard etc.) | ~5    | Yes |

---

## What CANNOT migrate to GraphQL (~20 endpoints)

Listed so the migration plan stops trying to be exhaustive.

### Streaming transports (8 endpoints)

GraphQL has subscriptions, but the `graphql-go` library's
subscription support is thin (no resolver-level streaming, no
WebSocket multiplexing primitives). The team's already-deployed
streaming surfaces use raw WebSockets / SSE with bespoke wire
formats, and the consumers (React SPA + Discord bots + the
`/ws/events` clients we just shipped) are tuned for that shape.

- `/ws/live` — spectator game state
- `/ws/spectate/{room_id}` — per-room spectator
- `/ws/events` — generic events stream (PR #345)
- `/api/contrib/connect` — contributor compute WebSocket
- `/api/tournaments/{owner}/{id}/events` — SSE gauntlet progress
- `/api/decks/{owner}/{id}/events` — SSE deck events
- (Plus future SSE / WS surfaces we add)

**Verdict:** stay REST/WS. Document them in the OpenAPI spec as
"transport: streaming" and call it done.

### Binary media (2 endpoints)

GraphQL is JSON-over-HTTP; both of these return bytes.

- `/api/card-art/{name}` — JPEG/PNG card images. Heavily cached at
  the CDN; the absolute hottest endpoint by request volume in the
  SPA. Moving this would require base64-in-GraphQL (huge payload
  blow-up) or out-of-band fetching (defeats the purpose).
- `/api/games/{id}/summary.pdf` — rendered PDF. Same story.

**Verdict:** stays REST. GraphQL would only need an `imageUrl`
field that returns the REST URL — which is what we already do
implicitly in the React layer.

### HTML share pages (6 endpoints)

These exist so Discord / Twitter / Mastodon unfurlers can fetch a
crawler-friendly `text/html` document with `<meta property="og:*">`
tags. The unfurlers don't speak GraphQL.

- `/decks/{owner}/{id}` — deck OG page
- `/share/{owner}/{id}` — legacy share alias
- `/cards/{name}` — card OG page
- `/operator/{owner}` — operator profile OG
- `/spectate` — live spectator landing
- `/leaderboard` — leaderboard landing

**Verdict:** stays REST. The data source feeding the OG meta
can migrate (the share-page handlers call existing helpers) —
but the route itself has to keep returning HTML.

### External webhook receivers (1 endpoint)

- `/api/kofi/webhook` — Ko-fi POSTs `application/x-www-form-urlencoded`
  with a JSON `data` field. We don't control the producer; can't
  move it to GraphQL.

### Fire-and-forget telemetry (2 endpoints)

- `/api/telemetry/pageview` — anonymous pageview ping. Bulk-burst
  from the browser; GraphQL would add per-call overhead (parse +
  validate the document) for what is essentially `INSERT INTO
  pageviews`. Negative payoff.
- `/api/telemetry/stitch` — anon-session → owner stitch on login.
  Same argument.

**Verdict:** stays REST.

### CSRF issuance (1 endpoint)

- `/api/csrf` — issues a one-shot HMAC'd token. The token guards
  destructive operations; once mutations move to GraphQL, this
  endpoint either stays REST (as a bootstrap) or merges into a
  GraphQL meta query. Either is fine; keeping it REST is simpler.

---

## What CAN migrate (~65 endpoints)

The list — these are all JSON-over-HTTP CRUD where GraphQL's
field-projection + batching ergonomics are real wins.

### Tier 1 — Hot-path reads (highest payoff)

These are called on every SPA page load / interaction. Field
projection alone would shrink wire size 60-80% for many of them.

- `GET /api/decks` — deck list page
- `GET /api/decks/{owner}/{id}` — deck detail
- `GET /api/decks/{owner}/{id}/matchups`
- `GET /api/decks/{owner}/{id}/elo-history`
- `GET /api/decks/{owner}/{id}/upgrade`
- `GET /api/decks/{owner}/{id}/archive`
- `GET /api/cards/{name}` — card detail popover (high frequency)
- `GET /api/cards/{name}/stats`
- `GET /api/cards/{name}/performance`
- `GET /api/card-stats/card/{cardName}`
- `GET /api/card-stats/card/{cardName}/by-commander`
- `GET /api/games` — recent games
- `GET /api/games/{id}/summary`
- `GET /api/games/{id}/replay`
- `GET /api/leaderboard`
- `GET /api/meta`
- `GET /api/profile`, `/api/profile/{owner}`, `/api/profiles`
- `GET /api/owner/{owner}/stats`, `/api/owner/{owner}/games`
- `GET /api/search`, `/api/cards/search`

**Cost estimate:** ~15 working days. Each endpoint becomes a
resolver; the data layer is already factored (most of these are
already `Showmatch.GetX` or `Handler.loadX` helpers).

### Tier 2 — Cross-resource batching wins

These are the queries where the SPA today fires 2-3 REST calls in
parallel to assemble one screen. GraphQL collapses them.

- Deck detail page: deck + matchups + elo-history + upgrade
- Game replay page: game + summary + per-seat deck info
- Operator page: owner profile + recent games + ELO history
- Card detail page: card + stats + by-commander + art URL

**Cost estimate:** the GraphQL side is "free" once Tier 1 is done
(it's just nested resolvers). The wins come from React-side
refactors that consolidate parallel REST calls into one GraphQL
operation — that's where the engineering hours go.

### Tier 3 — Mutations

GraphQL mutations need the same auth gating REST already has
(`X-HexDek-Owner` header + CSRF for destructive ops).
Implementation pattern: each mutation resolver checks ownership /
CSRF before delegating to the existing handler internals.

- `createDeck(input)` ← `POST /api/decks`
- `updateDeck(owner, id, input)` ← `PUT /api/decks/{owner}/{id}`
- `patchDeck(owner, id, input)` ← `PATCH /api/decks/{owner}/{id}`
- `deleteDeck(owner, id)` ← `DELETE /api/decks/{owner}/{id}`
- `cloneDeck(owner, id)` ← `POST /api/decks/{owner}/{id}/clone`
- `forkDeck(owner, id)` ← `POST /api/decks/{owner}/{id}/fork`
- `runAnalysis(owner, id)` ← `POST /api/decks/{owner}/{id}/analyze`
- `importMoxfield(url, owner)` ← `POST /api/import/moxfield`
- `startGauntlet(owner, id, games)` ← `POST /api/gauntlet/{owner}/{id}`
- `patchCurse(owner, id, knobs)` ← `PATCH /api/decks/{owner}/{id}/curse`
- `registerWebhook(input)` ← `POST /api/webhooks`
- `deleteWebhook(id)` ← `DELETE /api/webhooks/{id}`
- `submitFeedback(input)` ← `POST /api/feedback`
- `setLiveSpeed(multiplier)` ← `POST /api/live/speed`
- `spawnSpectateRoom(input)` ← `POST /api/spectate/spawn`

**Cost estimate:** ~10 working days. The CSRF middleware needs a
GraphQL-aware variant; everything else is wrapping.

### Tier 4 — Admin / low-traffic

Low-priority migration. Can stay REST forever without anyone
noticing.

- `/api/admin/verifications`
- `/api/admin/sanctions`
- `/api/admin/anticheat/stats`
- `/api/admin/conviction-events`
- `/api/admin/anomalies` + resolve

**Verdict:** defer indefinitely. Migrate only if a strong reason
emerges.

---

## Most-used endpoints (inference; needs measurement before commit)

We don't have request analytics today. Inferring from React SPA
call patterns:

| Endpoint | Why it's hot | Stays / migrates |
|----------|--------------|------------------|
| `GET /api/card-art/{name}` | Every card render in the SPA | **Stays REST** (binary) |
| `GET /api/cards/{name}` | Card popovers | Migrates |
| `GET /api/decks` | Deck list page | Migrates |
| `GET /api/decks/{owner}/{id}` | Deck detail | Migrates |
| `GET /api/meta` | App boot | Migrates |
| `GET /api/profile` + `/api/profiles` | Auth header + chat avatars | Migrates |
| `GET /api/leaderboard` | Homepage | Migrates |
| `GET /api/games` | Recent games | Migrates |
| `GET /api/games/{id}/summary` | Post-game UX | Migrates |
| `POST /api/telemetry/pageview` | Every nav | **Stays REST** (fire-and-forget) |
| `GET /ws/live` + `/ws/events` | Live spectator | **Stays WS** |

**Before committing to the roadmap:** instrument request counting
(an interceptor that bumps a `prometheus`-style counter per route,
or a one-week sample of nginx logs). The roadmap below assumes the
above ordering; the team should validate with real numbers before
investing in Tier 1.

---

## Hardest to migrate

In rough order of difficulty:

1. **`/api/decks/{owner}/{id}/upgrade`** — the response carries
   `Suggestions []UpgradeSuggestion` plus a free-form `Meta map[string]any`
   reason field. The `Meta` map has at least four documented shapes
   depending on which short-circuit fired (no oracle DB, no card
   stats, deck-size mismatch, normal). GraphQL prefers structured
   types — modeling `Meta` as a union of cases is more work than
   the rest of the endpoint combined.

2. **`/api/games/{id}/replay`** — the response is the entire
   observation snapshot bundle: per-turn state, per-seat hands /
   battlefields, every triggered ability. A "give me the full
   replay" GraphQL query is fine; a "give me just turns 10-15
   of seat 2" query is much better, but the underlying data
   layer doesn't support sparse turn projection today. Migrating
   the endpoint is easy; getting the projection wins requires a
   `LoadReplaySlice(gameID, turnRange, seats)` helper that doesn't
   exist yet.

3. **Mutation auth interop** — the CSRF middleware
   (`RequireCSRF`) wraps individual REST handlers via composition.
   GraphQL mutations all enter through one HTTP route, so the
   middleware can't gate per-mutation without inspecting the
   parsed AST. Two options:
   - Apply CSRF to all `POST /graphql` requests that contain at
     least one mutation. Cheapest; protects everything; over-
     applies (forces CSRF on queries-via-POST).
   - Resolver-level: each mutation resolver checks
     `r.Context()` for a validated-CSRF marker set by middleware.
     Cleanest; requires plumbing context-passing through the
     `graphql.Params{Context: r.Context()}` call (which we already
     do for cancellation).

4. **Streaming endpoints that *look* RESTish but aren't** — e.g.
   `GET /api/decks/{owner}/{id}/analysis` returns
   `{"status":"analyzing"}` when an analysis is in-flight and the
   real document once it lands. That's a poor-man's subscription
   already; GraphQL subscriptions would model it better, but
   `graphql-go` doesn't make subscriptions easy. Migrate to a
   query that returns the same `{status, document}` envelope and
   move on.

5. **File-upload-shaped imports** — `/api/import/moxfield` POSTs a
   URL, but a future "import a `.txt` deck list" endpoint would
   want multipart. GraphQL supports multipart via the GraphQL
   Multipart Request spec, but adding it just for this one flow
   is overkill. Keep file-upload-shaped mutations in REST.

---

## Phased migration plan

Calendar months, not engineering days. Each phase ends with REST
still working — there is never a hard cutover.

### Phase 0 — Measurement (1 month)

Before committing to anything beyond, instrument the REST surface.
A simple `prometheus` counter middleware (`hexapi_requests_total{
route, method, status}`) tells us within a week which 5 endpoints
account for 80% of traffic. This data should rank Tier 1 below;
the rank we have today is inference.

**Deliverable:** a one-page top-20-endpoints table backed by real
counts. Nothing else.

### Phase 1 — Schema foundation (1-2 months)

Grow the GraphQL schema from PR #389's 2 types / 4 queries to a
complete read-only graph for Tier 1 endpoints (~20 endpoints).
Each new resolver is one PR.

**Acceptance:** Tier 1 endpoints all reachable via GraphQL. REST
still works. Frontend not migrated.

### Phase 2 — Mutations (1 month)

Add the ~15 mutations listed under Tier 3. Resolve the CSRF
interop story (recommend the resolver-level approach; document
the trade-off). Frontend still uses REST.

**Acceptance:** every REST mutation has a GraphQL equivalent.
Auth posture unchanged.

### Phase 3 — Frontend migration (2-3 months)

Move React SPA pages from REST to GraphQL one screen at a time.
Each screen migration is its own PR; the REST endpoints stay
mounted. Once a screen lands on GraphQL, instrument shows that
REST endpoint's request count drop.

**Acceptance:** zero REST callsites from the SPA for the migrated
Tier 1 + Tier 3 endpoints. External callers (Discord bots, the
opsblu site, etc.) may still be on REST.

### Phase 4 — External caller migration (3-6 months calendar)

Add `Deprecation:` + `Sunset:` headers to the REST endpoints that
have a GraphQL equivalent, per RFC 8594 + RFC 9745. Communicate
the timeline to known external consumers (Discord channel
announcement + the opsblu site's API docs). The 6-month window
is generous; it can shrink if usage drops faster.

**Acceptance:** REST endpoints with GraphQL equivalents are
serving <1% of their pre-deprecation traffic for two consecutive
weeks.

### Phase 5 — Removal (one PR)

Delete the REST handlers from `internal/hexapi/`. Update the
OpenAPI spec to mark them gone. The drift detector tests
(introduced in PR #267 + #285) catch any handlers that get
missed.

**Acceptance:** `mux.HandleFunc` count drops from ~104 to ~35
(the streaming + binary + HTML + webhook-receiver + telemetry
core that was never migrating).

---

## Cost/benefit summary

**Total engineering investment (Phase 1-5):** ~6-8 calendar months
at one engineer's part-time attention. Most of that is Phase 3
(frontend migration) — the schema work itself is small.

**Operational wins:**

- Field projection on the hot-path endpoints saves wire bytes
  (rough estimate: 40-60% reduction on `GET /api/decks/{owner}/{id}`
  for SPA queries that only need name + commander + bracket).
- Cross-resource batching on the deck-detail page collapses 4-5
  REST calls into 1 GraphQL operation.
- Schema-as-documentation supplements the OpenAPI YAML.
- One transport for all CRUD reduces the auth / middleware
  surface area.

**Risks:**

- `graphql-go` is a small library; if it goes unmaintained we
  inherit it or migrate to gqlgen mid-flight.
- Mutation auth interop has real complexity; a bug there has a
  wider blast radius than a per-endpoint CSRF bug today.
- Frontend migration is the long pole — every screen needs
  testing, every regression hits user-visible behaviour.

**What we'd be giving up:**

- The OpenAPI spec stops being a complete description of the API
  — Tier 1 endpoints become "see the GraphQL schema." We'd want
  to publish the GraphQL schema as a sibling artifact and update
  the validator middleware (PR #285) to understand both surfaces
  or skip GraphQL paths.

---

## Open questions for the team

1. **Is the field-projection win worth the migration tax?** If
   the SPA's bottleneck isn't JSON payload size, the case for
   GraphQL is weaker. Measurement (Phase 0) tells us.

2. **Subscriptions over GraphQL, or keep the WebSocket surface?**
   If the team wants ONE transport for "real-time engine events"
   AND "discrete queries," we'd need to swap `graphql-go` for
   `gqlgen` or a library with first-class subscription support.
   Recommend keeping the existing `/ws/events` + REST hybrid
   unless the SPA's developer ergonomics actively suffer.

3. **Do external consumers exist?** Need to know before
   committing to a deprecation timeline. If the only consumer is
   the React SPA, the deprecation phase shortens dramatically.

4. **Persisted queries?** Worth doing before turning on a public
   GraphQL endpoint — DoS risk from arbitrarily expensive
   queries is real. Out of scope for the POC but in scope before
   Phase 4.

---

## Recommendation

**Spend Phase 0 (one month, ~5 days engineering).** Instrument
the REST surface. Look at the actual numbers. If the top 10
endpoints by traffic are mostly `card-art` (binary, won't move)
and `telemetry/pageview` (fire-and-forget, won't move), the
migration's payoff is smaller than the engineering cost — keep
the GraphQL POC as a niche tool for the few cross-resource
queries that benefit from it, and don't go further.

If the top 10 are dominated by `decks` / `games` / `card-stats`
reads where field projection helps, commit to Phase 1.

Either way: **don't promise external consumers a REST sunset
until Phase 3 ships.** The migration is a 6+ month project; the
team should learn how it actually feels before betting external
contracts on it.
