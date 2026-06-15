package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func creatureTokenCount(gs *GameState, seat int) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.IsToken() && p.IsCreature() {
			n++
		}
	}
	return n
}

func doPopulate(gs *GameState, src *Permanent) {
	ResolveEffect(gs, src, &gameast.ModificationEffect{ModKind: "populate"})
}

// (a)/(b)/(c) Populate copies a creature token's COPIABLE values (base P/T, no
// counters), and the result is itself a token.
func TestPopulate_CopiesCopiableValuesResultIsToken(t *testing.T) {
	gs := newCombatGame(t)
	src := addBattlefield(gs, 0, "Trostani", 0, 0, "enchantment")
	tok := addBattlefield(gs, 0, "Saproling", 1, 1, "token", "creature", "saproling")
	tok.AddCounter("+1/+1", 2) // current 3/3, but base/copiable is 1/1

	doPopulate(gs, src)

	if creatureTokenCount(gs, 0) != 2 {
		t.Fatalf("populate should create one new creature token (total 2), got %d", creatureTokenCount(gs, 0))
	}
	// Find the new copy (not the original).
	var copyPerm *Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p != tok && p.IsToken() && p.IsCreature() {
			copyPerm = p
		}
	}
	if copyPerm == nil {
		t.Fatal("no populated copy found")
	}
	if !copyPerm.IsToken() {
		t.Fatal("the populated copy must itself be a token (CR §701.32)")
	}
	if copyPerm.Card.BasePower != 1 || copyPerm.Card.BaseToughness != 1 {
		t.Fatalf("copy should use copiable base P/T 1/1, got %d/%d", copyPerm.Card.BasePower, copyPerm.Card.BaseToughness)
	}
	if copyPerm.Counters["+1/+1"] != 0 {
		t.Fatalf("copy must NOT inherit the original's +1/+1 counters, got %d", copyPerm.Counters["+1/+1"])
	}
}

// (d) No creature tokens → populate does nothing.
func TestPopulate_NoCreatureTokens_DoesNothing(t *testing.T) {
	gs := newCombatGame(t)
	src := addBattlefield(gs, 0, "Trostani", 0, 0, "enchantment")
	addBattlefield(gs, 0, "Real Bear", 2, 2, "creature") // a non-token creature

	doPopulate(gs, src)

	if creatureTokenCount(gs, 0) != 0 {
		t.Fatalf("populate with no creature tokens must create nothing, got %d", creatureTokenCount(gs, 0))
	}
}

// (g) Token doublers (Doubling Season) double the populated copy.
func TestPopulate_RespectsTokenDoublers(t *testing.T) {
	gs := newCombatGame(t)
	src := addBattlefield(gs, 0, "Trostani", 0, 0, "enchantment")
	addBattlefield(gs, 0, "Saproling", 1, 1, "token", "creature")
	addDoublerSource(gs, 0, "Doubling Season", RegisterDoublingSeason)

	doPopulate(gs, src)

	// Original + 2 copies (Doubling Season doubles the one populated token).
	if creatureTokenCount(gs, 0) != 3 {
		t.Fatalf("populate with Doubling Season should make 2 copies (total 3 tokens), got %d", creatureTokenCount(gs, 0))
	}
}

// (f) Populate fires the token_created trigger (token-matters payoffs).
func TestPopulate_FiresTokenCreatedTrigger(t *testing.T) {
	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	fired := 0
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev == "token_created" {
			fired++
		}
	}
	gs := newCombatGame(t)
	src := addBattlefield(gs, 0, "Trostani", 0, 0, "enchantment")
	addBattlefield(gs, 0, "Saproling", 1, 1, "token", "creature")

	doPopulate(gs, src)

	if fired == 0 {
		t.Fatal("populate should fire the token_created trigger for the new token")
	}
}
