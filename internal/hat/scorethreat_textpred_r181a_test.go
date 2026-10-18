package hat

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// r181a — differential test for the two scoreThreat oracle-text clusters
// moved onto the per-Card text-predicate memo (bits 22-26, see
// internal/hat/text_predicates_r181a.go).
//
// The risk in this change is purely TRANSCRIPTION. The four
// dangerousPermanents tests are INDEPENDENT ifs with four distinct weights
// (0.3 / 0.25 / 0.2 / 0.3) whose needles overlap heavily — "damage"
// appears in two, "loses" in two, "each opponent" in one but "opponent" is
// a substring of it — so a hand-written card set cannot tell "I moved the
// expressions correctly" apart from "I dropped a conjunct" or "I turned
// four ifs into a switch". The mill disjunction sits behind a
// myLibSize < 15 gate and three library-size tiers (0.8 / 0.4 / 0.2) that
// a corpus with one library size would never reach at all.
//
// So the legacy body is kept VERBATIM below and the two implementations
// are compared with EXACT float64 equality over a generated corpus of
// synthetic boards, with an asserted coverage histogram so the
// differential cannot pass vacuously.

// -----------------------------------------------------------------------------
// Verbatim legacy body. This is scoreThreat as of c83e0517, copied
// unchanged except for the function name. Do not "clean it up" — it exists
// to be diffed by eye against the original.
// -----------------------------------------------------------------------------

