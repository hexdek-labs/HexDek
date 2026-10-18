# HexDek API Reference

Index of every public API HexDek exposes, with quick-start examples for
each protocol. Three surfaces serve different needs:

- **REST** (PR #267) — primary CRUD surface for decks, games, friends,
  party + showmatch / gauntlet controls. ~127 endpoints, all under
  `/api/`. The canonical contract is the OpenAPI spec at
  `docs/hexapi-openapi.yaml`.
- **GraphQL** (PR #334) — **NOT MOUNTED. Code only.** A query-side
  overlay for cross-cutting fetches (e.g. "deck + analysis + ELO +
  matchups in one round-trip"). The handlers are implemented and
  tested, but `RegisterGraphQL` and `NewGraphQLSubscriptionsHandler`
  have **zero non-test callers** — `cmd/hexdek-server` never mounts
  them, and `/graphql` and `/graphql/subscriptions` both return **404
  on production** (verified against the live server 2026-10-18).
  `graphql.go:94` records the reason: deliberately kept off the
  default `Handler.Register` chain while GraphQL is a POC. Mounting it
  is an owner decision, not a doc fix — it would expose
  `importDeck`/`deleteDeck` mutations.
- **WebSocket streaming** (PR #338) — live event bridge over the
  internal EventBus. One mounted transport: `/ws/events` (raw events,
  REST-adjacent). (`/graphql/subscriptions` is written but unmounted —
  see above.) Also `/ws/showmatch/...` for spectator streams and
  `/ws/party/{id}` for live game state.

## REST quick-start

Default base URL: `http://localhost:8090` (DARKSTAR production:
`http://192.168.1.207:8090`). Browse decks owned by `josh`:

```bash
curl -s http://localhost:8090/api/decks | jq '.[] | select(.owner=="josh") | .name'
```

POST a deck import:

```bash
curl -X POST http://localhost:8090/api/decks/import \
  -H 'Content-Type: application/json' \
  -d '{"owner":"josh","name":"My Deck","format":"commander","raw":"1 Sol Ring\n1 Command Tower\n..."}'
```

Fetch a Freya analysis snapshot:

```bash
curl -s http://localhost:8090/api/decks/josh/my_deck/analysis | jq '.archetype, .bracket, .power_tier_counts'
```

Compare two decks:

```bash
curl -s 'http://localhost:8090/api/decks/compare?a=josh/deck_a&b=josh/deck_b' | jq
```

### REST endpoint categories

| Category | Sample route | Doc |
|---|---|---|
| Decks (CRUD + analysis) | `GET /api/decks/{owner}/{id}/analysis` | `docs/freya-api.md` |
| Deck workshop / archive | `GET /api/decks/{owner}/{id}/archive` | (registered in `internal/hexapi/handler.go`) |
| Friends / device | `POST /api/devices` | `docs/stub-hunt-frontend-api-r46.md` |
| Party + game | `POST /api/party/create`, `POST /api/party/{id}/start_game` | `internal/party/handler.go` |
| Showmatch + spectator | `POST /api/live/speed`, `POST /api/spectate/spawn` | `internal/hexapi/spectate_rooms.go` |
| Gauntlet runner | `POST /api/gauntlet/{owner}/{id}` | `internal/hexapi/showmatch.go` |
| Game replay | `GET /api/games/{id}/replay`, `GET /api/games/{id}/summary` | `internal/hexapi/game_summary.go` |
| Card oracle | `GET /api/oracle/card/{name}` | `internal/hexapi/oracle.go` |
| Telemetry + feedback | `POST /api/telemetry/pageview`, `POST /api/feedback` | (registered in `internal/hexapi/handler.go`) |

Complete machine-readable list: `docs/hexapi-openapi.yaml`. The spec
is self-validating at server boot (see `openapi_validate.go`); any
endpoint registered in code but undocumented in YAML fails the
`TestOpenAPICoverage_AllRegisteredRoutesAreDocumented` test.

## GraphQL quick-start

> **These examples do not work against a running HexDek server.**
> Neither route is mounted; both 404. They describe the shape the
> handlers implement, and are reachable only from a test that calls
> `RegisterGraphQL` on its own mux. Everything below is a
> specification of unmounted code, not a usable API.

Routes the code defines (unmounted): `POST /graphql` (queries +
mutations) and `GET /graphql/subscriptions` (WS-transport
subscriptions).

A deck-page fetch — name + commander + bracket + ELO + recent games
in one round-trip:

```bash
curl -s -X POST http://localhost:8090/graphql \
  -H 'Content-Type: application/json' \
  -d '{"query":"{ deck(owner:\"josh\", id:\"my_deck\") { name commanderName analysis { archetype bracket } elo { rating games } recentGames(limit:5) { winnerName turns endReason } } }"}' | jq
```

Mutation example — submit archetype feedback for an analysis:

```bash
curl -s -X POST http://localhost:8090/graphql \
  -H 'Content-Type: application/json' \
  -H 'X-CSRF-Token: <token-from-/api/csrf>' \
  -d '{"query":"mutation { submitArchetypeFeedback(owner:\"josh\", id:\"my_deck\", correctArchetype:\"combo\") { accepted } }"}'
```

Schema lives in `internal/hexapi/graphql_schema.go`. Three root types:

- `Query` — Deck, ELOEntry, GameSummary, MatchupMatrix, ...
- `Mutation` — submitArchetypeFeedback, voteDeckFavorite, ...
- `Subscription` — **`gameEnded` only.** (This list previously read
  "gameTurns, eloUpdates, gauntletProgress". None of those three exist
  in `graphql_schema.go`; the Subscription root has exactly one field,
  which fires once per completed game.)

See [`docs/rest-to-graphql-roadmap.md`](rest-to-graphql-roadmap.md)
for which REST endpoints are mirrored, which are pending, and the
overall convergence plan.

## WebSocket quick-start

### `/ws/events` — raw event bridge (PR #338)

```bash
wscat -c ws://localhost:8090/ws/events
# server sends JSON events as they fire on the internal bus
```

Recommended subscriber pattern (filter for game-finished only):

```bash
wscat -c 'ws://localhost:8090/ws/events?kind=game_finished'
```

Connection cap: 100 concurrent subscribers per process (see
`maxEventSubscribers` in `events_ws.go`).

### `/graphql/subscriptions` — typed GraphQL subscriptions (UNMOUNTED)

> **This endpoint returns 404.** The command below fails twice over:
> the route is not registered, and `eloUpdates` is not a field on the
> Subscription root. The only subscription field is `gameEnded`.

```bash
# Shape only — NOT runnable against a live server.
# sub-protocol: graphql-transport-ws
wscat -c ws://localhost:8090/graphql/subscriptions -s graphql-transport-ws
> {"type":"connection_init"}
> {"id":"1","type":"subscribe","payload":{"query":"subscription { gameEnded { winnerName turns endReason } }"}}
```

Each `next` message carries the same JSON shape as a one-shot
`POST /graphql` query — apps can reuse their query types.

### `/ws/showmatch/{room_id}` — live spectator stream

```bash
wscat -c 'ws://localhost:8090/ws/showmatch/sr-abc123'
# server pushes turn-by-turn snapshots until the room closes
```

Used by the spectator frontend (`hexdek/src/screens/SpectateScreen.jsx`).
Rooms are spawned via `POST /api/spectate/spawn`.

### `/ws/party/{id}` — party live state

```bash
wscat -c 'ws://localhost:8090/ws/party/X9F3JL'
# server fans out snapshots after each action; sends initial snapshot on connect
```

Used by the party-play frontend. Pairs with the REST `/api/party/{id}`
GET for cold-start state, then upgrades to live WS for the in-game
update stream.

## Auth + CSRF

- Read endpoints (GET) under `/api/` are public by default in dev.
- Mutating endpoints require an `X-CSRF-Token` header. Get one from
  `GET /api/csrf` (sets a cookie + returns the token); see
  `internal/hexapi/csrf_store.go`.
- Party + device endpoints use device-ID cookies rather than the CSRF
  pair — the cookie is bound to the device on first `POST /api/devices`.

## Versioning

REST + GraphQL are unversioned in URL. Breaking schema changes get
their own `*_v2.go` companion handler and the spec gains both versions
for the deprecation window. WS protocols are versioned via the
sub-protocol string (`graphql-transport-ws` is v1; future bumps would
be `graphql-transport-ws-v2`).

## Cross-references

- [`docs/hexapi-openapi.yaml`](hexapi-openapi.yaml) — machine spec
- [`docs/freya-api.md`](freya-api.md) — deck-analysis REST surface
- [`docs/rest-to-graphql-roadmap.md`](rest-to-graphql-roadmap.md) —
  migration plan
- [`docs/stub-hunt-frontend-api-r46.md`](stub-hunt-frontend-api-r46.md)
  — frontend-targeted endpoint additions
- [`docs/composition-elo.md`](composition-elo.md) — TrueSkill +
  composition prior reference
- [`docs/CONTRIBUTING.md`](CONTRIBUTING.md) — local dev / testing /
  card-handler authoring
