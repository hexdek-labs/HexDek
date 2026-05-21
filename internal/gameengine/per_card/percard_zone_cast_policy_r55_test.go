package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R55 — ZoneCastPolicy primitive extension (filter-driven multi-card
// alt-cast permissions). Complements the existing per-*Card
// gs.ZoneCastGrants registry with a per-FILTER gs.ZoneCastPolicies
// registry that supports:
//
//   - "Any player may cast..." (CasterScope = "any") — Aluren
//   - "Cast from outside the game" — Karn, the Great Creator wishboard
//   - "Cast from opp's graveyard" (OwnerScope = "opponents") — Tinybones
//   - Free I/S cast at hand ≥ 11 — Cecily, Haunted Mage
//   - Once-per-turn free I/S cast — Zaffai and the Tempests
//
// Each card asserts both policy registration on ETB / activation and
// FindZoneCastPolicy returning the expected match (with caster /
// owner scope correctly gating).

// ---------------------------------------------------------------------------
// Engine primitive smoke
// ---------------------------------------------------------------------------

func TestZoneCastPolicy_RegisterAndUnregister(t *testing.T) {
	gs := newGame(t, 2)
	p := addPerm(gs, 0, "Source", "creature")
	gs.RegisterZoneCastPolicy(&gameengine.ZoneCastPolicy{
		SourcePerm:     p,
		HandlerID:      "test_policy",
		Zone:           gameengine.ZoneHand,
		CasterScope:    "controller",
		ControllerSeat: 0,
		ManaCost:       0,
	})
	if len(gs.ZoneCastPolicies) != 1 {
		t.Fatalf("expected 1 policy registered; got %d", len(gs.ZoneCastPolicies))
	}
	gs.UnregisterZoneCastPoliciesForPermanent(p)
	if len(gs.ZoneCastPolicies) != 0 {
		t.Errorf("expected 0 after unregister; got %d", len(gs.ZoneCastPolicies))
	}
}

func TestZoneCastPolicy_FindByCasterAndOwnerScope(t *testing.T) {
	gs := newGame(t, 3)
	src := addPerm(gs, 0, "Source", "creature")
	gs.RegisterZoneCastPolicy(&gameengine.ZoneCastPolicy{
		SourcePerm:     src,
		HandlerID:      "opp_grave_free",
		Zone:           gameengine.ZoneGraveyard,
		OwnerScope:     "opponents",
		CasterScope:    "controller",
		ControllerSeat: 0,
		ManaCost:       0,
	})
	card := &gameengine.Card{Name: "Bolt", Owner: 1, Types: []string{"instant"}}

	// Caster = 0 (controller), card owner = 1 (opp) — should match.
	if p := gameengine.FindZoneCastPolicy(gs, 0, card, 1, gameengine.ZoneGraveyard); p == nil {
		t.Errorf("controller casting from opp's graveyard should match policy")
	}
	// Caster = 1 (opp), card owner = 1 — controller-scope rejects.
	if p := gameengine.FindZoneCastPolicy(gs, 1, card, 1, gameengine.ZoneGraveyard); p != nil {
		t.Errorf("opp casting should NOT match controller-scope policy")
	}
	// Caster = 0, card owner = 0 (controller's own card) — opponents-
	// scope rejects.
	if p := gameengine.FindZoneCastPolicy(gs, 0, card, 0, gameengine.ZoneGraveyard); p != nil {
		t.Errorf("controller's own card should NOT match opponents-scope policy")
	}
}

func TestZoneCastPolicy_PredicateFilters(t *testing.T) {
	gs := newGame(t, 2)
	src := addPerm(gs, 0, "Source", "creature")
	gs.RegisterZoneCastPolicy(&gameengine.ZoneCastPolicy{
		SourcePerm:     src,
		HandlerID:      "creature_only",
		Zone:           gameengine.ZoneHand,
		CasterScope:    "any",
		ControllerSeat: 0,
		Predicate: func(c *gameengine.Card) bool {
			if c == nil {
				return false
			}
			for _, t := range c.Types {
				if t == "creature" {
					return true
				}
			}
			return false
		},
	})
	creature := &gameengine.Card{Name: "Bear", Owner: 0, Types: []string{"creature"}}
	instant := &gameengine.Card{Name: "Bolt", Owner: 0, Types: []string{"instant"}}

	if p := gameengine.FindZoneCastPolicy(gs, 0, creature, 0, gameengine.ZoneHand); p == nil {
		t.Errorf("creature card should match predicate-creature policy")
	}
	if p := gameengine.FindZoneCastPolicy(gs, 0, instant, 0, gameengine.ZoneHand); p != nil {
		t.Errorf("non-creature card should NOT match predicate-creature policy")
	}
}

// ---------------------------------------------------------------------------
// Aluren
// ---------------------------------------------------------------------------

func TestAluren_RegistersAnyPlayerCreatureMV3HandPolicy(t *testing.T) {
	gs := newGame(t, 2)
	aluren := addPerm(gs, 0, "Aluren", "enchantment", "legendary")
	alurenETBRegisterPolicy(gs, aluren)
	if len(gs.ZoneCastPolicies) != 1 {
		t.Fatalf("Aluren ETB should register 1 policy; got %d", len(gs.ZoneCastPolicies))
	}
	if gs.Seats[0].Flags["aluren_flash_grant_active"] != 1 {
		t.Errorf("Aluren ETB should stamp flash flag on seat 0")
	}
	if gs.Seats[1].Flags["aluren_flash_grant_active"] != 1 {
		t.Errorf("Aluren ETB should stamp flash flag on seat 1 too (any player)")
	}

	cheap := &gameengine.Card{
		Name:          "Cheap Elf",
		Owner:         1,
		Types:         []string{"creature", "cmc:2"},
		BasePower:     1,
		BaseToughness: 1,
	}
	expensive := &gameengine.Card{
		Name:          "Big Beater",
		Owner:         1,
		Types:         []string{"creature", "cmc:5"},
		BasePower:     5,
		BaseToughness: 5,
	}
	nonCreature := &gameengine.Card{Name: "Bolt", Owner: 1, Types: []string{"instant", "cmc:1"}}

	// Opp (seat 1) casts a cheap creature from their own hand — should
	// match (any-player caster, self owner-scope from caster's POV).
	if p := gameengine.FindZoneCastPolicy(gs, 1, cheap, 1, gameengine.ZoneHand); p == nil {
		t.Errorf("opp should be allowed to cast cheap creature from own hand under Aluren")
	} else if p.ManaCost != 0 {
		t.Errorf("Aluren cast should be free; got ManaCost=%d", p.ManaCost)
	}
	// Expensive creature rejected.
	if p := gameengine.FindZoneCastPolicy(gs, 1, expensive, 1, gameengine.ZoneHand); p != nil {
		t.Errorf("MV-5 creature should NOT match Aluren MV-3 filter")
	}
	// Non-creature rejected.
	if p := gameengine.FindZoneCastPolicy(gs, 1, nonCreature, 1, gameengine.ZoneHand); p != nil {
		t.Errorf("non-creature should NOT match Aluren creature filter")
	}
}

func TestAluren_LTBUnregistersAndClearsFlashFlags(t *testing.T) {
	gs := newGame(t, 2)
	aluren := addPerm(gs, 0, "Aluren", "enchantment", "legendary")
	alurenETBRegisterPolicy(gs, aluren)
	alurenLTBUnregister(gs, aluren, map[string]interface{}{"perm": aluren})
	if len(gs.ZoneCastPolicies) != 0 {
		t.Errorf("LTB should unregister Aluren's policy")
	}
	for i := range gs.Seats {
		if _, has := gs.Seats[i].Flags["aluren_flash_grant_active"]; has {
			t.Errorf("LTB should clear flash flag on seat %d", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Karn, the Great Creator
// ---------------------------------------------------------------------------

func TestKarnGreatCreator_RegistersWishboardPolicy(t *testing.T) {
	gs := newGame(t, 2)
	karn := addPerm(gs, 0, "Karn, the Great Creator", "planeswalker", "legendary")
	karnGreatCreatorETB(gs, karn)
	if len(gs.ZoneCastPolicies) != 1 {
		t.Fatalf("Karn should register 1 wishboard policy; got %d", len(gs.ZoneCastPolicies))
	}
	// Static opp-artifact lockout stamped.
	if gs.Seats[1].Flags["opp_artifact_activation_disabled"] != 1 {
		t.Errorf("Karn should disable opp artifact activations")
	}

	wishArtifact := &gameengine.Card{Name: "Wishboard Artifact", Owner: 0, Types: []string{"artifact"}}
	wishCreature := &gameengine.Card{Name: "Wishboard Creature", Owner: 0, Types: []string{"creature"}}

	if p := gameengine.FindZoneCastPolicy(gs, 0, wishArtifact, 0, "outside_the_game"); p == nil {
		t.Errorf("Karn should allow controller to cast artifact from outside_the_game")
	}
	if p := gameengine.FindZoneCastPolicy(gs, 0, wishCreature, 0, "outside_the_game"); p != nil {
		t.Errorf("Karn should NOT allow non-artifact from outside_the_game")
	}
	if p := gameengine.FindZoneCastPolicy(gs, 1, wishArtifact, 1, "outside_the_game"); p != nil {
		t.Errorf("Karn's wishboard is controller-only; opp shouldn't match")
	}
}

func TestKarnGreatCreator_LTBClearsLockoutAndPolicy(t *testing.T) {
	gs := newGame(t, 2)
	karn := addPerm(gs, 0, "Karn, the Great Creator", "planeswalker", "legendary")
	karnGreatCreatorETB(gs, karn)
	karnGreatCreatorLTB(gs, karn, map[string]interface{}{"perm": karn})
	if len(gs.ZoneCastPolicies) != 0 {
		t.Errorf("LTB should unregister Karn's policy")
	}
	if _, has := gs.Seats[1].Flags["opp_artifact_activation_disabled"]; has {
		t.Errorf("LTB should clear opp artifact lockout")
	}
}

// ---------------------------------------------------------------------------
// Cecily, Haunted Mage
// ---------------------------------------------------------------------------

func TestCecily_FreeISCastPolicyOnAttackWithHand11(t *testing.T) {
	gs := newGame(t, 2)
	cecily := addPerm(gs, 0, "Cecily, Haunted Mage", "creature", "legendary")
	// Stage 11+ cards in the controller's hand so the trigger qualifies.
	for i := 0; i < 12; i++ {
		gs.Seats[0].Hand = append(gs.Seats[0].Hand, &gameengine.Card{
			Name: "F", Owner: 0, Types: []string{"instant"},
		})
	}
	addLibrary(gs, 0, "X")
	cecilyHauntedMageAttack(gs, cecily, map[string]interface{}{
		"attacker_perm": cecily,
	})
	if gs.Seats[0].Flags["cecily_free_is_cast_pending"] != 1 {
		t.Fatal("Cecily attack with hand >= 11 should stamp pending flag")
	}
	if len(gs.ZoneCastPolicies) != 1 {
		t.Fatalf("Cecily should register 1 policy; got %d", len(gs.ZoneCastPolicies))
	}
	// Controller casting an instant from own hand should match.
	inst := &gameengine.Card{Name: "Bolt", Owner: 0, Types: []string{"instant"}}
	if p := gameengine.FindZoneCastPolicy(gs, 0, inst, 0, gameengine.ZoneHand); p == nil {
		t.Errorf("Cecily controller should match free I/S policy")
	}
	// Non-I/S card filtered.
	creature := &gameengine.Card{Name: "Bear", Owner: 0, Types: []string{"creature"}}
	if p := gameengine.FindZoneCastPolicy(gs, 0, creature, 0, gameengine.ZoneHand); p != nil {
		t.Errorf("Cecily policy should filter to I/S only")
	}
}

func TestCecily_NoPolicyWhenHandUnder11(t *testing.T) {
	gs := newGame(t, 2)
	cecily := addPerm(gs, 0, "Cecily, Haunted Mage", "creature", "legendary")
	for i := 0; i < 5; i++ {
		gs.Seats[0].Hand = append(gs.Seats[0].Hand, &gameengine.Card{Name: "F", Owner: 0})
	}
	addLibrary(gs, 0, "X")
	cecilyHauntedMageAttack(gs, cecily, map[string]interface{}{
		"attacker_perm": cecily,
	})
	if len(gs.ZoneCastPolicies) != 0 {
		t.Errorf("Cecily attack with hand < 11 should NOT register policy; got %d", len(gs.ZoneCastPolicies))
	}
}

// ---------------------------------------------------------------------------
// Zaffai and the Tempests
// ---------------------------------------------------------------------------

func TestZaffai_RegistersFreeISCastPolicyOnETB(t *testing.T) {
	gs := newGame(t, 2)
	zaffai := addPerm(gs, 0, "Zaffai and the Tempests", "creature", "legendary")
	zaffaiAndTheTempestsETB(gs, zaffai)
	if len(gs.ZoneCastPolicies) != 1 {
		t.Fatalf("Zaffai should register 1 policy; got %d", len(gs.ZoneCastPolicies))
	}
	inst := &gameengine.Card{Name: "Bolt", Owner: 0, Types: []string{"instant"}}
	if p := gameengine.FindZoneCastPolicy(gs, 0, inst, 0, gameengine.ZoneHand); p == nil {
		t.Errorf("Zaffai controller should match free I/S policy")
	} else if p.ManaCost != 0 {
		t.Errorf("Zaffai cast should be free; got ManaCost=%d", p.ManaCost)
	}
}

func TestZaffai_LTBUnregisters(t *testing.T) {
	gs := newGame(t, 2)
	zaffai := addPerm(gs, 0, "Zaffai and the Tempests", "creature", "legendary")
	zaffaiAndTheTempestsETB(gs, zaffai)
	zaffaiLTBClearFlags(gs, zaffai, map[string]interface{}{"perm": zaffai})
	if len(gs.ZoneCastPolicies) != 0 {
		t.Errorf("Zaffai LTB should unregister policy; got %d", len(gs.ZoneCastPolicies))
	}
}

// ---------------------------------------------------------------------------
// Tinybones, the Pickpocket
// ---------------------------------------------------------------------------

func TestTinybones_RegistersOppGravePolicyOnCombatHit(t *testing.T) {
	gs := newGame(t, 2)
	tb := addPerm(gs, 0, "Tinybones, the Pickpocket", "creature", "legendary")
	// Opponent's graveyard contains nonland permanent cards (Tinybones'
	// picker requires creature/artifact/enchantment/PW/battle).
	pick := &gameengine.Card{Name: "Pickable Idol", Owner: 1, Types: []string{"artifact", "cmc:2"}}
	bigger := &gameengine.Card{Name: "Pickable Engine", Owner: 1, Types: []string{"artifact", "cmc:7"}}
	gs.Seats[1].Graveyard = []*gameengine.Card{pick, bigger}

	tinybonesPickpocketDamage(gs, tb, map[string]interface{}{
		"attacker_perm": tb,
		"defender_seat": 1,
	})

	if len(gs.ZoneCastPolicies) != 1 {
		t.Fatalf("Tinybones combat hit should register 1 policy; got %d", len(gs.ZoneCastPolicies))
	}
	// Tinybones picks the highest-MV non-land — bigger (cmc:7).
	if p := gameengine.FindZoneCastPolicy(gs, 0, bigger, 1, gameengine.ZoneGraveyard); p == nil {
		t.Errorf("Tinybones policy should allow controller to cast picked opp-grave card")
	} else if !p.SpendAnyColor {
		t.Errorf("Tinybones policy should allow any-color mana spend")
	}
	// The smaller card NOT in the picked-pointer predicate should be
	// rejected (predicate matches specific pointer only).
	if p := gameengine.FindZoneCastPolicy(gs, 0, pick, 1, gameengine.ZoneGraveyard); p != nil {
		t.Errorf("non-picked card should NOT match Tinybones policy")
	}
}

func TestTinybones_OwnGraveCardRejected(t *testing.T) {
	gs := newGame(t, 2)
	tb := addPerm(gs, 0, "Tinybones, the Pickpocket", "creature", "legendary")
	pick := &gameengine.Card{Name: "Opp Idol", Owner: 1, Types: []string{"artifact", "cmc:1"}}
	gs.Seats[1].Graveyard = []*gameengine.Card{pick}
	tinybonesPickpocketDamage(gs, tb, map[string]interface{}{
		"attacker_perm": tb,
		"defender_seat": 1,
	})
	// Fake an own-grave card with the SAME pointer (impossible in
	// practice, but the OwnerScope filter should also reject it).
	if p := gameengine.FindZoneCastPolicy(gs, 0, pick, 0, gameengine.ZoneGraveyard); p != nil {
		t.Errorf("Tinybones policy should reject own-graveyard cards via owner-scope")
	}
}

// ---------------------------------------------------------------------------
// PolicyToPermission bridge
// ---------------------------------------------------------------------------

func TestPolicyToPermission_BridgeToOldRegistry(t *testing.T) {
	gs := newGame(t, 2)
	src := addPerm(gs, 0, "Source", "creature")
	policy := &gameengine.ZoneCastPolicy{
		SourcePerm:            src,
		HandlerID:             "test_bridge",
		Zone:                  gameengine.ZoneHand,
		CasterScope:           "controller",
		ControllerSeat:        0,
		ManaCost:              0,
		SpendAnyColor:         true,
		LifeCostInsteadOfMana: 2,
		ExileOnResolve:        true,
	}
	gs.RegisterZoneCastPolicy(policy)
	bridged := gameengine.PolicyToPermission(policy, 0)
	if bridged == nil {
		t.Fatal("PolicyToPermission should return a permission")
	}
	if bridged.ManaCost != 0 || !bridged.SpendAnyColor || bridged.LifeCostInsteadOfMana != 2 || !bridged.ExileOnResolve {
		t.Errorf("bridge should preserve cost fields; got %+v", bridged)
	}
	if bridged.RequireController != 0 {
		t.Errorf("bridge should set RequireController=0 for controller scope; got %d", bridged.RequireController)
	}
}
