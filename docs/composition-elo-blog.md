# The Wrong Bias: How We Stopped Hunting Seat Position and Found Composition

A narrative companion to [`composition-elo.md`](composition-elo.md). The
reference doc tells you how the system works; this one tells you the
research story we walked to get there. Aimed at folks who think about
MTG game theory, deck-strength measurement, or rating-system design.

## The hypothesis we believed

Commander is a 4-player free-for-all. Seat order matters in ways that
1v1 chess doesn't — the player in seat 0 acts first every turn,
seat 3 acts last and can see what everyone else committed before
deciding whether to spend reactive mana. Political dynamics layer on
top: the perceived threat tends to sit in seat 0 or 1 (most-recently-
attacked-from is most-recently-counter-attacked), and the player in
seat 3 often gets a "free turn" as the table sorts out earlier
threats.

If seat-position effects are real, TrueSkill is silently broken in
exactly the way that matters most for deck-rating: a deck that
happens to draw the favored seat more often will accumulate more
wins, and its rating will climb for reasons unrelated to deck quality
or pilot skill.

The conventional fix is a per-seat ELO bonus / penalty: subtract some
points from seat-3 wins, add some to seat-0 wins, recover the
"true skill" residual. This is the standard treatment in most
multiplayer competitive frameworks (chess Swiss tournaments handle
bye-week distortion this way, for instance).

So we set out to measure the seat bias and apply the appropriate
prior. Spoiler: that approach is wrong, and the measurement told us
why.

## First measurement: looks like a real effect

