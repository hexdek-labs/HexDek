# Changelog — the r60 → r63 arc (the consolidation era)

This arc took HexDek from "a fleet of tools that each checked
correctness their own way" to **one Judge, one vocabulary, one score**.
The r60 fleet wave built breadth (hundreds of per-card handlers,
Heimdall analytics, Huginn 2.0, hat decision-quality); r61 hardened the
engine against a brutal external review (a sorcery that couldn't be
cast in a real main phase, a server-killing infinite trigger); r62
turned legality checking into a ride-along validator and used it to
sweep the mana-payment rules; and r63 consolidated everything into the
**Hex Judge** — six correctness dimensions, a single canonical
violation type logged at origin, a correctness score, a CI gate, and a
live grinder watchdog — then spent the dividend deleting ~16K LOC of
now-redundant tools and closing the learning loops. The current
honest topline: **99.80% correctness over a wider measurement than the
99.93% it replaced** — coverage widening traded a prettier number for
a truer one.

Entries are one line per PR, grouped by area, newest first within each
group. Direct-push commits (no PR number) are referenced by short hash.

---

## The Hex Judge

The headline of the era: ~13 scattered correctness surfaces folded into
one engine-embedded faculty (`internal/judge`) with six dimensions —
LEGALITY, CONSERVATION, STATE-INTEGRITY, PROGRESSION, OUTCOME,
LIVENESS.

### Consolidation (one vocabulary, log-at-origin)

- **#1070** — Muninn became the Judge's triage clerk: it consumes the
  live grinder violation stream, dedupes same-bug-different-seed into
  fingerprinted clusters, and emits a ranked triage report for humans.
- **`c1e02ca5`** — the parity checker's violation vocabulary folded
  into the canonical type; its Event schema had already been promoted
  as THE canonical event format.
