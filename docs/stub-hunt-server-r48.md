# Stub Hunt R48 — `cmd/hexdek-server/`

**Date:** 2026-05-20
**Branch:** `dev/stub-hunt-server-r48`
**Scope:** Server entry point + HTTP wiring + WebSocket bootstrap. Single file: `cmd/hexdek-server/main.go` (358 LoC). No tests in the package.

## Methodology

1. Read `main.go` end-to-end — flag setup → DB open → mux wiring → middleware → ListenAndServe.
2. Cross-check `LoadCardDB` / `LoadOwnerAliases` / `OnGameStart` signatures and call patterns into `internal/hexapi`, `internal/ai`, `internal/party`.
3. Trace every `_ = ...`, ignored-return, and `context.Background()` callsite.
4. Run the file through the standard server-hardening checklist: timeouts, CORS, graceful shutdown, pprof gating, static-file paths.

Result: **no classic TODO markers, but the file carries the usual server entry-point loose ends** — default timeouts, CORS cache miss, swallowed pprof error, no signal handling, one hardcoded path that should follow a flag.

## Findings, by severity

| # | Sev | Line(s) | Issue | In this PR |
|---|---|---|---|---|
| 1 | **High** | 343-357 | `corsMiddleware` echoes the request's `Origin` header without setting `Vary: Origin`. Any cache (CDN, browser) keys the response by URL alone, so a response cached for one allowed origin is served to a *different* origin (or to `null`) — silently breaking CORS or leaking allowed-origin behavior to disallowed callers. Also missing `Access-Control-Allow-Credentials: true` despite the cookie-based pincer session (the `pincer-secure-cookie` flag at line 43 proves cookies are in play) — XHR/fetch with `credentials: 'include'` strips the cookie. | **Yes** |
| 2 | **High** | 204-206 | `http.ListenAndServe(*addr, handler)` uses the package default `http.Server` — **no `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, or `ReadHeaderTimeout`**. Classic slow-loris exposure: a client that opens a TCP connection and sends bytes very slowly ties up a server goroutine indefinitely. | **Yes** |
| 3 | **Med** | 56-61, 204 | `defer database.Close()` is registered, but every error path uses `log.Fatalf` (which calls `os.Exit(1)` and bypasses deferred functions) — including the final `ListenAndServe` error. SIGINT/SIGTERM also bypass the defer. Add signal-based graceful shutdown via `http.Server.Shutdown`. | **Yes** |
| 4 | **Med** | 145-151 | `go func() { http.ListenAndServe("127.0.0.1:6060", nil) }()` — pprof goroutine drops the returned error. If port 6060 is in use (very common when running multiple dev instances) the goroutine silently exits and the operator sees the "pprof: listening" log message without pprof actually being available. | **Yes** |
| 5 | **Med** | 39 + 157-159 | `--deck` flag controls which deck is shuffled by the `/game/test/*` Ship 1 endpoints, but the convenience route `GET /api/test/yuriko-deck-json` hardcodes `"data/decks/yuriko_v1.json"`. Operator who sets `--deck=mydeck.json` sees the test endpoints work on `mydeck.json` while the served JSON is still the default — quiet UI/data divergence. | **Yes** |
| 6 | Low | 19 | Import block order: `"strings"` precedes `"strconv"` alphabetically. `goimports` would fix it. | Yes |
| 7 | Low | 154 | `mux.Handle("GET /ui/", http.StripPrefix("/ui/", http.FileServer(http.Dir("web"))))` — relative path `"web"`. If the server runs from any cwd other than the project root, this silently 404s. Same class as #5 but the path is a deployment convention; flagging only. | Skip |
| 8 | Info | 99-103 | `ai.Start(context.Background(), ...)` — autopilot goroutines per game can't be cancelled by server shutdown. With graceful shutdown landing in this PR, plumbing a derived `serverCtx` into `OnGameStart` would let them drain. Out of scope (needs `ai.Start` signature consideration). | Skip |
| 9 | Info | 52, 83 | `Handler.LoadCardDB` / `Handler.LoadOwnerAliases` return no error; failure-mode is log-and-continue. Intentional best-effort init (matches the `os.IsNotExist` warning style on the deck path at line 51). | Skip |
| 10 | Info | 209-214 | `server` struct mixes Ship 1 demo state (`deck`, `library`) into the same file as Ship 2-7 wiring. Worth splitting once Ship 1 endpoints retire, but not a bug. | Skip |
| 11 | Info | 237-251 | `handleRoot` writes `text/plain`-style output without setting `Content-Type`. Browsers may auto-detect, IE-style sniffing could pick HTML. Cosmetic. | Skip |
| 12 | Info | 195 | `hexapi.NewSpotCheckRunner(contribAST, contribAST, contribOracle)` — `contribAST` passed twice. Suspicious-looking but it's the spot-check runner's documented `(astPath, metaPath, oraclePath)` signature with the same JSONL file used for both AST and meta extraction. Not a bug. | Skip |

## Deliberately not flagged

- `defer database.Close()` itself is fine; the issue is that `log.Fatalf` skips defers — that's a process-lifecycle problem, not an unclosed handle.
- Use of relative paths (`data/...`, `web/`) — deployment convention; the project ships with these layouts.
- `flag.String` defaults reading from `os.Getenv` (lines 42, 165) — fine pattern.
- `_ = enc.Encode(payload)` in `handleTopN` — the early-return error is logged, so this is reasonable.

## Fixes shipped

### 1. CORS: add `Vary: Origin` + `Access-Control-Allow-Credentials`

Caches now key responses correctly per-origin, and frontend XHRs with `credentials: 'include'` retain the pincer session cookie.

### 2. HTTP server timeouts

Replace `http.ListenAndServe(*addr, handler)` with an explicit `&http.Server{...}` carrying `ReadHeaderTimeout: 10s`, `ReadTimeout: 30s`, `WriteTimeout: 60s`, `IdleTimeout: 120s`. WebSocket upgrades are unaffected — `coder/websocket.Accept` hijacks the connection, so the server's per-request timeouts no longer apply once the connection is hijacked.

### 3. Graceful shutdown

`server.ListenAndServe()` runs in a goroutine; main blocks on a `signal.Notify` channel for SIGINT/SIGTERM, then calls `srv.Shutdown(ctx)` with a 15-second drain window before letting the deferred `database.Close()` run. Existing connections get to finish; new connections are refused.

### 4. pprof bind error surfaced

The goroutine now logs `ListenAndServe` failures explicitly with a clear "pprof endpoint disabled" message, so an in-use port doesn't masquerade as a working pprof.

### 5. `/api/test/yuriko-deck-json` follows `--deck` flag

Captured `*deckPath` is now served, not the hardcoded `data/decks/yuriko_v1.json`. The route name keeps the historical `yuriko-deck-json` slug for client back-compat.

### Bonus tweak

- Import block reordered to alphabetical (`strconv` before `strings`).

## Tests

- `go build ./...` clean.
- `go test ./...` (no package-level tests for `cmd/hexdek-server`; ran scoped to `internal/hexapi`, `internal/party`, `internal/ws`, `internal/hub` — all green).
- Manual smoke: not run (no operator authorization in this turn).

## Open follow-ups

- Plumb a cancellable `serverCtx` into `OnGameStart` → `ai.Start(...)` so in-flight autopilots drain on shutdown.
- Replace `log.Fatalf` early-init calls with `return err` from a wrapped `run() error` so defers fire on init failures (Go server idiom).
- The relative `"web"` static dir and `"data/decks/imported"` cache dir should be made configurable via flag.
- `allowedOrigins` map is a literal in code; consider moving to flag/env so deploys can add origins without rebuilding.
