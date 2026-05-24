# Muninn R60 Bug-Signature Classification — 2026-05-24

Cross-reference of every distinct bug/gap signature surfaced in the five
2026-05-17 Muninn reports against the R60 era resolution log in
`CLAUDE.md`. Goal: identify what the Muninn snapshot week flagged that
the R60 engine work has since closed, vs. what is still live as a
visible Muninn signal.

Reports surveyed:

- `docs/muninn-saturation-report-2026-05-17.md`
- `docs/muninn-variant-coverage-2026-05-17.md`
- `docs/muninn-bulk-patterns-5.md`
- `docs/muninn-morning-2026-05-17.md`
- `docs/muninn-eod-final-2026-05-17.md`

A signature counts as **RESOLVED-IN-R60** when either (a) the CLAUDE.md
Resolved log records a matching fix in the R60 era window, or (b) the
named per-card handler / family file exists in the current tree on
`main`. **STILL-OPEN** signatures are ones the reports flagged that
have no matching fix on `main` today.

## Signatures

### Engine crash / invariant signatures

| # | Signature | Source | Classification | Notes |
|---|-----------|--------|----------------|-------|
| 1 | May-11 nil-deref burst — `abdelAdrianETB → moveCardBetweenZones → FireZoneChangeTriggers(perm=nil)` panic, recurring under `henzie_toolbox_torre`, `rashmi_and_ragavan`, `zask_skittering_swarmlord`, `tergrid_god_of_fright`, `ulalek_fused_atrocity`, `colfenor_the_last_yew`, `tannuk_memorial_ensign`, `alela_cunning_conqueror`, `athreos_shroud_veiled`, `teysa_orzhov_scion_b2_mollymauk76`, `the_second_doctor`, `slicer_hired_muscle` | EOD §4, Morning §5 | **RESOLVED-IN-R60** | CLAUDE.md 2026-05-16 "Muninn" entry — patched in commit `b348f4a` (nil-guard around LTB `FireCardTrigger`) and the `abdel_adrian.go` rewrite that routes through `gameengine.ExilePermanent`. Six sibling handlers flagged in the same forensic write-up (etrata, zabaz, zimone+dina, bilbo, thassa) carry the anti-pattern but no longer crash. Visible in Muninn only because DARKSTAR served the pre-merge binary across all 5 reports. |
| 2 | `The One Ring` "dead trigger" — `triggered_ability` count=84, last_seen 2026-04-30 | EOD §4, Morning §5 | **RESOLVED-IN-R60** | `per_card/the_one_ring.go` shipped in earlier per-card wave (saturation report wave 1-160 era). Entry is stale — `last_seen` predates the handler; will clear from Muninn on next post-deploy grinder cycle. |

### Dispatch / variant-coverage signatures

| # | Signature | Source | Classification | Notes |
|---|-----------|--------|----------------|-------|
| 3 | `lookupCandidates` misses ` Token` suffix without parens — 7 snippets, ~13 cumulative hits: `Claim Jumper Token`, `Sand Scout Token`, `Gau, Feral Youth Token`, `Phyrexian Myr Token`, `Kodama of the East Tree Token`, `Wistfulness Token`, `Myr Token`, `Rankle and Torbran Token` | Variant-coverage §"Gaps that remain" #1, Morning §4, EOD §3 | **STILL-OPEN** | Verified against `internal/gameengine/per_card/registry.go:195-217` on R60 main — only single-paren strip + DFC front-face. No trailing ` Token` arm. Four emit sites still in place: `ratadrabik_of_urborg.go:85`, `paradigm_echocasting_symposium.go:63`, `runo_stromkirk.go:174`, `sin_spiras_punishment.go:92`. |
| 4 | `lookupCandidates` doesn't iterate the paren strip — `Crown of Gondor (Urza copy) (Urza copy)` (1 hit) | Variant-coverage §"Gaps that remain" #2 | **STILL-OPEN** | Verified — `lookupCandidates` does a single `strings.LastIndex(nk, " (")` strip and stops. Stacked-rename cases (any Urza-copy-of-Urza-copy, Adrix-of-Miirym-token, etc.) fall through. |
| 5 | Bare token-type names polluting gap log — `Token`, `Construct Token`, `creature token scorpion dragon Token`, `creature token knight Token`, `creature token colorless myr artifact Token` (~4 hits, unrecoverable at dispatch layer) | Variant-coverage §"Gaps that remain" #3, Morning §4, EOD §3 | **STILL-OPEN** | Verified — `internal/heimdall/replay.go:310 ExtractParserGaps` walks `seat.Battlefield`, emits `p.Card.DisplayName()` verbatim with no filter. Bare-token names continue to flow into `parser_gaps.json`. |
| 6 | Parser-gap flag is set during effect resolution (`resolve.go:178/203`, `resolve_helpers.go:2136/4512`) independent of per-card dispatch — handler-coverage fixes don't shrink gap-log volume for cards whose oracle text still has parse-unhandled nodes the resolver walks | Variant-coverage §"Architectural observation" | **STILL-OPEN** | Architectural — explicitly out of scope for `c09db37`'s dispatch fallback. No follow-up landed. |

