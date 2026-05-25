package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// tla_flashback_grants_r60_test.go — regression suite for the R60
// graveyard-flashback grant sweep.
//
// Cards covered:
//   - Lier, Disciple of the Drowned (continuous, always-on)
//   - A-Lier, Disciple of the Drowned (continuous, OnlyActiveTurn)
//   - Return the Past (continuous, OnlyActiveTurn, enchantment)
//   - Backdraft Hellkite (attack-triggered EOT mass grant)
//
// All four use the GraveyardFlashbackGrant primitive landed in r59.
// Shared test helpers (newGame, addPerm, newGraveyardCard) live in
// per_card_test.go and iroh_grand_lotus_test.go.

// -----------------------------------------------------------------------------
// Lier, Disciple of the Drowned
// -----------------------------------------------------------------------------

func TestLier_RegistersAlwaysOnGraveyardFlashbackGrantOnETB(t *testing.T) {
	gs := newGame(t, 2)
	lier := addPerm(gs, 0, "Lier, Disciple of the Drowned", "creature", "legendary")

	gameengine.InvokeETBHook(gs, lier)

	if len(gs.GraveyardFlashbackGrants) != 1 {
		t.Fatalf("expected 1 grant after Lier ETB, got %d", len(gs.GraveyardFlashbackGrants))
	}
	g := gs.GraveyardFlashbackGrants[0]
	if g.Controller != 0 {
		t.Errorf("grant Controller = %d, want 0", g.Controller)
	}
	if g.OnlyActiveTurn {
		t.Errorf("Lier grant must NOT be active-turn-gated (always on)")
	}
	if g.SourceTimestamp != lier.Timestamp {
		t.Errorf("grant SourceTimestamp = %d, want lier.Timestamp %d", g.SourceTimestamp, lier.Timestamp)
	}
}

func TestLier_GrantAppliesOnOpponentTurn(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 1 // opponent's turn
	gs.Seats[0].ManaPool = 5

	lier := addPerm(gs, 0, "Lier, Disciple of the Drowned", "creature", "legendary")
	gameengine.InvokeETBHook(gs, lier)

	gy := newGraveyardCard("Counterspell", 0, 2)
	gy.Types = []string{"instant"}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	cost, ok := gameengine.EffectiveFlashbackCostFromGraveyardGrants(gs, 0, gy)
	if !ok {
		t.Fatal("Lier grant should apply on opponent's turn (always-on)")
	}
	if cost != 2 {
		t.Errorf("expected flashback cost = card CMC (2), got %d", cost)
	}
}

func TestLier_LTBRemovesGrant(t *testing.T) {
	gs := newGame(t, 2)
	lier := addPerm(gs, 0, "Lier, Disciple of the Drowned", "creature", "legendary")
	gameengine.InvokeETBHook(gs, lier)

	if len(gs.GraveyardFlashbackGrants) != 1 {
		t.Fatalf("setup: expected 1 grant, got %d", len(gs.GraveyardFlashbackGrants))
	}

	gameengine.FireCardTrigger(gs, "permanent_ltb", map[string]interface{}{"perm": lier})

	if len(gs.GraveyardFlashbackGrants) != 0 {
		t.Errorf("expected grant removed by LTB, got %d", len(gs.GraveyardFlashbackGrants))
	}
}

// -----------------------------------------------------------------------------
// A-Lier, Disciple of the Drowned
// -----------------------------------------------------------------------------

func TestALier_RegistersActiveTurnGraveyardFlashbackGrantOnETB(t *testing.T) {
	gs := newGame(t, 2)
	aLier := addPerm(gs, 0, "A-Lier, Disciple of the Drowned", "creature", "legendary")

	gameengine.InvokeETBHook(gs, aLier)

	if len(gs.GraveyardFlashbackGrants) != 1 {
		t.Fatalf("expected 1 grant after A-Lier ETB, got %d", len(gs.GraveyardFlashbackGrants))
	}
	g := gs.GraveyardFlashbackGrants[0]
	if !g.OnlyActiveTurn {
		t.Errorf("A-Lier grant must be active-turn-gated")
	}
	if g.SourceTimestamp != aLier.Timestamp {
		t.Errorf("grant SourceTimestamp mismatch")
	}
}

