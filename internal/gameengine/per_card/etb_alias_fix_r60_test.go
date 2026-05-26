package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// ETB alias r60 fix — Aura Shards / Field of the Dead / Sokka and Suki /
// Selvala, Heart of the Wilds. Pre-fix these handlers were registered under
// friendly aliases ("creature_enters_battlefield" / "land_entered_battlefield"
// / "permanent_entered_battlefield") that normalized to "etb" while the engine
// fired "permanent_etb" via FireCardTrigger — so the handlers never fired in
// real games. Audit: docs/audit-percard-registry-r60.md §1.
// -----------------------------------------------------------------------------

// Aura Shards — "Whenever a creature enters under your control, you may
// destroy target artifact or enchantment."
func TestAuraShards_FiresOnFriendlyCreatureETB(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Aura Shards", "enchantment")
	target := addPerm(gs, 1, "Sol Ring", "artifact")

	newcomer := addPerm(gs, 0, "Llanowar Elves", "creature")
	newcomer.Card.BasePower, newcomer.Card.BaseToughness = 1, 1
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            newcomer,
		"controller_seat": 0,
		"card":            newcomer.Card,
	})

	// Sol Ring should have been sacrificed.
	for _, p := range gs.Seats[1].Battlefield {
		if p == target {
			t.Fatalf("Aura Shards should have destroyed opp's Sol Ring; battlefield still has it")
		}
	}
	if hasEvent(gs, "per_card_handler") < 1 {
		t.Errorf("expected per_card_handler breadcrumb; events=%+v", gs.EventLog)
	}
}

// Aura Shards must NOT fire on non-creature ETBs (oracle says "a creature").
func TestAuraShards_IgnoresNoncreatureETB(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Aura Shards", "enchantment")
	target := addPerm(gs, 1, "Sol Ring", "artifact")

	// Friendly artifact enters — should NOT trigger Aura Shards.
	artifact := addPerm(gs, 0, "Talisman", "artifact")
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            artifact,
		"controller_seat": 0,
		"card":            artifact.Card,
	})

	found := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == target {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Sol Ring should still be on opp's battlefield (non-creature ETB must not fire Aura Shards)")
	}
}

// Aura Shards must NOT fire on opponent's creature ETB (oracle: "under your
// control").
func TestAuraShards_IgnoresOpponentCreatureETB(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Aura Shards", "enchantment")
	target := addPerm(gs, 1, "Sol Ring", "artifact")

	oppCreature := addPerm(gs, 1, "Grizzly Bears", "creature")
	oppCreature.Card.BasePower, oppCreature.Card.BaseToughness = 2, 2
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            oppCreature,
		"controller_seat": 1,
		"card":            oppCreature.Card,
	})

	for _, p := range gs.Seats[1].Battlefield {
		if p == target {
			return // expected
		}
	}
	t.Errorf("Sol Ring should still be on opp's battlefield (opp-controlled creature ETB must not fire Aura Shards)")
}

// Aura Shards must fire EXACTLY ONCE per ETB (pre-fix double-registration on
// two aliases that both normalized to the same key would have fired twice).
func TestAuraShards_FiresExactlyOncePerETB(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Aura Shards", "enchantment")
	addPerm(gs, 1, "Sol Ring", "artifact")
	addPerm(gs, 1, "Talisman", "artifact")

	newcomer := addPerm(gs, 0, "Llanowar Elves", "creature")
	newcomer.Card.BasePower, newcomer.Card.BaseToughness = 1, 1
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            newcomer,
		"controller_seat": 0,
		"card":            newcomer.Card,
	})

	// Count how many opp artifacts remain. Pre-fix the double-registration
	// would have destroyed two; with single registration only one is gone.
	oppArtifacts := 0
	for _, p := range gs.Seats[1].Battlefield {
		if p != nil && p.Card != nil && p.IsArtifact() {
			oppArtifacts++
		}
	}
	if oppArtifacts != 1 {
		t.Errorf("expected exactly 1 opp artifact left (Aura Shards fires once per ETB); got %d", oppArtifacts)
	}
}

// Field of the Dead — "Whenever Field of the Dead or another land enters
// under your control, if you control seven or more lands with different
// names, create a 2/2 black Zombie token."
func TestFieldOfTheDead_MintsZombieAtSevenUniqueLands(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Field of the Dead", "land")
	// Six other uniquely-named lands already on battlefield.
	addPerm(gs, 0, "Plains", "land")
	addPerm(gs, 0, "Island", "land")
	addPerm(gs, 0, "Swamp", "land")
	addPerm(gs, 0, "Mountain", "land")
	addPerm(gs, 0, "Forest", "land")
	addPerm(gs, 0, "Wastes", "land")

	// 7th uniquely-named land enters — should mint a Zombie. Battlefield
	// grows by 2: the entering land + the new Zombie token.
	newLand := addPerm(gs, 0, "Strip Mine", "land")
	pre := len(gs.Seats[0].Battlefield)
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            newLand,
		"controller_seat": 0,
		"card":            newLand.Card,
	})

	if len(gs.Seats[0].Battlefield) != pre+1 {
		t.Errorf("expected zombie token minted (battlefield grew by 1 post-ETB); pre=%d post=%d",
			pre, len(gs.Seats[0].Battlefield))
	}
	foundZombie := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Zombie Token" {
			foundZombie = true
			break
		}
	}
	if !foundZombie {
		t.Errorf("expected a Zombie Token on battlefield after 7th unique land ETB")
	}
}

