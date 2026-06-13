package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// elvish_warmaster.go — per_card handler for Elvish Warmaster.
//
// Oracle text (Creature — Elf Warrior, {1}{G}):
//
//	Whenever one or more other Elves you control enter, create a 1/1
//	green Elf Warrior creature token. This ability triggers only once
//	each turn.
//	{5}{G}{G}: Elves you control get +2/+2 and gain deathtouch until end
//	of turn.
//
// Why this needs a per_card handler: the ability parsed to inert
// scaffold, so the token engine never fired. Verified inert: no
// registered handler for "Elvish Warmaster".
//
// Implemented here: the once-per-turn "other Elf you control enters →
// 1/1 green Elf Warrior" trigger (the recurring value engine). The
// activated {5}{G}{G} overrun pump is deferred (logged) — it is an
// expensive, situational finisher rather than the card's identity.
//
// Note on "one or more": the engine fires permanent_etb per entering
// permanent; the once-per-turn gate (perm.Flags keyed to the turn
// number) collapses a multi-Elf entry into a single token, matching the
// "triggers only once each turn" clause.

func init() {
	registerElvishWarmaster(Global())
	AddResetHook(registerElvishWarmaster)
}

func registerElvishWarmaster(r *Registry) {
	if r == nil {
		return
	}
	r.OnTrigger("Elvish Warmaster", "permanent_etb", elvishWarmasterETB)
}

func elvishWarmasterETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "elvish_warmaster"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil || entering.Card == nil {
		return
	}
	// "other Elves you control" — same controller, an Elf, not Warmaster.
	if entering == perm {
		return
	}
	enteringSeat, _ := ctx["controller_seat"].(int)
	if enteringSeat != perm.Controller {
		return
	}
	if !entering.IsCreature() || !cardHasType(entering.Card, "elf") {
		return
	}

	// "Triggers only once each turn."
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["warmaster_token_turn"] == gs.Turn+1 {
		return
	}
	perm.Flags["warmaster_token_turn"] = gs.Turn + 1 // +1 so turn 0 isn't the zero-value "not yet fired"

	token := &gameengine.Card{
		Name:          "Elf Warrior Token",
		Owner:         perm.Controller,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "creature", "elf", "warrior"},
		Colors:        []string{"G"},
		TypeLine:      "Token Creature — Elf Warrior",
	}
	enterBattlefieldWithETB(gs, perm.Controller, token, false)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"entering": entering.Card.DisplayName(),
	})
}
