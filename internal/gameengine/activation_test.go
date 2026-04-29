package gameengine

// Wave 3a/3b tests — activated abilities on the stack + stax enforcement.
//
// Verifies:
//   - Null Rod suppresses artifact non-mana activations
//   - Null Rod does NOT suppress mana abilities
//   - Cursed Totem suppresses ALL creature activated abilities
//   - Grand Abolisher blocks opponent activations during controller's turn
//   - Drannith Magistrate blocks non-hand zone casts
//   - Activated abilities go on the stack (non-mana)
//   - Mana abilities resolve inline (never hit the stack)
//   - Split-second blocks non-mana activations
//   - Opposition Agent redirects tutor results to agent controller's exile

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Fixture helpers
// ---------------------------------------------------------------------------

// makeArtifactPerm creates a battlefield permanent typed "artifact" with
// an activated ability at index 0.
func makeArtifactPerm(gs *GameState, seat int, name string, effect gameast.Effect, hasTap bool) *Permanent {
	card := &Card{
		Name:  name,
		Owner: seat,
		Types: []string{"artifact"},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Activated{
					Cost:   gameast.Cost{Tap: hasTap},
					Effect: effect,
				},
			},
		},
	}
	perm := &Permanent{
		Card:       card,
		Controller: seat,
		Owner:      seat,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, perm)
	return perm
}

// makeCreaturePerm creates a battlefield permanent typed "creature" with
// an activated ability at index 0.
func makeCreaturePerm(gs *GameState, seat int, name string, effect gameast.Effect, hasTap bool) *Permanent {
	card := &Card{
		Name:  name,
		Owner: seat,
		Types: []string{"creature"},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Activated{
					Cost:   gameast.Cost{Tap: hasTap},
					Effect: effect,
				},
			},
		},
	}
	perm := &Permanent{
		Card:       card,
		Controller: seat,
		Owner:      seat,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, perm)
	return perm
}

// makeArtifactManaAbilityPerm creates an artifact with a mana ability
// (AddMana effect, no target).
func makeArtifactManaAbilityPerm(gs *GameState, seat int, name string) *Permanent {
	manaEff := &gameast.AddMana{AnyColorCount: 1}
	return makeArtifactPerm(gs, seat, name, manaEff, true)
}

// makeCreatureManaAbilityPerm creates a creature with a mana ability
// (AddMana effect, no target, {T} cost).
func makeCreatureManaAbilityPerm(gs *GameState, seat int, name string) *Permanent {
	manaEff := &gameast.AddMana{AnyColorCount: 1}
	return makeCreaturePerm(gs, seat, name, manaEff, true)
}

// countEventsOfKind counts events of a given Kind in the event log.
func countEventsOfKind(gs *GameState, kind string) int {
	n := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == kind {
			n++
		}
	}
	return n
}

// hasEventOfKind returns true if at least one event of the given Kind exists.
func hasEventOfKind(gs *GameState, kind string) bool {
	return countEventsOfKind(gs, kind) > 0
}

// ---------------------------------------------------------------------------
// 3a: Activated abilities go on the stack
// ---------------------------------------------------------------------------

func TestActivateAbility_NonMana_GoesOnStack(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].ManaPool = 10
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20

	// Create an artifact with a non-mana activated ability (damage).
	dmgEff := &gameast.Damage{
		Amount: gameast.NumberOrRef{IsInt: true, Int: 3},
		Target: gameast.Filter{Base: "player"},
	}
	perm := makeArtifactPerm(gs, 0, "Shock Rod", dmgEff, true)

	err := ActivateAbility(gs, 0, perm, 0, nil)
	if err != nil {
		t.Fatalf("ActivateAbility failed: %v", err)
	}

	// The stack should be empty after resolution (activation resolved).
	if len(gs.Stack) != 0 {
		t.Fatalf("expected empty stack after resolution, got %d items", len(gs.Stack))
	}

	// Should have logged an activate_ability event (non-mana goes on stack).
	if !hasEventOfKind(gs, "activate_ability") {
		t.Fatal("expected activate_ability event for non-mana ability")
	}

	// Should NOT log activate_mana_ability.
	if hasEventOfKind(gs, "activate_mana_ability") {
		t.Fatal("non-mana ability should not produce activate_mana_ability event")
	}

	// Should have a stack_push event (it went on the stack).
	if !hasEventOfKind(gs, "stack_push") {
		t.Fatal("expected stack_push event for activated ability")
	}
}

