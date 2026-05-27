package hat

import (
	"math"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// evaluator_ritual_fuel_r60_test.go — pins the ritual-fuel bonus
// added to scoreMana. The bonus rewards seats holding rituals
// (Dark Ritual, Cabal Ritual, Pyretic Ritual, Rite of Flame,
// Seething Song, Infernal Plunge) ONLY when there's a castable
// chain target whose CMC exceeds the seat's current untapped-source
// ceiling but fits under (ceiling + ritual net mana). A ritual with
// no chain target is dead this turn and earns 0 bonus.
//
// Sibling to evaluator_color_screw_r60_test.go (PR #564); both
// signals refine the in-hand tactical playability surface of
// scoreMana that the deck-level ColorDemand path doesn't see.

// makeRitualEvalGame builds a fresh 2-seat game with no Strategy
// ColorDemand so the ritual signal is isolated from the deck-level
// color-coverage path.
func makeRitualEvalGame(t *testing.T) (*GameStateEvaluator, *gameengine.GameState) {
	t.Helper()
	gs := newTestGame(t, 2)
	for _, s := range gs.Seats {
		s.Life = 40
		s.StartingLife = 40
	}
	gs.Active = 0
	ev := NewEvaluator(&StrategyProfile{})
	return ev, gs
}

// makeSpellInHand builds a non-land hand card with the named CMC.
// The "cost:N" type token is what gameengine.ManaCostOf reads, so we
// don't need a full ManaCostString here (rituals do need one for the
// canonical-name lookup path, but for chain-target detection only
// CMC matters).
func makeSpellInHand(name string, cmc int) *gameengine.Card {
	return newTestCardMinimal(name, []string{"sorcery"}, cmc, nil)
}

// makeRitualInHand uses the canonical lowercase name lookup so
// ritualNetMana resolves to the correct net value. The CMC arg sets
// the type-token "cost:N" so ManaCostOf returns the cost-side of the
// ritual (used internally to verify we filter rituals out of the
// chain-target scan).
func makeRitualInHand(name string, cmc int) *gameengine.Card {
	return newTestCardMinimal(name, []string{"instant"}, cmc, nil)
}

// -----------------------------------------------------------------------------
// ritualNetMana — name lookup
// -----------------------------------------------------------------------------

func TestRitualNetMana_CanonicalRituals(t *testing.T) {
	cases := []struct {
		name string
		want int
	}{
		{"Dark Ritual", 2},
		{"DARK RITUAL", 2}, // case-insensitive
		{"dark ritual", 2},
		{"Cabal Ritual", 1},
		{"Pyretic Ritual", 1},
		{"Rite of Flame", 1},
		{"Seething Song", 2},
		{"Infernal Plunge", 2},
		// Excluded rituals
		{"Desperate Ritual", 0}, // net 0 standalone, value lives in splice
		{"Manamorphose", 0},     // mana-neutral cantrip
		// Non-rituals
		{"Lightning Bolt", 0},
		{"Sol Ring", 0},
		{"", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			card := newTestCardMinimal(c.name, []string{"instant"}, 1, nil)
			got := ritualNetMana(card)
			if got != c.want {
				t.Errorf("ritualNetMana(%q) = %d, want %d", c.name, got, c.want)
			}
		})
	}
	if got := ritualNetMana(nil); got != 0 {
		t.Errorf("ritualNetMana(nil) = %d, want 0", got)
	}
}

// -----------------------------------------------------------------------------
// ritualFuelBonus — happy + edge cases
// -----------------------------------------------------------------------------

func TestRitualFuelBonus_EmptyHandZero(t *testing.T) {
	_, gs := makeRitualEvalGame(t)
	if got := ritualFuelBonus(gs.Seats[0]); got != 0 {
		t.Errorf("empty hand: got %v, want 0", got)
	}
}

func TestRitualFuelBonus_NoRitualsZero(t *testing.T) {
	_, gs := makeRitualEvalGame(t)
	seat := gs.Seats[0]
	seat.Hand = append(seat.Hand,
		makeSpellInHand("Lightning Bolt", 1),
		makeSpellInHand("Counterspell", 2),
	)
	if got := ritualFuelBonus(seat); got != 0 {
		t.Errorf("no rituals in hand: got %v, want 0", got)
	}
}

func TestRitualFuelBonus_DarkRitualWithChainTarget(t *testing.T) {
	// Seat has 1 untapped Island (playable cap = 1), Dark Ritual (+2),
	// and a 3-CMC spell in hand. Without the ritual we can't cast the
	// 3-CMC spell; with it we can. Bonus = 0.10 × 2 = 0.20.
	_, gs := makeRitualEvalGame(t)
	seat := gs.Seats[0]
	makeColoredLand(seat, "U", false) // 1 untapped source
	seat.Hand = append(seat.Hand,
		makeRitualInHand("Dark Ritual", 1),
		makeSpellInHand("Necropotence", 3),
	)
	got := ritualFuelBonus(seat)
	want := 0.20
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("Dark Ritual + chain target: got %v, want %v", got, want)
	}
}