func (e *GameStateEvaluator) scoreThreatLegacy(gs *gameengine.GameState, seatIdx int) float64 {
	seat := gs.Seats[seatIdx]
	if seat.Life <= 0 {
		return -1
	}

	var maxOppPow float64
	hardestToAnswer := 0.0
	dangerousPermanents := 0.0
	for i, s := range gs.Seats {
		if i == seatIdx || s.Lost || s.LeftGame {
			continue
		}
		bp := effectiveOffensivePower(gs, s)
		if bp > maxOppPow {
			maxOppPow = bp
		}
		if h := hardToAnswerScore(gs, s); h > hardestToAnswer {
			hardestToAnswer = h
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			ot := gameengine.OracleTextLower(p.Card)
			if strings.Contains(ot, "whenever a creature enters") && strings.Contains(ot, "damage") {
				dangerousPermanents += 0.3
			}
			if strings.Contains(ot, "whenever a creature dies") && strings.Contains(ot, "loses") {
				dangerousPermanents += 0.25
			}
			if strings.Contains(ot, "each opponent") && strings.Contains(ot, "loses") {
				dangerousPermanents += 0.2
			}
			if strings.Contains(ot, "double") && strings.Contains(ot, "damage") {
				dangerousPermanents += 0.3
			}
		}
	}

	// Vulnerability-aware threat: if an opponent controls a card we're
	// vulnerable to (from Freya threat assessment), increase threat.
	hoserPenalty := 0.0
	if e.Strategy != nil && len(e.Strategy.VulnerableTo) > 0 {
		for i, s := range gs.Seats {
			if i == seatIdx || s.Lost || s.LeftGame {
				continue
			}
			for _, p := range s.Battlefield {
				if p == nil || p.Card == nil {
					continue
				}
				name := strings.ToLower(p.Card.DisplayName())
				for _, hoser := range e.Strategy.VulnerableTo {
					if strings.EqualFold(name, hoser) {
						hoserPenalty += 0.4
					}
				}
			}
		}
		if hoserPenalty > 1.0 {
			hoserPenalty = 1.0
		}
	}

	lethalRatio := maxOppPow / float64(seat.Life)
	if lethalRatio >= 1.0 {
		return -1.0
	}

	// Hard-to-answer multiplier: threats with indestructible / hexproof /
	// ward / protection compound the lethal clock because they survive
	// generic removal — we burn into specific answers (exile, board wipe,
	// -X/-X) and may not have one. Adds up to +20% to the offensive
	// pressure score, capped so a single huge protected creature never
	// flips the dimension to lethal on its own.
	hardToAnswerPenalty := hardestToAnswer * lethalRatio * 0.2

	// Poison threat: high poison counters are an existential threat
	// regardless of combat board state. 10 = lethal per §704.5c.
	poisonPenalty := 0.0
	if seat.PoisonCounters >= 9 {
		poisonPenalty = 0.8 // one more counter = death
	} else if seat.PoisonCounters >= 7 {
		poisonPenalty = 0.5
	} else if seat.PoisonCounters >= 5 {
		poisonPenalty = 0.2
	}
	// Check if any opponent controls infect/toxic creatures — amplifies
	// the urgency when we already have poison counters.
	if seat.PoisonCounters > 0 {
		for i, s := range gs.Seats {
			if i == seatIdx || s.Lost || s.LeftGame {
				continue
			}
			for _, p := range s.Battlefield {
				if p == nil {
					continue
				}
				if p.HasKeyword("infect") || p.HasKeyword("toxic") {
					poisonPenalty += 0.15
					break // one infect source per opponent is enough
				}
			}
		}
		if poisonPenalty > 1.0 {
			poisonPenalty = 1.0
		}
	}

	// Mill threat: low library is dangerous if opponents have mill effects.
	millPenalty := 0.0
	myLibSize := len(seat.Library)
	if myLibSize < 15 {
		for i, s := range gs.Seats {
			if i == seatIdx || s.Lost || s.LeftGame {
				continue
			}
			for _, p := range s.Battlefield {
				if p == nil || p.Card == nil {
					continue
				}
				ot := gameengine.OracleTextLower(p.Card)
				if (strings.Contains(ot, "mill") || strings.Contains(ot, "cards from the top of") ||
					(strings.Contains(ot, "library") && strings.Contains(ot, "graveyard"))) &&
					(strings.Contains(ot, "opponent") || strings.Contains(ot, "target player") || strings.Contains(ot, "each player")) {
					if myLibSize < 5 {
						millPenalty = 0.8
					} else if myLibSize < 10 {
						millPenalty = 0.4
					} else {
						millPenalty = 0.2
					}
					break
				}
			}
			if millPenalty > 0 {
				break
			}
		}
	}

	// Commander damage proximity: §704.6c lethal at 21.
	cmdrPenalty := 0.0
	if gs.CommanderFormat && seat.CommanderDamage != nil {
		for _, cmdMap := range seat.CommanderDamage {
			for _, dmg := range cmdMap {
				if dmg >= 18 {
					cmdrPenalty = 0.6
				} else if dmg >= 15 && cmdrPenalty < 0.3 {
					cmdrPenalty = 0.3
				}
			}
		}
	}

	// R60: stack-pending pressure. The dimension previously never read
	// gs.Stack — a Murder targeting our Atraxa, a Counterspell aimed at
	// our wincon cast, or a Wrath of God on the stack all contributed
	// 0.0 to threat. That's the most concretely-missing signal:
	// information that's about to RESOLVE against us next priority pass.
	// Walk every stack item NOT controlled by this seat, score by who/
	// what it targets:
	//
	//   - direct seat target (Lightning Bolt at me, Mind Twist at me):
	//     0.30 per item — the effect lands on me regardless of board.
	//   - permanent target where the permanent is controlled by us:
	//     0.20 per item — generic removal; we might still respond.
	//   - stack-item target where the target stack item is one of OUR
	//     own spells (counterspell aimed at our cast): 0.35 per item —
	//     losing a spell mid-cast is more painful than losing a
	//     permanent because we already paid the mana.
	//
	// Cap total stack pressure at 0.80 so a flood of low-value triggers
	// can't flip the dimension to lethal on its own. Triggered abilities
	// (Kind=="triggered") still count — a Blood Artist drain triggered
	// against us is real threat — but copy items (CR §707.10) are
	// skipped because the original already counted.
	stackPressure := 0.0
	if len(gs.Stack) > 0 {
		for _, item := range gs.Stack {
			if item == nil || item.IsCopy {
				continue
			}
			if item.Controller == seatIdx {
				continue
			}
			for _, t := range item.Targets {
				switch t.Kind {
				case gameengine.TargetKindSeat:
					if t.Seat == seatIdx {
						stackPressure += 0.30
					}
				case gameengine.TargetKindPermanent:
					if t.Permanent != nil && t.Permanent.Controller == seatIdx {
						stackPressure += 0.20
					}
				case gameengine.TargetKindStackItem:
					if t.Stack != nil && t.Stack.Controller == seatIdx {
						stackPressure += 0.35
					}
				}
			}
		}
		if stackPressure > 0.80 {
			stackPressure = 0.80
		}
	}

	// R60: single-removal concentration penalty. wipeMagnet (below) is
	// the OVER-committed side ("a sweeper takes my whole army") — its
	// mirror is the UNDER-committed side: a single keystone creature
	// carrying most of our offensive power is one Murder away from
	// collapsing the position. Walk own creatures, compute max-power
	// share of total power; if the top creature owns ≥50% of our
	// offense AND total offense is non-trivial (>4 to gate out the 1/1
	// token early game), credit the share above 0.5 at 1.0 per unit,
	// capped at 0.50 so a one-creature board (share=1.0) reads exactly
	// the cap. Distinct from hardestToAnswer (THEIR scary creature
	// surviving OUR removal) — this is OUR keystone exposure to THEIR
	// spot removal.
	removalConcentration := 0.0
	{
		var totalPow, maxPow float64
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil || p.PhasedOut || !p.IsCreature() {
				continue
			}
			pow := float64(p.Power())
			if pow <= 0 {
				continue
			}
			totalPow += pow
			if pow > maxPow {
				maxPow = pow
			}
		}
		if totalPow > 4 && maxPow > 0 {
			share := maxPow / totalPow
			if share >= 0.5 {
				removalConcentration = share - 0.5
				if removalConcentration > 0.50 {
					removalConcentration = 0.50
				}
			}
		}
	}

	// R60: untap-step window — defensive-shell exposure. When the
	// active player isn't us, our tapped creatures can't block. If our
	// best-toughness creature is tapped AND noticeably bigger than the
	// best untapped one, the defensive shell is compromised for this
	// priority window. Gated on a non-trivial opponent offensive power
	// (>=4) so the signal doesn't fire over a single 2/2 we can ignore.
	// Reads gs.Active (the seat whose turn it is) — pre-r60 the
	// dimension was completely turn-agnostic, so a tapped Avenger of
	// Zendikar in opponent's combat step scored identically to the
	// same creature untapped on our own turn.
	untapWindow := 0.0
	if gs.Active != seatIdx && maxOppPow >= 4 {
		var bestUntappedTough, bestTappedTough int
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil || p.PhasedOut || !p.IsCreature() {
				continue
			}
			t := p.Toughness()
			if t <= 0 {
				continue
			}
			if p.Tapped {
				if t > bestTappedTough {
					bestTappedTough = t
				}
			} else {
				if t > bestUntappedTough {
					bestUntappedTough = t
				}
			}
		}
		if bestTappedTough > bestUntappedTough+2 {
			diff := bestTappedTough - bestUntappedTough
			untapWindow = float64(diff) * 0.05
			if untapWindow > 0.30 {
				untapWindow = 0.30
			}
		}
	}

	// R60: interaction-mana starvation during opponent priority. With
	// no untapped lands during an opponent's turn, every spell they
	// cast resolves unopposed — we can't pay for a Counterspell, a
	// Swords to Plowshares, a Heroic Intervention, etc. Hand contents
	// are unknown post-determinization, so we score on the WINDOW
	// (untapped-mana availability) regardless of what we hold. Gated
	// on opponent threat (>=4 max opp pow) so a peaceful early opp
	// turn doesn't trigger.
	interactionStarvation := 0.0
	if gs.Active != seatIdx && maxOppPow >= 4 {
		untappedLands := 0
		for _, p := range seat.Battlefield {
			if p == nil || p.PhasedOut || !p.IsLand() {
				continue
			}
			if !p.Tapped {
				untappedLands++
			}
		}
		switch untappedLands {
		case 0:
			interactionStarvation = 0.20
		case 1:
			interactionStarvation = 0.08
		}
	}

	// R60: wipe-magnet penalty. lethalRatio (maxOpp/life) is naturally
	// LOW when we're leading on board — opponents have small boards
	// because we're dominating. But that exact state is when the table
	// most wants to wipe us. Compare our effective offensive power to
	// the strongest opponent's; if we're commanding the board by a
	// wide margin, surface that as a removal-attractiveness penalty.
	// Gated on a non-trivial own-board floor (myBoard > 6) so the
	// early-game tie-state doesn't trigger.
	wipeMagnet := 0.0
	myBoard := effectiveOffensivePower(gs, seat)
	if myBoard > 6 && maxOppPow > 0 {
		ratio := myBoard / maxOppPow
		switch {
		case ratio >= 2.0:
			wipeMagnet = 0.30
		case ratio >= 1.5:
			wipeMagnet = 0.15
		}
	}

	return -lethalRatio*0.8 - dangerousPermanents*0.3 - hoserPenalty - poisonPenalty - millPenalty - cmdrPenalty - hardToAnswerPenalty - stackPressure - wipeMagnet - removalConcentration - untapWindow - interactionStarvation
}