### Per-card handler-coverage signatures (top-30 "Handler shipped — not live")

The Morning + EOD reports list 30 cards with status "Handler shipped —
not live" — every entry has a handler on `main` but stayed visible to
Muninn because DARKSTAR was not redeployed during the snapshot week.
Spot-checked `internal/gameengine/per_card/` for representative files:
`the_one_ring.go`, `land_tax_family.go`, `sam_loyal_attendant.go`,
`burnished_hart.go`, `trostani_selesnyas_voice.go` all present.

| # | Card | Source | Classification |
|---|------|--------|----------------|
|  7 | The One Ring | top-30 #1 | **RESOLVED-IN-R60** (handler shipped, not yet live) |
|  8 | Land Tax | top-30 #2 | **RESOLVED-IN-R60** |
|  9 | Bloodchief Ascension | top-30 #3 | **RESOLVED-IN-R60** |
| 10 | Necromancy | top-30 #4 | **RESOLVED-IN-R60** |
| 11 | Light-Paws, Emperor's Voice | top-30 #5 | **RESOLVED-IN-R60** |
| 12 | Kodama of the East Tree | top-30 #6 | **RESOLVED-IN-R60** |
| 13 | Tiamat | top-30 #7 | **RESOLVED-IN-R60** |
| 14 | Great Hall of the Biblioplex | top-30 #8 | **RESOLVED-IN-R60** |
| 15 | Knight of the White Orchid | top-30 #9 | **RESOLVED-IN-R60** |
| 16 | Vibrance | top-30 #10 | **RESOLVED-IN-R60** |
| 17 | Acererak the Archlich | top-30 #11 | **RESOLVED-IN-R60** |
| 18 | Oversold Cemetery | top-30 #12 | **RESOLVED-IN-R60** |
| 19 | Claim Jumper | top-30 #13 | **RESOLVED-IN-R60** |
| 20 | Chainer, Nightmare Adept | top-30 #14 | **RESOLVED-IN-R60** |
| 21 | Twilight Prophet | top-30 #15 | **RESOLVED-IN-R60** |
| 22 | Grave Venerations | top-30 #16 | **RESOLVED-IN-R60** |
| 23 | Birthing Ritual | top-30 #17 | **RESOLVED-IN-R60** |
| 24 | Frodo, Adventurous Hobbit | top-30 #18 | **RESOLVED-IN-R60** |
| 25 | Lasting Tarfire | top-30 #19 | **RESOLVED-IN-R60** |
| 26 | Valakut Exploration | top-30 #20 | **RESOLVED-IN-R60** |
| 27 | Wistfulness | top-30 #21 | **RESOLVED-IN-R60** |
| 28 | Wedding Ring | top-30 #22 | **RESOLVED-IN-R60** |
| 29 | Kaito Shizuki | top-30 #23 | **RESOLVED-IN-R60** |
| 30 | Taii Wakeen, Perfect Shot | top-30 #24 | **RESOLVED-IN-R60** |
| 31 | Sunderflock | top-30 #25 | **RESOLVED-IN-R60** |
| 32 | Lux Artillery | top-30 #26 | **RESOLVED-IN-R60** |
| 33 | Lathiel, the Bounteous Dawn | top-30 #27 | **RESOLVED-IN-R60** |
| 34 | Smirking Spelljacker | top-30 #28 | **RESOLVED-IN-R60** |
| 35 | Zoyowa Lava-Tongue | top-30 #29 | **RESOLVED-IN-R60** |
| 36 | Crackling Spellslinger | top-30 #30 | **RESOLVED-IN-R60** |

### Per-card handler-coverage signatures (new gaps during snapshot week)

