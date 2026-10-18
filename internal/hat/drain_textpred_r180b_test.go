package hat

import (
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// r180b — scoreDrainEngine and scoreOpponentGraveyard's enabler clause
// moved onto the per-Card text-predicate memo (bits 16-21).
//
// The whole risk of that move is TRANSCRIPTION, not design: five
// multi-clause boolean expressions were lifted verbatim out of the
// evaluator into named functions, and one of them ends in
// !strings.Contains(ot, "when") — a negation that a hand-written
// three-card test would almost certainly agree with by accident.
//
// So these tests keep a VERBATIM copy of the pre-r180b bodies and
// assert exact float64 equality against the live implementations over
// a large generated corpus of synthetic battlefields built from random
// combinations of the needle phrases plus filler. A changed evaluator
// score changes AI decisions and therefore ELO; "close enough" is not
// a pass condition here, so the comparison is ==, not a tolerance.

// --- Legacy (pre-r180b) implementations, copied verbatim --------------

// scoreDrainEngineLegacy is the body of scoreDrainEngine exactly as it
// stood before r180b. Do not "tidy" it: its value is being a literal
// copy of the thing under test.
func (e *GameStateEvaluator) scoreDrainEngineLegacy(gs *gameengine.GameState, seatIdx int) float64 {
	seat := gs.Seats[seatIdx]

	drainPayoffs := 0.0
	sacOutlets := 0.0
	fodder := 0.0
	lifeGainPayoffs := 0.0

	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		ot := gameengine.OracleTextLower(p.Card)

		// Death-trigger drain payoffs: Blood Artist, Zulaport Cutthroat,
		// Bastion of Remembrance, Vindictive Vampire, Syr Konrad, etc.
		if strings.Contains(ot, "whenever") &&
			(strings.Contains(ot, "creature dies") || strings.Contains(ot, "a creature dies")) &&
			(strings.Contains(ot, "loses") || strings.Contains(ot, "drain") ||
				strings.Contains(ot, "each opponent") || strings.Contains(ot, "life")) {
			drainPayoffs += 1.0
		}

		// Lifegain-trigger drain: Dina, Vito, Marauding Blight-Priest
		if strings.Contains(ot, "whenever you gain life") &&
			(strings.Contains(ot, "loses") || strings.Contains(ot, "each opponent")) {
			lifeGainPayoffs += 1.0
		}

		// ETB/LTB drain: Wayward Servant, Corpse Knight
		if strings.Contains(ot, "whenever") &&
			strings.Contains(ot, "enters") &&
			(strings.Contains(ot, "loses") || strings.Contains(ot, "each opponent")) {
			drainPayoffs += 0.7
		}

		// Sacrifice outlets (free or cheap)
		if (strings.Contains(ot, "sacrifice a creature") || strings.Contains(ot, "sacrifice another") ||
			strings.Contains(ot, "sacrifice an artifact") || strings.Contains(ot, "sacrifice a permanent")) &&
			!strings.Contains(ot, "when") {
			sacOutlets += 1.0
		}

		// Fodder: tokens and small creatures with death triggers
		if p.IsToken() && !p.IsCreature() {
			fodder += 0.3 // Treasure, Food, Clue tokens
		}
		if p.IsCreature() {
			if p.IsToken() {
				fodder += 0.5
			} else if strings.Contains(ot, "when") && strings.Contains(ot, "dies") {
				fodder += 0.8
			}
		}
	}

	if drainPayoffs == 0 && lifeGainPayoffs == 0 {
		return 0
	}

	// Score based on how many drain engine pieces are assembled.
	// Having payoff + outlet + fodder is much more valuable than just payoff.
	score := drainPayoffs * 0.5
	score += lifeGainPayoffs * 0.4

	if sacOutlets > 0 {
		score += sacOutlets * 0.4
		// Payoff + outlet synergy multiplier
		score *= 1.0 + math.Min(sacOutlets, 2)*0.3
	}

	if fodder > 0 {
		score += fodder * 0.2
		// Full engine assembled: payoff + outlet + fodder
		if sacOutlets > 0 {
			score *= 1.0 + math.Min(fodder, 4)*0.15
		}
	}

	// Opponent life totals factor: drain is more valuable when opponents
	// are already low.
	avgOppLife := 0.0
	oppN := 0
	for i, s := range gs.Seats {
		if i == seatIdx || s.Lost || s.LeftGame {
			continue
		}
		avgOppLife += float64(s.Life)
		oppN++
	}
	if oppN > 0 {
		avgOppLife /= float64(oppN)
		if avgOppLife < 20 {
			score *= 1.0 + (20-avgOppLife)/20*0.5
		}
	}

	return score
}

