package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// =============================================================================
// ModificationEffect handler tests — Wave 1a promoted labels
// =============================================================================

func TestModificationEffect_PhaseOutSelf(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Mist Dragon", 4, 4, "creature")

	e := &gameast.ModificationEffect{ModKind: "phase_out_self"}
	ResolveEffect(gs, src, e)

	if src.Flags["phased_out"] != 1 {
		t.Errorf("expected phased_out flag=1, got %d", src.Flags["phased_out"])
	}
	if countEvents(gs, "phase_out") != 1 {
		t.Errorf("expected 1 phase_out event, got %d", countEvents(gs, "phase_out"))
	}
}

func TestModificationEffect_StunTarget(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Stitcher's Graft", 0, 0, "artifact")
	target := addBattlefield(gs, 1, "Bear", 2, 2, "creature")

	e := &gameast.ModificationEffect{
		ModKind: "stun_target_next_untap",
		Args:    []interface{}{"target_creature_opponent"},
	}
	ResolveEffect(gs, src, e)

	if target.Flags["stun"] != 1 {
		t.Errorf("expected stun flag=1, got %d", target.Flags["stun"])
	}
	if countEvents(gs, "stun") != 1 {
		t.Errorf("expected 1 stun event, got %d", countEvents(gs, "stun"))
	}
}

func TestModificationEffect_Goad(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Disrupt Decorum", 0, 0, "sorcery")
	target := addBattlefield(gs, 1, "Bear", 2, 2, "creature")

	e := &gameast.ModificationEffect{
		ModKind: "goad",
		Args:    []interface{}{"target_creature_opponent"},
	}
	ResolveEffect(gs, src, e)

	if target.Flags["goaded"] != 1 {
		t.Errorf("expected goaded flag=1, got %d", target.Flags["goaded"])
	}
	if countEvents(gs, "goad") != 1 {
		t.Errorf("expected 1 goad event, got %d", countEvents(gs, "goad"))
	}
}

func TestModificationEffect_Investigate(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Bygone Bishop", 2, 3, "creature")

	e := &gameast.ModificationEffect{
		ModKind: "investigate",
		Args:    []interface{}{2},
	}
	ResolveEffect(gs, src, e)

	// Should create 2 Clue tokens + 1 investigate event.
	clueCount := 0
	for _, p := range gs.Seats[0].Battlefield {
		for _, tp := range p.Card.Types {
			if tp == "clue" {
				clueCount++
				break
			}
		}
	}
	if clueCount != 2 {
		t.Errorf("expected 2 clue tokens, got %d", clueCount)
	}
	if countEvents(gs, "investigate") != 1 {
		t.Errorf("expected 1 investigate event, got %d", countEvents(gs, "investigate"))
	}
}

func TestModificationEffect_Suspect(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Agent", 3, 2, "creature")

	e := &gameast.ModificationEffect{ModKind: "suspect"}
	ResolveEffect(gs, src, e)

	if src.Flags["suspected"] != 1 {
		t.Errorf("expected suspected flag=1, got %d", src.Flags["suspected"])
	}
	// Should also grant menace.
	hasMenace := false
	for _, a := range src.GrantedAbilities {
		if a == "menace" {
			hasMenace = true
			break
		}
	}
	if !hasMenace {
		t.Errorf("expected menace to be granted")
	}
	if countEvents(gs, "suspect") != 1 {
		t.Errorf("expected 1 suspect event, got %d", countEvents(gs, "suspect"))
	}
}

func TestModificationEffect_NoLifeGained(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Tibalt", 0, 0, "planeswalker")

	e := &gameast.ModificationEffect{ModKind: "no_life_gained"}
	ResolveEffect(gs, src, e)

	if gs.Flags["no_life_gained"] != 1 {
		t.Errorf("expected no_life_gained flag=1, got %d", gs.Flags["no_life_gained"])
	}
}

func TestModificationEffect_SuppressPrevention(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Leyline", 0, 0, "enchantment")

	e := &gameast.ModificationEffect{ModKind: "suppress_prevention"}
	ResolveEffect(gs, src, e)

	if gs.Flags["suppress_prevention"] != 1 {
		t.Errorf("expected suppress_prevention flag=1, got %d", gs.Flags["suppress_prevention"])
	}
}