[PR #258](../docs/seat-bias-measurement-r60.md) ran a single
4-deck rotate-mode tournament with 1000 games. The four decks were:

- **Phenax** (Mill)
- **Wyleth** (Voltron)
- **Kalamax** (Spellslinger)
- **Lord Windgrace** (LandsMatter)

After rotation, each deck spent ~250 games in each seat position.
Aggregate winrate by seat:

| Seat | Winrate |
|---:|---:|
| 0 | 24.8% |
| 1 | 24.7% |
| 2 | 24.4% |
| 3 | **26.1%** |

Seat 3 was +1.1pp above expected. With four seats and the same fair
rotation across 1000 games, seat 0/1/2 should have averaged 25.0%
each; the deviation to seat 3 was ~22 wins above the expected 250.
At n=500 effective games per seat the standard error is ~1.94pp,
so the observed +1.1pp deviation is ~0.6σ. Suggestive but not
significant.

The per-(commander, seat) decomposition was much more interesting:

- **Phenax (Mill) preferred seat 0** at 70.2% vs its mean 66.9%
  (~3σ above the per-commander baseline). Story: Mill spells resolve
  before opponents can disrupt — going first matters.
- **Kalamax (Spellslinger) preferred seat 3** at 17.8% vs mean 14.3%
  (~3σ). Story: reactive mana decisions improve with full information
  about what opponents already did.
- **Voltron uniformly weak** at ~3% across all seats (σ=0.3) — the
  single-creature plan dies to identical-AI removal regardless of
  position.

Two 3σ-significant per-archetype patterns + a directionally-plausible
global skew = we had what looked like a real, measurable seat bias.
The next step seemed obvious: scale up the measurement and derive a
per-archetype seat-penalty lookup table for TrueSkill to consume.

## The meta-study, and the surprise

[PR #322](../docs/seat-bias-meta-study-r60.md) was the scale-up.
The methodology was simple: if seat bias is real, it should show up
the same way across DIFFERENT 4-deck compositions, because the
mechanism (act-first / act-last advantage) doesn't depend on which
other 3 decks are at the table.

We picked 5 pod compositions spanning 18 of the 22 archetypes the
hat ships with, ran 5 seeds × 1500 games per composition, and made a
deliberate design choice: **Reanimator and LandsMatter would each
appear in TWO compositions** (Reanimator via Meren in C2 and Karador
in C5; LandsMatter via Lord Windgrace in C1 and Aesi in C5). These
were our cross-composition test cells — the only archetypes where we
could measure whether seat preference replicates across pod changes.

If seat bias is real (we'll call this the ELECTRON hypothesis: real,
stable, reproducible across contexts), the two cross-composition
archetypes should have matching seat patterns in both their pods.

If seat bias is confounded with composition (we'll call this QUARK:
the apparent signal is actually a property of the specific 4-deck
combination), the two archetypes should show wildly different
patterns in different pods.

37,500 games and 42 minutes later, the result was unambiguous.

### LandsMatter

- Pod C1 (Lord Windgrace, in {Mill, Voltron, Spellslinger,
  LandsMatter}): **27% winrate**
- Pod C5 (Aesi, in {LandsMatter, Blink, Reanimator, Stax}): **~50%
  winrate**

A **23 percentage-point swing** in absolute winrate from changing the
other 3 decks at the table. The within-pod seat-range — the actual
"seat preference" we were trying to measure — was 2.38pp. Composition
moved the baseline **10× more than seat did**.

### Reanimator

Less dramatic but the same shape: Reanimator's cross-composition
standard deviation was 7.27pp, while its within-composition seat range
was 0.64pp.

### Verdict

**QUARK.** Both directly-testable archetypes classified as composition-
confounded. The "+22 wins in seat 3" from PR #258 wasn't a stable
seat-position effect — it was specifically the C1 composition's
shape. Run the same measurement against a different 4-deck pool and
the headline number wouldn't replicate.

To rub it in, the global per-seat winrate across the 5 meta-study
pods showed seat-3 advantage in 3 pods, seat-1 advantage in 1, and
seat-FIRST advantage (yes, the opposite direction) in pod C5. The
direction of the "bias" depended entirely on which decks were
present.

### Why this is the right kind of disproof

The QUARK finding is more useful than a successful confirmation
would have been. It tells us:

1. The naïve per-seat prior would have been actively harmful — it
   would have corrected for a phenomenon that doesn't exist in
   general, while leaving the actual rating-distortion mechanism
   (composition) unmitigated.
2. The measurement framework works: we caught the confounding
   variable in the original data instead of after deployment.
3. The real problem is now clearly visible: **composition matters
   10× more than seat**, and we have no prior conditioning on it.

Lord Windgrace at 27% in one pod vs Aesi at 50% in another isn't a
deck-quality difference — both decks are reasonable LandsMatter
builds. It's the pod itself producing the difference.

## Reframing: the right prior is composition-conditioned

Once we accepted that composition is what we need to condition on,
the design space opens up. [PR #398](../docs/composition-elo-prior-r60.md)
laid out three options:

1. **Pairwise matchup table.** For each pair `(arch_a, arch_b)`,
   store a winrate. Predict a deck's expected winrate in a 4-pod by
   averaging its pairwise rates against the 3 opponents. Storage:
   ~250 cells (22 archetypes × 22 / 2). Pros: small, generalizes
   to unseen pods. Cons: assumes pairwise rates decompose additively,
   which they don't (the LandsMatter swing isn't captured by any pair
   of opponents alone).

2. **Full 4-archetype lookup.** Store a winrate for every
   `(a, b, c, d)` composition. Captures the LandsMatter
   non-additivity correctly. Storage: C(22, 4) = 7,315 cells. Cost:
   filling each at meta-study density (1500g per cell) would take
   ~11 million games. Tractable as eventual data but useless as a
   bootstrap.

3. **Tiered fallback.** Combine both — try the full-pod table first,
   fall back to pairwise, fall back to uniform on cold start.
   Best of both worlds; gets used as the full-pod table fills
   organically from live games.

We shipped Option 1 as the MVP ([PR #403](../docs/composition-elo-prior-r60.md)):
pairwise approximation, in-memory, fully self-contained. The bet was
that even a lossy approximation would beat the no-prior baseline,
because composition is so dominant that any conditioning at all
should help.

## The math

The prior stores two tables of pairwise counters: wins and games per
ordered `(deck-archetype, opponent-archetype)` pair. Given a deck X
in a 4-archetype pod {X, Y, Z, W}, the expected winrate is the mean
of X's pairwise winrate against each of Y, Z, W (skipping mirror seats
when X = Y / Z / W — a deck doesn't beat itself in expectation).

The prior outputs three things per query:

- A **point estimate** of the expected winrate
- A **confidence** scalar — `1 − exp(−n/50)` where n is the average
  pairwise sample count. Calibrated so n=50 gives 0.63 ("we have
  meaningful but not certain evidence") and n=150 gives 0.95
  ("we know what's going on here").
- A **Wilson 95% interval** — concrete `(low, high)` bounds on the
  winrate, derived from binomial proportion CI math. The Wilson form
  is more robust than the normal approximation for small n or extreme
  p̂, and degrades gracefully toward `(0, 1)` as n → 0 instead of
  producing undefined or negative bounds.

The prior plugs into TrueSkill via a μ-shift. For each player in a
specific game, an offset is computed:

```
offset = Weight × Confidence × MuOffsetScale × (ExpectedWinrate − 1/podSize)
```

The Gaussian rating update is fed `μ + offset` instead of raw `μ`,
and the resulting Δμ is mapped back to raw μ. Because the Gaussian
update is invariant under uniform shifts but breaks symmetry under
differential shifts, the offsets encode the composition's tilt and
the residual (post-update Δμ) is the player-skill signal cleanly
separated.

The cold-start property is load-bearing: Confidence is 0 when the
prior has no data, so the offset is 0, so the update reduces to
standard TrueSkill byte-exactly. The prior cannot make things worse
when it doesn't know what's going on.

## Validation

[PR #411](../docs/composition-elo-validation-r60.md) ran a
synthetic-outcome gauntlet across 5 pods × 5 seeds × 500 test games
= 2,500 prediction trials. The prior was bootstrapped on 1,000
held-out games per seed; both TrueSkill systems started cold and
made predict-then-observe-then-update calls on the test stream.

Two metrics:

- **Top-1 accuracy**: did `argmax(effective μ)` match the actual
  winner?
- **Mean log-loss**: `−log(softmax(effective μ / β)[actual_winner])`,
  measuring full-distribution prediction quality, not just argmax.

Critical implementation lesson during the validation: the first run
showed the prior HURTING prediction by 3pp. The bug was that I was
predicting from raw μ for both systems, but the prior-aware system
stores μ as "skill modulo composition" (offsets are baked OUT during
training so the rating represents what the deck would achieve in an
AVERAGE pod). At prediction time the offset has to be ADDED BACK to
get the pod-conditioned strength. Once fixed:

| Metric | Standard | Prior-aware | Δ |
|---|---:|---:|---:|
| Top-1 accuracy | 30.9% | 32.3% | **+1.4 pp** |
| Mean log-loss | 1.495 | 1.460 | **+0.036** |

**All 5 seeds improved on both metrics — no regression.** 4 of 5
pods improved on accuracy in the reference seed-42 run; the fifth
(C3) lost 1pp on accuracy but gained on log-loss, meaning the prior
shifted predicted probabilities closer to truth even when argmax
flipped to a slightly-less-frequent outcome.

The improvements are modest by design. Both systems converge with
100 in-distribution games, so the prior's value is the accelerated
cold-start window plus archetype-level transfer to unseen pods. In
production deployment the value should be larger because real
gauntlets see thousands of distinct compositions, most of which the
prior has partial knowledge of via the pairwise approximation.

## What this looks like in live games

[PR #415](../docs/composition-elo.md) put the prior into
showmatch's `updateELO` path. Every finished game now does:

1. Build the pod's archetype list from each deck's Freya profile.
2. Compute offsets for each seat via `ComputeCompositionOffset`.
3. Run the rating update in shifted-μ space.
4. Feed this game's outcome back to the prior via `ObserveGame`.

[PR #420](../docs/composition-elo.md) added monitoring — every game
emits a `[]CompositionPriorEffect` to Heimdall capturing what the
prior thought would happen and what actually happened, with a per-
seat **MuDeltaVsBaseline** field that isolates the prior's effect
(prior-applied μ-change minus what vanilla TrueSkill would have
produced on the same game).

[PR #424](../docs/composition-elo.md) added the
`hexdek-composition-replay` CLI for spot-checking individual games:

```
$ hexdek-composition-replay 555555
seat result   archetype     predicted%   confidence   offset_μ   Δ_vs_van   interpretation
0    —        Mill              62.0%        0.72       1.800     -0.900    expected better → amplified μ loss
1    WINNER   Voltron            5.0%        0.72      -1.200      2.100    upset win → amplified μ gain
2    —        Aggro             18.0%        0.50       0.000     -0.600    expected loss → dampened μ loss
3    —        Combo             15.0%        0.50      -0.600     -0.600    expected loss → dampened μ loss
```

You can see the prior's reasoning at a glance: Voltron beat Mill in a
pod where Mill was 62% favored, so Voltron's μ rises 2.1 points
MORE than vanilla TrueSkill would have given it. That's the prior
correctly identifying the upset as a strong skill signal rather than
weighing it the same as a chalk win.

## What we still don't know

The +1.4pp validation was synthetic — outcomes drawn from a known
distribution. Live data should produce larger gains in the cold-start
window for novel decks but might also reveal that our archetype labels
are too coarse (or too fine) for the composition signal to load
cleanly. Two follow-up studies the system invites:

1. **Cross-archetype interactions beyond pairwise.** PR #322 already
   showed that the LandsMatter swing isn't captured by any pair of
   matchups in isolation. As more games accumulate, the design's
   Option 3 (tiered fallback to a full-pod table for high-traffic
   compositions) should outperform pure pairwise — but we won't know
   how much without running it.
2. **Hat-conditioned variants.** The prior currently conditions on
   archetype alone. Live deployment with mixed hats (humans + bots,
   or different bot generations) may surface different patterns; the
   architecture supports adding a hat dimension if needed.

We also haven't deployed against actual human play yet. The play app
has the persistence wired ([PR #321](../docs/seat-bias-measurement-r60.md))
but as of this writing the game count is zero. Once humans start
playing, the prior will start accumulating real-world data and the
showmatch and human-game tracks will share the same composition table.
At that point the monitoring (PR #420) becomes the operative diagnostic
for whether the prior generalizes.

## What we learned

The biggest lesson isn't about TrueSkill or about composition — it's
methodological. The original PR #258 measurement looked like a
clear signal: +22 wins in seat 3, two 3σ per-archetype patterns,
plausible mechanism, plausible direction. Every check we routinely
apply to a single experiment came back positive.

But we built the meta-study anyway, with the deliberate design that
two archetypes would appear in multiple compositions. **That one
design decision is what produced the QUARK verdict.** Without
cross-composition observation we'd have shipped a per-seat ELO prior,
deployed it, and watched it produce confusing rating distortions for
months while we tried to figure out what was wrong.

The cheap reproducibility check is what made the difference. If you
think you've measured a real effect on N=1 contexts, build N≥2 next
— even at a 5× cost in compute — because the cost of acting on a
confounded measurement is much higher than the cost of one
additional measurement.

The other lesson is about graceful fallback. The prior has three
tiers (pairwise → archetype baseline → uniform) and a Confidence
gate that zeros out the offset when sample count is too low. None of
this would matter if the cold-start case mattered less than 0.1% of
production traffic. But because we wired the gate carefully — and
verified with byte-equivalent tests that cold-start matches vanilla
TrueSkill exactly — we get to deploy the prior into a live system
without any risk of regression on novel compositions.

## Related reading

- [`composition-elo.md`](composition-elo.md) — the formal reference
- [`seat-bias-measurement-r60.md`](seat-bias-measurement-r60.md) — PR #258
- [`seat-bias-meta-study-r60.md`](seat-bias-meta-study-r60.md) — PR #322
- [`composition-elo-prior-r60.md`](composition-elo-prior-r60.md) — PR #398 design
- [`composition-elo-validation-r60.md`](composition-elo-validation-r60.md) — PR #411 validation
