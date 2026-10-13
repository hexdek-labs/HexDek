package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// r64 (second cluster) — until-EOT keyword grants never expired.
//
// Handlers granted a temporary keyword by writing
// Permanent.Flags["kw:<name>"] = 1, usually beside a marker flag named
// for the duration ("double_strike_eot", "kw_vigilance_until_eot",
// "trample_until_eot"). The grant half worked — HasKeyword reads that
// flag — but nothing reads the markers, and the §514.2 cleanup sweep
// clears GrantedAbilities and Modifications, never the Flags map.
//
// So the keyword was granted for the rest of the game. Rafiq's exalted
// double strike, Strider's first strike and Rosa's lifelink all stuck.
//
// These tests assert both halves: present during the turn, gone after
// cleanup.
// ---------------------------------------------------------------------------

func TestKeywordEOTR64_RafiqDoubleStrikeExpires(t *testing.T) {
	gs := newGame(t, 2)
	rafiq := addPerm(gs, 0, "Rafiq of the Many", "creature", "legendary", "human", "knight")
	rafiq.Card.BasePower, rafiq.Card.BaseToughness = 3, 3
	lone := addPerm(gs, 0, "Sword Dancer", "creature", "human")
	lone.Card.BasePower, lone.Card.BaseToughness = 2, 2
	lone.Flags["attacking"] = 1
	lone.Flags["declared_attacker_this_combat"] = 1

	gameengine.FireCardTrigger(gs, "declare_attackers", map[string]interface{}{
		"attacker_perm": lone,
		"seat":          0,
	})
	if !lone.HasKeyword("double strike") {
		t.Fatal("exalted attacker should have double strike for the turn")
	}

	gameengine.ExpireKeywordGrantsEOT(gs)
	if lone.HasKeyword("double strike") {
		t.Error("double strike must be gone after the cleanup step (CR §514.2)")
	}
}

func TestKeywordEOTR64_RosaLifelinkExpires(t *testing.T) {
	gs := newGame(t, 2)
	rosa := addPerm(gs, 0, "Rosa, Resolute White Mage", "creature", "legendary", "human", "cleric")
	rosa.Card.BasePower, rosa.Card.BaseToughness = 2, 3
	ally := addPerm(gs, 0, "Savannah Lions", "creature", "cat")
	ally.Card.BasePower, ally.Card.BaseToughness = 2, 1

	rosaCombatBegin(gs, rosa, map[string]interface{}{"active_seat": 0})

	var target *gameengine.Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p.HasKeyword("lifelink") {
			target = p
		}
	}
	if target == nil {
		t.Fatal("Rosa should grant lifelink to a creature")
	}

	gameengine.ExpireKeywordGrantsEOT(gs)
	if target.HasKeyword("lifelink") {
		t.Error("lifelink must be gone after cleanup")
	}
	if target.Counters["+1/+1"] != 1 {
		t.Errorf("the +1/+1 counter is permanent and must survive cleanup, got %v",
			target.Counters)
	}
}

func TestKeywordEOTR64_PhelddagrifTrampleExpires(t *testing.T) {
	gs := newGame(t, 2)
	phel := addPerm(gs, 0, "Phelddagrif", "creature", "legendary", "hippo")
	phel.Card.BasePower, phel.Card.BaseToughness = 4, 4

	phelddagrifTrampleAndHippo(gs, phel, map[string]interface{}{})
	if !phel.HasKeyword("trample") {
		t.Fatal("Phelddagrif should have trample for the turn")
	}

	gameengine.ExpireKeywordGrantsEOT(gs)
	if phel.HasKeyword("trample") {
		t.Error("trample must be gone after cleanup")
	}
}

func TestKeywordEOTR64_PermanentKeywordFlagsSurviveCleanup(t *testing.T) {
	gs := newGame(t, 2)
	// A token minted WITH flying, or a face-down permanent's disguise
	// ward, sets Flags["kw:<name>"] with no expiry marker. The sweep must
	// not touch those — stripping a keyword that was never temporary
	// would be a worse bug than the one being fixed.
	tok := addPerm(gs, 0, "Spirit Token", "creature", "token", "spirit")
	tok.Card.BasePower, tok.Card.BaseToughness = 1, 1
	tok.Flags["kw:flying"] = 1

	gameengine.ExpireKeywordGrantsEOT(gs)

	if !tok.HasKeyword("flying") {
		t.Error("a permanent keyword flag with no kw_eot marker must survive cleanup")
	}
}

func TestKeywordEOTR64_TajicBattalionPumpIsReal(t *testing.T) {
	gs := newGame(t, 2)
	tajic := addPerm(gs, 0, "Tajic, Blade of the Legion", "creature", "legendary", "human", "soldier")
	tajic.Card.BasePower, tajic.Card.BaseToughness = 2, 3
	tajic.Flags["attacking"] = 1
	for i, n := range []string{"Boros Elite", "Daring Skyjek", "Wojek Halberdiers"} {
		p := addPerm(gs, 0, n, "creature", "human", "soldier")
		p.Card.BasePower, p.Card.BaseToughness = 2, 2
		p.Flags["attacking"] = 1
		_ = i
	}

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": tajic,
		"seat":          0,
	})

	if p, tg := pt(gs, tajic); p != 7 || tg != 8 {
		t.Errorf("battalion should make Tajic 7/8 (base 2/3 +5/+5), got %d/%d", p, tg)
	}
}
