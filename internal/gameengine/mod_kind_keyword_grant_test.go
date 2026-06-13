package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// mod_kind_keyword_grant_test.go — regressions for the generic
// keyword_grant anthem handler (worker hex-dev-5).

func kwAnthemLord(gs *GameState, seat, pow, tough int, kw, scope string) *Permanent {
	args := []interface{}{kw}
	if scope != "" {
		args = append(args, scope)
	}
	ast := &gameast.CardAST{
		Name: "Lord",
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{
				ModKind: "keyword_grant", Args: args, Layer: "",
			}},
		},
	}
	lord := addBattlefieldWithAST(gs, seat, "Lord", pow, tough, ast, "creature")
	RegisterContinuousEffectsForPermanent(gs, lord)
	return lord
}

func TestKeywordGrant_OtherCreaturesYouControl(t *testing.T) {
	gs := newFixtureGame(t)
	lord := kwAnthemLord(gs, 0, 4, 4, "trample", "other") // Aggressive Mammoth
	buddy := addBattlefield(gs, 0, "Buddy", 2, 2, "creature")
	opp := addBattlefield(gs, 1, "Opp", 2, 2, "creature")

	if !gs.HasKeywordOf(buddy, "trample") {
		t.Errorf("other creature you control should have trample")
	}
	if gs.HasKeywordOf(lord, "trample") {
		t.Errorf("'other' scope must NOT grant to the source lord")
	}
	if gs.HasKeywordOf(opp, "trample") {
		t.Errorf("opponent creature must not be granted trample")
	}
}

func TestKeywordGrant_AllYourCreaturesIncludesSelf(t *testing.T) {
	gs := newFixtureGame(t)
	// Rage Reflection: "Creatures you control have double strike" (no scope).
	lord := kwAnthemLord(gs, 0, 0, 0, "double strike", "")
	buddy := addBattlefield(gs, 0, "Buddy", 2, 2, "creature")
	if !gs.HasKeywordOf(buddy, "double strike") || !gs.HasKeywordOf(lord, "double strike") {
		t.Errorf("no-scope grant should include all your creatures (incl. source)")
	}
}

func TestKeywordGrant_DynamicNewCreature(t *testing.T) {
	gs := newFixtureGame(t)
	kwAnthemLord(gs, 0, 2, 2, "haste", "other")
	later := addBattlefield(gs, 0, "Latecomer", 1, 1, "creature")
	gs.InvalidateCharacteristicsCache()
	if !gs.HasKeywordOf(later, "haste") {
		t.Errorf("a creature entering later should pick up the granted keyword")
	}
}

func TestKeywordGrant_NonEvergreenSkipped(t *testing.T) {
	gs := newFixtureGame(t)
	kwAnthemLord(gs, 0, 2, 2, "total power 8 or greater", "other")
	buddy := addBattlefield(gs, 0, "Buddy", 2, 2, "creature")
	if gs.HasKeywordOf(buddy, "total power 8 or greater") {
		t.Errorf("non-evergreen / garbage keyword must be skipped, not granted")
	}
}
