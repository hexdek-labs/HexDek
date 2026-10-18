package hat

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// r180a — differential tests for the two predicate clusters moved onto the
// per-Card text-predicate memo.
//
// The whole risk in this change is TRANSCRIPTION: the switch in
// scoreStaxLock is ORDERED (first match wins) and carries six distinct
// value constants, and the needles overlap heavily ("whenever" appears in
// two cases, "opponent"/"opponents" in four). A handful of hand-written
// cards cannot distinguish "I moved the expressions correctly" from "I
// reordered a case" or "I dropped a conjunct". So the legacy bodies are
// kept VERBATIM below and the two implementations are compared over a
// large generated corpus assembled from the needle phrases themselves plus
// filler, so that every case and every near-miss (a conjunct present
// without its partner, a case that would match if an earlier case had not)
// is exercised.

// -----------------------------------------------------------------------------
// Verbatim legacy bodies. Do not "clean these up" — they exist to be
// diffed by eye against the originals.
// -----------------------------------------------------------------------------

func (e *GameStateEvaluator) scoreStaxLockLegacy(gs *gameengine.GameState, seatIdx int) float64 {
	seat := gs.Seats[seatIdx]
	var score float64
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		ot := gameengine.OracleTextLower(p.Card)
		value := 0.0
		switch {
		case strings.Contains(ot, "nonland permanents") && strings.Contains(ot, "don't untap"):
			value = 1.5
		case strings.Contains(ot, "can't cast") || (strings.Contains(ot, "each opponent") && strings.Contains(ot, "can't")):
			value = 1.2
		case strings.Contains(ot, "additional") && strings.Contains(ot, "cost") && strings.Contains(ot, "pay"):
			value = 1.0
		case strings.Contains(ot, "enters the battlefield tapped") && strings.Contains(ot, "opponents"):
			value = 0.8
		case strings.Contains(ot, "whenever") && strings.Contains(ot, "opponent") && (strings.Contains(ot, "loses") || strings.Contains(ot, "damage")):
			value = 0.6
		case strings.Contains(ot, "sacrifice") && strings.Contains(ot, "each") && strings.Contains(ot, "opponent"):
			value = 0.8
		}
		if value > 0 {
			score += value
		}
	}
	return score / 2.0
}

func (h *YggdrasilHat) bestOpponentRemovalScoreLegacy(gs *gameengine.GameState, seatIdx int) float64 {
	if gs == nil {
		return 0
	}
	best := 0.0
	for i, s := range gs.Seats {
		if i == seatIdx || s == nil || s.Lost || s.LeftGame {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			score := 0.40
			if p.IsCreature() {
				if gs.PowerOf(p) >= 4 {
					score = 0.60
				}
			}
			if p.IsPlaneswalker() {
				score = 0.60
			}
			if h.isComboRelevant(p.Card) {
				score = 0.95
			} else if h.isValueEngineKey(p.Card) {
				score = 0.80
			} else if isCommanderCard(gs, i, p.Card) {
				if score < 0.75 {
					score = 0.75
				}
			} else if p.Card.AST != nil {
				// Has triggered abilities → engine piece.
				ot := gameengine.OracleTextLower(p.Card)
				if strings.Contains(ot, "whenever") || strings.Contains(ot, "at the beginning") {
					if score < 0.55 {
						score = 0.55
					}
				}
			}
			if score > best {
				best = score
			}
		}
	}
	return best
}

// -----------------------------------------------------------------------------
// Corpus generation
// -----------------------------------------------------------------------------

// r180aFragments are every needle the two clusters test for, plus the
// near-miss variants that make an ordered switch observable: a conjunct on
// its own, a plural/singular swap, and words that are substrings of a
// needle without being the needle.
var r180aFragments = []string{
	// case 1
	"nonland permanents", "don't untap", "nonland", "permanents don't",
	// case 2
	"can't cast", "each opponent", "can't", "can't be cast",
	// case 3
	"additional", "cost", "pay", "as an additional cost to cast this spell",
	// case 4
	"enters the battlefield tapped", "opponents", "enters the battlefield",
	// case 5
	"whenever", "opponent", "loses", "damage", "loses life",
	// case 6
	"sacrifice", "each", "sacrifices a creature",
	// bestOpponentRemovalScore
	"at the beginning", "at the beginning of your upkeep", "beginning",
	// filler that must never flip anything
	"target creature gets +1/+1", "draw a card", "flying", "", "untap",
	"casts", "costs {1} more", "the beginning", "oppose", "lose",
}

