package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// blitz_behavior_r63_test.go — r63 mechanic-probe (CR §702.152, blitz).
// Blitz is an alternative cost to cast a creature spell: the creature gains
// haste, "When this dies, draw a card", and is sacrificed at the next end
// step. Before the fix, printed blitz had NO alt-cost cast path (ApplyBlitz
// was only reachable via Henzie's cost-granter), and the death-draw was an
// on_event delayed trigger never pumped in live play. Fix: wire blitz into
// CastSpell's alt-cost stage + apply ApplyBlitz on resolution, and route the
// death-draw through the canonical dies-chokepoint (CheckBlitzDeathDraw).

// blitzCreatureCard builds a vanilla creature spell with printed blitz {cost}.
func blitzCreatureCard(name string, baseCost, pow, tough int, blitzCost string) *Card {
	return &Card{
		Name:          name,
		Owner:         0,
		BasePower:     pow,
		BaseToughness: tough,
		Types:         []string{"creature", "cost:" + itoa(baseCost)},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "blitz", Args: []interface{}{blitzCost}},
			},
		},
	}
}

func countBlitzDraws(gs *GameState) int {
	n := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "blitz_draw" {
			n++
		}
	}
	return n
}

func findBlitzPerm(gs *GameState, seat int, name string) *Permanent {
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == name {
			return p
		}
	}
	return nil
}

// (1)+(2) Casting for blitz grants haste and wires the EOT sacrifice; the
// sacrifice fires the death-draw at end of turn.
func TestBlitz_CastGrantsHasteAndSacrificesAtEOTWithDraw(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &payHat{} // opt into the blitz alt cost
	gs.Seats[0].ManaPool = 10

	card := blitzCreatureCard("Mezzio Mugger", 5, 3, 3, "{2}{r}") // blitz cost = 3
	gs.Seats[0].Hand = []*Card{card}
	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}

	perm := findBlitzPerm(gs, 0, "Mezzio Mugger")
	if perm == nil {
		t.Fatal("blitz creature should have resolved onto the battlefield")
	}
	// (1)/(2) ApplyBlitz ran via the alt-cost cast path.
	if perm.Flags["blitz"] != 1 {
		t.Error("creature cast for blitz must carry the blitz flag (ApplyBlitz wired)")
	}
	if perm.Flags["kw:haste"] != 1 {
		t.Error("(2) a creature cast for blitz must gain haste")
	}
	// EOT-sacrifice delayed trigger registered.
	foundEOT := false
	for _, dt := range gs.DelayedTriggers {
		if dt != nil && dt.TriggerAt == "end_of_turn" {
			foundEOT = true
		}
	}
	if !foundEOT {
		t.Error("(4) blitz must register an end-of-turn sacrifice delayed trigger")
	}

	// Fire the end-step: the creature is sacrificed AND the death-draw fires.
	FireDelayedTriggers(gs, "ending", "end")

	if findBlitzPerm(gs, 0, "Mezzio Mugger") != nil {
		t.Error("(4) blitz creature must be sacrificed at the next end step")
	}
	if got := countBlitzDraws(gs); got != 1 {
		t.Errorf("(3) death-draw must fire exactly once off the EOT sacrifice, got %d", got)
	}
}

// (5) If the creature dies EARLIER, the draw fires once and the EOT sacrifice
// is a no-op (no double-draw, no leak).
func TestBlitz_EarlyDeathDrawsOnceEOTNoOp(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &payHat{}
	gs.Seats[0].ManaPool = 10

	card := blitzCreatureCard("Riveteers Requisitioner", 4, 2, 2, "{2}{r}")
	gs.Seats[0].Hand = []*Card{card}
	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	perm := findBlitzPerm(gs, 0, "Riveteers Requisitioner")
	if perm == nil {
		t.Fatal("blitz creature should have resolved")
	}

	// Dies early (combat/removal stand-in).
	SacrificePermanent(gs, perm, "test_early_death")
	if got := countBlitzDraws(gs); got != 1 {
		t.Fatalf("(3) death-draw must fire once on the early death, got %d", got)
	}

	// End step: the EOT sacrifice is a no-op (already dead) — no second draw.
	FireDelayedTriggers(gs, "ending", "end")
	if got := countBlitzDraws(gs); got != 1 {
		t.Errorf("(5) EOT sacrifice must be a no-op after early death — no double-draw, got %d", got)
	}
}

// Optionality: declining blitz casts the creature normally — no haste, no
// blitz flag, no EOT sacrifice (it persists).
func TestBlitz_DeclinedCastsNormally(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &declineHat{}
	gs.Seats[0].ManaPool = 10

	card := blitzCreatureCard("Night Clubber", 4, 2, 2, "{2}{b}")
	gs.Seats[0].Hand = []*Card{card}
	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	perm := findBlitzPerm(gs, 0, "Night Clubber")
	if perm == nil {
		t.Fatal("creature should have resolved (cast at normal cost)")
	}
	if perm.Flags["blitz"] == 1 {
		t.Error("declined blitz must NOT set the blitz flag")
	}
	if perm.Flags["kw:haste"] == 1 {
		t.Error("declined blitz must NOT grant haste")
	}
	FireDelayedTriggers(gs, "ending", "end")
	if findBlitzPerm(gs, 0, "Night Clubber") == nil {
		t.Error("a normally-cast creature must NOT be sacrificed at end of turn")
	}
	if countBlitzDraws(gs) != 0 {
		t.Error("declined blitz must not draw")
	}
}
