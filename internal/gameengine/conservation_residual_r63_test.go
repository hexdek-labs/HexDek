package gameengine

// conservation_residual_r63_test.go — CONSERVATION rare-residual class
// (r63), the two stale-ceased fabrication mechanisms caught live at seed
// 5150 (games 780 and 921, ~1 game per 1,000 strict-census fuzz):
//
// GAME 780 — "Final Parting present in graveyard but ceased". Mid-
// CastSpell the card lives only in a handler-local (out of hand, not
// yet on the stack); the cast trigger's immediate resolution killed a
// seat, HandleSeatElimination ran SweepOrphanedInstanceIDs, and the
// sweep ceased the LIVING caster's in-flight card. Its arrival in the
// graveyard then read as fabrication every tick after. Fix: CastSpell
// tracks the card in gs.ResolvingCards (the established mid-flight
// limbo registry, counted as zone presence by both the census and the
// sweep) for the whole cast window.
//
// GAME 921 — "Island present in exile but ceased". A leaver-owned card
// sat in a SURVIVOR's exile (impulse-play pile) with a ZoneCastGrants
// entry. The Phase E purge only removed already-ceased duplicates, so
// it skipped the card; the sideband ZoneCastGrants cleanup that runs
// AFTER the purge then ceased the ID, leaving present-but-ceased in a
// walked zone. Fix: Phase E ceases + purges leaver-owned cards in
// surviving seats' zones unconditionally (§800.4a — owned objects
// leave the game wherever they sit).

import (
	"testing"
)

// Game-780 shape, reproduced at the real seam: a cast trigger fires
// inside CastSpell's transit window (card out of hand, not yet on the
// stack) and runs the orphan sweep — exactly what an elimination
// cascade does. The in-flight card must be census-present and survive
// unceased.
func TestCastSpell_TransitWindow_OrphanSweepDoesNotCeaseInFlightCard(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Phase = "main"

	spell := &Card{Name: "Final Parting", Owner: 0, Types: []string{"sorcery"}}
	MintOGInstanceID(gs, spell)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, spell)

	prevHook := TriggerHook
	defer func() { TriggerHook = prevHook }()
	sweptInWindow := -1
	ceasedInWindow := false
	TriggerHook = func(hgs *GameState, event string, ctx map[string]interface{}) {
		if event != "spell_cast" {
			return
		}
		// The elimination-cascade shape: a sweep fires while the spell
		// is in cast transit.
		sweptInWindow = SweepOrphanedInstanceIDs(hgs)
		_, ceasedInWindow = hgs.CeasedInstanceIDs[spell.InstanceID]
	}

	if err := CastSpell(gs, 0, spell, nil); err != nil {
		t.Fatalf("CastSpell: %v", err)
	}

	if sweptInWindow == -1 {
		t.Fatal("fixture failure: cast trigger hook never fired")
	}
	if ceasedInWindow {
		t.Error("in-flight cast card ceased by a mid-window orphan sweep — gs.ResolvingCards must cover the CastSpell transit")
	}
	if _, ceased := gs.CeasedInstanceIDs[spell.InstanceID]; ceased {
		t.Error("cast card must not be ceased after the cast completes")
	}
	if len(gs.ResolvingCards) != 0 {
		t.Errorf("CastSpell must pop its transit entry: %d left", len(gs.ResolvingCards))
	}
	if err, ok := ZoneConservationStrict(gs); !ok || err != nil {
		t.Errorf("strict census must be clean post-cast: ok=%v err=%v", ok, err)
	}
}

// The transit entry must also unwind on the cost-failure rollback path.
func TestCastSpell_TransitWindow_RollbackPopsResolvingCards(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Phase = "main"

	spell := &Card{Name: "Expensive Spell", Owner: 0, Types: []string{"sorcery"}, CMC: 9, ManaCostString: "{9}"}
	MintOGInstanceID(gs, spell)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, spell)

	if err := CastSpell(gs, 0, spell, nil); err == nil {
		t.Skip("fixture seat unexpectedly afforded {9} — cost model changed; transit pop covered by the success-path test")
	}
	if len(gs.ResolvingCards) != 0 {
		t.Errorf("failed cast must pop its transit entry: %d left", len(gs.ResolvingCards))
	}
	if err, ok := ZoneConservationStrict(gs); !ok || err != nil {
		t.Errorf("strict census must be clean after rollback: ok=%v err=%v", ok, err)
	}
}

// Game-921 shape: a leaver-owned card in a SURVIVOR's exile with a
// ZoneCastGrants entry must leave the game cleanly at elimination —
// purged from the zone AND ceased, never present-but-ceased.
func TestElimination_LeaverOwnedCardInSurvivorExile_PurgedAndCeased(t *testing.T) {
	gs := newFixtureGame(t)
	island := &Card{Name: "Island", Owner: 1, Types: []string{"land"}}
	MintOGInstanceID(gs, island)
	// Impulse-play shape: seat 0 exiled seat 1's card with a play grant.
	gs.Seats[0].Exile = append(gs.Seats[0].Exile, island)
	if gs.ZoneCastGrants == nil {
		gs.ZoneCastGrants = map[*Card]*ZoneCastPermission{}
	}
	gs.ZoneCastGrants[island] = &ZoneCastPermission{Zone: "exile", Keyword: "static_exile_cast"}

	HandleSeatElimination(gs, 1)

	for _, c := range gs.Seats[0].Exile {
		if c == island {
			t.Error("leaver-owned card must be purged from the survivor's exile (§800.4a)")
		}
	}
	if _, ceased := gs.CeasedInstanceIDs[island.InstanceID]; !ceased {
		t.Error("leaver-owned card must cease")
	}
	if err, ok := ZoneConservationStrict(gs); !ok || err != nil {
		t.Errorf("strict census must be clean after elimination: ok=%v err=%v", ok, err)
	}
}