// -----------------------------------------------------------------------------
// Corpus
// -----------------------------------------------------------------------------

// r181aFragments are every needle the two clusters test for, plus the
// substring traps that make a dropped conjunct observable: singular /
// plural pairs where one contains the other ("lose"/"loses",
// "damage"/"damages", "double"/"doubled", "enter"/"enters"), the
// "opponent" / "each opponent" containment pair, and mixed case (which
// OracleTextLower folds, so both implementations must agree on it).
var r181aFragments = []string{
	// dangerousPermanents test 1 (0.3)
	"whenever a creature enters", "damage", "whenever a creature enters the battlefield",
	"whenever a creature enter", "enters", "enter",
	// dangerousPermanents test 2 (0.25)
	"whenever a creature dies", "loses", "whenever a creature die", "lose",
	// dangerousPermanents test 3 (0.2)
	"each opponent", "opponent", "opponents", "each opponent loses 1 life",
	// dangerousPermanents test 4 (0.3)
	"double", "doubled", "damages", "if a source would deal damage, it deals double that damage instead",
	// mill disjunction
	"mill", "cards from the top of", "library", "graveyard", "target player",
	"each player", "mills three cards", "cards from the top of their library",
	// mixed case — must fold identically in both implementations
	"Whenever A Creature ENTERS", "DAMAGE", "Each Opponent LOSES", "DoUbLe", "MILL",
	// filler that must never flip anything
	"draw a card", "flying", "", "target creature gets +1/+1", "untap",
	"the top of", "milled", "libraries", "graveyards", "opposing", "doubling season",
}

