# Parity Tool Audit — R60

> Scope: `internal/paritycheck/paritycheck.go` (733 LOC) +
> `cmd/hexdek-parity/main.go` (114 LOC) + `internal/paritycheck/paritycheck_test.go` (143 LOC).
>
> Reviewed against the actual repo state on `main` at R60; no code
> changes shipped with this audit — see "Recommended Next Steps" for
> the action queue.

## Headline

**The parity tool is structurally non-runnable.** The Python reference
harness it diffs against (`scripts/parity_harness.py`) was removed in
commit `8f21aff` ("python-purge: remove 30K lines of legacy Python
engine code"). Every invocation of `hexdek-parity` since that commit
has taken the `python_available: false` path — record the Go side,
skip the diff, write a report whose only useful row is the Go
outcomes. The tool *appears* operational (clean exit, markdown
written, no errors logged) which makes the gap silent.

`docs/architecture/Tool - Parity.md` documents the broader intent
("archived as Go reaches feature parity") but does not flag that the
harness itself is gone. The CLI still defaults
`--python-harness scripts/parity_harness.py`; `paritycheck.Run`
silently downgrades on `os.Stat` failure.

## What Parity Currently Checks (Assuming Python Harness Existed)

The diff path in `Diff()` (paritycheck.go:609) considers exactly three
dimensions:

| # | Dimension | Source | Strength |
|---|---|---|---|
| 1 | `Outcome.Winner` equality | `outcomesEqual` at paritycheck.go:592 | hard match |
| 2 | `Outcome.Turns` equality | inline check at paritycheck.go:661 | hard match, separate `turn_count` category |
| 3 | Per-`Event.Kind` histogram counts | `eventKindCounts` at paritycheck.go:673 | order-insensitive; counts only |

Everything else captured in the canonical `Event` and `Outcome`
schemas is collected but never compared.

## What's Captured But Not Compared

`Event` carries these fields (paritycheck.go:54) — all are normalized
on both sides, none participate in the diff:

- `Seq` — monotonic sequence number
- `Turn` — turn number per-event (separate from outcome turn count)
- `Phase` — CR §500 phase
- `Step` — step within phase
- `Seat` — owning seat
- `Source` — source card name
- `Target` — target seat
- `Amount` — numeric payload (damage / life / cards)
- `Rule` — CR citation when present

`Outcome` carries these fields (paritycheck.go:98), of which only
`Winner` + `Turns` are diffed:

- `WinnerName` — commander name
- `EndReason` — `last_seat_standing` / `draw` / `turn_cap_leader` / `turn_cap_tie` / `turn_cap_all_dead`
- `LifeTotals` — final life per seat
- `LostBySeat` — per-seat elimination flag

`outcomesEqual` (paritycheck.go:592) explicitly punts on `EndReason`:

> // We don't strictly require turn counts / end reasons match for
> // outcome parity — Python and Go may round differently on "when did
> // the game end" depending on which SBA pass the final kill hits.

Reasonable as written, but the consequence is that the only outcome
field checked is the winner index. A game where Go ends 4 turns later
than Python with three different seats eliminated in a different
order — but the same seat surviving — counts as a *match*.

## Gap Ranking (Worst First)

### 1. Python harness missing — tool can never run end-to-end

The diff path is unreachable. Every parity invocation hits
`paritycheck.go:209-211`:

```go
if _, err := os.Stat(cfg.PythonHarnessPath); err != nil {
    pythonAvailable = false
}
```

…sets `pythonAvailable=false`, runs only the Go side, and reports
nothing. This is invisible to the operator unless they read
`python_available: false` in the markdown output.

**Severity:** blocker. Nothing else in this audit matters until this
is resolved one way or the other.

**Resolution paths:**
- *Restore the harness* if the Python reference is still considered
  useful as a regression backstop (`docs/architecture/Tool - Parity.md`
  claims it is). Recovering `scripts/parity_harness.py` from commit
  `8f21aff~1` gives a 171-line starting point, though the Python
  engine itself is also gone so a parallel restore is required.
- *Retire the tool* if the Python reference is genuinely archived.
  Delete `cmd/hexdek-parity/`, `internal/paritycheck/`, and the
  architecture doc, leave a note in `CLAUDE.md` that engine
  regression coverage now lives in `internal/gameengine/` unit tests
  + Loki fuzz + Goldilocks.
- *Repurpose* — rename to `hexdek-replay` and turn the existing
  Go-side recorder into a deterministic replay-capture tool for use
  by Heimdall / Loki post-mortems. Drops the comparison surface
  entirely and admits what the tool actually does now.

### 2. Outcome diff ignores `LifeTotals`, `LostBySeat`, `EndReason`

Even within the existing diff design, three captured fields are
silently dropped. A meaningful divergence shape — same winner
emerging with very different mid-game pressure — produces zero
divergence rows. The fix is mechanical: extend `outcomesEqual` (or
add a second comparator that emits per-field divergences instead of
a binary equal/not-equal) to walk all four outcome fields and emit
one `Divergence` per mismatched field.

`EndReason` specifically deserves comparison *with tolerance* — the
existing comment correctly notes Python and Go differ on which SBA
pass kills the last seat, but `last_seat_standing` vs
`turn_cap_leader` is a meaningful semantics divergence; the punt
should be narrower than "any end_reason mismatch is OK."

**Severity:** high. Closing this would catch the exact class of
divergence parity is meant to catch.

### 3. Event-kind histogram is the only event-stream signal

The diff reduces both event streams to `map[kind]count` and reports
when counts differ. This catches "Python emitted 12 draws, Go emitted
11" but misses everything that gives the count its meaning:

- **Per-seat drift** — `draw=12` on both sides but split [3,3,3,3] in
  Python vs [3,4,2,3] in Go is invisible.
- **Per-turn drift** — turn 1 draws differ but turn 8 draws differ
  the other way, net zero.
- **Source-card attribution** — `cast=10` on both sides but the cards
  cast are completely different.
- **Amount drift** — `combat_damage=4` on both sides but Python deals
  [3,1] and Go deals [2,2].

The right tool is a Needleman-Wunsch-style alignment on the canonical
event stream with a configurable cost function — but even a much
simpler `(kind, seat, turn)` triple-key histogram would surface
nine-tenths of the per-seat drift the current diff hides.

**Severity:** medium. The current report is reassuring without being
informative; a `--strict` flag that bucketed by `(kind, seat, turn)`
would be a 30-line change.

### 4. `normalizeKind` is a 17-line hand-written synonym table

Maintained as a manual rolodex (`enter_battlefield`, `etb`, `etbed`,
`enters_battlefield` → `enter_battlefield`; `combat_damage_dealt`,
`combat_damage` → `combat_damage`; etc.). When Python emits a kind
the table doesn't know about, the kind goes through unmolested and
gets diffed against the Go side raw. This is the right shape for now
— there are roughly a dozen entries — but it pre-dates
`internal/gameengine/event_aliases.go` (`NormalizeEventSingle`), the
canonical event-alias table the engine itself uses for trigger
dispatch.

**Severity:** low. The two tables don't have to agree (parity
normalization may want to be looser than dispatch normalization), but
the duplication is a future drift hazard.

