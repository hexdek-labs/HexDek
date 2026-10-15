# r121 — the §701 keyword-action citations had drifted, and the gate that named six of them could not see them

## What was wrong

The engine stamps a CR citation into `Details["rule"]` on keyword-action
events. That string reaches the event log, so it is **outcome data, not a
comment** — the same class as r120's SBA rule letters.

Thirty-four of those citations named the wrong rule:

| event `Kind` | cited | which is | correct |
|---|---|---|---|
| `scry` | 701.18 | Play | **701.22** |
| `surveil` | 701.46 | Adapt | **701.25** |
| `sacrifice` | 701.17 | Mill | **701.21** |
| `destroy` | 701.7 | Create | **701.8** |
| `create_token` | 701.30 | Clash | **701.7** |
| `regenerate` / `regenerated` | 701.15a | Goad | **701.19a** |
| `goad_force_attack` | 701.39 | Bolster | **701.15** |
| `amass` | 701.43 | Exert | **701.47** |
| `adapt` | 701.43 | Exert | **701.46** |
| `monstrosity` | 701.31 | Planeswalk | **701.37** |
| `populate` | 701.30 | Clash | **701.36** |
| `proliferate` | 701.27 | Transform | **701.34** |
| `manifest` ×2 | 701.34 | Proliferate | **701.40** |
| `explore_land` / `explore_nonland` | 701.40 | Manifest | **701.44** |
| `bolster` | 701.32 | Set in Motion | **701.39** |
| `support` | 701.35 | Detain | **701.41** |
| `connive` | 701.48 | Learn | **701.50** |
| `learn` | 701.45 | Assemble | **701.48** |
| `discover` | 701.55 | Face a Villainous Choice | **701.57** |
| `discover_cast` / `discover_to_hand` | 701.51 | Open an Attraction | **701.57** |
| `cloak` | 701.56 | Time Travel | **701.58** |
| `ring_tempts` | 701.52 | Roll to Visit Your Attractions | **701.54** |
| `venture_dungeon` | 701.46 | Adapt | **701.49** |
| the five vote events | 701.18a/b, 701.20a/b | Play, Reveal | **701.38a/b** |

Plus two in §702 found by the repaired gate: `mutate` cited 702.139
(Companion, correct 702.140) and `bargain_paid` cited 702.176 (Impending,
correct 702.166a).

## Why §701 and not §702

**§701 keyword actions are inserted alphabetically; §702 keyword abilities
are appended.** Every new keyword action pushes every later number up by
one, so the §701 block drifts on every set that adds one — while §702
survives because new keywords land at the end. The same mechanism as r120's
§704.5 shift, in a family where it recurs on a schedule.

Measured: of 236 distinct §701/§702 rules cited across `internal/gameengine`,
the §702 block was essentially clean and the §701 block was not.

## The gate already named six of them

`internal/crcite/wrong_number_by_topic_r94_test.go` pairs a rule number with
a topic word and fails if both appear **on the same line**. Its table already
contained `{701.27, proliferate}`, `{701.34, manifest}`, `{701.51, discover}`
and `{701.15, regenerat}`.

An event-log citation does not have that shape:

```go
gs.LogEvent(Event{
    Kind: "proliferate",          // <- the topic word
    Details: map[string]interface{}{
        "rule": "701.27",         // <- the number, four lines away
```

So six of the 34 were named in the gate's own table and shipped anyway.
r121 widens the window: for a `"rule": "..."` literal, scan back to the
nearest `Kind:` line and treat the whole block as context. That took the
gate from 0 violations to 7 on the pre-change tree — the five §701 sites
plus the two §702 ones nobody had found.

## Measured and deliberately NOT applied

A bare table entry does not match its own subrules: `701.15` never matched
the live `701.15a`, which is why the `regenerate` event escaped a pair that
was sitting in the table. Appending `[a-z]?` closes that gap and takes the
gate **from 0 violations to 51** — `hat/greedy.go` citing §702.110b for
menace, `tournament/turn.go` citing §702.34a for madness, and 49 more.

