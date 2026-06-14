package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// CR §702.137b: magecraft triggers "whenever you cast OR COPY an instant
// or sorcery spell." Storm makes copies (§707.10). Regression for the r63
// prowess/magecraft probe: ApplyStormCopy pushed the copies but never
// fired magecraft, so a magecraft permanent triggered only on the single
// original cast and silently missed every storm copy.

func smc_addMagecraftPerm(gs *GameState, seat int, name string) *Permanent {
	c := &Card{
		Name:  name,
		Owner: seat,
		Types: []string{"creature"},
		AST: &gameast.CardAST{
			Name:      name,
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "magecraft"}},
		},
	}
	p := &Permanent{
		Card: c, Controller: seat, Owner: seat,
		Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func smc_countMageEvents(gs *GameState, kind string) int {
	n := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == kind {
			n++
		}
	}
	return n
}

func TestStorm_FiresMagecraftPerCopy(t *testing.T) {
	gs := newStormGame(t)
	smc_addMagecraftPerm(gs, 0, "Archmage Emeritus")

	original := stormStackItem(stormSpellCard("Grapeshot"), 0) // instant
	gs.Stack = append(gs.Stack, original)

	if got := ApplyStormCopy(gs, original, 3); got != 3 {
		t.Fatalf("expected 3 storm copies, got %d", got)
	}
	// One magecraft permanent, 3 copies → 3 magecraft triggers (the
	// original cast's magecraft fires on the cast path, not here).
	if got := smc_countMageEvents(gs, "magecraft_trigger"); got != 3 {
		t.Errorf("expected 3 magecraft_trigger events (one per storm copy), got %d", got)
	}
}

// Two magecraft permanents × 2 copies → 4 triggers (cardinality).
func TestStorm_MagecraftCardinalityMultiplePermanents(t *testing.T) {
	gs := newStormGame(t)
	smc_addMagecraftPerm(gs, 0, "Archmage Emeritus")
	smc_addMagecraftPerm(gs, 0, "Storm-Kiln Artist")

	original := stormStackItem(stormSpellCard("Grapeshot"), 0)
	gs.Stack = append(gs.Stack, original)

	ApplyStormCopy(gs, original, 2)
	if got := smc_countMageEvents(gs, "magecraft_trigger"); got != 4 {
		t.Errorf("2 magecraft perms × 2 copies → want 4 triggers, got %d", got)
	}
}

// A storm copy of a NON-instant/sorcery spell must NOT trigger magecraft.
func TestStorm_MagecraftIgnoresNonInstantSorceryCopy(t *testing.T) {
	gs := newStormGame(t)
	smc_addMagecraftPerm(gs, 0, "Archmage Emeritus")

	// An artifact spell carrying storm (e.g. a hypothetical artifact with
	// storm) — copies are NOT instant/sorcery, so magecraft stays silent.
	artifact := &Card{
		Name: "Storm Artifact", Owner: 0, CMC: 1, Types: []string{"artifact"},
		AST: &gameast.CardAST{Name: "Storm Artifact",
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "storm"}}},
	}
	original := stormStackItem(artifact, 0)
	gs.Stack = append(gs.Stack, original)

	ApplyStormCopy(gs, original, 3)
	if got := smc_countMageEvents(gs, "magecraft_trigger"); got != 0 {
		t.Errorf("non-instant/sorcery storm copies must not trigger magecraft, got %d", got)
	}
}

// Property (d)-adjacent: magecraft is "you" — an OPPONENT's magecraft
// permanent does not trigger off the caster's storm copies.
func TestStorm_MagecraftOnlyCasterControlled(t *testing.T) {
	gs := newStormGame(t)
	// Magecraft permanent controlled by the OPPONENT (seat 1).
	smc_addMagecraftPerm(gs, 1, "Opponent Archmage")

	original := stormStackItem(stormSpellCard("Grapeshot"), 0) // cast by seat 0
	gs.Stack = append(gs.Stack, original)

	ApplyStormCopy(gs, original, 3)
	if got := smc_countMageEvents(gs, "magecraft_trigger"); got != 0 {
		t.Errorf("opponent's magecraft must not trigger off the caster's copies, got %d", got)
	}
}