// r181aTestSatisfiers are minimal texts that each satisfy exactly one of
// the four dangerousPermanents tests, plus one that satisfies the mill
// disjunction. Combining subsets of these is what proves the four tests
// are INDEPENDENT ifs and not a switch: a text satisfying all four must
// contribute 0.3+0.25+0.2+0.3, not whichever one matched first.
var r181aTestSatisfiers = []string{
	"whenever a creature enters, it deals damage to any target",
	"whenever a creature dies, each player loses nothing in particular",
	"each opponent loses 2 life",
	"if a source would deal damage, double that damage",
	"each opponent mills two cards",
}

// r181aTexts builds n distinct oracle texts under a fixed seed.
func r181aTexts(n int) []string {
	rng := rand.New(rand.NewSource(0x181a))
	out := make([]string, 0, n)

	// Every non-empty subset of the five satisfiers, forward and
	// reversed, so ZERO / ONE / MULTIPLE simultaneous matches are all
	// present and each combination is seen with the text itself ordered
	// two different ways. 31 subsets x 2 = 62 probes.
	for mask := 1; mask < 1<<len(r181aTestSatisfiers); mask++ {
		var parts []string
		for i, s := range r181aTestSatisfiers {
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

	// Every single fragment, then every ORDERED PAIR of fragments, so
	// each conjunct appears both alone (must not fire) and beside its
	// partner (must fire) before any random filling.
	out = append(out, r181aFragments...)
	for i := range r181aFragments {
		for j := range r181aFragments {
			if len(out) >= n {
				break
			}
			out = append(out, r181aFragments[i]+" "+r181aFragments[j])
		}
	}
	for len(out) < n {
		k := 1 + rng.Intn(5)
		parts := make([]string, 0, k)
		for i := 0; i < k; i++ {
			parts = append(parts, r181aFragments[rng.Intn(len(r181aFragments))])
		}
		out = append(out, strings.Join(parts, " "))
	}
	return out[:n]
}

// r181aCard builds a card whose lowered oracle text is `text`, through the
// real AST path so OracleTextLower behaves exactly as in production. The
// type is enchantment, not creature, so these probes contribute no
// offensive power — a lethalRatio >= 1.0 would early-return -1.0 and the
// mill block would never be reached.
func r181aCard(name, text string) *gameengine.Card {
	return &gameengine.Card{
		Name:  name,
		Types: []string{"enchantment"},
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{&gameast.Static{Raw: text}},
		},
	}
}

const r181aCorpusSize = 2400

// r181aBoard is one synthetic board: a 4-seat game where seat 0 is us,
// with probe permanents spread across the three opponents and a library of
// the chosen size.
type r181aBoard struct {
	texts   []string
	libSize int
}

// r181aCorpus builds the boards. libSize cycles through all three mill
// tiers AND above the < 15 cutoff, so every tier and the gated-off case
// are exercised.
func r181aCorpus(n int) []r181aBoard {
	texts := r181aTexts(n)
	rng := rand.New(rand.NewSource(0x181ab))
	// Deliberately spans each tier: < 5, < 10, < 15, and >= 15.
	libSizes := []int{0, 2, 4, 5, 7, 9, 10, 12, 14, 15, 20, 40}
	out := make([]r181aBoard, 0, n)
	for i := 0; i < n; i++ {
		k := 1 + rng.Intn(3)
		ts := make([]string, 0, k)
		for j := 0; j < k; j++ {
			ts = append(ts, texts[(i+j*7)%len(texts)])
		}
		out = append(out, r181aBoard{texts: ts, libSize: libSizes[i%len(libSizes)]})
	}
	return out
}

// build materialises a board into a GameState. Probes are spread across
// seats 1-3 so the mill block's per-seat break is exercised.
func (b r181aBoard) build() *gameengine.GameState {
	gs := gameengine.NewGameState(4, rand.New(rand.NewSource(1)), nil)
	for i := range gs.Seats {
		gs.Seats[i].Life = 40
	}
	lib := make([]*gameengine.Card, 0, b.libSize)
	for i := 0; i < b.libSize; i++ {
		lib = append(lib, r181aCard("R181a Library Filler", "draw a card"))
	}
	gs.Seats[0].Library = lib
	for i, text := range b.texts {
		seat := 1 + i%3
		gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield,
			&gameengine.Permanent{Card: r181aCard("R181a Threat Probe", text), Controller: seat})
	}
	return gs
}