Those 51 are **unread**. Baselining unread sites is how
`mismatch_baseline.txt` became a worklist, so the widening is recorded in a
comment at the site and left off until someone triages them.

## Verification

- Runtime regression `internal/gameengine/event_rule_citation_r121_test.go`
  drives `Scry`, `Surveil`, `DestroyPermanent` and `SacrificePermanent` and
  asserts the emitted citation. This is a real semantic change, so each arm
  got a **real revert**, not a mutation check: all four go red on exactly
  their own assertion.
- `text_lock.txt` diff is its own bijection check — every line removed
  carries a wrong-topic heading (Mill, Reveal, Adapt, Create) and every line
  added carries the right one (Scry, Surveil, Vote, Amass, Destroy, Exert,
  Monstrosity, Venture into the Dungeon, The Ring Tempts You).
- Dangling sweep: `§701.21b` (which r121 nearly introduced — §701.21 has only
  an `a`, and the indestructible clause lives inside it) and a pre-existing
  `§701.16d` in `per_card/lord_of_tresserhorn.go`.

## Lesson

**A gate's matching window is part of what it guards.** r94 wrote down the
right pairs. The one-line window meant the gate could only ever see
citations in comments, and every citation that reaches the event log — the
ones that are outcome data — was structurally invisible to it.

---

## The r120 detector's 17 candidates — triage complete

r120 shipped `scripts/detect_citation_text_mismatch_r120.py` as a script, not
a gate, with 17 untriaged candidates. All 17 are now read against the CR.
**20 (pre-r120) → 17 (r120) → 8 (r121).**

**Nine were real and are fixed:**

| site | was | is | now |
|---|---|---|---|
| `scry.go:3` ×3 | 701.18a/b, 701.46a | Play, Adapt | 701.22a/b, 701.25a |
| `zone_change.go:36` | 701.7a | Create | 701.8a |
| `zone_change.go:275` | 701.17a/b | Mill | 701.21a (§701.21 has no `b`) |
| `keywords_exert.go:3` | 702.112a/b | Renown | 701.43d / 701.43a |
| `keywords_champion.go:9` | 702.72b/c | linked-abilities note, "championed by" | 702.72a — both triggered abilities live there |
| `exploit_dispatch.go:59` | 702.110b | "exploits a creature" definition | 702.110a — the ETB ability |
| `keywords_mayhem.go:3` | 702.187b/c | misdescribed | 702.187b/c as the CR actually words them |

`keywords_champion.go` also carried §702.72b/c through its body, including
**two event-log strings**, all corrected to §702.72a.

**Eight are the known false-positive mode** — a header block citing several
related rules while quoting one of them, or quoting text the detector then
matches to a sibling rule elsewhere in the CR. Each verified correct as
written:

- `keywords_flashback.go:3` — §702.34 has only an `a`; all three quoted
  clauses, including the exile carveout, live in that one paragraph. The
  detector matched the carveout to Escape's §702.138a.
- `combat.go:220` — §508.1g/a/f are right for enters-attacking; §506.3d is a
  sibling about what can attack.
- `commander.go:8` ×2 — a §903 file header; the matched §903.6/§903.8 are in
  the same section the header is about.
- `etb_vanishing_fading.go:10` ×2 — cites §614.1c for the replacement AND
  §702.63/§702.32 for the keywords; the matched §702.63a/§702.32a are the
  keywords it already names.
- `per_card/clone_as_enters_once_r90.go:5` — §614.1c is right for the
  enters-as-a-copy replacement; §614.5 is a defensible sibling for the
  once-only claim, not a correction.
- `zone_change.go:1133` — §115.2/§601.2c/§602.2b/§603.3d are right for
  announcement-time target validation; §115.5 is a different claim.

**Decision on promoting it to a gate: not this round, and the reason is
runtime, not confidence.** The remaining 8 are read, so a baseline would now
be an honest one. But the script re-normalises the CR against every comment
block and takes 2–4 minutes — against `crcite`'s current 33s for the whole
package. It should be made incremental before it joins the gate suite.
