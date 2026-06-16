package tournament

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

// Day/night audit r63 (CR §726.3a/§726.4). The turn loop drives the day/night
// transition off gs.SpellsCastByActiveLastTurn, but that field was populated
// only by the paritycheck harness — never by the production TakeTurn loop —
// so it always read 0: day flipped to night on turn 2 and night NEVER reverted
// to day (needs ≥2), stranding daybound werewolves on their nightbound face.
// These pin that TakeTurn feeds the previous turn's spell count.

func tnWerewolf(gs *gameengine.GameState, seat int) *gameengine.Permanent {
	front := &gameast.CardAST{Name: "Probe Wolf (human)", FullyParsed: true,
		Abilities: []gameast.Ability{&gameast.Keyword{Name: "daybound", Raw: "Daybound"}}}
	back := &gameast.CardAST{Name: "Probe Wolf (wolf)", FullyParsed: true,
		Abilities: []gameast.Ability{&gameast.Keyword{Name: "nightbound", Raw: "Nightbound"}}}
	card := &gameengine.Card{AST: front, Name: "Probe Wolf (human)", Owner: seat,
		BasePower: 2, BaseToughness: 2, Types: []string{"creature"}, TypeLine: "Creature — Human Werewolf"}
	p := &gameengine.Permanent{Card: card, Controller: seat, Owner: seat,
		FrontFaceAST: front, BackFaceAST: back,
		Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func tnSetup(t *testing.T) *gameengine.GameState {
	t.Helper()
	gs := gameengine.NewGameState(2, nil, nil)
	for i := 0; i < 2; i++ {
		gs.Seats[i].Hat = &hat.GreedyHat{}
		gs.Seats[i].Life = 20
		for j := 0; j < 5; j++ {
			gs.Seats[i].Library = append(gs.Seats[i].Library, &gameengine.Card{Name: "Forest", Types: []string{"land"}, Owner: i})
		}
	}
	gs.Active = 0
	gs.Turn = 2
	return gs
}

// THE BUG: night + 2 spells last turn must become DAY (§726.4b). Pre-fix the
// field read 0, so it stayed night and the werewolf stayed on its wolf face.
func TestDayNight_Wiring_NightTwoSpellsBecomesDay(t *testing.T) {
	gs := tnSetup(t)
	w := tnWerewolf(gs, 0)
	gameengine.SetDayNight(gs, gameengine.DayNightNight, "test", "726.1")
	if !gameengine.PermHasNightbound(w) {
		t.Fatal("setup: werewolf should be on its nightbound back face at night")
	}
	gs.SpellsCastThisTurn = 2 // two spells cast during the previous turn

	TakeTurn(gs)

	if gs.DayNight != gameengine.DayNightDay {
		t.Fatalf("§726.4b: night + 2 spells last turn must become DAY, got %q "+
			"(SpellsCastByActiveLastTurn not fed from the turn loop?)", gs.DayNight)
	}
	if !gameengine.PermHasDaybound(w) {
		t.Fatal("werewolf must transform back to its daybound front face when it becomes day")
	}
}

// Symmetry: day + 0 spells last turn becomes night (§726.4a). This held even
// pre-fix (the field defaulted to 0), but pin it so the fix doesn't regress it.
func TestDayNight_Wiring_DayZeroSpellsBecomesNight(t *testing.T) {
	gs := tnSetup(t)
	w := tnWerewolf(gs, 0)
	gameengine.SetDayNight(gs, gameengine.DayNightDay, "test", "726.1")
	gs.SpellsCastThisTurn = 0

	TakeTurn(gs)

	if gs.DayNight != gameengine.DayNightNight {
		t.Fatalf("§726.4a: day + 0 spells last turn must become NIGHT, got %q", gs.DayNight)
	}
	if !gameengine.PermHasNightbound(w) {
		t.Fatal("werewolf must transform to its nightbound face when it becomes night")
	}
}