func TestALier_GrantInactiveOnOpponentTurn(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 1 // opponent's turn

	aLier := addPerm(gs, 0, "A-Lier, Disciple of the Drowned", "creature", "legendary")
	gameengine.InvokeETBHook(gs, aLier)

	gy := newGraveyardCard("Lightning Bolt", 0, 1)
	gy.Types = []string{"instant"}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	if _, ok := gameengine.EffectiveFlashbackCostFromGraveyardGrants(gs, 0, gy); ok {
		t.Fatal("A-Lier grant should be inactive on opponent's turn")
	}
}

func TestALier_GrantActiveOnOwnTurn(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0

	aLier := addPerm(gs, 0, "A-Lier, Disciple of the Drowned", "creature", "legendary")
	gameengine.InvokeETBHook(gs, aLier)

	gy := newGraveyardCard("Lightning Bolt", 0, 1)
	gy.Types = []string{"instant"}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	cost, ok := gameengine.EffectiveFlashbackCostFromGraveyardGrants(gs, 0, gy)
	if !ok {
		t.Fatal("A-Lier grant should be active on controller's turn")
	}
	if cost != 1 {
		t.Errorf("expected flashback cost = CMC (1), got %d", cost)
	}
}

// -----------------------------------------------------------------------------
// Return the Past
// -----------------------------------------------------------------------------

func TestReturnThePast_RegistersActiveTurnGrantOnETB(t *testing.T) {
	gs := newGame(t, 2)
	rtp := addPerm(gs, 0, "Return the Past", "enchantment")

	gameengine.InvokeETBHook(gs, rtp)

	if len(gs.GraveyardFlashbackGrants) != 1 {
		t.Fatalf("expected 1 grant after Return the Past ETB, got %d", len(gs.GraveyardFlashbackGrants))
	}
	g := gs.GraveyardFlashbackGrants[0]
	if !g.OnlyActiveTurn {
		t.Errorf("Return the Past grant must be active-turn-gated")
	}
	if g.Controller != 0 {
		t.Errorf("grant Controller = %d, want 0", g.Controller)
	}
}

func TestReturnThePast_GrantsFlashbackToOwnGraveyardOnOwnTurn(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	gs.Seats[0].ManaPool = 6

	rtp := addPerm(gs, 0, "Return the Past", "enchantment")
	gameengine.InvokeETBHook(gs, rtp)

	gy := newGraveyardCard("Mind's Desire", 0, 6)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	cost, ok := gameengine.EffectiveFlashbackCostFromGraveyardGrants(gs, 0, gy)
	if !ok {
		t.Fatal("expected Return the Past grant to apply on own turn")
	}
	if cost != 6 {
		t.Errorf("expected flashback cost = CMC (6), got %d", cost)
	}

	if _, err := gameengine.CastFlashback(gs, 0, gy, -1); err != nil {
		t.Fatalf("CastFlashback via Return the Past grant failed: %v", err)
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected 6 - 6 = 0 mana left, got %d", gs.Seats[0].ManaPool)
	}
}

func TestReturnThePast_LTBRemovesGrant(t *testing.T) {
	gs := newGame(t, 2)
	rtp := addPerm(gs, 0, "Return the Past", "enchantment")
	gameengine.InvokeETBHook(gs, rtp)

	if len(gs.GraveyardFlashbackGrants) != 1 {
		t.Fatalf("setup: expected 1 grant, got %d", len(gs.GraveyardFlashbackGrants))
	}

	gameengine.FireCardTrigger(gs, "permanent_ltb", map[string]interface{}{"perm": rtp})

	if len(gs.GraveyardFlashbackGrants) != 0 {
		t.Errorf("expected grant removed on LTB, got %d", len(gs.GraveyardFlashbackGrants))
	}
}

// -----------------------------------------------------------------------------
// Backdraft Hellkite
// -----------------------------------------------------------------------------