func TestModificationEffect_LoseAbilityEOT(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Torpid Moloch", 3, 2, "creature")

	e := &gameast.ModificationEffect{
		ModKind: "lose_ability_eot",
		Args:    []interface{}{"defender"},
	}
	ResolveEffect(gs, src, e)

	if src.Flags["lost_defender"] != 1 {
		t.Errorf("expected lost_defender flag=1, got %d", src.Flags["lost_defender"])
	}
	if countEvents(gs, "lose_ability") != 1 {
		t.Errorf("expected 1 lose_ability event, got %d", countEvents(gs, "lose_ability"))
	}
}

func TestModificationEffect_AttackWithoutDefender(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Skyclave Squid", 3, 2, "creature")

	e := &gameast.ModificationEffect{ModKind: "attack_without_defender_eot"}
	ResolveEffect(gs, src, e)

	if src.Flags["attack_without_defender"] != 1 {
		t.Errorf("expected attack_without_defender flag=1, got %d", src.Flags["attack_without_defender"])
	}
}

func TestModificationEffect_DoublePowerEOT(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Mr. Orfeo", 0, 0, "creature")
	target := addBattlefield(gs, 1, "Bear", 2, 2, "creature")
	_ = target // opponent's creature will be doubled

	e := &gameast.ModificationEffect{ModKind: "double_power_eot"}
	ResolveEffect(gs, src, e)

	// The handler picks opponent's creature. Bear was 2/2, should now be 4/2.
	if target.Power() != 4 {
		t.Errorf("expected doubled power 4, got %d", target.Power())
	}
	if target.Toughness() != 2 {
		t.Errorf("expected toughness still 2, got %d", target.Toughness())
	}
}

func TestModificationEffect_HeroicP1P1(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "War-Wing Siren", 1, 3, "creature")

	e := &gameast.ModificationEffect{ModKind: "heroic_rider_p1p1_self"}
	ResolveEffect(gs, src, e)

	if src.Power() != 2 || src.Toughness() != 4 {
		t.Errorf("expected 2/4 after heroic +1/+1, got %d/%d", src.Power(), src.Toughness())
	}
}

func TestModificationEffect_HeroicTwoP1P1(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Setessan Oathsworn", 1, 1, "creature")

	e := &gameast.ModificationEffect{ModKind: "heroic_rider_two_p1p1_self"}
	ResolveEffect(gs, src, e)

	if src.Power() != 3 || src.Toughness() != 3 {
		t.Errorf("expected 3/3 after heroic two +1/+1, got %d/%d", src.Power(), src.Toughness())
	}
}

func TestModificationEffect_HeroicAnthemEOT(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Hero of the Nyxborn", 2, 2, "creature")
	ally := addBattlefield(gs, 0, "Soldier", 1, 1, "creature")

	e := &gameast.ModificationEffect{
		ModKind: "heroic_rider_anthem_eot",
		Args:    []interface{}{1, 0},
	}
	ResolveEffect(gs, src, e)

	// Both creatures on seat 0 should get +1/+0.
	if src.Power() != 3 {
		t.Errorf("expected src power 3, got %d", src.Power())
	}
	if ally.Power() != 2 {
		t.Errorf("expected ally power 2, got %d", ally.Power())
	}
}

func TestModificationEffect_CounterUnlessPay(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Stack = append(gs.Stack, &StackItem{ID: 1, Controller: 1})
	src := addBattlefield(gs, 0, "Glasskite", 0, 0, "creature")

	e := &gameast.ModificationEffect{
		ModKind: "counter_that_spell_unless_pay",
		Args:    []interface{}{2},
	}
	ResolveEffect(gs, src, e)

	if !gs.Stack[0].Countered {
		t.Errorf("expected stack item 0 to be countered")
	}
}

func TestModificationEffect_DrawUnlessPay(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Rhystic Study", 0, 0, "enchantment")
	addLibrary(gs, 0, "Card A", "Card B")

	e := &gameast.ModificationEffect{
		ModKind: "draw_unless_pay",
		Args:    []interface{}{1},
	}
	ResolveEffect(gs, src, e)

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 card drawn, got %d", len(gs.Seats[0].Hand))
	}
}

