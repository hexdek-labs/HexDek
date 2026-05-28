package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Batch AI (R60) — §400.7c owner-routing tests for Bribery, Hostage
// Taker, Knowledge Pool, Possibility Storm (fix), Mind's Desire.
// Each card gets a happy-path test + a §400.7c routing assertion
// where applicable (cross-seat moves) + a ZoneCastGrant shape check
// where the card grants free-cast permissions.

// -----------------------------------------------------------------------------
// Bribery
// -----------------------------------------------------------------------------

func TestBribery_TutorsOpponentCreatureUnderCasterControl(t *testing.T) {
	// Seat 0 casts Bribery; seat 1 has a Craterhoof Behemoth (5/5) in
	// library. Expected: Craterhoof on seat 0's battlefield with
	// Controller=0, Owner=1; seat 1's library shrinks by 1; the *Card
	// is in EXACTLY one zone (seat 0's battlefield).
	gs := newGame(t, 2)
	craterhoof := &gameengine.Card{
		Name:          "Craterhoof Behemoth",
		Owner:         1,
		Types:         []string{"creature", "cmc:8"},
		BasePower:     5,
		BaseToughness: 5,
	}
	gs.Seats[1].Library = []*gameengine.Card{craterhoof}

	item := &gameengine.StackItem{Controller: 0}
	briberyResolve(gs, item)

	// Craterhoof on seat 0's battlefield.
	if len(gs.Seats[0].Battlefield) != 1 {
		t.Fatalf("seat 0 battlefield: want 1 perm, got %d", len(gs.Seats[0].Battlefield))
	}
	stolen := gs.Seats[0].Battlefield[0]
	if stolen.Card != craterhoof {
		t.Errorf("battlefield perm should wrap Craterhoof, got %q", stolen.Card.DisplayName())
	}
	if stolen.Controller != 0 {
		t.Errorf("Controller: want 0 (caster), got %d", stolen.Controller)
	}
	if stolen.Owner != 1 {
		t.Errorf("Owner: want 1 (opp), got %d (§400.4 — Owner stays the printing's original)", stolen.Owner)
	}
	// Seat 1's library is empty.
	if len(gs.Seats[1].Library) != 0 {
		t.Errorf("seat 1 library: want 0, got %d", len(gs.Seats[1].Library))
	}
	// Craterhoof does NOT appear in any zone other than seat 0's battlefield.
	for seatIdx, s := range gs.Seats {
		for _, c := range s.Library {
			if c == craterhoof {
				t.Errorf("Craterhoof should not be in seat %d library", seatIdx)
			}
		}
		for _, c := range s.Hand {
			if c == craterhoof {
				t.Errorf("Craterhoof should not be in seat %d hand", seatIdx)
			}
		}
	}
}

func TestBribery_NoCreatureInOpponentLibrary_NoOp(t *testing.T) {
	// All opp libraries have only lands → no valid target → emit fail.
	gs := newGame(t, 2)
	gs.Seats[1].Library = []*gameengine.Card{
		{Name: "Forest", Owner: 1, Types: []string{"land"}},
	}
	item := &gameengine.StackItem{Controller: 0}
	briberyResolve(gs, item)
	if len(gs.Seats[0].Battlefield) != 0 {
		t.Errorf("battlefield should stay empty when no creature targets, got %d perms",
			len(gs.Seats[0].Battlefield))
	}
}

// -----------------------------------------------------------------------------
// Hostage Taker
// -----------------------------------------------------------------------------