// -----------------------------------------------------------------------------
// Coverage instrumentation
//
// These replicate the legacy expressions INDEPENDENTLY of the production
// predicates, so that mutating a production predicate turns the
// differential red without also silently shrinking the coverage counts.
// -----------------------------------------------------------------------------

func r181aLegacyTest1(ot string) bool {
	return strings.Contains(ot, "whenever a creature enters") && strings.Contains(ot, "damage")
}
func r181aLegacyTest2(ot string) bool {
	return strings.Contains(ot, "whenever a creature dies") && strings.Contains(ot, "loses")
}
func r181aLegacyTest3(ot string) bool {
	return strings.Contains(ot, "each opponent") && strings.Contains(ot, "loses")
}
func r181aLegacyTest4(ot string) bool {
	return strings.Contains(ot, "double") && strings.Contains(ot, "damage")
}
func r181aLegacyMill(ot string) bool {
	return (strings.Contains(ot, "mill") || strings.Contains(ot, "cards from the top of") ||
		(strings.Contains(ot, "library") && strings.Contains(ot, "graveyard"))) &&
		(strings.Contains(ot, "opponent") || strings.Contains(ot, "target player") || strings.Contains(ot, "each player"))
}

// -----------------------------------------------------------------------------
// The differential
// -----------------------------------------------------------------------------

func TestR181a_ScoreThreatDifferential(t *testing.T) {
	corpus := r181aCorpus(r181aCorpusSize)
	if len(corpus) < 2000 {
		t.Fatalf("corpus too small: %d", len(corpus))
	}
	e := &GameStateEvaluator{}

	mismatches := 0
	for i, b := range corpus {
		gs := b.build()

		// Cold: nothing memoized yet for these Cards.
		cold := e.scoreThreat(gs, 0)
		// Warm: every bit now cached. A mis-keyed bit shows up here.
		warm := e.scoreThreat(gs, 0)

		// Legacy on a FRESH board, so the memo cannot influence it.
		want := b.build()
		legacy := e.scoreThreatLegacy(want, 0)

		if cold != legacy || warm != legacy {
			mismatches++
			if mismatches <= 10 {
				t.Errorf("board %d (lib=%d): cold=%v warm=%v legacy=%v texts=%q",
					i, b.libSize, cold, warm, legacy, b.texts)
			}
		}
	}
	if mismatches > 0 {
		t.Fatalf("%d/%d corpus boards disagreed", mismatches, len(corpus))
	}
	t.Logf("scoreThreat differential: %d boards, exact float64 agreement cold and warm", len(corpus))
}