func TestActivateAbility_ManaAbility_InlineResolution(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].ManaPool = 0
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20

	perm := makeArtifactManaAbilityPerm(gs, 0, "Sol Ring")

	err := ActivateAbility(gs, 0, perm, 0, nil)
	if err != nil {
		t.Fatalf("ActivateAbility failed: %v", err)
	}

	// Mana ability should log activate_mana_ability, NOT stack_push.
	if !hasEventOfKind(gs, "activate_mana_ability") {
		t.Fatal("expected activate_mana_ability event")
	}

	// Should NOT have gone on the stack.
	if hasEventOfKind(gs, "activate_ability") {
		t.Fatal("mana ability should not produce activate_ability event")
	}
}

// ---------------------------------------------------------------------------
// 3b: Null Rod suppresses artifact non-mana activations
// ---------------------------------------------------------------------------

func TestStaxCheck_NullRod_SuppressesArtifactNonMana(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Flags["null_rod_count"] = 1

	dmgEff := &gameast.Damage{Amount: gameast.NumberOrRef{IsInt: true, Int: 5}}
	perm := makeArtifactPerm(gs, 0, "Aetherflux Reservoir", dmgEff, false)

	supp := StaxCheck(gs, 0, perm, 0)
	if !supp.Suppressed {
		t.Fatal("Null Rod should suppress non-mana artifact activation")
	}
	if supp.Reason != "null_rod" {
		t.Fatalf("wrong suppression reason: %s", supp.Reason)
	}
}

func TestStaxCheck_NullRod_ExemptsManaAbility(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Flags["null_rod_count"] = 1

	perm := makeArtifactManaAbilityPerm(gs, 0, "Sol Ring")

	supp := StaxCheck(gs, 0, perm, 0)
	if supp.Suppressed {
		t.Fatal("Null Rod should NOT suppress mana abilities")
	}
}

func TestActivateAbility_NullRod_Suppresses(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].ManaPool = 10
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20
	gs.Flags["null_rod_count"] = 1

	dmgEff := &gameast.Damage{Amount: gameast.NumberOrRef{IsInt: true, Int: 5}}
	perm := makeArtifactPerm(gs, 0, "Shock Rod", dmgEff, false)

	err := ActivateAbility(gs, 0, perm, 0, nil)
	if err == nil {
		t.Fatal("expected error when Null Rod suppresses activation")
	}

	if !hasEventOfKind(gs, "activation_suppressed") {
		t.Fatal("expected activation_suppressed event")
	}
}

// ---------------------------------------------------------------------------
// 3b: Cursed Totem suppresses creature activations
// ---------------------------------------------------------------------------

func TestStaxCheck_CursedTotem_SuppressesCreature(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Flags["cursed_totem_count"] = 1

	dmgEff := &gameast.Damage{Amount: gameast.NumberOrRef{IsInt: true, Int: 1}}
	perm := makeCreaturePerm(gs, 0, "Walking Ballista", dmgEff, false)

	supp := StaxCheck(gs, 0, perm, 0)
	if !supp.Suppressed {
		t.Fatal("Cursed Totem should suppress creature activated ability")
	}
	if supp.Reason != "cursed_totem" {
		t.Fatalf("wrong suppression reason: %s", supp.Reason)
	}
}

func TestStaxCheck_CursedTotem_SuppressesManaAbility(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Flags["cursed_totem_count"] = 1

	// Cursed Totem suppresses ALL creature activated abilities, including
	// mana abilities (unlike Null Rod).
	perm := makeCreatureManaAbilityPerm(gs, 0, "Birds of Paradise")

	supp := StaxCheck(gs, 0, perm, 0)
	if !supp.Suppressed {
		t.Fatal("Cursed Totem should suppress creature mana abilities too")
	}
}

func TestStaxCheck_CursedTotem_DoesNotAffectArtifacts(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Flags["cursed_totem_count"] = 1

	dmgEff := &gameast.Damage{Amount: gameast.NumberOrRef{IsInt: true, Int: 5}}
	perm := makeArtifactPerm(gs, 0, "Shock Rod", dmgEff, false)

	supp := StaxCheck(gs, 0, perm, 0)
	if supp.Suppressed {
		t.Fatal("Cursed Totem should NOT affect artifact activations")
	}
}

// ---------------------------------------------------------------------------
// 3b: Grand Abolisher blocks opponent activations
// ---------------------------------------------------------------------------

