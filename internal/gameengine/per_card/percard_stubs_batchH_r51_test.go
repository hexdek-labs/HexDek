package per_card

import (
	"fmt"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// dev/percard-stubs-batchH-r51 — 10 fresh stub ports.
// One regression test per port.
// ---------------------------------------------------------------------------

// 1. Mabel, Heir to Cragflame — auto-attaches Cragflame to best
//    friendly Mouse and stamps vigilance/trample/haste.
func TestStubsBatchH_Mabel_AutoAttachesCragflameToMouse(t *testing.T) {
	gs := newGame(t, 2)
	mouse := addPerm(gs, 0, "Battle Mouse", "creature", "mouse")
	mouse.Card.BasePower = 3
	mouse.Card.BaseToughness = 3
	mabel := addPerm(gs, 0, "Mabel, Heir to Cragflame", "creature", "legendary", "mouse")

	gameengine.InvokeETBHook(gs, mabel)

	// Cragflame should be on the battlefield, attached to mouse.
	var cragflame *gameengine.Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.DisplayName() == "Cragflame" {
			cragflame = p
		}
	}
	if cragflame == nil {
		t.Fatalf("expected Cragflame token on battlefield")
	}
	if cragflame.AttachedTo != mouse {
		t.Errorf("expected Cragflame attached to the Battle Mouse, got %v", cragflame.AttachedTo)
	}
	if mouse.Flags["kw:vigilance"] != 1 || mouse.Flags["kw:trample"] != 1 || mouse.Flags["kw:haste"] != 1 {
		t.Errorf("expected mouse to gain vigilance/trample/haste, got %v", mouse.Flags)
	}
}

// 2. Raphael, Ninja Destroyer — ETB stamps must_be_blocked.
func TestStubsBatchH_RaphaelNinjaDestroyer_ETBStampsMustBeBlocked(t *testing.T) {
	gs := newGame(t, 2)
	raphael := addPerm(gs, 0, "Raphael, Ninja Destroyer", "creature", "legendary")
	gameengine.InvokeETBHook(gs, raphael)
	if raphael.Flags["must_be_blocked"] != 1 {
		t.Errorf("expected must_be_blocked flag set, got %v", raphael.Flags)
	}
}

// 3. The Twelfth Doctor — demonstrate seat flag armed at ETB; consumed
//    when controller casts a spell from a non-hand zone.
func TestStubsBatchH_TwelfthDoctor_DemonstrateGrantArmedAndConsumed(t *testing.T) {
	gs := newGame(t, 2)
	doctor := addPerm(gs, 0, "The Twelfth Doctor", "creature", "legendary")
	doctor.Card.BasePower = 5
	doctor.Card.BaseToughness = 5
	gameengine.InvokeETBHook(gs, doctor)
	if gs.Seats[0].Flags["twelfth_doctor_demonstrate_pending"] != 1 {
		t.Errorf("expected demonstrate grant armed at ETB, got %v", gs.Seats[0].Flags)
	}

	// Cast from hand should NOT consume the grant.
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"cast_zone":   "hand",
		"card":        &gameengine.Card{Name: "Lightning Bolt", Types: []string{"instant"}},
	})
	if gs.Seats[0].Flags["twelfth_doctor_demonstrate_pending"] != 1 {
		t.Errorf("expected demonstrate grant still pending after hand cast, got %v", gs.Seats[0].Flags)
	}

	// Cast from graveyard (flashback/etc.) consumes the grant.
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"cast_zone":   "graveyard",
		"card":        &gameengine.Card{Name: "Cathartic Reunion", Types: []string{"sorcery"}},
	})
	if gs.Seats[0].Flags["twelfth_doctor_demonstrate_pending"] != 0 {
		t.Errorf("expected demonstrate grant consumed on non-hand cast, got %v", gs.Seats[0].Flags)
	}
}

// 4. Storm, Force of Nature — consume storm_grant_pending on next I/S cast.
func TestStubsBatchH_StormForceOfNature_ConsumeGrantOnInstantOrSorcery(t *testing.T) {
	gs := newGame(t, 2)
	storm := addPerm(gs, 0, "Storm, Force of Nature", "creature", "legendary")
	storm.Card.BasePower = 5
	storm.Card.BaseToughness = 5
	gs.Seats[0].Flags = map[string]int{"storm_grant_pending": 1}
	_ = storm

	// Casting a creature spell does NOT consume the grant.
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        &gameengine.Card{Name: "Some Bear", Types: []string{"creature"}},
	})
	if gs.Seats[0].Flags["storm_grant_pending"] != 1 {
		t.Errorf("expected grant still pending after creature cast, got %v", gs.Seats[0].Flags)
	}

	// Casting an instant consumes it.
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        &gameengine.Card{Name: "Lightning Bolt", Types: []string{"instant"}},
	})
	if gs.Seats[0].Flags["storm_grant_pending"] != 0 {
		t.Errorf("expected grant consumed after instant cast, got %v", gs.Seats[0].Flags)
	}
}

// 5. Cloud, Midgar Mercenary — registers a would_fire_etb_trigger
//    replacement that doubles Cloud's ETB triggers (and his attached
//    Equipment's triggers) while Cloud is equipped.
func TestStubsBatchH_Cloud_EquippedTriggerDoubler(t *testing.T) {
	gs := newGame(t, 2)
	cloud := addPerm(gs, 0, "Cloud, Midgar Mercenary", "creature", "legendary")
	gameengine.InvokeETBHook(gs, cloud)

	// Initially: nothing equipped → replacement should NOT fire.
	got, _ := gameengine.FireETBTriggerEvent(gs, cloud)
	if got != 1 {
		t.Errorf("expected count 1 when Cloud not equipped, got %d", got)
	}

	// Attach a creator Equipment to Cloud.
	equip := addPerm(gs, 0, "Buster Sword", "artifact", "equipment")
	equip.AttachedTo = cloud
	got, _ = gameengine.FireETBTriggerEvent(gs, cloud)
	if got != 2 {
		t.Errorf("expected ETB trigger doubled when equipped, got %d", got)
	}
}