- **`a2d4776f`** — goldilocks (Thor's dead-effect sweep) retired into
  the OUTCOME dimension's "Dead" sub-class — OUTCOME checks the
  *right* thing changed, strictly stronger than *something* changed.
- **`16039aa5`** — consolidation step 4: every violation, from every
  surface, now flows through one `judge.LogViolation` router at the
  moment it's detected. This seam is what everything later plugs into.
- **#1048** — one policy-governed event log replaced the dual
  event-store (whose silent-drop split had once blinded an entire
  verification pass), and loss-classification dual-writes completed.
- **`e930fd12`** — win/loss classification got a single canonical
  mapper, killing drift between the three places that each decided
  "why did this player lose" slightly differently.
- **`4ba4bc0b`** — consolidation step 1: the canonical violation and
  event types introduced, additive, zero behavior change.

### The six dimensions

- **#1067** — LIVENESS went to production: a sampled watchdog rides the
  live grinder, every runaway-loop guard emits one uniform event, and
  hung games self-report with a reproducible seed.
- **#1066** — LIVENESS, the sixth dimension: does the game terminate,
  and do the loop guards actually fire when it shouldn't?
- **#1068** — PROGRESSION widening 3b: trigger coverage 2,167 → 3,108,
  and the wider net caught three whole families of triggers that had
  NEVER fired (gain-life, becomes-tapped, impulse-exile — see Engine).
- **#1064** — PROGRESSION widening phase 3: coverage 1,847 → 2,167;
  four trigger-dispatch bugs found and fixed by the checker itself.
- **`671aa6ff` / `0c5e9572` / `4b0fa7ca` / `3eb60fe1`** — OUTCOME
  interpreter phases 4–7: effect coverage climbed 9,373 → 18,980 at a
  held 100% pass rate, fixing a tutor-whiff, a distribute-counters
  no-op, an own-land bounce policy, and an impulse-exile no-op along
  the way; the remaining coverage ceiling is documented, not hidden.
- **`9e39042e`** — OUTCOME reached 100.00%: duplicate ETB counters and
  a phantom life-drain were the last two real bugs behind the number.
- **`1f6f9b9d`** — tuned-deck coverage closed the "only chaos decks
  were being measured" caveat — the score now reflects real decks.
- **`db83c321`** — PROGRESSION reached 100.00% on its then-current
  scope by gating AST dispatch against per-card handlers (the
  double-fire class).
- **#1058** — OUTCOME phase 3 took on the hard effect kinds: copies,
  for-each, multi-pick, replacement counters.
- **#1057** — PROGRESSION phase 2: a 1,843-trigger vocabulary plus an
  end-to-end CR §603.4 intervening-if check.
- **`645a5734`** — OUTCOME + PROGRESSION registered: **the Judge is
  whole** — all dimensions live in one faculty.
- **#1056** — the LEGALITY dimension fold; Freya's duplicate
  deck-legality module deleted, Freya became a thin driver of
  `judge.CheckDeckLegality`.
- **`44ef5859`** — the STATE-INTEGRITY dimension fold.
- **`44e84b1e`** — `internal/judge` promoted as a package; the
  CONSERVATION fold (InstanceID strict census as the one authority).
- **#1055** — PROGRESSION phase 1: the trigger-correctness checker,
  which immediately found an APNAP-ordering bug in SBA death sweeps.
- **#1052 / #1053** — OUTCOME phases 1–2: an interpreter that derives
  each card's *expected* state delta from its AST and replays the
  effect against a real game state; first corpus catches landed
  immediately.

### Scoring, gating, watching

- **`1aeefdca`** — the sampled Judge watchdog over the live grinder
  (default off, env-tunable): game-level dimensions on a sample of
  games, corpus dimensions once per unique card, every hit logged with
  a reproducing seed.
- **#1060** — the Judge became a standing CI gate: a committed
  fingerprint baseline means NEW violation classes fail the build while
  known residuals don't block.
- **`6e9f7148`** — `hexdek-correctness`: the one-command correctness
  score, per-dimension and topline.
- **`b78f08aa`** — a 3,800-game chaos sweep certified CR compliance
  ahead of the consolidation (#1018).

## Engine correctness

### The §800.4a family — eliminated players' cards must cease

One root cause, found and re-found across the era's biggest
conservation cluster: when a player loses, every card they own leaves
the game — and half a dozen code paths quietly disagreed.

- **`67810ac9` / `c8ef4331` / `75fa22f4`** — the final residuals:
  death-trigger graveyard lifts (Gisa + 3 siblings), the CastSpell
  transit window, the survivor-zone sweep gap, and merged/mutated
  permanents unmerging onto a dead seat. CONSERVATION hit a clean 100%.
- **#1062** — Athreos returned an eliminated player's creature through
  `createPermanent`; the guard now lives at that chokepoint too.
- **#1049** — a commander redirect (§704.6d) left a linked-exile claim
  dangling — the 1-in-500-games strict-census residual, run to ground
  from a single seed.
- **#1046** — an eliminated player's *stolen* permanents vanished from
  all zones (they belong to the thief's board until the game says
  otherwise).
- **#1044** — Knowledge Pool granted a free cast of an eliminated
  player's already-ceased card.
- **#1043** — eliminating a player mid-resolution orphaned their own
  resolving spell in a limbo window.
- **#1041** — the originating fix: a zone-move guard so an eliminated
  owner's card can never re-materialize (the fabrication class).

### Trigger dispatch — silent and double-firing triggers

- **#1072** — the liveness firehose exposed zero-handler dispatch
  starvation, and the cap-contract guard family was split so each
  runaway class is separately accountable.
- **#1068** — three corpus-wide silent families fixed: "whenever you
  gain life" (77 cards) and "becomes tapped" (82 cards) never fired at
  all, and a fizzle-gate bug was countering every "exile the top card
  of your library" trigger (~84 cards) before it could resolve.
- **#1065** — Plargg's upkeep loop spun forever over an eliminated
  seat's retained library (same shape latent in Possibility Storm and
  Chaos Wand).
- **#1059** — attack triggers re-read their raw oracle text to recover
  actor phrases the parser drops ("whenever ~ attacks" vs "whenever a
  creature attacks") — fail-closed, ending a class of wrong-actor
  fires.
- **#1034** — the Wave-1b double-dispatch class closed for good:
  hardcoded inline observer arms deleted wherever a generic path now
  covers the card (Rhystic Study and Mystic Remora were double-taxing,
  #1032).
- **#1036** — the double-registration fix generalized across every
  per-card registry map (Celestial Unicorn was double-counting), plus a
  21-site mana-pool drift sweep and a defensive-gate double-pay fix.
- **`cd04b3fc`** — SBA death sweeps now order simultaneous triggers by
  APNAP (§603.3b).

### Casting, costs, and mana — the r62 legality sweep

A ride-along validator watched real games and each finding became a
fix:

- **#1034** — restricted mana ("spend only on X") was being stranded
  by the post-spend sync — the under-pay class.
- **#1033** — mana abilities living in per-card handlers were invisible
  to the §605 mana-ability check.
- **#1030** — Esper Sentinel double-taxed (and the tax wasn't credited
  as payment) — the over-pay class.
- **#1026** — Birgi and friends now add mana through the one AddMana
  chokepoint, and the priority-window response-cast path got the same
  legality coverage as main-phase casts.
- **#1025** — the stack drains after a commander cast, killing a
  302-hit "cast from an illegal mid-stack window" cluster.
- **#1024** — in-window auxiliary payments (extort and kin) are
  credited; they no longer read as overpayment.
- **#1021 / #1022** — the ride-along rules-legality validator itself
  (an owner design), plus game-end ordering: player-loss SBAs apply
  before the game is declared over.
- **#1016** — targets are chosen at announcement time per
  §601.2c (the hat's ChooseTarget wired live), not at resolution.
- **#1005 / #1006** — the optional-cost framework: kicker, multikicker,
  and the wider cost-mechanic family payable at cast time.
- **#1004** — spells with no legal target fizzle per §608.2b instead of
  resolving against nothing.
- **#1031** — combat declarations get an engine-level CR 508.1/509.1
  backstop regardless of which AI declared them.

### State, ownership, and identity

- **#1050** — ownership is immutable (§108.3): a theft-effects audit
  found "temporary" steals that were silently permanent; one shared
  return-to-owner op now exists.
- **`851f1870`** — a transformed MDFC's back-face land was still
  carrying its front-face spell type — the correctness baseline's lone
  state-integrity offender at the time.
- **#1047** — per-seat win/loss self-checking phase 1, with Card.Owner
  as the elimination authority (a second live leak fell out of it).
- **#1011** — **the era's most humbling bug**: sorceries were rejected
  in every *real* main phase (a phase-name comparison nobody had
  re-checked); caught by external review, not by us.
- **#1012** — a per-card trigger recursion could kill the server
  (sba704_5y); a depth guard now ends the game as a draw instead of
  crashing the process.
- **#1013** — condition evaluation fails closed, and Tiamat no longer
  tutors twice.
- **#1015** — planeswalker loyalty abilities pay loyalty counters, not
  life.

## Cleanup / EOL

The full retirement ledger lives in **[LEGACY-EOL.md](LEGACY-EOL.md)**.

- **#1073** — round-2 close-out: Heimdall/Freya correctness verified
  already-folded into the Judge (the duplicate path was dead); the one
  residual — Heimdall's spectator display — became a thin adapter over
  the canonical vocabulary.
- **#1071** — the Huginn learning loop CLOSED: the graduation step had
  never been wired (and carried a latent quadratic double-count, now
  fixed with a drained inbox); confirmed card interactions finally flow
  play → observe → graduate → Freya analysis → hat strategy.
- **#1070** — Muninn matured into the Judge triage clerk (see Judge
  section).
- **#1069** — three orphaned cmd tools deleted (1,774 LOC) and the
  LEGACY-EOL retirement log created; two borderline tools kept on
  live-runbook evidence.
- **#1054** — dead-code sweep: 196 unreachable functions deleted
  (−5,472 LOC), chiefly an entire never-wired keyword-primitive
  library.
- **#1051** — kill-list Part E: 9 stub cmd tools + 7 dead Thor modules
  deleted (−8,985 LOC) — odin, valkyrie, huginn-cli, oracle-sync and
  friends, each verified reference-free first.

## Frontend

- **`ddb4fe0f`** — "login only works in private browsing": a stale
  sign-in email wedged the auth flow; now recoverable in place.
- **#1063 / `1cba6098`** — a whitelisted staging gate for
  dev.hexdek.dev with a reviewer passphrase bypass, so deck-editor
  review can happen without exposing work-in-progress.
- **#1061** — deck editor MVP: card search filtered to the commander's
  color identity (name, type, and oracle-text matching) with a
  structured add/remove staging list.
- **#1017** — a `pls` ReferenceError crashed the page for some users;
  fixed, plus frontend CI, GA4, and first-party error telemetry so the
  next one is seen before it's reported.
- **`0b0007d0` / `26cfce43`** — mobile blank tabs: lazily-loaded routes
  rendered nothing on slow connections when a chunk failed; routes are
  now wrapped in Suspense + a real error boundary with a retry path.

## AI player & analytics

- **#1029** — Heimdall's win_reason now points the right direction, and
  all kill classifiers collapsed onto the canonical mapper.
- **#1010** — hat decision-quality batch: activation, mulligan,
  counterspell, and valuation judgment cleanups.
- **#1009** — graceful budget degradation on complex boards +
  beneficial effects stopped targeting opponents' permanents.
- **#1008** — the hat plays instants on opponents' turns (reactive
  interaction, not just sorcery-speed value).
- **#1007** — lazy mana-tap: the hat stops tapping out when it has no
  play to make (a long-standing tell and resource leak).
- **#1003** — every game gets a win_reason label from the canonical
  kill classifier.
- **#989** — TrueSkill's 4-player FFA decomposition fixed tied-loser
  miscalibration (all-pairs, not winner-vs-each).

## Tooling / Infra

- **#1039 / `11968496` (#1038)** — measurement honesty fixes: the
  goldilocks untap scaffold was tapping the whole board (131 → 101
  failures once re-baselined), and loki's nightmare boards never minted
  InstanceIDs — the strict census had been vacuously passing on that
  entire surface.
- **#1040** — two false-positive classes retired: replacement effects
  that redirect zones now declare it (§614.1a), and a winner at
  negative life (Platinum Angel) stopped tripping the life invariant.
- **#1028** — loki resolves DFC front/back faces when the full card
  name misses the corpus.
- **#1027** — maintenance mode halts running rated games cleanly, and
  TrueSkill mu/sigma persist across restarts.
- **#1023** — the anticheat seal/verify digest was asymmetric — honest
  games failed verification; fixed.
- **#1020** — seed-replay determinism: same seed + same decks now
  reproduces the same game across engine, hat, and tournament layers.
- **#1019** — land drops route through the ETB dispatcher like
  everything else, and the parser↔engine cost-framework data contract
  is enforced end-to-end.
- **#998** — HEXDEK_MAINTENANCE mode: pause the grinder, 503 the
  gauntlet/spectate endpoints, deploy safely.
- **#993–#997** — DARKSTAR-rebuild fallout week: unified .txt+.json
  deck pool with live reload (#993), Marchesa deck restored (#994),
  www-less Moxfield URLs accepted (#995), runaway ELO clamped (#996),
  and Gifts Ungiven un-banned per the 2024 RC update (#997).
- **#991 / #992** — Windows deployment: Freya binary discovery handles
  .exe + exe-dir, and the observability writer stopped breaking
  WebSocket upgrades.

## Earlier: the r60 fleet wave (context)

The arc opened with a parallel-fleet breadth push too large to itemize
here (roughly PRs #930–#990): per-card stub sweeps and high-EDHREC
commander batches in the hundreds of handlers, Heimdall analytics
batches 3–5 (archetype fingerprinting, decision attribution, narrative
summaries), Huginn 2.0 (reverse index, cycle detection, recommender,
predict CLI), the hat's 3rd Eye opponent model and WinConPursuit,
the Heimdall→hat self-tune feedback loop, deckparser support for the
major export formats, spectator accessibility and mobile audits, and
Freya's HTML/tier-list reporting. That breadth is what r61–r63's
consolidation then made trustworthy.