// scoreOpponentGraveyardLegacy is the body of scoreOpponentGraveyard
// exactly as it stood before r180b. Only the battlefield-enabler clause
// changed; the rest is copied so the comparison covers the whole
// function and not just the clause in isolation.
func (e *GameStateEvaluator) scoreOpponentGraveyardLegacy(gs *gameengine.GameState, seatIdx int) float64 {
	threat := 0.0
	for i, s := range gs.Seats {
		if i == seatIdx || s.Lost || s.LeftGame {
			continue
		}
		for _, c := range s.Graveyard {
			if c == nil {
				continue
			}
			// High-CMC creatures are prime reanimation targets.
			isCreature := false
			for _, t := range c.Types {
				if t == "creature" {
					isCreature = true
					break
				}
			}
			if isCreature && c.CMC >= 5 {
				threat -= 0.20
			} else if isCreature && c.CMC >= 3 {
				threat -= 0.08
			}

			// Spells with flashback/escape/unearth/retrace can be re-cast.
			// r172 — cached on bit 5; same four needles as before.
			if cachedOpponentGraveyardRecastKeyword(c) {
				threat -= 0.15
			}
		}

		// Check if opponent has reanimation enablers on battlefield.
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			ot := gameengine.OracleTextLower(p.Card)
			if (strings.Contains(ot, "return") || strings.Contains(ot, "put")) &&
				strings.Contains(ot, "graveyard") &&
				strings.Contains(ot, "battlefield") {
				threat -= 0.30
			}
		}

		// Commanders known for graveyard abuse amplify the threat.
		for _, cn := range s.CommanderNames {
			cnLower := strings.ToLower(cn)
			if strings.Contains(cnLower, "meren") ||
				strings.Contains(cnLower, "muldrotha") ||
				strings.Contains(cnLower, "karador") ||
				strings.Contains(cnLower, "chainer") ||
				strings.Contains(cnLower, "araumi") ||
				strings.Contains(cnLower, "nethroi") ||
				strings.Contains(cnLower, "sefris") {
				threat -= 0.40
				break
			}
		}
	}
	return threat
}

// --- Corpus generation -------------------------------------------------

// r180bFragments are the needle phrases the predicates under test read,
// plus near-miss variants and inert filler. The near misses matter more
// than the hits: "whenever" without "dies", "sacrifice a creature" in a
// sentence that also says "when", "returns" vs "return", and so on.
var r180bFragments = []string{
	// Exact needles.
	"whenever",
	"creature dies",
	"a creature dies",
	"loses",
	"drain",
	"each opponent",
	"life",
	"whenever you gain life",
	"enters",
	"sacrifice a creature",
	"sacrifice another",
	"sacrifice an artifact",
	"sacrifice a permanent",
	"when",
	"dies",
	"return",
	"put",
	"graveyard",
	"battlefield",
	// Near misses and substring traps.
	"whenev",
	"creature die",
	"lose",
	"losese",
	"each opponents",
	"lifelink",
	"you gain life",
	"whenever you gain",
	"enter the battlefield",
	"sacrifice a land",
	"sacrifice",
	"another creature",
	"whenevr",
	"returns",
	"putrid",
	"graveyards",
	"battlefields",
	"at the beginning of your upkeep",
	"target player",
	"{t}:",
	"this creature gets +1/+1",
	"exile",
	"draw a card",
	"",
	// Composite phrases. Without these the three-needle clauses (ETB
	// drain, reanimation enabler) fire only a handful of times in
	// 2,400 cards and the equality is nearly vacuous for them.
	"whenever a creature enters",
	"enters, each opponent loses 1 life",
	"whenever this creature enters the battlefield",
	"return target creature card from a graveyard",
	"put a creature card from a graveyard onto the battlefield",
	"return it to the battlefield",
	"from your graveyard to the battlefield",
	"put target card from graveyard",
	"whenever a creature dies, each opponent loses 1 life",
	"whenever you gain life, each opponent loses 1 life",
}