// 6. Lightning, Army of One — staggered seat takes double damage via
//    the registered would_be_dealt_damage replacement.
func TestStubsBatchH_Lightning_StaggerDoubleDamage(t *testing.T) {
	gs := newGame(t, 2)
	lightning := addPerm(gs, 0, "Lightning, Army of One", "creature", "legendary")
	lightning.Card.BasePower = 5
	lightning.Card.BaseToughness = 5
	gameengine.InvokeETBHook(gs, lightning)

	// Arm stagger on seat 1 by firing the combat-damage-to-player
	// trigger directly.
	gameengine.FireCardTrigger(gs, "combat_damage_to_player", map[string]interface{}{
		"attacker_perm": lightning,
		"defender":      1,
	})
	expiresKey := fmt.Sprintf("lightning_stagger_seat%d_until_turn", 1)
	if gs.Flags[expiresKey] == 0 {
		t.Fatalf("expected stagger arm key set, got %v", gs.Flags)
	}

	// Fire damage event at seat 1 — should be doubled.
	got, _ := gameengine.FireDamageEvent(gs, lightning, 1, nil, 3)
	if got != 6 {
		t.Errorf("expected damage doubled to 6 (was 3), got %d", got)
	}

	// Damage at seat 0 (controller) — should NOT be doubled.
	got, _ = gameengine.FireDamageEvent(gs, lightning, 0, nil, 4)
	if got != 4 {
		t.Errorf("expected damage at non-staggered seat unchanged (4), got %d", got)
	}
}

// 7. The Master of Keys — ETB stamps the enchantment-escape grant seat
//    flag.
func TestStubsBatchH_TheMasterOfKeys_EscapeGrantFlag(t *testing.T) {
	gs := newGame(t, 2)
	master := addPerm(gs, 0, "The Master of Keys", "creature", "legendary")
	gameengine.InvokeETBHook(gs, master)
	if gs.Seats[0].Flags["master_of_keys_enchantment_escape_grant"] != 1 {
		t.Errorf("expected escape-grant seat flag, got %v", gs.Seats[0].Flags)
	}
}

// 8. Aminatou, Veil Piercer — enchantment cost reduced by {4} (miracle
//    approximation) when Aminatou is on the controller's battlefield.
func TestStubsBatchH_Aminatou_EnchantmentMiracleCostReduction(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	aminatou := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Aminatou, Veil Piercer", Types: []string{"creature", "legendary"}},
		Controller: 0,
		Owner:      0,
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, aminatou)
	enchant := &gameengine.Card{Name: "Sphinx's Decree", Types: []string{"enchantment", "cost:7"}}
	cost := gameengine.CalculateTotalCost(gs, enchant, 0)
	if cost != 3 {
		t.Errorf("expected cost 3 (7 - 4 miracle), got %d", cost)
	}
	// Creature spell unaffected.
	bear := &gameengine.Card{Name: "Grizzly Bears", Types: []string{"creature", "cost:2"}}
	cost = gameengine.CalculateTotalCost(gs, bear, 0)
	if cost != 2 {
		t.Errorf("expected creature cost 2 (miracle doesn't apply), got %d", cost)
	}
}

// 9. Jasmine Boreal — {T}: produces G+W into the pool and stamps the
//    no-ability-creature restriction tag.
func TestStubsBatchH_JasmineBoreal_ActivatedTapAddsTwoMana(t *testing.T) {
	gs := newGame(t, 2)
	jasmine := addPerm(gs, 0, "Jasmine Boreal of the Seven", "creature", "legendary")
	before := gs.Seats[0].ManaPool
	gameengine.InvokeActivatedHook(gs, jasmine, 0, nil)
	if !jasmine.Tapped {
		t.Errorf("expected Jasmine tapped after activation")
	}
	if gs.Seats[0].ManaPool != before+2 {
		t.Errorf("expected mana pool +2 (was %d, now %d)", before, gs.Seats[0].ManaPool)
	}
	if gs.Seats[0].Flags["jasmine_no_ability_creature_mana"] == 0 {
		t.Errorf("expected restriction flag set, got %v", gs.Seats[0].Flags)
	}
}

// 10. Commander Mustard — while soldier_attack_ping is armed, soldier
//     attackers deal 1 damage to defending player.
func TestStubsBatchH_CommanderMustard_SoldierAttackPings(t *testing.T) {
	gs := newGame(t, 2)
	mustard := addPerm(gs, 0, "Commander Mustard", "creature", "legendary")
	soldier := addPerm(gs, 0, "Steel Soldier", "creature", "soldier")
	gs.Seats[0].Flags = map[string]int{"mustard_soldier_attack_ping": 1}
	gs.Seats[1].Life = 40
	_ = mustard

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": soldier,
		"defender_seat": 1,
	})

	if gs.Seats[1].Life != 39 {
		t.Errorf("expected defending seat life 39 (40 - 1), got %d", gs.Seats[1].Life)
	}

	// Non-soldier attacker should NOT ping.
	bear := addPerm(gs, 0, "Grizzly Bear", "creature", "bear")
	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": bear,
		"defender_seat": 1,
	})
	if gs.Seats[1].Life != 39 {
		t.Errorf("expected non-Soldier attacker to NOT ping, life=%d", gs.Seats[1].Life)
	}
}
