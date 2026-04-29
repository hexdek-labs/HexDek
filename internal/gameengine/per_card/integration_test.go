package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestIntegration_CastSpellFiresResolveHook verifies that when a spell
// with a per-card ResolveHandler is CAST via the engine's real
// CastSpell pipeline, the handler fires (not just when we manually
// InvokeResolveHook). This is the acid test that the stack.go wiring
// added for this batch actually hits the per_card subsystem.
func TestIntegration_CastSpellFiresResolveHook(t *testing.T) {
	gs := newGame(t, 2)
	// Library to burn through.
	addLibrary(gs, 0, "A", "B", "C", "D", "E", "F", "G", "H", "I", "J")
	// Cast Demonic Consultation: put the card in hand, then cast.
	card := addCard(gs, 0, "Demonic Consultation", "sorcery")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	err := gameengine.CastSpell(gs, 0, card, nil)
	if err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}

	// After resolution, library should be empty (combo-line behavior).
	if len(gs.Seats[0].Library) != 0 {
		t.Errorf("library should be empty after Consultation, got %d", len(gs.Seats[0].Library))
	}
	// And the per_card_handler event should be in the log.
	if hasEvent(gs, "per_card_handler") < 1 {
		t.Errorf("expected per_card_handler event from Consultation resolve")
	}
}

// TestIntegration_ThoracleViaETBHookInCastSpell verifies that when a
// Thassa's Oracle is CAST (not just dropped onto the battlefield), the
// ETB hook fires via resolvePermanentSpellETB -> InvokeETBHook and
// correctly wins the game when library <= devotion.
func TestIntegration_ThoracleViaETBHookInCastSpell(t *testing.T) {
	gs := newGame(t, 2)
	// Empty library → any devotion wins.
	oracleCard := addCard(gs, 0, "Thassa's Oracle", "creature", "pip:U", "pip:U")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, oracleCard)

	err := gameengine.CastSpell(gs, 0, oracleCard, nil)
	if err != nil {
		t.Fatalf("CastSpell(Thassa's Oracle) failed: %v", err)
	}

	if hasEvent(gs, "per_card_win") < 1 {
		t.Errorf("expected per_card_win event from Oracle ETB via CastSpell; events: %+v", gs.EventLog)
	}
	if !gs.Seats[0].Won {
		t.Errorf("expected seat 0 to win")
	}
}

// TestIntegration_RhysticStudyFiresOnOpponentCast verifies Rhystic
// Study draws a card when an OPPONENT casts a spell through the real
// CastSpell pipeline. This is the end-to-end wiring test for the cast-
// time trigger fan-out added to stack.go.
func TestIntegration_RhysticStudyFiresOnOpponentCast(t *testing.T) {
	gs := newGame(t, 2)
	// Seat 0 controls Rhystic Study.
	addPerm(gs, 0, "Rhystic Study", "enchantment")
	// Library has a card we can draw.
	addLibrary(gs, 0, "Brainstorm")
	// Seat 1 casts a noncreature spell. We give them zero mana so they
	// CAN'T pay the tax.
	gs.Seats[1].ManaPool = 0
	spell := addCard(gs, 1, "Ponder", "sorcery")
	gs.Seats[1].Hand = append(gs.Seats[1].Hand, spell)

	err := gameengine.CastSpell(gs, 1, spell, nil)
	if err != nil {
		t.Fatalf("opponent CastSpell failed: %v", err)
	}

	// Rhystic controller should have drawn one card.
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected Rhystic to draw 1 card when opponent couldn't pay; hand=%d", len(gs.Seats[0].Hand))
	}
	if hasEvent(gs, "draw") < 1 {
		t.Errorf("expected a draw event from Rhystic")
	}
}

// TestIntegration_FullThoracleCombo_CastConsultationThenOracle simulates
// the canonical cEDH combo: cast Demonic Consultation (empties library),
// then cast Thassa's Oracle (ETB checks library <= devotion → wins).
// Uses the full CastSpell pipeline twice.
//
// This is THE "does cEDH work" test — before this batch landed, neither
// Consultation nor Oracle would do anything, and Kraum+Tymna couldn't
// win via its primary line.
func TestIntegration_FullThoracleCombo_CastConsultationThenOracle(t *testing.T) {
	gs := newGame(t, 2)
	// Library: 30 cards to burn through.
	libNames := make([]string, 30)
	for i := range libNames {
		libNames[i] = "Card_" + intToStr(i)
	}
	addLibrary(gs, 0, libNames...)

	// Hand: Consultation + Oracle.
	consultation := addCard(gs, 0, "Demonic Consultation", "sorcery")
	oracle := addCard(gs, 0, "Thassa's Oracle", "creature", "pip:U", "pip:U")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, consultation, oracle)

	// Turn 1: cast Consultation → library empties.
	if err := gameengine.CastSpell(gs, 0, consultation, nil); err != nil {
		t.Fatalf("CastSpell(Consultation): %v", err)
	}
	if len(gs.Seats[0].Library) != 0 {
		t.Fatalf("Consultation should have emptied library, has %d", len(gs.Seats[0].Library))
	}
	if gs.Seats[0].Won {
		t.Fatalf("Consultation alone should NOT have won the game")
	}

	// Turn 2: cast Oracle → ETB triggers win check, library (0) <= devotion (2) → win.
	if err := gameengine.CastSpell(gs, 0, oracle, nil); err != nil {
		t.Fatalf("CastSpell(Oracle): %v", err)
	}
	if !gs.Seats[0].Won {
		t.Fatalf("Oracle should have won after library was emptied; state: %+v", gs.Seats[0])
	}
	if !gs.Seats[1].Lost {
		t.Fatalf("Opponent should be marked Lost")
	}

	// Verify the combo log chain is intact.
	gotConsultationEvent := false
	gotWinEvent := false
	for _, ev := range gs.EventLog {
		if ev.Source == "Demonic Consultation" && ev.Kind == "per_card_handler" {
			gotConsultationEvent = true
		}
		if ev.Source == "Thassa's Oracle" && ev.Kind == "per_card_win" {
			gotWinEvent = true
		}
	}
	if !gotConsultationEvent {
		t.Errorf("expected per_card_handler event for Consultation")
	}
	if !gotWinEvent {
		t.Errorf("expected per_card_win event for Oracle")
	}
}

// TestIntegration_KittenFlickersOnNoncreatureCast verifies Displacer
// Kitten fires via the engine's cast pipeline.
func TestIntegration_KittenFlickersOnNoncreatureCast(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Displacer Kitten", "creature")
	target := addPerm(gs, 0, "Sol Ring", "artifact")
	prevTimestamp := target.Timestamp

	// Cast a noncreature spell.
	spell := addCard(gs, 0, "Ponder", "sorcery")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, spell)
	if err := gameengine.CastSpell(gs, 0, spell, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}

	// Look for the flickered permanent with a new timestamp.
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.DisplayName() == "Sol Ring" && p.Timestamp > prevTimestamp {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Displacer Kitten should have flickered Sol Ring; events: %+v", gs.EventLog)
	}
}