// r180bOracleText assembles a pseudo-random oracle text. Mixed case on
// purpose: OracleTextLower lowercases, and a predicate that somehow
// escaped that path would show up here.
func r180bOracleText(rng *rand.Rand) string {
	n := 1 + rng.Intn(6)
	parts := make([]string, 0, n)
	for i := 0; i < n; i++ {
		f := r180bFragments[rng.Intn(len(r180bFragments))]
		switch rng.Intn(4) {
		case 0:
			f = strings.ToUpper(f)
		case 1:
			f = strings.Title(f) //nolint:staticcheck // ASCII test fixture only
		}
		parts = append(parts, f)
	}
	sep := []string{" ", ". ", ", ", " — "}[rng.Intn(4)]
	return strings.Join(parts, sep)
}

// r180bCard builds a Card whose OracleTextLower is derived from text.
func r180bCard(name, text string, types []string, cmc int) *gameengine.Card {
	return &gameengine.Card{
		Name:  name,
		Types: types,
		CMC:   cmc,
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{&gameast.Static{Raw: text}},
		},
	}
}

// r180bTypes picks a type line so the fodder branch sees every
// combination of token/nontoken and creature/noncreature.
func r180bTypes(rng *rand.Rand) ([]string, int) {
	switch rng.Intn(4) {
	case 0:
		return []string{"creature"}, 1 + rng.Intn(8)
	case 1:
		return []string{"creature", "token"}, 0
	case 2:
		return []string{"artifact", "token"}, 0
	default:
		return []string{"enchantment"}, 1 + rng.Intn(8)
	}
}

// --- Differential tests -------------------------------------------------

const r180bCorpusCards = 2400

// TestR180b_ScoreDrainEngine_DifferentialVsLegacy is the point of the
// round: new and legacy must return bit-identical float64 over a large
// generated corpus, including the sac-outlet negation.
func TestR180b_ScoreDrainEngine_DifferentialVsLegacy(t *testing.T) {
	rng := rand.New(rand.NewSource(0x180b))
	e := &GameStateEvaluator{}

	// Sanity counters: a corpus that never fires a clause proves nothing.
	var fired [5]int
	cards := 0

	// ~2,400 cards spread over battlefields of 1..12 permanents, so the
	// aggregate branches (sacOutlets > 0, fodder > 0, the multipliers)
	// are exercised at many different combinations.
	for board := 0; cards < r180bCorpusCards; board++ {
		gs := gameengine.NewGameState(4, rand.New(rand.NewSource(int64(board))), nil)
		// Vary opponent life so the avgOppLife < 20 multiplier is hit
		// on both sides of the branch.
		for i := range gs.Seats {
			gs.Seats[i].Life = 1 + rng.Intn(45)
		}
		n := 1 + rng.Intn(12)
		for j := 0; j < n; j++ {
			text := r180bOracleText(rng)
			types, cmc := r180bTypes(rng)
			c := r180bCard("r180b-drain", text, types, cmc)
			ot := strings.ToLower(text + " ")
			if isDrainDeathTriggerPayoff(ot) {
				fired[0]++
			}
			if isDrainLifegainPayoff(ot) {
				fired[1]++
			}
			if isDrainETBPayoff(ot) {
				fired[2]++
			}
			if isDrainSacOutlet(ot) {
				fired[3]++
			}
			if isDrainFodderDeathTrigger(ot) {
				fired[4]++
			}
			gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield,
				&gameengine.Permanent{Card: c, Controller: 0, Owner: 0})
			cards++
		}

		want := e.scoreDrainEngineLegacy(gs, 0)
		got := e.scoreDrainEngine(gs, 0)
		if got != want {
			t.Fatalf("board %d (%d permanents): scoreDrainEngine = %v, legacy = %v\nfirst card text: %q",
				board, n, got, want, gameengine.OracleTextLower(gs.Seats[0].Battlefield[0].Card))
		}
		// Run both a second time: the memo is now warm, and a stale or
		// mis-keyed bit would show up only on the second pass.
		if got2, want2 := e.scoreDrainEngine(gs, 0), e.scoreDrainEngineLegacy(gs, 0); got2 != want2 || got2 != got {
			t.Fatalf("board %d warm-cache pass: got %v want %v (cold was %v)", board, got2, want2, got)
		}
	}

	t.Logf("corpus: %d cards; clause hits death=%d lifegain=%d etb=%d sac=%d fodder=%d",
		cards, fired[0], fired[1], fired[2], fired[3], fired[4])
	names := []string{"death-trigger", "lifegain", "etb", "sac-outlet", "fodder-death"}
	for i, n := range fired {
		if n < 50 {
			t.Fatalf("clause %q fired only %d times over %d cards — too thin for "+
				"the equality to mean anything; widen the fragment pool", names[i], n, cards)
		}
		if n == cards {
			t.Fatalf("clause %q fired on EVERY one of %d cards — the corpus has no "+
				"negative case for it", names[i], cards)
		}
	}
}

