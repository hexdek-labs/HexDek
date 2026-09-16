# 2026-09-15 — the silent-failure sweep

Five defects found and closed in one session. They are written up together
because they are the same bug, wearing five costumes:

> **A component knew something and had no way to say it.**

In every case the build was green, the tests passed, the operation reported
success, and a user got a wrong or missing result. Not one of them raised an
error. That is what made them expensive: the repairs were all cheap — a
version stamp, a timestamp, a counter, an honest error string — and the cost
was entirely in the interval before anyone knew.

This document records what each one was, how it was found, and what was done,
so the pattern is recognisable the next time it appears.

---

## 1. Freya's report cache was not invalidated by code changes

**What it was.** The Freya analysis cache is keyed on `(deck contents +
FreyaVersion)`, where `FreyaVersion` was a hand-maintained constant with a
comment asking the author to remember to bump it. Nothing in the key described
the code. A classifier fix therefore reached nobody until someone remembered.

**How it surfaced.** A `cuttable_cards` fix was made, the binary rebuilt, and
the output was byte-identical except for a timestamp. Every deck was serving
an r60.1 cache entry to an r60.2 binary. The only signal was that the binary
provably did not contain the string its own output was printing.

**What was done.** The token is now derived from the build
(`cmd/hexdek-freya/version.go`):

- clean build with VCS info → `base + short revision`, so any commit
  invalidates the corpus automatically;
- build from a modified tree → the cache is **disabled outright**, because the
  revision cannot distinguish one edit from the next, and that is exactly the
  situation that produced the incident;
- no VCS info → fall back to the base constant, matching the old behaviour.

`FREYA_CACHE_PIN=1` restores the legacy behaviour for profiling runs.

**A limitation worth knowing.** The Go toolchain stamps **no `vcs.revision`
when building from a linked git worktree** (`.git` as a file). Verified, not
assumed: the same code, same command, stamps from the primary clone and stamps
nothing from a worktree. In that case the mechanism is inert and the old trap
is live again — so `--version` now prints the active regime rather than
leaving it to be inferred from behaviour:

```
hexdek-freya r60.2
  cache mode:   no-vcs (auto-invalidation inactive)
  cache in use: true
```

---

## 2. Stored analyses carried no provenance

**What it was.** `strategy.json` had 37 fields and none of them was a
timestamp or a version. "This analysis may be out of date" was not a claim
anything could make, however true it was — nothing on the page, in the hat, or
in a human's hands could tell whether a stored analysis reflected current
logic.

A user noticed this before we did, and was right to be suspicious: after
editing a deck they had no way to distinguish "the analysis re-ran and nothing
changed" from "the analysis never re-ran".

**What was done.** Freya stamps `freya_version` and `generated_at` on every
write. `hexapi` resolves the running binary's version once per process and
annotates `GET /api/decks/{owner}/{id}/analysis` with `current_freya_version`
and `stale`. When the version cannot be determined the response says nothing
at all — a missing binary must not nag every user that their analysis is
stale.

The deck page and Forge render a banner with a Refresh control.

---

## 3. "Refresh" did not refresh

**What it was.** `POST /api/decks/{owner}/{id}/analyze` re-ran Freya without
`--no-cache`. For an unchanged deck that is a ~5ms cache read that rewrites
the identical analysis and reports success.

A control that claims a refresh it did not perform is worse than no control:
it teaches the user that refreshing does not help, rather than that the
analysis is current.

**What was done.** Refresh routes through `runFreyaFresh`, which passes
`--no-cache`. The endpoint was already rate-limited per-IP and per-owner and
serialised behind the existing mutex, so the added cost is bounded.

---

## 4. Cards were dropped from analyses in silence

**What it was.** A deck-list entry the oracle corpus did not recognise was
skipped by a bare `continue` in the classification loop, plus a log line
nobody reads. Every downstream analytic — archetype, combos, curve, roles,
bracket — was then computed as if the card were not in the deck, and the
report looked complete and confident.

**What was done.** Two signals, reported separately because the consequence
differs:

| field | meaning | consequence |
|---|---|---|
| `unresolved_cards` | no oracle record at all | the analysis below was computed **without** the card |
| `unsupported_cards` | oracle knows it, Thor has no parsed AST | the analysis is correct; the **engine** does nothing with the card, so simulated results understate the deck |

Collapsing these into one "problem cards" list would state something false
about one of them.

