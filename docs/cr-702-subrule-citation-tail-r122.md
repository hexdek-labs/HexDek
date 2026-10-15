# r122 — the 51 were the tail of r94's own fix, and r121's sweep had the mirror blind spot

## What the 51 were

r121 left 51 unread candidates: turning on the `[a-z]?` subrule widening in
`wrong_number_by_topic_r94_test.go` took that gate from 0 violations to 51.
All 51 are now read. They are **not** 51 independent judgements — they are
**nine (rule, topic) pairs**, and **every one of the nine was already in
r94's own table**:

| cited | topic | which is | correct | rows |
|---|---|---|---|---|
| 702.110b | menace | Exploit | **702.111b** | 20 |
| 702.34a | madness | Flashback | **702.35a** | 10 |
| 702.137a | magecraft | Spectacle | **§207.2c** (ability word, no entry) | 10 |
| 702.139a/d | mutate | Companion | **702.140a/d** | 3 |
| 702.61b/c | suspend | Split Second | **702.62a** | 2 |
| 702.121b | tribute | Melee | **702.104b** | 2 |
| 701.15a | regenerat | Goad | **701.19a** | 2 |
| 702.171 | cloak | Saddle | **701.58** | 1 |
| 701.62a | suspect | Manifest Dread | **701.60a** | 1 |

**r94 found the right answers, fixed the bare-number instances, and its own
regex excluded sub-lettered ones** (`\b701.15([^0-9a-z]|$)` cannot match
`701.15a`). So the fix left its own tail behind, invisible to the gate that
defined it, for 27 rounds. r121 widened the *window*; r122 widens the
*pattern*, and the widening is now **on**.

## The magecraft group is a different fix

Magecraft is an **ability word**. Per §207.2c ability words "have no special
rules meaning and no individual entries in the Comprehensive Rules", so
there is no number to cite — §702.137 is Spectacle.

`keywords_magecraft.go`'s own header says exactly this:

> Magecraft is an ABILITY WORD. Per CR §207.2c … there is no number to cite.
> This file carried §702.137 until r96 — that is Spectacle.

**Four lines below that paragraph the file cites `CR §702.137a` again**, and
did so ten more times across six files, including an event-log string. r96
wrote the explanation, converted one line to "CR (ability word, no rule)",
and left the rest. A further **11 files** carry
`CR §702.137a / "whenever you copy a spell"` for the magecraft copy branch —
invisible to the gate because the word "magecraft" is not on those lines.
All now cite §207.2c and say why.

Two sites are left citing §702.137 deliberately: both are historical notes
recording that the file *used* to carry it.

## r121's sweep had the mirror blind spot

r121 keyed on the event `Kind` and checked the citation — but filtered to
citations beginning `701.`. **A §701 keyword action cited as a §702 rule was
structurally invisible to it**, exactly as an event-log citation had been
invisible to r94. Checking both directions found six more, five of them
event-log strings:

| event | cited | which is | correct |
|---|---|---|---|
| `adapt` | 702.137 | Spectacle | **701.46** |
| `monstrosity` | 702.94 | Miracle | **701.37** |
| `exert` | 702.112 | Renown | **701.43** |
| `visit` | 702.177 | Exhaust | **702.159** |
| `partner_with_search` / `_no_match` | 702.124g | choosing among partner abilities | **702.124j** |
| typecycling `reveal` | 702.29c | cycling *triggers* | **702.29e** |

The last two are the subtle kind: right base rule, wrong sub-letter, and the
cited subrule says something real but **not** what the code claims.
§702.124g is about choosing between multiple partner abilities; §702.29c is
about cycling triggers. Neither performs the reveal the code logs.

## The new gate derives the answer instead of listing it

`TestCRCitations_EventRuleNamesItsAction_R122` joins each event's `Kind`
against the CR's own §701/§702 headings. **233 actions known from the CR, 0
violations.** It needs no maintenance when a keyword is added and catches a
wrong citation the first time one is written — where the r94 table can only
catch a mistake someone has already made somewhere else.

Two sites are allowlisted in the test with their reason, both read against
the CR: a generic `reveal` event citing the specific ability that caused it
(`partner with`, typecycling), each carrying a `reason` field that names it.

## Verification

Both gates red pre-change — **51** for the widened r94 pattern, **6** for the
derived r122 join — and 0 after.

## Lesson

**A fix is not finished until the detector that defined it can see its own
tail.** r94 wrote nine correct (wrong-number, topic) pairs and fixed what its
regex matched. The unmatched remainder sat for 27 rounds *behind the gate
that knew the answer*. Three rounds running the defect has been the shape of
the instrument rather than the content of it: r120 an escape hatch, r121 a
one-line window, r122 a pattern that excluded its own subrules.
