# Zone-Cast Grant Expiry Audit — r60 follow-up

## Background

The `ZoneCastPermission` lifecycle has three legitimate expiry modes
(see `internal/gameengine/zone_cast.go`):

| `Duration`                  | Cleanup mechanism                              | Invariant checks |
|-----------------------------|------------------------------------------------|------------------|
| `""` (default — permanent)  | `RemoveZoneCastGrant` on card leaving exile    | not flagged      |
| `"until_end_of_turn"`       | `ExpireZoneCastGrants` at EOT cleanup          | yes              |
| `"until_end_of_next_turn"`  | `ExpireZoneCastGrants` at next-turn EOT        | yes              |
| `"while_source_on_bf"`      | `ExpireSourceGrants` on source LTB             | yes              |

A grant whose oracle text declares an expiry window but whose
`Duration` field is empty is a silent leak: `ExpireZoneCastGrants`
won't reap it, and `checkZoneCastGrantExpiry` won't flag it (the
invariant's `grantIsLeaked` switch only fires on the three recognized
Duration strings). The grant survives until the card moves zones —
which for impulse-exile + opponent-controlled targets can be never.

The r59 and r60 work pinned the four `resolve_helpers.go` arms that
build EOT-bound grants from AST modkinds and residual oracle text
(`impulse_play` structured + residual, `heist`, `may_play_exiled_free`).
This audit sweeps the rest of the engine + per_card surface for the
same anti-pattern.

## Methodology

`grep -rn "ZoneCastPermission{" internal/gameengine/` returns 41 call
sites. For each, I read the surrounding handler + oracle comment and
classified the declared duration against the assigned `Duration` field.

## Findings

### Clean (38 sites)

The four `resolve_helpers.go` arms, the keyword-builder helpers
(`flashback`, `escape`, `unearth`, `foretell`, `disturb`, `madness`,
`omen`, `plot`, `discover`, `warp`, `retrace`, `jump_start`,
`aftermath`, `adventure`), the `Underworld Breach` escape factory, the
`NewOncePerTurnGraveyardCastPermission` family (Kess, Maestros,
Karador, Lurrus, Gisa & Geralf), and 24 per_card handlers
(`chitinous_crawler`, `gen_eruth_tormented_prophet`,
`gen_urabrask_heretic_praetor` × 2, `ashling_limitless`,
`narset_enlightened_master`, `commanders_batch` (Narset),
`gen_galea_kindler_of_hope`, `edward_kenway`, `gen_neriv_crackling_vanguard`,
`ob_nixilis_captive`, `meria`, `gen_noctis_prince_of_lucis`,
`prosper_tome_bound`, `cruelclaw`, `gonti_canny_acquisitor`,
`urza_lord_high_artificer`, `paradigm_improvisation_capstone`,
`rivaz_of_the_claw`, `golbez`, `delayed_trigger_cards` (Release to the
Wind), `commander_staples` (Dauthi Voidwalker), `tutor_resolve.go`
(Opposition Agent)) all set the appropriate `Duration` (or
intentionally leave it empty for the "for as long as remains exiled"
and static permission idioms).

### Gaps — fixed in this audit (5 sites)