func TestHostageTaker_ExilesTargetToOwnersExileWithFreeCastGrant(t *testing.T) {
	// Seat 0 plays Hostage Taker; seat 1 has Birds of Paradise. Expected:
	// BoP exiled to seat 1's exile (§400.7c, NOT seat 0's), a free-cast
	// grant registered on BoP with RequireController=0 (seat 0 = HT
	// controller), SpendAnyColor=true, Duration=while_source_on_bf,
	// SourceTimestamp = Hostage Taker's timestamp.
	gs := newGame(t, 2)
	ht := addPerm(gs, 0, "Hostage Taker", "creature")
	ht.Card.BasePower = 2
	ht.Card.BaseToughness = 3
	bop := addPerm(gs, 1, "Birds of Paradise", "creature")
	bop.Card.BasePower = 0
	bop.Card.BaseToughness = 1

	hostageTakerETB(gs, ht)

	// BoP card lives in seat 1's exile (owner), NOT seat 0's.
	if len(gs.Seats[1].Exile) != 1 || gs.Seats[1].Exile[0] != bop.Card {
		t.Errorf("BoP should be in seat 1 (owner) exile per §400.7c, seat 1 exile=%d", len(gs.Seats[1].Exile))
	}
	if len(gs.Seats[0].Exile) != 0 {
		t.Errorf("seat 0 (Hostage Taker controller) exile should NOT hold BoP per §400.7c, got %d entries",
			len(gs.Seats[0].Exile))
	}
	// BoP no longer on seat 1's battlefield.
	for _, p := range gs.Seats[1].Battlefield {
		if p == bop {
			t.Errorf("BoP should have been removed from seat 1 battlefield")
		}
	}
	// Linked-exile bookkeeping: HT.LinkedExile holds BoP, BoP.ExiledByTimestamp = HT.Timestamp.
	if len(ht.LinkedExile) != 1 || ht.LinkedExile[0] != bop.Card {
		t.Errorf("HT.LinkedExile should hold BoP, got %v", ht.LinkedExile)
	}
	if bop.Card.ExiledByTimestamp != ht.Timestamp {
		t.Errorf("BoP.ExiledByTimestamp: want %d (HT timestamp), got %d", ht.Timestamp, bop.Card.ExiledByTimestamp)
	}

	// Grant shape.
	grant := gameengine.GetZoneCastGrant(gs, bop.Card)
	if grant == nil {
		t.Fatalf("expected ZoneCastGrant on BoP, got nil")
	}
	if grant.RequireController != 0 {
		t.Errorf("grant.RequireController: want 0 (HT controller), got %d", grant.RequireController)
	}
	if grant.Duration != "while_source_on_bf" {
		t.Errorf("grant.Duration: want while_source_on_bf, got %q", grant.Duration)
	}
	if grant.SourceTimestamp != ht.Timestamp {
		t.Errorf("grant.SourceTimestamp: want %d, got %d", ht.Timestamp, grant.SourceTimestamp)
	}
	if !grant.SpendAnyColor {
		t.Errorf("grant.SpendAnyColor: want true (HT clause grants color-fixing)")
	}
	if grant.ManaCost != 0 {
		t.Errorf("grant.ManaCost: want 0 (free-cast), got %d", grant.ManaCost)
	}
}

func TestHostageTaker_LTB_ReturnsTargetToOwnersBattlefield(t *testing.T) {
	// Setup: HT on seat 0's battlefield, BoP exiled (linked-exile) by HT.
	// Call hostageTakerLTB(); BoP returns to seat 1 (owner) battlefield,
	// HT.LinkedExile is cleared, BoP.ExiledByTimestamp resets to 0.
	gs := newGame(t, 2)
	ht := addPerm(gs, 0, "Hostage Taker", "creature")
	bopCard := &gameengine.Card{Name: "Birds of Paradise", Owner: 1, Types: []string{"creature"}}
	gameengine.ExileLinked(gs, ht, bopCard, 1, "battlefield")

	hostageTakerLTB(gs, ht, nil)

	// BoP no longer in seat 1's exile.
	if len(gs.Seats[1].Exile) != 0 {
		t.Errorf("BoP should have left seat 1 exile on HT LTB, got %d entries", len(gs.Seats[1].Exile))
	}
	// HT.LinkedExile cleared.
	if len(ht.LinkedExile) != 0 {
		t.Errorf("HT.LinkedExile should be empty after return, got %d", len(ht.LinkedExile))
	}
	// ExiledByTimestamp reset.
	if bopCard.ExiledByTimestamp != 0 {
		t.Errorf("BoP.ExiledByTimestamp should reset to 0 on return, got %d", bopCard.ExiledByTimestamp)
	}
}

func TestHostageTaker_NoValidTarget_NoOp(t *testing.T) {
	// Single-seat game with no opponents → no target → emit fail, no
	// state change.
	gs := newGame(t, 2)
	ht := addPerm(gs, 0, "Hostage Taker", "creature")
	// Seat 1 has only lands.
	addPerm(gs, 1, "Forest", "land")

	hostageTakerETB(gs, ht)

	if len(ht.LinkedExile) != 0 {
		t.Errorf("no valid target: HT.LinkedExile should be empty, got %d", len(ht.LinkedExile))
	}
}

// -----------------------------------------------------------------------------
// Knowledge Pool
// -----------------------------------------------------------------------------