**The guard that matters.** The unsupported check runs **only when an AST
corpus is actually loaded**, and a 0-card corpus is refused at load. Without
that, a truncated data file would mark every card in every deck unsupported —
a statement about our infrastructure presented to a user as a statement about
their deck. There are tests pinning the difference between "no corpus" and
"corpus says no".

---

## 5. The engine deck pool never learned about new decks

**What it was.** `buildDeckPool` ran once, at process start. `findDeckInPool`
walked only that slice. A deck imported after boot was invisible to the
gauntlet permanently, or until a human remembered to `POST
/api/showmatch/reload-pool`.

**How it surfaced.** A user reported that the gauntlet would not run. The pool
had been in that state for **~50 hours**: boot at 16:12 on 09-13, and the
first affected deck landed 18 minutes later. Every deck imported by anyone in
that window was un-gauntletable. Their deck parsed perfectly — commander
found, 99 library cards, zero unresolved — so it was never the deck.

**The part that made it expensive.** The error read:

> `deck not in engine pool — re-import or check the deck id`

Re-importing produces another deck that also lands outside the pool. The
reporting user re-imported three times, generating three orphaned decks,
because the message told them to. **A message that blames the user converts a
server-side bug into silence**, and that — not the pool logic — is the single
largest multiplier on the 50 hours.

**What was done.** Three changes, because there were three independent causes:

1. **Read-through** (`internal/hexapi/deckpool_readthrough.go`). On a pool
   miss, parse the deck file from disk and splice it into the live pool. This
   removes the failure class rather than adding another thing to remember: a
   scheduled or manual reload is a process control guarding a failure that
   raises no error, which is the same shape as item 1 above.

   The read-through applies the **same four gates as `buildDeckPool`, in the
   same order** (parse / commander present / ≥80 cards / not banned). A deck
   the startup path rejects must not enter by another door, or the two paths
   would disagree about what a legal deck is. The splice re-checks membership
   under the write lock, because a concurrent reload would otherwise give that
   deck two entries in the random-opponent draw.

2. **An honest error message.** It now says the problem is ours, and that
   re-importing will not help.

3. **A counter on `/api/health`** — `deck_pool.{misses, recovered, rejected}`.
   `rejected` non-zero means real users were refused. `recovered` versus
   `rejected` separates "the read-through is working" from "the bulk loader is
   missing a directory", which is worth knowing apart.

---

## Also found, not yet fixed

### The additional-cost gap (CBC stage one)

A separate investigation, run in parallel, established that **mandatory
additional costs are never paid**. Cards reading *"As an additional cost to
cast this spell, sacrifice a creature / discard a card / pay N life"* parse to
an `additional_cost` AST node that no cast path reads: `collectSpellEffect`
walks past it to the spell body, and resolution runs only `item.Effect`.

**366 cards, 2,821 copies across `data/decks/`.** Village Rites draws two
cards for `{B}` with nothing dying. Eaten Alive is a one-mana unconditional
exile.

The notable finding is that **the enforcement engine already exists**:
`CastSpellWithCosts` (`costs.go:706`) has `CanPayAdditionalCost`,
`PayAdditionalCost` and a §601.2f rollback — and has two production callers
that both pass `nil` for the costs. The missing piece is one adapter.

Full taxonomy, including the classes that fail in the *player's disfavour*
(Phyrexian mana, pitch spells, and all ten shocklands always entering tapped —
a Python-parser bug, not an engine one), is in the stage-one sweep. Fold two —
the enforcement policy — is deliberately deferred until the wiring is mapped.

### The `--all-decks` walker skips directories

`listDeckFiles` (`cmd/hexdek-freya/main.go`) hard-skips `benched` and `test`
and globs only `*.txt`, so `.json` decks are invisible to bulk analysis. Found
by `scripts/freya-refresh-stale.sh`, which reported 23 decks unstamped after a
full corpus run **that Freya exited 0 on throughout** — the script caught it
only because it verifies the artifact on disk rather than trusting an exit
code. Worked around in the script; the walker itself is filed, not fixed.

---

## The recognisable shape

If a component can complete an operation while doing less than it claims, and
nothing anywhere records that it did so, it will eventually do less than it
claims. The countermeasure is almost never clever:

- **stamp the artifact** — what produced this, and when;
- **count the refusals** — an operation that says no to a user must increment
  something;
- **make the error honest** — an error that blames the user suppresses the
  report that would have found the bug;
- **verify the artifact, not the exit code** — a process can exit 0 having
  written nothing useful;
- **derive, don't remember** — a constant a human must bump is a constant that
  will not be bumped.