| # | Card / Snippet | Source | Classification |
|---|----------------|--------|----------------|
| 37 | `Sam, Loyal Attendant` | Saturation report wave 161-180 | **RESOLVED-IN-R60** (`sam_loyal_attendant.go`) |
| 38 | `Burnished Hart` | EOD §3 + wave 181-200 | **RESOLVED-IN-R60** (`burnished_hart.go`) |
| 39 | `Trostani, Selesnya's Voice` | Morning §4 + wave 181-200 | **RESOLVED-IN-R60** (`trostani_selesnyas_voice.go`) |
| 40 | `Life of the Party (Life-of-the-Party token)` | EOD §3 (23 hits) | **RESOLVED-IN-R60** (paren-strip via `lookupCandidates`, handler exists) |
| 41 | `Claim Jumper (Restore-Relic token)` | EOD §3 (21 hits) | **RESOLVED-IN-R60** (paren-strip via `lookupCandidates`, handler exists) |
| 42 | `Eccentric Pestfinder // Turn Stones (cascade)` | EOD §3 | **RESOLVED-IN-R60** (cascade strip + DFC front-face in `lookupCandidates`) |

### Bulk-pattern families (preventive coverage)

| # | Family | Source | Classification |
|---|--------|--------|----------------|
| 43 | `etb_basic_land_ramp_family` (Farhaven Elf, Civic Wayfinder, Borderland Ranger, Sylvan Ranger, Pilgrim's Eye) | bulk-patterns-5 §Family 1 | **RESOLVED-IN-R60** |
| 44 | `etb_drain_target_opponent_family` (Skymarch Bloodletter, Vampire Sovereign, Highway Robber, Dakmor Ghoul, Bloodborn Scoundrels) | bulk-patterns-5 §Family 2 | **RESOLVED-IN-R60** |
| 45 | Deferred: Forest-fetcher sibling (Wood Elves), sac-cost ramp activated-ability family (Sakura-Tribe Elder, Dawntreader Elk), end-step gained-or-lost-life gate (Starseer Mentor), Soul Sister enter-trigger family | bulk-patterns-5 §Investigation notes | **STILL-OPEN** (intentionally deferred until a covered grinder-run surfaces a hit; no engine bug, no current Muninn signal) |

## Final tally

| Bucket | Count |
|--------|------:|
| RESOLVED-IN-R60 (engine fix or handler on main) | **40** |
| STILL-OPEN | **5** |
| **Total distinct signatures** | **45** |

Resolution rate: **40 / 45 = 88.9%**.

Of the 5 STILL-OPEN signatures:

- 3 are dispatch/variant-coverage polish items (#3, #4, #5) that the
  variant-coverage report explicitly framed as one-file follow-ups to
  `c09db37` and were not picked up during R60.
- 1 is an architectural disconnect (#6) between dispatch-coverage and
  parser-coverage that needs work in `gameast` or `resolve_helpers.go`.
- 1 is a deferred-by-design family backlog (#45).

## Worst remaining bug surface

**Signature #6 — parser-gap flag set during effect resolution
independent of per-card dispatch.** This is the highest-leverage open
item by a wide margin: it is the reason the saturation report could
land 111 bespoke handlers + 10 bulk-pattern families + a dispatch
fallback and *still* not collapse the gap log proportionally. Every
fix in buckets #3, #4, #37-#42 wins back handler-firing on a variant,
but the parser-walk in `resolve.go:178/203` and
`resolve_helpers.go:2136/4512` will continue to flip `parser_gap=1`
on the permanent for any oracle node the resolver doesn't recognize —
even when a per-card handler is wired and did fire. The gap-log
metric and the dispatch-coverage metric are measuring different
things and the conflation has been hiding parser-coverage work behind
handler-coverage work all month.

The other open items are real but small:

- #3 (`X Token` suffix) and #4 (iterative paren strip) together cost
  ~14 cumulative gap-log hits and are a single-file edit to
  `lookupCandidates`.
- #5 (bare token-type names) costs ~4 gap-log hits and is a single
  filter in `ExtractParserGaps`.
- #45 is preventive — no current Muninn signal, no urgency.

#6 is the one worth opening a focused PR against; the rest are
cleanup-tier.

## Caveats

- The Muninn reports were written against a pre-merge binary on
  DARKSTAR; status labels of "Handler shipped — not live" reflect the
  binary running 2026-05-14, not the code on `main`. Once a
  post-R60-merge binary actually deploys and a grinder cycle runs,
  expect the top-30 to plateau and uniques to drop into the
  ~120-140 band as the reports predict.
- This audit does *not* attempt to verify every named handler matches
  oracle text correctly — it only checks that a handler file exists
  on `main`. A handler that exists but is wrong would still show as
  RESOLVED-IN-R60 here while continuing to mis-resolve in play. Real
  correctness verification belongs to Goldilocks + Loki, not this
  classification pass.
- The "Handler shipped — not live" labels were the snapshot author's
  own audit; this report trusts them rather than re-deriving handler
  presence card-by-card for all 30 entries.