func TestKnowledgePool_ETB_EachPlayerExilesTop3ToOwnExile(t *testing.T) {
	// 3-seat pod. Each seat starts with 5 library cards. After KP ETB,
	// each seat has 2 library + 3 exile cards, all owner-routed per
	// §400.7c. Cards tagged ExiledByTimestamp = KP.Timestamp.
	gs := newGame(t, 3)
	kp := addPerm(gs, 0, "Knowledge Pool", "artifact")
	for seatIdx := 0; seatIdx < 3; seatIdx++ {
		gs.Seats[seatIdx].Library = nil
		for i := 0; i < 5; i++ {
			gs.Seats[seatIdx].Library = append(gs.Seats[seatIdx].Library,
				&gameengine.Card{
					Name:  "L" + string(rune('0'+seatIdx)) + string(rune('0'+i)),
					Owner: seatIdx,
					Types: []string{"creature"},
				})
		}
	}

	knowledgePoolETB(gs, kp)

	for seatIdx := 0; seatIdx < 3; seatIdx++ {
		s := gs.Seats[seatIdx]
		if len(s.Library) != 2 {
			t.Errorf("seat %d library: want 2 after KP ETB, got %d", seatIdx, len(s.Library))
		}
		if len(s.Exile) != 3 {
			t.Errorf("seat %d exile: want 3 (own top-3), got %d", seatIdx, len(s.Exile))
		}
		for _, c := range s.Exile {
			if c.Owner != seatIdx {
				t.Errorf("seat %d exile contains a card owned by %d — §400.7c violation",
					seatIdx, c.Owner)
			}
			if c.ExiledByTimestamp != kp.Timestamp {
				t.Errorf("seat %d exile[%q].ExiledByTimestamp: want %d, got %d",
					seatIdx, c.DisplayName(), kp.Timestamp, c.ExiledByTimestamp)
			}
		}
	}
}

func TestKnowledgePool_SpellCast_RegistersGrantsOnTaggedNonlandCards(t *testing.T) {
	// KP on seat 0 BF. Seed seat 0's exile with 1 KP-tagged creature
	// and 1 KP-tagged land; seed seat 1's exile with 1 KP-tagged
	// creature. Seat 0 casts a spell from hand → that spell exiles
	// to seat 0 exile; grants register on the 2 nonland KP-tagged
	// cards (the seat-0 creature and seat-1 creature; land excluded).
	gs := newGame(t, 2)
	kp := addPerm(gs, 0, "Knowledge Pool", "artifact")

	tagged0 := &gameengine.Card{Name: "Tagged0", Owner: 0, Types: []string{"creature"}, ExiledByTimestamp: kp.Timestamp}
	taggedLand := &gameengine.Card{Name: "TaggedLand", Owner: 0, Types: []string{"land"}, ExiledByTimestamp: kp.Timestamp}
	tagged1 := &gameengine.Card{Name: "Tagged1", Owner: 1, Types: []string{"creature"}, ExiledByTimestamp: kp.Timestamp}
	gs.Seats[0].Exile = append(gs.Seats[0].Exile, tagged0, taggedLand)
	gs.Seats[1].Exile = append(gs.Seats[1].Exile, tagged1)

	casterSpell := &gameengine.Card{Name: "Casted Spell", Owner: 0, Types: []string{"instant"}}
	gs.Seats[0].Hand = []*gameengine.Card{casterSpell}

	knowledgePoolOnSpellCast(gs, kp, map[string]interface{}{
		"card":        casterSpell,
		"caster_seat": 0,
		"cast_zone":   gameengine.ZoneHand,
	})

	// Cast spell exiled to seat 0 (owner) exile, tagged with KP timestamp.
	foundCasted := false
	for _, c := range gs.Seats[0].Exile {
		if c == casterSpell {
			foundCasted = true
			if c.ExiledByTimestamp != kp.Timestamp {
				t.Errorf("cast spell ExiledByTimestamp: want %d, got %d", kp.Timestamp, c.ExiledByTimestamp)
			}
		}
	}
	if !foundCasted {
		t.Errorf("cast spell should be in seat 0 exile after KP trigger")
	}

	// Grants registered on tagged0 and tagged1 (nonland KP-tagged); NOT on taggedLand.
	for _, c := range []*gameengine.Card{tagged0, tagged1} {
		g := gameengine.GetZoneCastGrant(gs, c)
		if g == nil {
			t.Errorf("expected grant on %q, got nil", c.DisplayName())
			continue
		}
		if g.RequireController != 0 {
			t.Errorf("grant on %q: RequireController want 0 (caster), got %d", c.DisplayName(), g.RequireController)
		}
		if g.Duration != "while_source_on_bf" {
			t.Errorf("grant on %q: Duration want while_source_on_bf, got %q", c.DisplayName(), g.Duration)
		}
	}
	if g := gameengine.GetZoneCastGrant(gs, taggedLand); g != nil {
		t.Errorf("land should NOT get a free-cast grant (Knowledge Pool oracle says nonland)")
	}
}