func TestModificationEffect_FlipCoinUntilLose(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Zndrsplt", 0, 0, "creature")

	e := &gameast.ModificationEffect{ModKind: "flip_coin_until_lose"}
	ResolveEffect(gs, src, e)

	if countEvents(gs, "flip_coin") != 1 {
		t.Errorf("expected 1 flip_coin event, got %d", countEvents(gs, "flip_coin"))
	}
}

func TestModificationEffect_ChooseColor(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Painter's Servant", 0, 0, "artifact")

	e := &gameast.ModificationEffect{ModKind: "choose_color"}
	ResolveEffect(gs, src, e)

	if countEvents(gs, "choose_color") != 1 {
		t.Errorf("expected 1 choose_color event, got %d", countEvents(gs, "choose_color"))
	}
	ev := lastEventOfKind(gs, "choose_color")
	color, ok := ev.Details["color"].(string)
	if !ok || color == "" {
		t.Errorf("expected a color to be chosen, got %v", ev.Details)
	}
}

func TestModificationEffect_Regenerate(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Troll Ascetic", 3, 2, "creature")

	e := &gameast.ModificationEffect{ModKind: "regenerate"}
	ResolveEffect(gs, src, e)

	if src.Flags["regeneration_shield"] != 1 {
		t.Errorf("expected regeneration_shield flag=1, got %d", src.Flags["regeneration_shield"])
	}
}

func TestModificationEffect_Plot(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Outlaw", 0, 0, "creature")

	e := &gameast.ModificationEffect{ModKind: "plot"}
	ResolveEffect(gs, src, e)

	if src.Flags["plotted"] != 1 {
		t.Errorf("expected plotted flag=1, got %d", src.Flags["plotted"])
	}
}

func TestModificationEffect_UnknownKindEmitsModificationEvent(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Weird Card", 0, 0, "instant")

	e := &gameast.ModificationEffect{
		ModKind: "some_future_kind",
		Args:    []interface{}{"arg1"},
	}
	ResolveEffect(gs, src, e)

	// Should NOT emit unknown_effect — should emit modification_effect.
	if countEvents(gs, "unknown_effect") != 0 {
		t.Errorf("expected 0 unknown_effect events, got %d", countEvents(gs, "unknown_effect"))
	}
	if countEvents(gs, "modification_effect") != 1 {
		t.Errorf("expected 1 modification_effect event, got %d", countEvents(gs, "modification_effect"))
	}
	ev := lastEventOfKind(gs, "modification_effect")
	if ev.Details["mod_kind"] != "some_future_kind" {
		t.Errorf("expected mod_kind=some_future_kind, got %v", ev.Details["mod_kind"])
	}
}

func TestModificationEffect_NilSourceNoPanic(t *testing.T) {
	gs := newFixtureGame(t)
	// All kinds should handle nil src gracefully.
	kinds := []string{
		"phase_out_self", "suspect", "plot", "becomes_prepared",
		"attack_without_defender_eot", "no_life_gained",
		"suppress_prevention", "regenerate",
	}
	for _, kind := range kinds {
		e := &gameast.ModificationEffect{ModKind: kind}
		ResolveEffect(gs, nil, e) // should not panic
	}
}

func TestModificationEffect_ExileTopLibrary(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Exiler", 0, 0, "creature")
	addLibrary(gs, 0, "Card A", "Card B")

	e := &gameast.ModificationEffect{ModKind: "exile_top_library"}
	ResolveEffect(gs, src, e)

	// One card should have been removed from the top of the library.
	if len(gs.Seats[0].Library) != 1 {
		t.Errorf("expected 1 card left in library, got %d", len(gs.Seats[0].Library))
	}
}

func TestModificationEffect_RollD20(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Dungeon Master", 0, 0, "creature")

	e := &gameast.ModificationEffect{ModKind: "roll_d20"}
	ResolveEffect(gs, src, e)

	if countEvents(gs, "roll_d20") != 1 {
		t.Errorf("expected 1 roll_d20 event, got %d", countEvents(gs, "roll_d20"))
	}
	ev := lastEventOfKind(gs, "roll_d20")
	if ev.Amount < 1 || ev.Amount > 20 {
		t.Errorf("expected d20 result 1-20, got %d", ev.Amount)
	}
}