// TestR180b_ScoreOpponentGraveyard_DifferentialVsLegacy covers the
// enabler clause moved to bit 21, with graveyard contents and
// commander names varied so the untouched parts of the function are
// included in the comparison rather than assumed unchanged.
func TestR180b_ScoreOpponentGraveyard_DifferentialVsLegacy(t *testing.T) {
	rng := rand.New(rand.NewSource(0x180bca))
	e := &GameStateEvaluator{}

	cmdrPool := []string{
		"Meren of Clan Nel Toth", "Muldrotha, the Gravetide", "Karador, Ghost Chieftain",
		"Chainer, Nightmare Adept", "Araumi of the Dead Tide", "Nethroi, Apex of Death",
		"Sefris of the Hidden Ways", "Atraxa, Praetors' Voice", "Krenko, Mob Boss",
	}
	recastKw := []string{"Flashback", "Escape", "Unearth", "Retrace", "Trample", "Flying"}

	enablerHits, enablerMisses, cards := 0, 0, 0

	for board := 0; cards < r180bCorpusCards; board++ {
		gs := gameengine.NewGameState(4, rand.New(rand.NewSource(int64(board)+7919)), nil)

		for seat := 1; seat < len(gs.Seats); seat++ {
			// Graveyard: creatures at a spread of CMCs plus recast keywords.
			for j := 0; j < rng.Intn(4); j++ {
				types := []string{"creature"}
				if rng.Intn(3) == 0 {
					types = []string{"instant"}
				}
				c := &gameengine.Card{
					Name:  "r180b-yard",
					Types: types,
					CMC:   rng.Intn(9),
					AST: &gameast.CardAST{Abilities: []gameast.Ability{
						&gameast.Keyword{Name: recastKw[rng.Intn(len(recastKw))]},
					}},
				}
				gs.Seats[seat].Graveyard = append(gs.Seats[seat].Graveyard, c)
			}

			// Battlefield: the clause under test.
			for j := 0; j < 1+rng.Intn(5); j++ {
				text := r180bOracleText(rng)
				types, cmc := r180bTypes(rng)
				if isOpponentReanimationEnabler(strings.ToLower(text + " ")) {
					enablerHits++
				} else {
					enablerMisses++
				}
				gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield,
					&gameengine.Permanent{
						Card:       r180bCard("r180b-enabler", text, types, cmc),
						Controller: seat,
						Owner:      seat,
					})
				cards++
			}

			if rng.Intn(2) == 0 {
				gs.Seats[seat].CommanderNames = []string{cmdrPool[rng.Intn(len(cmdrPool))]}
			}
			gs.Seats[seat].Lost = rng.Intn(8) == 0
		}

		want := e.scoreOpponentGraveyardLegacy(gs, 0)
		got := e.scoreOpponentGraveyard(gs, 0)
		if got != want {
			t.Fatalf("board %d: scoreOpponentGraveyard = %v, legacy = %v", board, got, want)
		}
		if got2, want2 := e.scoreOpponentGraveyard(gs, 0), e.scoreOpponentGraveyardLegacy(gs, 0); got2 != want2 || got2 != got {
			t.Fatalf("board %d warm-cache pass: got %v want %v (cold was %v)", board, got2, want2, got)
		}
	}

	t.Logf("corpus: %d opponent permanents; enabler clause true=%d false=%d",
		cards, enablerHits, enablerMisses)
	if enablerHits < 50 || enablerMisses < 50 {
		t.Fatalf("enabler clause barely varied (true=%d false=%d) — equality is vacuous",
			enablerHits, enablerMisses)
	}
}