// r180aCaseSatisfiers are minimal texts that each satisfy exactly one of
// scoreStaxLock's six cases. Combining them is what makes the ORDER of the
// switch observable: a text satisfying cases 5 and 6 must score 0.6, not
// 0.8, and nothing else in the corpus proves that. A control that swaps
// two cases must go red, and without these it does not.
var r180aCaseSatisfiers = []string{
	"nonland permanents don't untap",
	"each opponent can't cast spells",
	"as an additional cost to cast this spell, pay 2 life",
	"lands enter the battlefield tapped for opponents",
	"whenever an opponent loses life, deal damage",
	"each opponent sacrifices a creature",
}

// r180aCorpus builds n distinct oracle texts from random fragment
// combinations under a fixed seed.
func r180aCorpus(n int) []string {
	rng := rand.New(rand.NewSource(0x180a))
	out := make([]string, 0, n)

	// Every non-empty subset of the six case-satisfiers, in both forward
	// and reverse concatenation order, so each pair/triple of cases is
	// presented to the switch twice with the text itself ordered
	// differently. 63 subsets x 2 = 126 ordering probes.
	for mask := 1; mask < 1<<len(r180aCaseSatisfiers); mask++ {
		var parts []string
		for i, s := range r180aCaseSatisfiers {
			if mask&(1<<i) != 0 {
				parts = append(parts, s)
			}
		}
		out = append(out, strings.Join(parts, ". "))
		rev := make([]string, len(parts))
		for i := range parts {
			rev[i] = parts[len(parts)-1-i]
		}
		out = append(out, strings.Join(rev, ". "))
	}

	// Seed the corpus with every single fragment and every PAIR of the
	// first 24 fragments, so the deterministic part covers each case's
	// conjuncts in isolation and in combination before random filling.
	for _, f := range r180aFragments {
		out = append(out, f)
	}
	for i := 0; i < 24 && i < len(r180aFragments); i++ {
		for j := 0; j < 24 && j < len(r180aFragments); j++ {
			if len(out) >= n {
				break
			}
			out = append(out, r180aFragments[i]+" "+r180aFragments[j])
		}
	}
	for len(out) < n {
		k := 1 + rng.Intn(5)
		parts := make([]string, 0, k)
		for i := 0; i < k; i++ {
			parts = append(parts, r180aFragments[rng.Intn(len(r180aFragments))])
		}
		out = append(out, strings.Join(parts, " "))
	}
	return out[:n]
}

// r180aCard builds a card whose lowered oracle text is `text`, through the
// real AST path so OracleTextLower behaves exactly as in production.
func r180aCard(name, text string) *gameengine.Card {
	return &gameengine.Card{
		Name:  name,
		Types: []string{"creature"},
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{&gameast.Static{Raw: text}},
		},
	}
}

const r180aCorpusSize = 2400

// -----------------------------------------------------------------------------
// A. scoreStaxLock
// -----------------------------------------------------------------------------

func TestR180a_ScoreStaxLockDifferential(t *testing.T) {
	corpus := r180aCorpus(r180aCorpusSize)
	if len(corpus) < 2000 {
		t.Fatalf("corpus too small: %d", len(corpus))
	}
	e := &GameStateEvaluator{}

	// Per-card, so a disagreement names the exact text rather than
	// showing up as a mismatched sum.
	mismatches := 0
	for i, text := range corpus {
		gs := gameengine.NewGameState(2, rand.New(rand.NewSource(1)), nil)
		card := r180aCard("R180a Stax Probe", text)
		gs.Seats[0].Battlefield = []*gameengine.Permanent{{Card: card, Controller: 0}}

		want := e.scoreStaxLockLegacy(gs, 0)
		got := e.scoreStaxLock(gs, 0)
		if got != want {
			mismatches++
			if mismatches <= 10 {
				t.Errorf("card %d: scoreStaxLock=%v legacy=%v for oracle text %q",
					i, got, want, text)
			}
		}
	}
	if mismatches > 0 {
		t.Fatalf("%d/%d corpus cards disagreed", mismatches, len(corpus))
	}

	// And once more in bulk, with the whole corpus on one battlefield, so
	// the accumulation and the /2.0 are compared too.
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(1)), nil)
	bf := make([]*gameengine.Permanent, 0, len(corpus))
	for i, text := range corpus {
		bf = append(bf, &gameengine.Permanent{Card: r180aCard("R180a Bulk", text), Controller: 0})
		_ = i
	}
	gs.Seats[0].Battlefield = bf
	if got, want := e.scoreStaxLock(gs, 0), e.scoreStaxLockLegacy(gs, 0); got != want {
		t.Fatalf("bulk scoreStaxLock=%v legacy=%v", got, want)
	}

	t.Logf("scoreStaxLock differential: %d cards, exact float64 agreement", len(corpus))
}