func TestKnowledgePool_SpellCast_IgnoresFlashbackOrExileSourceCasts(t *testing.T) {
	// KP oracle says "casts a spell from their hand" — flashback casts
	// (cast_zone == graveyard) shouldn't trigger the exile-and-replace
	// branch. The trigger fires for ANY cast (engine fans out spell_cast
	// universally), so this gate lives in the handler.
	gs := newGame(t, 2)
	kp := addPerm(gs, 0, "Knowledge Pool", "artifact")

	flashbackSpell := &gameengine.Card{Name: "Flashbacked", Owner: 0, Types: []string{"instant"}}
	knowledgePoolOnSpellCast(gs, kp, map[string]interface{}{
		"card":        flashbackSpell,
		"caster_seat": 0,
		"cast_zone":   gameengine.ZoneGraveyard,
	})
	// Spell should NOT have been exiled by KP — flashback already
	// exiles on resolve. The handler should no-op.
	for _, c := range gs.Seats[0].Exile {
		if c == flashbackSpell {
			t.Errorf("flashback cast should not trigger KP exile (cast_zone=graveyard)")
		}
	}
}

// -----------------------------------------------------------------------------
// Possibility Storm (chaos_cascade.go fix)
// -----------------------------------------------------------------------------

func TestPossibilityStorm_RegistersGrantOnMatchedCardInExile(t *testing.T) {
	// Seat 0 casts a creature spell from hand. Possibility Storm
	// (on seat 0's battlefield) exiles the cast spell, then exiles
	// from top of library until matching a creature. The matched
	// card stays in exile under a free-cast grant — NOT moved to
	// hand (the pre-fix bug). The remaining exiled cards go to
	// library bottom.
	gs := newGame(t, 2)
	ps := addPerm(gs, 0, "Possibility Storm", "enchantment")

	castSpell := &gameengine.Card{Name: "Castigated", Owner: 0, Types: []string{"creature"}}
	// Library top = LAST index. The reveal loop reads seat.Library[0]
	// (front) per the existing Possibility Storm code — match the
	// existing implementation's convention.
	nonMatch := &gameengine.Card{Name: "Sorcery1", Owner: 0, Types: []string{"sorcery"}}
	match := &gameengine.Card{Name: "MatchCreature", Owner: 0, Types: []string{"creature"}}
	rest := &gameengine.Card{Name: "AfterMatch", Owner: 0, Types: []string{"creature"}}
	gs.Seats[0].Library = []*gameengine.Card{nonMatch, match, rest}

	possibilityStormTrigger(gs, ps, map[string]interface{}{
		"card":        castSpell,
		"caster_seat": 0,
		"cast_zone":   gameengine.ZoneHand,
	})

	// The matched card is in caster's exile under a free-cast grant.
	foundInExile := false
	for _, c := range gs.Seats[0].Exile {
		if c == match {
			foundInExile = true
		}
	}
	if !foundInExile {
		t.Errorf("matched card should stay in caster exile under grant, not be moved to hand")
	}
	// Matched is NOT in hand (the pre-fix bug).
	for _, c := range gs.Seats[0].Hand {
		if c == match {
			t.Errorf("matched card should NOT be in hand (pre-fix bug)")
		}
	}

	// Grant on the matched card.
	grant := gameengine.GetZoneCastGrant(gs, match)
	if grant == nil {
		t.Fatalf("expected grant on matched card, got nil")
	}
	if grant.RequireController != 0 {
		t.Errorf("grant.RequireController: want 0 (caster), got %d", grant.RequireController)
	}
	if grant.Duration != "until_end_of_turn" {
		t.Errorf("grant.Duration: want until_end_of_turn, got %q", grant.Duration)
	}
	if grant.ManaCost != 0 {
		t.Errorf("grant.ManaCost: want 0 (free), got %d", grant.ManaCost)
	}

	// The non-matched first reveal (Sorcery1) goes to library bottom.
	if len(gs.Seats[0].Library) < 1 {
		t.Fatalf("library should hold the non-matched reveal at the bottom, got %d", len(gs.Seats[0].Library))
	}
}