func TestStaxCheck_GrandAbolisher_BlocksOpponent(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Active = 0
	// Seat 0 has Grand Abolisher.
	gs.Flags["grand_abolisher_active_seat_0"] = 1

	dmgEff := &gameast.Damage{Amount: gameast.NumberOrRef{IsInt: true, Int: 1}}
	// Seat 1 tries to activate an artifact during seat 0's turn.
	perm := makeArtifactPerm(gs, 1, "Opponent's Rod", dmgEff, false)

	supp := StaxCheck(gs, 1, perm, 0)
	if !supp.Suppressed {
		t.Fatal("Grand Abolisher should suppress opponent's activation during your turn")
	}
	if supp.Reason != "grand_abolisher" {
		t.Fatalf("wrong suppression reason: %s", supp.Reason)
	}
}

func TestStaxCheck_GrandAbolisher_AllowsController(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Active = 0
	gs.Flags["grand_abolisher_active_seat_0"] = 1

	dmgEff := &gameast.Damage{Amount: gameast.NumberOrRef{IsInt: true, Int: 1}}
	perm := makeArtifactPerm(gs, 0, "Own Rod", dmgEff, false)

	supp := StaxCheck(gs, 0, perm, 0)
	if supp.Suppressed {
		t.Fatal("Grand Abolisher should NOT suppress controller's own activations")
	}
}

// ---------------------------------------------------------------------------
// 3b: Split-second blocks non-mana activations
// ---------------------------------------------------------------------------

func TestStaxCheck_SplitSecond_BlocksNonMana(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	// Put a split-second spell on the stack.
	ssCard := &Card{
		Name:  "Krosan Grip",
		Owner: 1,
		Types: []string{"instant"},
		AST: &gameast.CardAST{
			Name: "Krosan Grip",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "split_second"},
				&gameast.Activated{Effect: &gameast.Destroy{Target: gameast.Filter{Base: "artifact"}}},
			},
		},
	}
	gs.Stack = append(gs.Stack, &StackItem{
		Controller: 1,
		Card:       ssCard,
	})

	dmgEff := &gameast.Damage{Amount: gameast.NumberOrRef{IsInt: true, Int: 1}}
	perm := makeArtifactPerm(gs, 0, "Shock Rod", dmgEff, false)

	supp := StaxCheck(gs, 0, perm, 0)
	if !supp.Suppressed {
		t.Fatal("split-second should suppress non-mana activations")
	}
	if supp.Reason != "split_second" {
		t.Fatalf("wrong reason: %s", supp.Reason)
	}
}

func TestStaxCheck_SplitSecond_AllowsManaAbility(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	ssCard := &Card{
		Name:  "Krosan Grip",
		Owner: 1,
		Types: []string{"instant"},
		AST: &gameast.CardAST{
			Name: "Krosan Grip",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "split_second"},
				&gameast.Activated{Effect: &gameast.Destroy{Target: gameast.Filter{Base: "artifact"}}},
			},
		},
	}
	gs.Stack = append(gs.Stack, &StackItem{
		Controller: 1,
		Card:       ssCard,
	})

	perm := makeArtifactManaAbilityPerm(gs, 0, "Sol Ring")

	supp := StaxCheck(gs, 0, perm, 0)
	if supp.Suppressed {
		t.Fatal("split-second should allow mana abilities")
	}
}

// ---------------------------------------------------------------------------
// 3b: Drannith Magistrate blocks non-hand zone casts
// ---------------------------------------------------------------------------

func TestDrannithMagistrate_BlocksZoneCast(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	// Seat 1 has Drannith Magistrate.
	gs.Flags["drannith_active_seat_1"] = 1

	card := &Card{
		Name:  "Dark Ritual",
		Owner: 0,
		Types: []string{"instant", "cost:1"},
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)
	gs.Seats[0].ManaPool = 5

	perm := &ZoneCastPermission{
		Zone:              ZoneGraveyard,
		Keyword:           "flashback",
		ManaCost:          1,
		RequireController: 0,
	}

	_, err := CastFromZone(gs, 0, card, ZoneGraveyard, perm, nil)
	if err == nil {
		t.Fatal("expected Drannith Magistrate to block graveyard cast")
	}
	ce, ok := err.(*CastError)
	if !ok || ce.Reason != "drannith_magistrate" {
		t.Fatalf("wrong error: %v", err)
	}

	if !hasEventOfKind(gs, "cast_suppressed") {
		t.Fatal("expected cast_suppressed event")
	}
}

