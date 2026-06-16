package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Loki seed 41076465 / game 4107 (the r63 verification sweep): a seat-2
// Vorinclex, Monstrous Raider halved an opponent's Clockwork Beetle (2 → 1)
// and Clockwork Avian (4 → 2) as they entered. That halving is rules-correct
// (CR §122.1g — Vorinclex's opponent arm halves counters an opponent puts,
// including a creature's own "enters with N counters" self-replacement), but
// the §614.1c legality validator false-flagged it by comparing the final
// tally against the RAW printed count. These regressions pin both the
// engine's placement (plain + Doubling-Season-doubled) and the validator's
// new §616-aware guard.

// clockworkLikeCard builds a minimal "enters with N <kind> counters" creature
// (one etb_with_counters Static, no other abilities — the ETB counter path is
// all these tests exercise).
func clockworkLikeCard(name string, n int, kind string) *Card {
	return &Card{
		Name:          name,
		TypeLine:      "Artifact Creature — Construct",
		Types:         []string{"artifact", "creature"},
		BasePower:     0,
		BaseToughness: 3,
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Static{
					Modification: &gameast.Modification{
						ModKind: "etb_with_counters",
						Args:    []interface{}{n, kind},
					},
					Raw: "this creature enters with counters on it",
				},
			},
		},
	}
}

func enterClockwork(gs *GameState, seat int, name string, n int, kind string) *Permanent {
	perm := &Permanent{
		Card:       clockworkLikeCard(name, n, kind),
		Controller: seat,
		Owner:      seat,
		Counters:   map[string]int{},
		Flags:      map[string]int{},
		Timestamp:  gs.NextTimestamp(),
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, perm)
	ApplyStaticETBCounters(gs, perm)
	return perm
}

// TestEntersWithCounters_PlainClockwork — with no counter-modifying
// replacement in play, Clockwork Beetle enters with exactly 2 +1/+1 and
// Clockwork Avian with exactly 4 +1/+0. (The literal-count path.)
func TestEntersWithCounters_PlainClockwork(t *testing.T) {
	gs := newTestGameState(2)
	beetle := enterClockwork(gs, 0, "Clockwork Beetle", 2, "+1/+1")
	if got := beetle.Counters["+1/+1"]; got != 2 {
		t.Fatalf("Clockwork Beetle plain ETB: +1/+1 = %d, want 2", got)
	}
	avian := enterClockwork(gs, 0, "Clockwork Avian", 4, "+1/+0")
	if got := avian.Counters["+1/+0"]; got != 4 {
		t.Fatalf("Clockwork Avian plain ETB: +1/+0 = %d, want 4", got)
	}
}

// TestEntersWithCounters_DoublingSeasonDoubles — Doubling Season (controlled
// by the entering creature's controller) doubles the ETB self-replacement
// counters of ALL kinds: Beetle 2 → 4, Avian 4 → 8 (CR §616 / §122.1g).
func TestEntersWithCounters_DoublingSeasonDoubles(t *testing.T) {
	gs := newTestGameState(2)
	addDoublerSource(gs, 0, "Doubling Season", RegisterDoublingSeason)

	beetle := enterClockwork(gs, 0, "Clockwork Beetle", 2, "+1/+1")
	if got := beetle.Counters["+1/+1"]; got != 4 {
		t.Fatalf("Clockwork Beetle under Doubling Season: +1/+1 = %d, want 4", got)
	}
	avian := enterClockwork(gs, 0, "Clockwork Avian", 4, "+1/+0")
	if got := avian.Counters["+1/+0"]; got != 8 {
		t.Fatalf("Clockwork Avian under Doubling Season: +1/+0 = %d, want 8", got)
	}
}

// TestEntersWithCounters_VorinclexOpponentHalves_NoFalsePositive — the
// canonical game-4107 case. An OPPONENT's Vorinclex halves the entering
// creature's counters (Beetle 2 → 1, Avian 4 → 2), which is rules-correct, and
// the §614.1c legality validator must NOT flag it.
func TestEntersWithCounters_VorinclexOpponentHalves_NoFalsePositive(t *testing.T) {
	gs := newTestGameState(2)
	gs.Legality = NewLegalityValidator(41076465)
	// Vorinclex controlled by seat 1 (an opponent of the Clockwork controller).
	addDoublerSource(gs, 1, "Vorinclex, Monstrous Raider", RegisterVorinclexMonstrousRaider)

	beetle := enterClockwork(gs, 0, "Clockwork Beetle", 2, "+1/+1")
	if got := beetle.Counters["+1/+1"]; got != 1 {
		t.Fatalf("Beetle under opponent Vorinclex: +1/+1 = %d, want 1 (halved)", got)
	}
	avian := enterClockwork(gs, 0, "Clockwork Avian", 4, "+1/+0")
	if got := avian.Counters["+1/+0"]; got != 2 {
		t.Fatalf("Avian under opponent Vorinclex: +1/+0 = %d, want 2 (halved)", got)
	}

	gs.Legality.ObserveETB(gs, beetle)
	gs.Legality.ObserveETB(gs, avian)
	for _, lv := range gs.Legality.Violations {
		if lv.Rule == "614.1c" {
			t.Fatalf("614.1c false-positive under opponent Vorinclex halve: %s", lv.String())
		}
	}
}

// TestEntersWithCounters_LegalityStillCatchesMissingCounter — the District
// Mascot bug this check exists for still fires: a literal "enters with N
// counters" static whose counters were NEVER applied, with NO counter-
// modifying replacement in play, is a real §614.1c violation.
func TestEntersWithCounters_LegalityStillCatchesMissingCounter(t *testing.T) {
	gs := newTestGameState(2)
	gs.Legality = NewLegalityValidator(1)

	// Build the perm but deliberately DO NOT call ApplyStaticETBCounters —
	// simulate the counter never being applied.
	perm := &Permanent{
		Card:       clockworkLikeCard("Clockwork Beetle", 2, "+1/+1"),
		Controller: 0,
		Owner:      0,
		Counters:   map[string]int{},
		Flags:      map[string]int{},
		Timestamp:  gs.NextTimestamp(),
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	gs.Legality.ObserveETB(gs, perm)

	found := false
	for _, lv := range gs.Legality.Violations {
		if lv.Rule == "614.1c" {
			found = true
		}
	}
	if !found {
		t.Fatalf("614.1c must still fire when ETB counters are genuinely missing and no replacement modifies the count")
	}
}