// Every value constant must actually be produced by the corpus, otherwise
// the differential above is green because nothing matched.
func TestR180a_CorpusReachesEveryStaxCase(t *testing.T) {
	corpus := r180aCorpus(r180aCorpusSize)
	e := &GameStateEvaluator{}
	seen := map[float64]int{}
	for _, text := range corpus {
		gs := gameengine.NewGameState(2, rand.New(rand.NewSource(1)), nil)
		gs.Seats[0].Battlefield = []*gameengine.Permanent{
			{Card: r180aCard("R180a Coverage", text), Controller: 0},
		}
		// score is value/2.0 for a single permanent.
		seen[e.scoreStaxLock(gs, 0)*2.0]++
	}
	for _, v := range []float64{0.0, 1.5, 1.2, 1.0, 0.8, 0.6} {
		if seen[v] == 0 {
			t.Errorf("no corpus card produced value %v — the differential is vacuous for that case", v)
		}
	}
	t.Logf("value histogram: %v", seen)
}

// -----------------------------------------------------------------------------
// B. bestOpponentRemovalScore
// -----------------------------------------------------------------------------

func TestR180a_BestOpponentRemovalScoreDifferential(t *testing.T) {
	corpus := r180aCorpus(r180aCorpusSize)
	h := &YggdrasilHat{}

	mismatches := 0
	for i, text := range corpus {
		gs := gameengine.NewGameState(2, rand.New(rand.NewSource(1)), nil)
		card := r180aCard("R180a Removal Probe", text)
		gs.Seats[1].Battlefield = []*gameengine.Permanent{{Card: card, Controller: 1}}

		want := h.bestOpponentRemovalScoreLegacy(gs, 0)
		got := h.bestOpponentRemovalScore(gs, 0)
		if got != want {
			mismatches++
			if mismatches <= 10 {
				t.Errorf("card %d: bestOpponentRemovalScore=%v legacy=%v for oracle text %q",
					i, got, want, text)
			}
		}
	}
	if mismatches > 0 {
		t.Fatalf("%d/%d corpus cards disagreed", mismatches, len(corpus))
	}

	// Bulk, several permanents per seat, so the max-over-board path and
	// the seat loop are compared as well.
	gs := gameengine.NewGameState(4, rand.New(rand.NewSource(1)), nil)
	for i, text := range corpus {
		seat := 1 + i%3
		gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield,
			&gameengine.Permanent{Card: r180aCard("R180a Bulk Removal", text), Controller: seat})
	}
	if got, want := h.bestOpponentRemovalScore(gs, 0), h.bestOpponentRemovalScoreLegacy(gs, 0); got != want {
		t.Fatalf("bulk bestOpponentRemovalScore=%v legacy=%v", got, want)
	}

	// The nil-AST guard must be preserved: a card with no AST must not be
	// scored as an engine piece by either implementation.
	gsNil := gameengine.NewGameState(2, rand.New(rand.NewSource(1)), nil)
	nilAST := &gameengine.Card{Name: "R180a No AST", Types: []string{"creature"}}
	gsNil.Seats[1].Battlefield = []*gameengine.Permanent{{Card: nilAST, Controller: 1}}
	if got, want := h.bestOpponentRemovalScore(gsNil, 0), h.bestOpponentRemovalScoreLegacy(gsNil, 0); got != want {
		t.Fatalf("nil-AST card: got %v want %v", got, want)
	}

	t.Logf("bestOpponentRemovalScore differential: %d cards, exact float64 agreement", len(corpus))
}