func TestDrannithMagistrate_DoesNotBlockOwnController(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	// Seat 0 has Drannith Magistrate — does NOT restrict own zone casts.
	gs.Flags["drannith_active_seat_0"] = 1

	card := &Card{
		Name:  "Dark Ritual",
		Owner: 0,
		Types: []string{"instant", "cost:1"},
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)
	gs.Seats[0].ManaPool = 5

	perm := &ZoneCastPermission{
		Zone:              ZoneGraveyard,
		Keyword:           "flashback",
		ManaCost:          1,
		RequireController: 0,
		ExileOnResolve:    true,
	}

	// Seat 0's own Drannith should not block seat 0's flashback.
	// Note: CastFromZone may still fail for other reasons (card not in zone etc.)
	// but should NOT fail due to drannith_magistrate.
	_, err := CastFromZone(gs, 0, card, ZoneGraveyard, perm, nil)
	if err != nil {
		ce, ok := err.(*CastError)
		if ok && ce.Reason == "drannith_magistrate" {
			t.Fatal("Drannith should not restrict its own controller's zone casts")
		}
		// Other errors are acceptable (card already removed from graveyard, etc.)
	}
}

// ---------------------------------------------------------------------------
// 3b: Opposition Agent redirects tutor results
// ---------------------------------------------------------------------------

func TestOppositionAgent_RedirectsTutor(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20

	// Seat 1 has Opposition Agent.
	gs.Flags["opposition_agent_seat_1"] = 1

	// Seat 0 has a library with a target card.
	target := &Card{Name: "Demonic Tutor Target", Owner: 0}
	filler := &Card{Name: "Filler", Owner: 0}
	gs.Seats[0].Library = []*Card{target, filler}

	// Resolve a tutor effect controlled by seat 0.
	src := &Permanent{
		Card:       &Card{Name: "Demonic Tutor", Owner: 0},
		Controller: 0,
		Flags:      map[string]int{},
	}
	tutorEffect := &gameast.Tutor{
		Count:       gameast.NumberOrRef{IsInt: true, Int: 1},
		Query:       gameast.Filter{Base: "card"},
		Destination: "hand",
	}

	resolveTutor(gs, src, tutorEffect)

	// The target card should be in seat 1's exile (Opposition Agent's
	// controller), NOT in seat 0's hand.
	found := false
	for _, c := range gs.Seats[1].Exile {
		if c.Name == "Demonic Tutor Target" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Opposition Agent should redirect tutored card to controller's exile")
	}

	// Verify it's NOT in seat 0's hand.
	for _, c := range gs.Seats[0].Hand {
		if c.Name == "Demonic Tutor Target" {
			t.Fatal("tutored card should NOT be in searching player's hand")
		}
	}

	if !hasEventOfKind(gs, "opposition_agent_exile") {
		t.Fatal("expected opposition_agent_exile event")
	}
}

// ---------------------------------------------------------------------------
// IsManaAbility detection
// ---------------------------------------------------------------------------

func TestIsManaAbility_AddMana(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	perm := makeArtifactManaAbilityPerm(gs, 0, "Sol Ring")
	if !IsManaAbility(perm, 0) {
		t.Fatal("AddMana ability should be classified as mana ability")
	}
}

func TestIsManaAbility_DamageIsNot(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	dmgEff := &gameast.Damage{Amount: gameast.NumberOrRef{IsInt: true, Int: 1}}
	perm := makeArtifactPerm(gs, 0, "Shock Rod", dmgEff, true)
	if IsManaAbility(perm, 0) {
		t.Fatal("Damage ability should NOT be classified as mana ability")
	}
}

func TestIsManaAbility_TargetedManaNotMana(t *testing.T) {
	// A targeted AddMana is NOT a mana ability per CR §605.1a.
	gs := NewGameState(2, nil, nil)
	card := &Card{
		Name:  "Weird Mana",
		Owner: 0,
		Types: []string{"artifact"},
		AST: &gameast.CardAST{
			Name: "Weird Mana",
			Abilities: []gameast.Ability{
				&gameast.Activated{
					Cost: gameast.Cost{Tap: true},
					Effect: &gameast.Sequence{
						Items: []gameast.Effect{
							&gameast.Damage{
								Amount: gameast.NumberOrRef{IsInt: true, Int: 1},
								Target: gameast.Filter{Base: "creature", Targeted: true},
							},
							&gameast.AddMana{AnyColorCount: 1},
						},
					},
				},
			},
		},
	}
	perm := &Permanent{
		Card:       card,
		Controller: 0,
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	if IsManaAbility(perm, 0) {
		t.Fatal("targeted mana-producing ability should NOT be a mana ability")
	}
}