| Site                                       | Oracle expiry phrase                                    | Fix                                                      |
|--------------------------------------------|---------------------------------------------------------|----------------------------------------------------------|
| `per_card/ragavan.go:88`                   | "Until end of turn, you may cast that card."            | `Duration: "until_end_of_turn"` + `GrantTurn: gs.Turn`   |
| `per_card/emry_lurker_of_the_loch.go:104`  | "You may cast it this turn."                            | `Duration: "until_end_of_turn"` + `GrantTurn: gs.Turn`   |
| `per_card/game_changers.go:808` (Jeska's Will) | "Until end of turn, you may play those cards."      | `Duration: "until_end_of_turn"` + `GrantTurn: gs.Turn`   |
| `per_card/vivi_ornitier.go:102`            | One-shot per-trigger; engine treats as EOT-bound        | `Duration: "until_end_of_turn"` + `GrantTurn: gs.Turn`   |
| `per_card/gen_yasmin_khan.go:62`           | "Until your next end step, you may play it."            | `Duration: "until_end_of_turn"` (was `"until_next_end_step"`, unrecognized; {T} sorcery speed means "next end step" = current EOT) |

#### Severity ordering

1. **Ragavan, Nimble Pilferer (highest)** — cEDH staple. No delayed-trigger
   cleanup either: the original handler relied purely on the (empty)
   Duration to bound the grant. The grant therefore persisted until the
   exiled card moved zones; if the controller didn't cast it that turn,
   the AI on a later turn (or any other turn) would still see the grant
   in `ZoneCastGrants` and would consider casting an opponent's exiled
   card despite the rules window having closed. Real misplay surface.

2. **Jeska's Will, Emry, Vivi** — same anti-pattern but each had a
   `RegisterDelayedTrigger("end_of_turn", … RemoveZoneCastGrant)`
   safety net, so the practical impact was bounded. The
   belt-and-suspenders model the r59/r60 fixes adopted is: set Duration
   AS WELL AS any existing delayed-trigger cleanup. The delayed trigger
   can race with the grant's own consumption or with weird timing
   (player loss before EOT, mandatory-loop draw, the §727 no-op
   detector); having `Duration` stamped means `ExpireZoneCastGrants`
   reaps deterministically regardless.

3. **Yasmin Khan** — `Duration: "until_next_end_step"` was a silent
   no-op: not in the `shouldExpireGrant` switch and not in the
   `grantIsLeaked` switch. Cleanup happened only via the delayed
   trigger. Normalised to the canonical `"until_end_of_turn"` value so
   the engine's reaper sees it.

### Follow-up — residual-text SourceTimestamp stamp (1 site)

| Site                                    | Gap                                                      | Fix                                              |
|-----------------------------------------|----------------------------------------------------------|--------------------------------------------------|
| `resolve_helpers.go:4823` (resolveResidualByText impulse_play) | `SourceTimestamp` not stamped — only EOT cleanup could reap | Stamp `perm.SourceTimestamp = src.Timestamp` mirroring the structured arm at line 1571 |

Severity: defense-in-depth. The grant still expires via `ExpireZoneCastGrants` at EOT cleanup — but EOT cleanup is skipped on (a) mandatory-loop draw (SBA cap, `sba.go:158-172`), (b) mid-combat game-end before reaching the cleanup step (game-end purge in `CheckEnd` is the safety net but only fires once), and (c) seat elimination mid-priority. Without `SourceTimestamp` the `ExpireSourceGrants` LTB path can't reap on source death pre-EOT. The structured `impulse_play` arm at line 1571 got this stamp in r60 round 2; the residual-text sibling was missed. Regression in `zonecast_residual_source_ltb_r60_test.go`.

### Out of scope — flagged for future passes

- **Opposition Agent (`tutor_resolve.go:183`)**: oracle reads "you may
  spend mana as though it were mana of any color to cast those
  spells." The grant correctly has no `Duration` ("for as long as they
  remain exiled" — naturally bounded by zone movement). However the
  grant is missing the `SpendAnyColor: true` flag. Not an expiry bug;
  separate misfeature.
- **Dauthi Voidwalker (`commander_staples.go:788`)**: oracle "You may
  cast it without paying its mana cost" has no turn boundary in the
  printed text, but the activation cost sacrifices Voidwalker so the
  natural "while source on bf" model can't apply. Empty Duration is
  defensible; flagging for review only.
- **`keywords_discover.go:236` (Discover) / `keywords_batch6.go:1779`
  (Warp) / `cascade.go`**: each grants permission with no Duration.
  The cast-or-bottom decision is meant to be made in a single
  resolution window, but the engine currently leaves the grant in
  `ZoneCastGrants` after the resolution completes. If the AI doesn't
  consume it inside the priority window, the grant outlives its
  intended one-shot scope. Not flagged by the invariant (Duration is
  empty). Treat as a follow-up — needs a one-shot-on-resolve cleanup
  hook on these arms rather than a Duration stamp, since the rule is
  "now or never," not "this turn."

## Verification

- `internal/gameengine/per_card/zonecast_grant_audit_r60_test.go` — 5
  regressions: one per fixed site, asserting `Duration` +
  `GrantTurn` are stamped and that `ExpireZoneCastGrants` + the
  `checkZoneCastGrantExpiry` invariant agree the grant is reclaimed
  at the cleanup of its grant turn (or +1 for Yasmin Khan if the cast
  window straddles a turn).
- `go build ./...` clean.
- `go test ./internal/gameengine/... -count=1` clean.