// Sanity: the corpus actually drives the 0.55 engine-piece bump, otherwise
// the differential never exercises the cached bit.
func TestR180a_CorpusReachesTriggerWordingBump(t *testing.T) {
	corpus := r180aCorpus(r180aCorpusSize)
	h := &YggdrasilHat{}
	bumps, plain := 0, 0
	for _, text := range corpus {
		gs := gameengine.NewGameState(2, rand.New(rand.NewSource(1)), nil)
		gs.Seats[1].Battlefield = []*gameengine.Permanent{
			{Card: r180aCard("R180a Trigger Coverage", text), Controller: 1},
		}
		switch h.bestOpponentRemovalScore(gs, 0) {
		case 0.55:
			bumps++
		case 0.40:
			plain++
		}
	}
	if bumps == 0 || plain == 0 {
		t.Fatalf("corpus did not exercise both sides of the trigger-wording bump (bumps=%d plain=%d)", bumps, plain)
	}
	t.Logf("trigger-wording bump: %d bumped, %d plain", bumps, plain)
}

// -----------------------------------------------------------------------------
// The memo must be AST-keyed, not compute-once
// -----------------------------------------------------------------------------

// Reassigning Card.AST (what a DFC transform does) must invalidate every
// r180a bit. r157 shipped a "have we computed this" bool and spent a round
// serving the front face of a transformed card for the rest of the game.
func TestR180a_PredicatesAreASTKeyed(t *testing.T) {
	card := r180aCard("R180a Transform Probe", "nonland permanents don't untap")

	if !cachedStaxNonlandDontUntap(card) {
		t.Fatalf("front face: want true")
	}
	if cachedStaxCantCast(card) {
		t.Fatalf("front face: cachedStaxCantCast want false")
	}
	if cachedHasTriggerWording(card) {
		t.Fatalf("front face: cachedHasTriggerWording want false")
	}

	// Transform: a DIFFERENT AST with different text, assigned in place.
	card.AST = &gameast.CardAST{
		Abilities: []gameast.Ability{
			&gameast.Static{Raw: "each opponent can't cast spells. whenever this attacks, draw a card"},
		},
	}

	if cachedStaxNonlandDontUntap(card) {
		t.Fatalf("back face: cachedStaxNonlandDontUntap still true — the memo is not AST-keyed")
	}
	if !cachedStaxCantCast(card) {
		t.Fatalf("back face: cachedStaxCantCast want true — the memo is not AST-keyed")
	}
	if !cachedHasTriggerWording(card) {
		t.Fatalf("back face: cachedHasTriggerWording want true — the memo is not AST-keyed")
	}
}

// The scorers must actually route through the memo; otherwise a revert to
// the inline strings.Contains stays green and this round tests nothing.
func TestR180a_ScorersRouteThroughTheMemo(t *testing.T) {
	gameengine.ResetTextPredicateStatsForTest()
	t.Cleanup(gameengine.ResetTextPredicateStatsForTest)

	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Seats[0].Battlefield = []*gameengine.Permanent{
		{Card: r180aCard("R180a Winter Orb", "nonland permanents don't untap"), Controller: 0},
	}
	gs.Seats[1].Battlefield = []*gameengine.Permanent{
		{Card: r180aCard("R180a Engine", "whenever this creature attacks, draw a card"), Controller: 1},
	}

	e := &GameStateEvaluator{}
	h := &YggdrasilHat{}

	e.scoreStaxLock(gs, 0)
	h.bestOpponentRemovalScore(gs, 0)
	h1, m1, _ := gameengine.TextPredicateStats()
	if h1+m1 == 0 {
		t.Fatalf("the scorers consulted the memo zero times — not wired through TextPredicate")
	}

	e.scoreStaxLock(gs, 0)
	h.bestOpponentRemovalScore(gs, 0)
	h2, m2, _ := gameengine.TextPredicateStats()
	if h2 <= h1 {
		t.Fatalf("second pass produced no cache hit (hits %d -> %d)", h1, h2)
	}
	if m2 != m1 {
		t.Fatalf("second pass recomputed a predicate (misses %d -> %d)", m1, m2)
	}
}
