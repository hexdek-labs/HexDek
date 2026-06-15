package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// casualty_behavior_r63_test.go — behavioral mechanic-probe for CASUALTY
// (CR §702.153). The pre-existing cost_family_optional_cost_r61_test.go covers
// the paid/declined cost gate; these tests drive the full cast→resolve path to
// pin the properties that gate didn't reach:
//   (c) the sacrificed creature's power is checked at cast time
//   (d) a permanent (creature) casualty spell's COPY resolves as a token
//   (e) the sacrifice fires its dies/zone-change machinery, AND the copy
//       fires the magecraft / spell-copied copy triggers (CR §702.137a /
//       "whenever you copy a spell")

// creatureCasualtyCard builds a vanilla creature spell carrying casualty N.
func creatureCasualtyCard(name string, baseCost, pow, tough, n int) *Card {
	return &Card{
		Name:          name,
		Owner:         0,
		BasePower:     pow,
		BaseToughness: tough,
		Types:         []string{"creature", "cost:" + itoa(baseCost)},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "casualty", Args: []interface{}{float64(n)}},
			},
		},
	}
}

func countMagecraftCopyTriggers(gs *GameState) int {
	n := 0
	for _, ev := range gs.EventLog {
		if ev.Kind != "magecraft_trigger" {
			continue
		}
		if v, ok := ev.Details["is_copy"].(bool); ok && v {
			n++
		}
	}
	return n
}

// (c) Power is checked at cast time: a casualty-3 spell with only a power-2
// creature available cannot pay (no sacrifice, no copy). With a power-3
// creature it can.
func TestCasualty_PowerCheckedAtCast(t *testing.T) {
	// Insufficient power → declined automatically (CanPayCasualty false).
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &payHat{}
	gs.Seats[0].ManaPool = 2
	small := addKWCombatBattlefield(gs, 0, "Small", 2, 2, "creature")

	card := instantSpellCard("Body Count", 2, nil,
		&gameast.Keyword{Name: "casualty", Args: []interface{}{float64(3)}})
	gs.Seats[0].Hand = []*Card{card}
	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	for _, p := range gs.Seats[0].Battlefield {
		if p == small {
			goto ok
		}
	}
	t.Fatal("power-2 creature must NOT be sacrificed for casualty 3")
ok:
	for _, ev := range gs.EventLog {
		if ev.Kind == "casualty_copy" {
			t.Fatal("casualty 3 must not copy when no power-3 creature exists")
		}
	}

	// Sufficient power → pays.
	gs2 := newKWCombatGame(t)
	gs2.Seats[0].Hat = &payHat{}
	gs2.Seats[0].ManaPool = 2
	big := addKWCombatBattlefield(gs2, 0, "Big", 3, 3, "creature")
	card2 := instantSpellCard("Body Count", 2, nil,
		&gameast.Keyword{Name: "casualty", Args: []interface{}{float64(3)}})
	gs2.Seats[0].Hand = []*Card{card2}
	if err := CastSpell(gs2, 0, card2, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	for _, p := range gs2.Seats[0].Battlefield {
		if p == big {
			t.Fatal("power-3 creature should be sacrificed for casualty 3")
		}
	}
}

// (d) A permanent (creature) casualty spell: original resolves as a real
// permanent, the copy resolves as a token (CR §707.10f).
func TestCasualty_PermanentSpell_CopyResolvesAsToken(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &payHat{}
	gs.Seats[0].ManaPool = 2
	victim := addKWCombatBattlefield(gs, 0, "Sacrifice Fodder", 3, 3, "creature")

	card := creatureCasualtyCard("Bortuk Bonerattle", 2, 4, 4, 2)
	gs.Seats[0].Hand = []*Card{card}
	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}

	// victim sacrificed → graveyard.
	for _, p := range gs.Seats[0].Battlefield {
		if p == victim {
			t.Error("casualty victim should have been sacrificed")
		}
	}
	// Two Bortuks on the battlefield: one real, one token copy.
	real, token := 0, 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil || p.Card.Name != "Bortuk Bonerattle" {
			continue
		}
		if p.IsToken() {
			token++
		} else {
			real++
		}
	}
	if real != 1 {
		t.Errorf("expected exactly 1 real Bortuk on battlefield, got %d", real)
	}
	if token != 1 {
		t.Errorf("expected exactly 1 token-copy Bortuk on battlefield (CR §707.10f), got %d", token)
	}
}

// (e, sacrifice half) The sacrificed creature reaches the graveyard and its
// zone-change machinery fires (dies-triggers, blood-artist payoffs key on the
// sacrifice / creature-leaves-battlefield event chain).
func TestCasualty_Sacrifice_FiresDiesMachinery(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &payHat{}
	gs.Seats[0].ManaPool = 2
	victim := addKWCombatBattlefield(gs, 0, "Doomed Dissenter", 3, 3, "creature")

	card := instantSpellCard("Halana", 2, nil,
		&gameast.Keyword{Name: "casualty", Args: []interface{}{float64(2)}})
	gs.Seats[0].Hand = []*Card{card}
	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}

	inGrave := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == victim.Card {
			inGrave = true
		}
	}
	if !inGrave {
		t.Error("sacrificed casualty creature should be in the graveyard")
	}
	sacFired := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "sacrifice" {
			sacFired = true
		}
	}
	if !sacFired {
		t.Error("casualty sacrifice should emit a sacrifice (dies) event")
	}
}

// (e, copy-trigger half) The casualty copy of an instant/sorcery spell fires
// the magecraft trigger (CR §702.137a: "whenever you cast OR COPY an instant
// or sorcery spell") — exactly as the canonical copy paths (resolveCopySpell,
// storm) do. The cast itself fires magecraft with is_copy=false; the copy must
// add an is_copy=true firing.
func TestCasualty_Copy_FiresMagecraft(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &payHat{}
	gs.Seats[0].ManaPool = 2
	// A magecraft permanent the controller owns — power 1 so casualty 2 can't
	// pick it as the sacrifice (the magecraft creature must survive the copy).
	addKWCombatBattlefieldWithKeyword(gs, 0, "Archmage Emeritus", 1, 2, "magecraft", "creature")
	// The only power-2 creature → the casualty sacrifice.
	addKWCombatBattlefield(gs, 0, "Fodder", 2, 2, "creature")

	card := instantSpellCard("Burn Down the House", 2, nil,
		&gameast.Keyword{Name: "casualty", Args: []interface{}{float64(2)}})
	gs.Seats[0].Hand = []*Card{card}
	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}

	if got := countMagecraftCopyTriggers(gs); got < 1 {
		t.Errorf("casualty copy must fire magecraft with is_copy=true (CR §702.137a); got %d copy-firings", got)
	}
}