// A differential whose corpus never exercises a branch passes vacuously.
// Every one of the four dangerousPermanents tests must fire at least 50
// times and every mill tier at least 25 times, or this corpus is not fit
// for purpose.
func TestR181a_CorpusCoverageHistogram(t *testing.T) {
	corpus := r181aCorpus(r181aCorpusSize)

	var fired [4]int
	millTier := map[float64]int{}
	multi := map[int]int{} // how many of the four fired on one card
	aboveCutoff := 0

	for _, b := range corpus {
		gs := b.build()
		millHit := false
		for seatIdx, s := range gs.Seats {
			if seatIdx == 0 {
				continue
			}
			for _, p := range s.Battlefield {
				ot := gameengine.OracleTextLower(p.Card)
				n := 0
				if r181aLegacyTest1(ot) {
					fired[0]++
					n++
				}
				if r181aLegacyTest2(ot) {
					fired[1]++
					n++
				}
				if r181aLegacyTest3(ot) {
					fired[2]++
					n++
				}
				if r181aLegacyTest4(ot) {
					fired[3]++
					n++
				}
				multi[n]++
				if r181aLegacyMill(ot) {
					millHit = true
				}
			}
		}
		if b.libSize >= 15 {
			aboveCutoff++
			continue
		}
		if !millHit {
			continue
		}
		switch {
		case b.libSize < 5:
			millTier[0.8]++
		case b.libSize < 10:
			millTier[0.4]++
		default:
			millTier[0.2]++
		}
	}

	for i, n := range fired {
		if n < 50 {
			t.Errorf("dangerousPermanents test %d fired only %d times, want >= 50 — the differential is near-vacuous for it", i+1, n)
		}
	}
	for _, tier := range []float64{0.8, 0.4, 0.2} {
		if millTier[tier] < 25 {
			t.Errorf("mill tier %v reached only %d times, want >= 25", tier, millTier[tier])
		}
	}
	if aboveCutoff < 25 {
		t.Errorf("only %d boards had myLibSize >= 15; the gated-off path is untested", aboveCutoff)
	}
	if multi[0] == 0 {
		t.Errorf("no card satisfied ZERO of the four tests")
	}
	if multi[1] == 0 {
		t.Errorf("no card satisfied exactly ONE of the four tests")
	}
	if multi[2]+multi[3]+multi[4] == 0 {
		t.Errorf("no card satisfied MULTIPLE of the four tests — the independent-ifs property is untested")
	}

	t.Logf("dangerousPermanents firings: t1=%d t2=%d t3=%d t4=%d", fired[0], fired[1], fired[2], fired[3])
	t.Logf("simultaneous-match histogram (tests satisfied per card): %v", multi)
	t.Logf("mill tiers: 0.8=%d 0.4=%d 0.2=%d; boards above the <15 cutoff: %d",
		millTier[0.8], millTier[0.4], millTier[0.2], aboveCutoff)
}

// The four tests are independent ifs, not a switch: a card satisfying all
// four must contribute every weight. Pinned explicitly because a
// switch-shaped regression would still pass most of the corpus.
func TestR181a_FourTestsAreIndependent(t *testing.T) {
	e := &GameStateEvaluator{}
	all := "whenever a creature enters, it deals damage. whenever a creature dies, " +
		"each opponent loses 2 life. double that damage."
	gs := gameengine.NewGameState(4, rand.New(rand.NewSource(1)), nil)
	for i := range gs.Seats {
		gs.Seats[i].Life = 40
	}
	gs.Seats[1].Battlefield = []*gameengine.Permanent{
		{Card: r181aCard("R181a All Four", all), Controller: 1},
	}
	// dangerousPermanents = 0.3+0.25+0.2+0.3 = 1.05, contributed at *0.3.
	got := e.scoreThreat(gs, 0)
	want := -1.05 * 0.3
	if got != want {
		t.Fatalf("all-four card: scoreThreat=%v want %v (a switch would give a single weight)", got, want)
	}
	if legacy := e.scoreThreatLegacy(gs, 0); legacy != want {
		t.Fatalf("legacy disagrees with the hand-computed value: %v want %v", legacy, want)
	}
}

