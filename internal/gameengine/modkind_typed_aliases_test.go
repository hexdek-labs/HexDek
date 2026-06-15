package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// modkind_typed_aliases_test.go — pins that parser "_typed" scaffold
// variants now resolve as their handled base ModKind instead of falling
// through to the parser_gap default.

func aliasTestPerm(gs *GameState, seat int, types ...string) *Permanent {
	p := &Permanent{
		Card:       &Card{Name: "Alias Source", Owner: seat, Types: append([]string{}, types...)},
		Controller: seat,
		Owner:      seat,
		Flags:      map[string]int{},
		Counters:   map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func resolveModKind(gs *GameState, src *Permanent, kind string, args ...interface{}) {
	resolveModificationEffect(gs, src, &gameast.ModificationEffect{ModKind: kind, Args: args})
}

// No alias kind should ever leave a parser_gap flag — that is the
// universal signal that the kind fell through to the unhandled default.
func TestTypedAlias_NoParserGap_AllKinds(t *testing.T) {
	for kind := range typedAliasBase {
		gs := newTestGameState(2)
		// Give the source a creature type + a stocked library so
		// creature-gated mechanics (explore) and library mechanics
		// (discover/cloak) have something to act on.
		src := aliasTestPerm(gs, 0, "creature", "elf")
		for i := 0; i < 10; i++ {
			gs.Seats[0].Library = append(gs.Seats[0].Library, &Card{Name: "Stock", Owner: 0, Types: []string{"creature"}})
		}
		gs.Seats[0].Hand = append(gs.Seats[0].Hand, &Card{Name: "Hand", Owner: 0, Types: []string{"creature"}})

		args := []interface{}{}
		if kind == "discover_typed" {
			args = []interface{}{4}
		}
		resolveModKind(gs, src, kind, args...)

		if src.Flags["parser_gap"] != 0 {
			t.Errorf("kind %q fell through to parser_gap default (should re-dispatch to base)", kind)
		}
	}
}

func TestTypedAlias_BecomeMonarch(t *testing.T) {
	gs := newTestGameState(2)
	src := aliasTestPerm(gs, 1, "creature")
	resolveModKind(gs, src, "become_monarch_typed")
	if gs.Flags["has_monarch"] != 1 || gs.Flags["monarch_seat"] != 1 {
		t.Fatalf("become_monarch_typed should crown the controller (seat 1); has=%d seat=%d",
			gs.Flags["has_monarch"], gs.Flags["monarch_seat"])
	}
}

func TestTypedAlias_VentureDungeon(t *testing.T) {
	gs := newTestGameState(2)
	src := aliasTestPerm(gs, 0, "creature")
	resolveModKind(gs, src, "venture_dungeon_typed")
	if gs.Seats[0].Flags["dungeon_level"] != 1 {
		t.Fatalf("venture_dungeon_typed should advance the dungeon to level 1, got %d",
			gs.Seats[0].Flags["dungeon_level"])
	}
}

func TestTypedAlias_RingTempts(t *testing.T) {
	gs := newTestGameState(2)
	src := aliasTestPerm(gs, 0, "creature")
	resolveModKind(gs, src, "ring_tempts_typed")
	// r63: the resolve path now routes through the canonical TheRingTemptsYou,
	// which advances the cumulative ring LEVEL (read via GetRingLevel) and
	// designates a Ring-bearer — not the old dead `ring_temptation` counter.
	if got := GetRingLevel(gs, 0); got != 1 {
		t.Fatalf("ring_tempts_typed should raise the ring level to 1, got %d", got)
	}
	if src.Flags["ring_bearer"] != 1 {
		t.Fatal("ring_tempts_typed should designate the controller's creature as Ring-bearer")
	}
}

func TestTypedAlias_Explore_NonlandPutsCounter(t *testing.T) {
	gs := newTestGameState(2)
	src := aliasTestPerm(gs, 0, "creature", "elf")
	// Nonland on top of library → explore puts a +1/+1 counter on the creature.
	gs.Seats[0].Library = append([]*Card{{Name: "Top Nonland", Owner: 0, Types: []string{"creature"}}}, gs.Seats[0].Library...)
	resolveModKind(gs, src, "explore_typed")
	if src.Counters["+1/+1"] != 1 {
		t.Fatalf("explore_typed on a creature (nonland revealed) should add a +1/+1 counter, got %d",
			src.Counters["+1/+1"])
	}
}

// Re-dispatch must not recurse: the base kind is not itself an alias key.
func TestTypedAlias_BaseKindsAreNotAliasKeys(t *testing.T) {
	for _, base := range typedAliasBase {
		if _, isAlias := typedAliasBase[base]; isAlias {
			t.Errorf("base kind %q is also an alias key — would risk re-dispatch recursion", base)
		}
	}
}
