# r158 — HasKeyword re-normalized constants on every call

## Where the round came from

r157 established the rule: re-take the CPU profile after every round,
because the shape changes. On the r157 binary:

```
strings.Contains                 20.53% of all CPU   (was 20.69%)
(*Permanent).HasKeyword          12.48% cum          (was 11.12%)
strings.ToLower                   7.82%
GetEffectiveCharacteristics       6.28% cum
```

`HasKeyword` is now the **largest single named function in the engine**.

## The measurement, before the fix

`-list` on the production profile, rather than reading the function and
guessing:

```
line 58  want := ToLower(TrimSpace(name))          950ms
line 61  range p.Card.AST.Abilities                170ms
line 62  type-assert *gameast.Keyword              180ms
line 63  ToLower(TrimSpace(kw.Name)) == want       820ms
line 81  p.Flags["kw:"+want]                      2510ms
line 94  p.Counters[want]                          390ms
                                        cum        7.47s
```

**I would have guessed the AST loop. The biggest single line is the
Flags probe** — a string concatenation plus a map hash, per call.

## What this round changes, and what it deliberately does not

**Fixed: 58, 61, 62, 63 — 2.12s, 28% of the function, ~3.5% of all
CPU.** All four re-derive values that never change between calls.

- `normalizeKeyword(s)` is `ToLower(TrimSpace(s))` with a fast path
  that allocates nothing and stops at the first offending byte when the
  string is already trimmed lowercase ASCII — which it nearly always
  is, because callers pass compile-time literals. Interior spaces are
  left alone, since "first strike" is one keyword.
- `astKeywordNames(c)` normalizes the AST's Keyword names **once per
  AST** instead of once per call, which also removes the per-call type
  assertion over every ability on the card.

**Not fixed: line 81, the 2510ms one.** Removing it means either moving
135 production `Flags["kw:"...]` sites to a prefix-free map, or caching
`HasKeyword`'s answer. The answer depends on `GrantedAbilities`,
`Flags` and `Counters` — all mutable at runtime, none carrying an epoch.
**r157 is the standing warning against building a cache on an
invalidation nobody maintains**, and it would be a poor read of that
round to immediately do the thing it warned about. Measured, documented,
and left for a round that can afford the invalidation design.

## Keying — r157 applied forward

The new per-Card keyword list is keyed on the **AST pointer**, like
`oracleTextAST`, not on a readiness bool. A DFC transform reassigns
`Card.AST`, so a transformed card recomputes by construction. r157 found
that bug by being bitten; r158 is the first cache added after it and is
built the right way from the start. The regression proves it: flying
before the transform, trample after.

## Controls

**5 of 5 red — but three of them had to be redone, which is the part
worth recording:**

| control | result |
|---|---|
| A — drop the trailing-whitespace check | 1 red |
| B — fast path ignores uppercase ASCII | **patch silently failed first**; redone → 2 red |
| C — fast path ignores non-ASCII bytes | **patch failed, then broke the build**; redone twice → 1 red |
| D — cache keyed on a readiness bool (the r157 bug) | 2 red, incl. the derived gate |
| E — normalization dropped entirely | 2 red |

B and C hit the r152 trap exactly as the brief describes it: the assert
threw, and the shell printed `ok` from the *unmodified* run immediately
afterwards. A control whose patch did not apply tested nothing. Caught
because the traceback was visible in the same output as the `ok`.

The differential test is the load-bearing one: `normalizeKeyword` is
checked against `strings.ToLower(strings.TrimSpace(s))` over a table
that includes NBSP, U+0085, interior spaces, and mixed-case non-ASCII —
inputs nastier than production, because a fast path is only worth having
if it is identical on everything.

## Prediction

**~2-3% throughput, allocation unchanged or very slightly down.** The
2.12s removed is 3.5% of CPU, and the fast path removes allocations the
old `ToLower` made on any non-lowercase input, but allocation per game
is dominated elsewhere. Stated before measuring.

## ELO

**Comparable.** No rules behaviour changes — `normalizeKeyword` is
proven identical to what it replaces, and the cached list holds the same
strings the loop computed.