func TestRitualFuelBonus_NoChainTargetDeadRitual(t *testing.T) {
	// Seat has 5 untapped sources (playable cap = 5), Dark Ritual (+2),
	// but the only other spell in hand is a 1-CMC bolt we can already
	// cast. The ritual is dead — the chain target test fails because
	// 1 ≤ playableCap. Bonus = 0.
	_, gs := makeRitualEvalGame(t)
	seat := gs.Seats[0]
	for i := 0; i < 5; i++ {
		makeColoredLand(seat, "B", false)
	}
	seat.Hand = append(seat.Hand,
		makeRitualInHand("Dark Ritual", 1),
		makeSpellInHand("Lightning Bolt", 1), // already castable, no fuel needed
	)
	if got := ritualFuelBonus(seat); got != 0 {
		t.Errorf("dead ritual (no spell needs the burst): got %v, want 0", got)
	}
}

func TestRitualFuelBonus_TargetOverCeilingDeadRitual(t *testing.T) {
	// Seat has 2 untapped sources, Dark Ritual (+2), and a 10-CMC
	// spell. Ceiling = 2 + 2 = 4. 10 > 4 so the spell is out of reach
	// even with the ritual. Bonus = 0 (ritual still dead because the
	// "target" isn't reachable).
	_, gs := makeRitualEvalGame(t)
	seat := gs.Seats[0]
	for i := 0; i < 2; i++ {
		makeColoredLand(seat, "B", false)
	}
	seat.Hand = append(seat.Hand,
		makeRitualInHand("Dark Ritual", 1),
		makeSpellInHand("Emrakul, the Aeons Torn", 10),
	)
	if got := ritualFuelBonus(seat); got != 0 {
		t.Errorf("target above ceiling: got %v, want 0", got)
	}
}

func TestRitualFuelBonus_TwoRitualsStack(t *testing.T) {
	// Seat has 1 untapped source (cap = 1), two Dark Rituals (+2 each
	// = +4 total), and a 5-CMC spell. Ceiling = 1 + 4 = 5, target = 5.
	// Bonus = 0.10 × 4 = 0.40 (at cap).
	_, gs := makeRitualEvalGame(t)
	seat := gs.Seats[0]
	makeColoredLand(seat, "B", false)
	seat.Hand = append(seat.Hand,
		makeRitualInHand("Dark Ritual", 1),
		makeRitualInHand("Dark Ritual", 1),
		makeSpellInHand("Wheel of Fortune", 5),
	)
	got := ritualFuelBonus(seat)
	want := 0.40
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("2 Dark Rituals stacking: got %v, want %v", got, want)
	}
}

func TestRitualFuelBonus_CappedAt04(t *testing.T) {
	// 3 Dark Rituals (+2 × 3 = +6). Bonus = 0.60 raw, capped at 0.40.
	_, gs := makeRitualEvalGame(t)
	seat := gs.Seats[0]
	makeColoredLand(seat, "B", false)
	seat.Hand = append(seat.Hand,
		makeRitualInHand("Dark Ritual", 1),
		makeRitualInHand("Dark Ritual", 1),
		makeRitualInHand("Dark Ritual", 1),
		makeSpellInHand("Massive Sorcery", 7),
	)
	got := ritualFuelBonus(seat)
	want := 0.40
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("3 Dark Rituals capped: got %v, want %v", got, want)
	}
}

func TestRitualFuelBonus_RitualNotItsOwnChainTarget(t *testing.T) {
	// Two rituals + zero non-ritual spells. The rituals must NOT count
	// each other as chain targets (Dark Ritual at CMC 1 IS castable
	// for 1 mana, but the eval should only credit rituals when there's
	// a non-ritual spell that benefits). Bonus = 0.
	_, gs := makeRitualEvalGame(t)
	seat := gs.Seats[0]
	makeColoredLand(seat, "B", false)
	seat.Hand = append(seat.Hand,
		makeRitualInHand("Dark Ritual", 1),
		makeRitualInHand("Seething Song", 3),
	)
	if got := ritualFuelBonus(seat); got != 0 {
		t.Errorf("rituals alone (no payoff spell): got %v, want 0", got)
	}
}

func TestRitualFuelBonus_LandsInHandIgnored(t *testing.T) {
	// Dark Ritual + a land card in hand. Lands don't cast — they
	// shouldn't satisfy the chain-target check.
	_, gs := makeRitualEvalGame(t)
	seat := gs.Seats[0]
	makeColoredLand(seat, "B", false) // cap = 1
	land := newTestCardMinimal("Cabal Coffers", []string{"land"}, 0, nil)
	seat.Hand = append(seat.Hand,
		makeRitualInHand("Dark Ritual", 1),
		land,
	)
	if got := ritualFuelBonus(seat); got != 0 {
		t.Errorf("land in hand isn't a chain target: got %v, want 0", got)
	}
}