func TestBackdraftHellkite_AttackRegistersEOTGrant(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0

	hellkite := addPerm(gs, 0, "Backdraft Hellkite", "creature", "dragon")

	gameengine.FireCardTrigger(gs, "attacks", map[string]interface{}{
		"attacker_perm": hellkite,
		"attacker_seat": hellkite.Controller,
		"attacker_card": hellkite.Card,
	})

	if len(gs.GraveyardFlashbackGrants) != 1 {
		t.Fatalf("expected 1 EOT grant after Hellkite attack, got %d", len(gs.GraveyardFlashbackGrants))
	}
	g := gs.GraveyardFlashbackGrants[0]
	if !g.ExpiresAtCleanup {
		t.Errorf("Hellkite grant should expire at cleanup (EOT)")
	}
	if g.SourceTimestamp != 0 {
		t.Errorf("Hellkite EOT grant should have SourceTimestamp=0 (no permanent tie), got %d", g.SourceTimestamp)
	}
	if g.OnlyActiveTurn {
		t.Errorf("EOT grant should not be active-turn-gated (lasts till end of THIS turn anyway)")
	}
	if g.Controller != 0 {
		t.Errorf("grant Controller = %d, want 0", g.Controller)
	}
}

func TestBackdraftHellkite_DoesNotFireWhenOtherCreatureAttacks(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0

	hellkite := addPerm(gs, 0, "Backdraft Hellkite", "creature", "dragon")
	_ = hellkite
	other := addPerm(gs, 0, "Grizzly Bears", "creature")

	// Some other creature attacks. The "attacks" trigger fires the
	// handler for every Backdraft Hellkite on the battlefield, but
	// the handler must filter ctx["attacker_perm"] != perm.
	gameengine.FireCardTrigger(gs, "attacks", map[string]interface{}{
		"attacker_perm": other,
		"attacker_seat": other.Controller,
		"attacker_card": other.Card,
	})

	if len(gs.GraveyardFlashbackGrants) != 0 {
		t.Fatalf("Hellkite should not register a grant when another creature attacks, got %d grants",
			len(gs.GraveyardFlashbackGrants))
	}
}

func TestBackdraftHellkite_GrantedFlashbackCastSucceeds(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	gs.Seats[0].ManaPool = 3

	hellkite := addPerm(gs, 0, "Backdraft Hellkite", "creature", "dragon")

	gy := newGraveyardCard("Faithless Looting", 0, 1)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	gameengine.FireCardTrigger(gs, "attacks", map[string]interface{}{
		"attacker_perm": hellkite,
		"attacker_seat": hellkite.Controller,
		"attacker_card": hellkite.Card,
	})

	cost, ok := gameengine.EffectiveFlashbackCostFromGraveyardGrants(gs, 0, gy)
	if !ok {
		t.Fatal("expected Hellkite-emitted grant to cover graveyard card")
	}
	if cost != 1 {
		t.Errorf("expected flashback cost = CMC (1), got %d", cost)
	}

	if _, err := gameengine.CastFlashback(gs, 0, gy, -1); err != nil {
		t.Fatalf("CastFlashback via Hellkite grant failed: %v", err)
	}
	if gs.Seats[0].ManaPool != 2 {
		t.Errorf("expected 3 - 1 = 2 mana left, got %d", gs.Seats[0].ManaPool)
	}
}

func TestBackdraftHellkite_GrantExpiresAtEOTCleanup(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0

	hellkite := addPerm(gs, 0, "Backdraft Hellkite", "creature", "dragon")

	gameengine.FireCardTrigger(gs, "attacks", map[string]interface{}{
		"attacker_perm": hellkite,
		"attacker_seat": hellkite.Controller,
		"attacker_card": hellkite.Card,
	})

	if len(gs.GraveyardFlashbackGrants) != 1 {
		t.Fatalf("setup: expected 1 grant, got %d", len(gs.GraveyardFlashbackGrants))
	}

	gameengine.ExpireEOTGraveyardFlashbackGrants(gs)

	if len(gs.GraveyardFlashbackGrants) != 0 {
		t.Errorf("expected Hellkite EOT grant flushed on cleanup sweep, got %d",
			len(gs.GraveyardFlashbackGrants))
	}
}
