# r120 — the §704.5 subrule shift, the rule it hid, and the gate that knew

## The shift

From `sba704_5w` onward the engine's state-based-action functions were one
letter behind the Comprehensive Rules. Every comment's quoted TEXT was
right; every NUMBER was one behind.

```
code (before)        really is      what it does
sba704_5v            v AND w        battle at defense 0 → graveyard
sba704_5w            704.5x         battle with no protector
sba704_5x            704.5y         protector who can't be protector
sba704_5y            704.5z         Role uniqueness
sba704_5z            704.5aa        start your engines!
```

The cause is visible in the first row. The CR split defense-0 battles into
two rules — §704.5v for Sieges and §704.5w for non-Sieges — and the engine
kept one merged function. Every later letter slid up by one to fill the gap
the code never opened.

After this round the engine's §704.5 citation set is **exactly** the CR's:
25 letters, a–k, m, n, p–aa, bijective, nothing dangling and nothing
missing. That set is checked programmatically rather than by eye.

## The rule the merge was hiding

This was filed as a renumbering. It is not only that.

```
704.5v  If a SIEGE battle has defense 0 and it isn't the source of an
        ability that has triggered but not yet left the stack, it's put
        into its owner's graveyard.
704.5w  If a NON-SIEGE battle has defense 0, it's put into its owner's
        graveyard.
```

The triggered-ability clause belongs to the Siege arm **only**. The merged
function applied it to every battle, so a non-Siege battle at defense 0 that
was the source of a trigger still on the stack stayed on the battlefield
when the CR puts it into the graveyard immediately.

Reachable, and counted rather than assumed: the corpus holds **39 battle
front-faces, 37 Siege and 2 non-Siege** — `Occupation of Kulrath` and
`Occupation of Llanowar`, both `Battle — Control Point`.

Splitting the function both fixes the rule and frees the name `w`, so the
renumbering and the behavioural fix are one change rather than two.

## Could crcite have caught it? Yes, and it was built to

The brief asked whether the text-lock ratchet can catch a number shifting
under a stable meaning. Reading `textlock_test.go`'s own header answers it:

> The August 2026 CR inserts a new §704.5w without renumbering anything.
> Every letter that existed still exists — but from w onward each one now
> means what the PREVIOUS letter used to mean. Feb's §704.5y ("a permanent
> has more than one Role") is Aug's §704.5z; Aug's §704.5y is a different
> rule about battle protectors.

**The gate's author described this exact defect, named the exact rules, and
built the mechanism to catch it.** The lock pins each cited rule's text by
digest, so on the edition bump every affected citation goes red.

It did not fire because the lock is regenerated with
`CRCITE_UPDATE_LOCK=1`, which re-pins the NEW text to the OLD citation and
returns to green. Its own comment says "Regenerate ONLY as part of a
deliberate CR edition adoption" and "the edition bump and the citation
sweep therefore cannot land apart" — but that is a human discipline, not a
check. The bump was adopted, the lock was regenerated, the sweep did not
happen, and the lock has reported `0 drifted` ever since.

So: the ratchet catches this class exactly once, at the bump, and the
escape hatch erases the evidence.

### A correction I nearly published

While reading the lock I measured that **all 425 pinned subrules carry text
that is not their own** — `704.5aa` pinned to "The state-based actions are
as follows:", `702.34a` to "Flashback" — and started writing that up as a
100% defect in the instrument. It is not a defect. Column 3 is
`c.Heading(r)`, a human-readable label with a documented parent fallback;
column 2 is `Digest(c.Text(r))` over the real rule line. Recomputing the
digest of §704.5aa's actual text gives `d6475c1dd87c`, which is exactly
what the lock holds. The claim was wrong and is withdrawn.

## The detector the lock cannot be

`scripts/detect_citation_text_mismatch_r120.py` compares each comment
block's QUOTED rule text against the CR text of the rule number that block
cites, and reports which rule the quote actually belongs to. It needs no
edition bump, survives a lock regeneration, and works from the code alone.

Run against the tree immediately before this round, it flags the shift:

```
sba.go:1774  cites ['704.5y']  but the quoted text is ['303.7a', '704.5z']
sba.go:1864  cites ['704.5z']  but the quoted text is ['704.5aa']
sba.go:1873  cites ['704.5z']  but the quoted text is ['704.5aa']
```

Run against the tree after it, those three rows are gone: 20 → 17.

**The remaining 17 are candidates, not findings.** They are untriaged, and
the detector has a known false-positive mode — a file-header block that
cites several related rules while quoting one of them legitimately. Several
do look like the same edition-drift class (`scry.go` cites 701.18a/b while
quoting 701.22a/b; `zone_change.go` cites 701.17a/b while quoting 701.21a),
but none of that is established here and none of it was changed. It is a
worklist for a later round, in the shape r95/r96 established for
`mismatch_baseline.txt`.

This is deliberately a script and not a gate: wiring it green would require
baselining 17 sites nobody has read yet, and a suppression file written
before the triage is how `mismatch_baseline.txt` became a worklist in the
first place.