func TestRitualFuelBonus_RitualPlusZeroLandsZeroBonus(t *testing.T) {
	// Seat has 0 untapped sources but a Dark Ritual + a 2-CMC spell.
	// Ceiling = 0 + 2 = 2, target CMC = 2 satisfies (target > 0 cap AND
	// target ≤ ceiling). Bonus = 0.20.
	_, gs := makeRitualEvalGame(t)
	seat := gs.Seats[0]
	// No lands.
	seat.Hand = append(seat.Hand,
		makeRitualInHand("Dark Ritual", 1),
		makeSpellInHand("Cabal Coffers Backup", 2),
	)
	got := ritualFuelBonus(seat)
	want := 0.20
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("zero-land ritual: got %v, want %v", got, want)
	}
}

func TestRitualFuelBonus_NilSeatSafe(t *testing.T) {
	if got := ritualFuelBonus(nil); got != 0 {
		t.Errorf("nil seat must return 0, got %v", got)
	}
}

// -----------------------------------------------------------------------------
// Integration — scoreMana lifts when ritual + chain target present
// -----------------------------------------------------------------------------

func TestScoreMana_RitualFuel_LiftsScore(t *testing.T) {
	// Two parallel seats with identical 2-land boards and identical
	// 3-CMC payoff in hand. Only difference: one has a Dark Ritual,
	// the other has a vanilla spell. The ritual-holder must score
	// higher by exactly the +0.20 bonus.
	ev, gsRitual := makeRitualEvalGame(t)
	seat := gsRitual.Seats[0]
	makeColoredLand(seat, "B", false)
	makeColoredLand(seat, "B", false)
	seat.Hand = append(seat.Hand,
		makeRitualInHand("Dark Ritual", 1),
		makeSpellInHand("Necropotence", 3),
	)
	// Equalize opp source count so the delta isolates the ritual signal.
	makeColoredLand(gsRitual.Seats[1], "B", false)
	makeColoredLand(gsRitual.Seats[1], "B", false)
	ritualScore := ev.scoreMana(gsRitual, 0)

	ev2, gsNoRitual := makeRitualEvalGame(t)
	seat2 := gsNoRitual.Seats[0]
	makeColoredLand(seat2, "B", false)
	makeColoredLand(seat2, "B", false)
	// Wait — without a ritual the Necropotence isn't actually
	// castable. To make this a fair A/B (isolate the ritual signal
	// from "spell castability"), put a CMC-2 spell on the no-ritual
	// side that IS castable, and Necropotence on the ritual side
	// that ISN'T without burst. The ritual is what unlocks the
	// payoff — that's the entire signal.
	seat2.Hand = append(seat2.Hand,
		makeSpellInHand("Dark Confidant", 2), // 2 ≤ cap, castable now
		makeSpellInHand("Necropotence", 3),
	)
	makeColoredLand(gsNoRitual.Seats[1], "B", false)
	makeColoredLand(gsNoRitual.Seats[1], "B", false)
	noRitualScore := ev2.scoreMana(gsNoRitual, 0)

	if !(ritualScore > noRitualScore) {
		t.Errorf("ritual+chain-target seat must score HIGHER than equivalent ritual-less seat: ritual=%v noRitual=%v",
			ritualScore, noRitualScore)
	}
	delta := ritualScore - noRitualScore
	if math.Abs(delta-0.20) > 1e-6 {
		t.Errorf("expected score delta ~+0.20 (Dark Ritual bonus), got %v", delta)
	}
}

func TestScoreMana_RitualFuel_NoChainTargetDoesNothing(t *testing.T) {
	// Sanity: a ritual in hand without a chain target must NOT lift
	// the score. Otherwise the eval over-rewards rituals in matchups
	// where the deck topdecks fast mana with nothing to spend it on.
	ev, gsRitual := makeRitualEvalGame(t)
	seat := gsRitual.Seats[0]
	for i := 0; i < 5; i++ {
		makeColoredLand(seat, "B", false)
	}
	seat.Hand = append(seat.Hand,
		makeRitualInHand("Dark Ritual", 1),
		makeSpellInHand("Lightning Bolt", 1), // already castable
	)
	for i := 0; i < 5; i++ {
		makeColoredLand(gsRitual.Seats[1], "B", false)
	}
	ritualScore := ev.scoreMana(gsRitual, 0)

	ev2, gsNoRitual := makeRitualEvalGame(t)
	seat2 := gsNoRitual.Seats[0]
	for i := 0; i < 5; i++ {
		makeColoredLand(seat2, "B", false)
	}
	seat2.Hand = append(seat2.Hand,
		makeSpellInHand("Counterspell", 2),
		makeSpellInHand("Lightning Bolt", 1),
	)
	for i := 0; i < 5; i++ {
		makeColoredLand(gsNoRitual.Seats[1], "B", false)
	}
	noRitualScore := ev2.scoreMana(gsNoRitual, 0)

	if math.Abs(ritualScore-noRitualScore) > 1e-9 {
		t.Errorf("dead ritual must not lift score: ritual=%v noRitual=%v", ritualScore, noRitualScore)
	}
}