// Reassigning Card.AST (what a DFC transform does) must invalidate every
// r181a bit. r157 shipped a "have we computed this" bool and spent a round
// serving the front face of a transformed card for the rest of the game.
func TestR181a_PredicatesAreASTKeyed(t *testing.T) {
	card := r181aCard("R181a Transform Probe",
		"whenever a creature enters, it deals damage to any target")

	if !cachedThreatCreatureEntersDamage(card) {
		t.Fatalf("front face: cachedThreatCreatureEntersDamage want true")
	}
	for name, got := range map[string]bool{
		"cachedThreatCreatureDiesLoses": cachedThreatCreatureDiesLoses(card),
		"cachedThreatEachOpponentLoses": cachedThreatEachOpponentLoses(card),
		"cachedThreatDoubleDamage":      cachedThreatDoubleDamage(card),
		"cachedThreatMillSource":        cachedThreatMillSource(card),
	} {
		if got {
			t.Fatalf("front face: %s want false", name)
		}
	}

	// Transform: a DIFFERENT AST with different text, assigned in place.
	card.AST = &gameast.CardAST{
		Abilities: []gameast.Ability{&gameast.Static{
			Raw: "whenever a creature dies, each opponent loses 1 life and mills a card. " +
				"double that damage.",
		}},
	}

	if cachedThreatCreatureEntersDamage(card) {
		t.Fatalf("back face: cachedThreatCreatureEntersDamage still true — the memo is not AST-keyed")
	}
	for name, got := range map[string]bool{
		"cachedThreatCreatureDiesLoses": cachedThreatCreatureDiesLoses(card),
		"cachedThreatEachOpponentLoses": cachedThreatEachOpponentLoses(card),
		"cachedThreatDoubleDamage":      cachedThreatDoubleDamage(card),
		"cachedThreatMillSource":        cachedThreatMillSource(card),
	} {
		if !got {
			t.Fatalf("back face: %s want true — the memo is not AST-keyed", name)
		}
	}
}

// The bits this round claims must be the ones it documents, and must not
// collide with r164/r172/r180a/r180b.
func TestR181a_BitAssignmentsAreDisjoint(t *testing.T) {
	mine := map[uint]string{
		22: "tpThreatCreatureEntersDamage",
		23: "tpThreatCreatureDiesLoses",
		24: "tpThreatEachOpponentLoses",
		25: "tpThreatDoubleDamage",
		26: "tpThreatMillSource",
	}
	actual := map[uint]string{
		tpThreatCreatureEntersDamage: "tpThreatCreatureEntersDamage",
		tpThreatCreatureDiesLoses:    "tpThreatCreatureDiesLoses",
		tpThreatEachOpponentLoses:    "tpThreatEachOpponentLoses",
		tpThreatDoubleDamage:         "tpThreatDoubleDamage",
		tpThreatMillSource:           "tpThreatMillSource",
	}
	if len(actual) != len(mine) {
		t.Fatalf("r181a bits collide with each other: %v", actual)
	}
	for bit, name := range mine {
		if actual[bit] != name {
			t.Fatalf("bit %d is %q, want %q", bit, actual[bit], name)
		}
	}
	taken := []uint{
		tpImpulseDrawEngine, tpPersistentScryEngine, tpPersistentDrawEngine,
		tpGraveyardCastKeyword, tpManaOnlyAbility, tpOpponentGraveyardRecastKeyword,
		tpStaxNonlandDontUntap, tpStaxCantCast, tpStaxAdditionalCostPay,
		tpStaxEntersTappedOpp, tpStaxWheneverOppLoses, tpStaxSacEachOpponent,
		tpHasTriggerWording,
		tpDrainDeathTriggerPayoff, tpDrainLifegainPayoff, tpDrainETBPayoff,
		tpDrainSacOutlet, tpDrainFodderDeathTrigger, tpOpponentReanimationEnabler,
	}
	for _, b := range taken {
		if _, clash := mine[b]; clash {
			t.Fatalf("bit %d is already owned by an earlier round", b)
		}
	}
}