// TestR180b_SacOutletNegationIsNotAccidental pins the one clause whose
// transcription error would be hardest to see: the trailing
// !strings.Contains(ot, "when"). Every drain needle paired with the
// word "when" must NOT count as a sac outlet.
func TestR180b_SacOutletNegationIsNotAccidental(t *testing.T) {
	outlets := []string{
		"sacrifice a creature", "sacrifice another", "sacrifice an artifact",
		"sacrifice a permanent",
	}
	for _, o := range outlets {
		if !isDrainSacOutlet(o) {
			t.Fatalf("%q should be a sac outlet", o)
		}
		for _, w := range []string{"when", "whenever", "when this creature dies"} {
			combined := o + " " + w
			if isDrainSacOutlet(combined) {
				t.Fatalf("%q contains %q and must NOT be a sac outlet", combined, w)
			}
		}
	}
	if isDrainSacOutlet("sacrifice a land") {
		t.Fatal(`"sacrifice a land" is not one of the four outlet needles`)
	}
}

// TestR180b_PredicatesAreASTKeyed — the memo keys on Card.AST, so
// reassigning the AST (what a DFC transform does) must recompute.
// r157 lost a whole round to a cache that keyed on a readiness bool
// instead; this asserts the new bits do not repeat it.
func TestR180b_PredicatesAreASTKeyed(t *testing.T) {
	type probe struct {
		name  string
		fn    func(*gameengine.Card) bool
		front string
		back  string
		wantF bool
		wantB bool
	}
	probes := []probe{
		{"death-trigger", cachedDrainDeathTriggerPayoff,
			"whenever a creature dies, each opponent loses 1 life", "flying", true, false},
		{"lifegain", cachedDrainLifegainPayoff,
			"whenever you gain life, each opponent loses 1 life", "flying", true, false},
		{"etb", cachedDrainETBPayoff,
			"whenever a creature enters, each opponent loses 1 life", "flying", true, false},
		{"sac-outlet", cachedDrainSacOutlet,
			"{t}: sacrifice a creature.", "when this dies, draw a card", true, false},
		{"fodder-death", cachedDrainFodderDeathTrigger,
			"when this creature dies, draw a card", "flying", true, false},
		{"reanimation-enabler", cachedOpponentReanimationEnabler,
			"return target creature card from a graveyard to the battlefield", "flying", true, false},
	}

	for _, p := range probes {
		c := r180bCard("r180b-dfc", p.front, []string{"creature"}, 3)
		if got := p.fn(c); got != p.wantF {
			t.Fatalf("%s front face: got %v want %v", p.name, got, p.wantF)
		}
		// Warm: same answer, served from the memo.
		if got := p.fn(c); got != p.wantF {
			t.Fatalf("%s front face (warm): got %v want %v", p.name, got, p.wantF)
		}
		// Transform: a DIFFERENT AST pointer with different text.
		c.AST = &gameast.CardAST{
			Abilities: []gameast.Ability{&gameast.Static{Raw: p.back}},
		}
		if got := p.fn(c); got != p.wantB {
			t.Fatalf("%s after AST swap: got %v want %v — the memo did not "+
				"recompute, so it is not AST-keyed", p.name, got, p.wantB)
		}
	}
}

// TestR180b_EvaluatorRoutesThroughTheMemo — a revert of the evaluator to
// the inline predicates must go red somewhere. Counting memo traffic is
// the only thing that can see it, since the differential test passes
// either way by construction.
func TestR180b_EvaluatorRoutesThroughTheMemo(t *testing.T) {
	gameengine.ResetTextPredicateStatsForTest()
	t.Cleanup(gameengine.ResetTextPredicateStatsForTest)

	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &gameengine.Permanent{
		Card: r180bCard("r180b Blood Artist",
			"whenever a creature dies, each opponent loses 1 life",
			[]string{"creature"}, 2),
		Controller: 0, Owner: 0,
	})
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, &gameengine.Permanent{
		Card: r180bCard("r180b Reanimator",
			"return target creature card from a graveyard to the battlefield",
			[]string{"enchantment"}, 4),
		Controller: 1, Owner: 1,
	})

	e := &GameStateEvaluator{}
	e.scoreDrainEngine(gs, 0)
	e.scoreOpponentGraveyard(gs, 0)

	h1, m1, _ := gameengine.TextPredicateStats()
	if h1+m1 == 0 {
		t.Fatal("neither scorer consulted the memo — the evaluator is not wired " +
			"through TextPredicate")
	}

	e.scoreDrainEngine(gs, 0)
	e.scoreOpponentGraveyard(gs, 0)
	h2, m2, _ := gameengine.TextPredicateStats()
	if h2 <= h1 {
		t.Fatalf("second pass produced no cache hit (hits %d -> %d)", h1, h2)
	}
	if m2 != m1 {
		t.Fatalf("second pass recomputed a predicate (misses %d -> %d)", m1, m2)
	}
}