// Field of the Dead must NOT fire on non-land ETBs.
func TestFieldOfTheDead_IgnoresNonLandETB(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Field of the Dead", "land")
	addPerm(gs, 0, "Plains", "land")
	addPerm(gs, 0, "Island", "land")
	addPerm(gs, 0, "Swamp", "land")
	addPerm(gs, 0, "Mountain", "land")
	addPerm(gs, 0, "Forest", "land")
	addPerm(gs, 0, "Wastes", "land")

	creature := addPerm(gs, 0, "Llanowar Elves", "creature")
	creature.Card.BasePower, creature.Card.BaseToughness = 1, 1
	pre := len(gs.Seats[0].Battlefield)
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            creature,
		"controller_seat": 0,
		"card":            creature.Card,
	})

	if len(gs.Seats[0].Battlefield) != pre {
		t.Errorf("creature ETB should NOT mint a zombie; battlefield grew unexpectedly pre=%d post=%d",
			pre, len(gs.Seats[0].Battlefield))
	}
}

// Field of the Dead must NOT fire at fewer than 7 unique lands.
func TestFieldOfTheDead_NoZombieBelowThreshold(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Field of the Dead", "land")
	addPerm(gs, 0, "Plains", "land")
	addPerm(gs, 0, "Island", "land")
	addPerm(gs, 0, "Swamp", "land")
	addPerm(gs, 0, "Mountain", "land")
	// Only 5 unique lands incl. Field — adding a 6th must NOT mint.

	newLand := addPerm(gs, 0, "Forest", "land")
	pre := len(gs.Seats[0].Battlefield)
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            newLand,
		"controller_seat": 0,
		"card":            newLand.Card,
	})

	if len(gs.Seats[0].Battlefield) != pre {
		t.Errorf("below 7-unique threshold; no zombie expected. pre=%d post=%d",
			pre, len(gs.Seats[0].Battlefield))
	}
}

// Selvala, Heart of the Wilds — "Whenever another creature enters, its
// controller may draw a card if its power is greater than each other
// creature's power." Pre-fix the trigger was registered under
// "creature_enters_battlefield" and never fired.
func TestSelvalaHeart_DrawsOnHighestPowerCreatureETB(t *testing.T) {
	gs := newGame(t, 2)
	selvala := addPerm(gs, 0, "Selvala, Heart of the Wilds", "creature", "legendary")
	selvala.Card.BasePower, selvala.Card.BaseToughness = 2, 3
	weak := addPerm(gs, 0, "Llanowar Elves", "creature")
	weak.Card.BasePower, weak.Card.BaseToughness = 1, 1
	addLibrary(gs, 0, "DrawMe")

	beast := addPerm(gs, 0, "Craterhoof", "creature")
	beast.Card.BasePower, beast.Card.BaseToughness = 5, 5

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            beast,
		"controller_seat": 0,
		"card":            beast.Card,
	})

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 card drawn (Selvala fires for highest-power ETB); hand=%d", len(gs.Seats[0].Hand))
	}
}

// Sokka and Suki — "Whenever an Equipment you control enters, create a
// 1/1 white Ally creature token." Pre-fix the ally-attach arm was
// registered under "creature_enters_battlefield" and never fired; this
// pins the equipment-mint arm continues to fire under the new alias.
func TestSokkaAndSuki_MintsAllyOnEquipmentETB(t *testing.T) {
	gs := newGame(t, 2)
	sokka := addPerm(gs, 0, "Sokka and Suki", "creature", "legendary", "ally")
	sokka.Card.BasePower, sokka.Card.BaseToughness = 2, 2

	eq := addPerm(gs, 0, "Skullclamp", "artifact", "equipment")
	pre := len(gs.Seats[0].Battlefield)
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            eq,
		"controller_seat": 0,
		"card":            eq.Card,
	})

	if len(gs.Seats[0].Battlefield) != pre+1 {
		t.Errorf("expected 1 ally token minted post-ETB (post=%d pre=%d)",
			len(gs.Seats[0].Battlefield), pre)
	}
	foundAlly := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Ally" {
			foundAlly = true
			break
		}
	}
	if !foundAlly {
		t.Errorf("expected Ally token on battlefield after Skullclamp ETB")
	}
}