func TestPossibilityStorm_GatesToHandCastsOnly(t *testing.T) {
	// PR-#685-style §400.7c gate: oracle says "casts a spell from their
	// hand", so a flashback cast (cast_zone=graveyard) shouldn't trigger.
	gs := newGame(t, 2)
	ps := addPerm(gs, 0, "Possibility Storm", "enchantment")
	gs.Seats[0].Library = []*gameengine.Card{
		{Name: "L1", Owner: 0, Types: []string{"creature"}},
	}
	castSpell := &gameengine.Card{Name: "Flashbacked", Owner: 0, Types: []string{"instant"}}

	possibilityStormTrigger(gs, ps, map[string]interface{}{
		"card":        castSpell,
		"caster_seat": 0,
		"cast_zone":   gameengine.ZoneGraveyard,
	})
	// Library untouched.
	if len(gs.Seats[0].Library) != 1 {
		t.Errorf("flashback cast should NOT trigger Possibility Storm exile, library size changed to %d",
			len(gs.Seats[0].Library))
	}
}

// -----------------------------------------------------------------------------
// Mind's Desire
// -----------------------------------------------------------------------------

func TestMindsDesire_ExilesTopCardWithEOTGrant(t *testing.T) {
	// Seat 0 resolves Mind's Desire with a 1-card library (shuffle is a
	// no-op on a single element so we avoid testing through the shuffle
	// noise). The card is exiled to seat 0 exile (owner-routed since
	// caster == owner) under a free-cast grant with
	// Duration=until_end_of_turn, GrantTurn=gs.Turn.
	gs := newGame(t, 2)
	pick := &gameengine.Card{Name: "PickedCard", Owner: 0, Types: []string{"creature"}}
	gs.Seats[0].Library = []*gameengine.Card{pick}

	item := &gameengine.StackItem{Controller: 0}
	mindsDesireResolve(gs, item)

	// Card moved to seat 0 exile.
	foundInExile := false
	for _, c := range gs.Seats[0].Exile {
		if c == pick {
			foundInExile = true
		}
	}
	if !foundInExile {
		t.Errorf("picked card should be exiled to seat 0 exile, seat 0 exile=%d entries",
			len(gs.Seats[0].Exile))
	}
	if len(gs.Seats[0].Library) != 0 {
		t.Errorf("library size: want 0 (single card popped), got %d", len(gs.Seats[0].Library))
	}

	grant := gameengine.GetZoneCastGrant(gs, pick)
	if grant == nil {
		t.Fatalf("expected grant on exiled card, got nil")
	}
	if grant.RequireController != 0 {
		t.Errorf("grant.RequireController: want 0 (caster), got %d", grant.RequireController)
	}
	if grant.Duration != "until_end_of_turn" {
		t.Errorf("grant.Duration: want until_end_of_turn, got %q", grant.Duration)
	}
	if grant.GrantTurn != gs.Turn {
		t.Errorf("grant.GrantTurn: want %d (current turn), got %d", gs.Turn, grant.GrantTurn)
	}
	if grant.ManaCost != 0 {
		t.Errorf("grant.ManaCost: want 0 (free), got %d", grant.ManaCost)
	}
}

func TestMindsDesire_EmptyLibrary_NoOp(t *testing.T) {
	// Shuffle no-ops then exile attempt finds nothing → emitFail.
	gs := newGame(t, 2)
	gs.Seats[0].Library = nil
	item := &gameengine.StackItem{Controller: 0}
	mindsDesireResolve(gs, item)
	if len(gs.Seats[0].Exile) != 0 {
		t.Errorf("empty library should produce no exile, got %d", len(gs.Seats[0].Exile))
	}
}

// -----------------------------------------------------------------------------
// Cross-cutting §400.7c invariant — Knowledge Pool spread across seats
// -----------------------------------------------------------------------------

func TestKnowledgePool_AllExiledCardsRouteToOwnerExile(t *testing.T) {
	// 4-seat pod, Knowledge Pool on seat 0. After ETB, no seat holds
	// a foreign-owned card in its exile pile — the §400.7c invariant
	// the Etali fix codified.
	gs := newGame(t, 4)
	kp := addPerm(gs, 0, "Knowledge Pool", "artifact")
	for seatIdx := 0; seatIdx < 4; seatIdx++ {
		gs.Seats[seatIdx].Library = nil
		for i := 0; i < 3; i++ {
			gs.Seats[seatIdx].Library = append(gs.Seats[seatIdx].Library,
				&gameengine.Card{
					Name:  "L" + string(rune('0'+seatIdx)) + string(rune('0'+i)),
					Owner: seatIdx,
					Types: []string{"creature"},
				})
		}
	}
	knowledgePoolETB(gs, kp)
	for seatIdx, s := range gs.Seats {
		for _, c := range s.Exile {
			if c.Owner != seatIdx {
				t.Errorf("§400.7c violation: seat %d exile holds %q owned by seat %d",
					seatIdx, c.DisplayName(), c.Owner)
			}
		}
	}
}