### 5. `paritycheck_test.go` doesn't test the full pipeline

Coverage is on the small helpers — `Diff` with hand-built replays,
`normalizeKind`, `WriteMarkdown`, `parsePythonReplay`. There's no
end-to-end test that runs a real Go game through `RecordGoGame` and
asserts the event stream looks reasonable. That's fine when the
diff dimensions are themselves under-tested (catching a regression
in `RecordGoGame` would surface as a divergence in the absent
diff path), but it means the Go-side recording surface is also
unverified.

**Severity:** low. Worth covering once #1 is resolved.

## Recommended Next Steps

In priority order:

1. **Decide #1.** Pick one of {restore harness, retire tool,
   repurpose to replay-capture}. The audit can't proceed past this
   choice; each path implies different work below.

2. **If restoring:** add LifeTotals + EndReason + LostBySeat to
   `outcomesEqual` (gap #2) before re-running the harness, otherwise
   the first new parity run will under-report.

3. **If retiring:** delete the package + cmd + architecture doc;
   open the deletion PR with this audit as the rationale.

4. **If repurposing:** rename to `hexdek-replay`, drop the `Diff`
   path entirely, keep `RecordGoGame` + `normalizeGoEvents` + the
   markdown writer (renamed). The current tool already implements
   90% of a deterministic replay-capture surface.

5. **Gap #3 stretch goal:** when the diff is being re-enabled,
   change the histogram key from `Kind` to `(Kind, Seat, Turn)` for
   per-seat drift visibility, behind a `--strict` flag.

## Out of Scope

- The CLI flag surface (paritycheck.go has stable flags; no churn
  needed).
- The deck loading path (paritycheck.go:185-204 — exercises the same
  astload + deckparser code the tournament runner uses; any bugs
  surface there first).
- The Go-side event-recording fidelity (`RecordGoGame` and
  `normalizeGoEvents` are direct subsets of `tournament.runOneGame`
  + the engine's `EventLog`; their parity surface is the same as the
  production runner's).

## Reproducing This Audit

```
grep -nE "outcomesEqual|Outcome\." internal/paritycheck/paritycheck.go
ls scripts/parity_harness.py            # confirms absence
git show 8f21aff --stat | grep parity   # confirms purge commit
go test ./internal/paritycheck/ -v      # current test surface
```
