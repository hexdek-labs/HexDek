package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// afterlife_behavior_r63_test.go — behavioral regressions for Afterlife
// (CR §702.135). Afterlife was unwired (TriggerAfterlife had no caller) and
// minted its Spirit tokens with a raw loop that bypassed the token-doubler
// pipeline + token_created. These pin the dies-chokepoint wiring, the token
// characteristics, the doubler-aware count, and the death-specific trigger.

func afterlifeCreature(gs *GameState, seat int, name string, n int) *Permanent {
	p := addKWCombatBattlefield(gs, seat, name, 2, 2, "creature")
	p.Card.AST = &gameast.CardAST{
		Name:      name,
		Abilities: []gameast.Ability{&gameast.Keyword{Name: "afterlife", Args: []interface{}{float64(n)}}},
	}
	return p
}

func countSpirits(gs *GameState, seat int) []*Permanent {
	var out []*Permanent
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == "Spirit" {
			out = append(out, p)
		}
	}
	return out
}

// (a)+(e) Afterlife triggers when the creature DIES, creating its printed N
// tokens. Driven end-to-end through SacrificePermanent → the dies chokepoint.
func TestAfterlife_FiresOnDeathWithPrintedN(t *testing.T) {
	gs := newKWCombatGame(t)
	witness := afterlifeCreature(gs, 0, "Seraph of the Scales", 2)

	SacrificePermanent(gs, witness, "test")

	spirits := countSpirits(gs, 0)
	if len(spirits) != 2 {
		t.Fatalf("(a/e) afterlife 2 should create 2 Spirit tokens on death, got %d", len(spirits))
	}
}

// (b) The tokens are 1/1, white AND black, Spirit, with flying.
func TestAfterlife_TokenCharacteristics(t *testing.T) {
	gs := newKWCombatGame(t)
	w := afterlifeCreature(gs, 0, "Hunted Witness", 1)
	SacrificePermanent(gs, w, "test")

	spirits := countSpirits(gs, 0)
	if len(spirits) != 1 {
		t.Fatalf("setup: expected 1 spirit, got %d", len(spirits))
	}
	s := spirits[0]
	if s.Power() != 1 || s.Toughness() != 1 {
		t.Errorf("(b) Spirit token must be 1/1, got %d/%d", s.Power(), s.Toughness())
	}
	if !cardHasType(s.Card, "spirit") {
		t.Error("(b) token must be a Spirit")
	}
	if !s.HasKeyword("flying") {
		t.Error("(b) Spirit token must have flying")
	}
	white, black := false, false
	for _, c := range s.Card.Colors {
		if c == "W" {
			white = true
		}
		if c == "B" {
			black = true
		}
	}
	if !white || !black {
		t.Errorf("(b) Spirit token must be white AND black, got colors %v", s.Card.Colors)
	}
}

// (c) The token count respects token doublers (Doubling Season) and fires
// token_created.
func TestAfterlife_RespectsTokenDoublers(t *testing.T) {
	gs := newKWCombatGame(t)
	ds := addKWCombatBattlefield(gs, 0, "Doubling Season", 0, 0, "enchantment")
	RegisterDoublingSeason(gs, ds)
	w := afterlifeCreature(gs, 0, "Tithe Taker", 1)

	tokensBefore := gs.Seats[0].Turn.TokensCreated
	SacrificePermanent(gs, w, "test")

	spirits := countSpirits(gs, 0)
	if len(spirits) != 2 {
		t.Errorf("(c) afterlife 1 under Doubling Season should make 2 Spirits, got %d", len(spirits))
	}
	if gs.Seats[0].Turn.TokensCreated <= tokensBefore {
		t.Error("(c) afterlife must route through the canonical token creator (token_created / Turn counters)")
	}
}

// (d) Afterlife triggers on DEATH specifically — not on other zone changes
// (e.g. bouncing the creature to hand).
func TestAfterlife_OnlyOnDeath(t *testing.T) {
	gs := newKWCombatGame(t)
	w := afterlifeCreature(gs, 0, "Tithe Taker", 2)

	// Leave the battlefield to HAND (not the graveyard) — not a death.
	FireZoneChangeTriggers(gs, w, w.Card, "battlefield", "hand")

	if n := len(countSpirits(gs, 0)); n != 0 {
		t.Errorf("(d) afterlife must NOT fire on a non-death zone change (battlefield→hand); got %d spirits", n)
	}
}
