package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMagnusTheRed wires Magnus the Red.
//
// Oracle text (Scryfall, verified):
//
//	Flying
//	Unearthly Power — Instant and sorcery spells you cast cost {1} less
//	to cast for each creature token you control.
//	Blade of Magnus — Whenever Magnus the Red deals combat damage to a
//	player, create a 3/3 red Spawn creature token.
//
// Implementation (R36 stub port):
//   - Flying is AST keyword pipeline.
//   - "Unearthly Power" cost reduction static is engine territory
//     (ScanCostModifiers); emitPartial breadcrumb on ETB.
//   - "Blade of Magnus" — port the testable triggered ability:
//     OnTrigger("combat_damage_to_player") filtered to source_perm
//     == this Magnus. Create a 3/3 red Spawn creature token via the
//     standard token mint path so ETB triggers, token-created
//     observers, and zone accounting fire correctly.
func registerMagnusTheRed(r *Registry) {
	r.OnETB("Magnus the Red", magnusTheRedETB)
	r.OnTrigger("Magnus the Red", "combat_damage_to_player", magnusTheRedCombatDamage)
}

func magnusTheRedETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "magnus_the_red_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	// Unearthly Power cost reduction is wired in
	// gameengine/cost_modifiers.go (landed in a prior R49 batch).
}

func magnusTheRedCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "magnus_the_red_spawn_token"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	// Source filter: only fire when THIS Magnus dealt the combat damage.
	// Prefer source_perm pointer match; fall back to source_seat for
	// engines that don't thread the perm pointer through (defense in
	// depth — mirror Storm, Force of Nature's pattern).
	srcPerm, _ := ctx["source_perm"].(*gameengine.Permanent)
	if srcPerm == nil {
		// r63 self-gate fix: the combat_damage_player fire site threads only
		// source_seat + source_card (name), never a source_perm pointer, so
		// the old seat-only fallback minted a Spawn whenever ANY creature the
		// Magnus-controller controls connected. Require the source CARD NAME
		// to match this Magnus (mirrors Port Razer / Glissa / Sidar).
		ss, ok := ctx["source_seat"].(int)
		if !ok || ss != perm.Controller {
			return
		}
		if name, _ := ctx["source_card"].(string); name != perm.Card.DisplayName() {
			return
		}
	} else if srcPerm != perm {
		return
	}

	token := &gameengine.Card{
		Name:          "Spawn Token",
		Owner:         perm.Controller,
		BasePower:     3,
		BaseToughness: 3,
		Types:         []string{"token", "creature", "spawn"},
		Colors:        []string{"R"},
		TypeLine:      "Token Creature — Spawn",
	}
	enterBattlefieldWithETB(gs, perm.Controller, token, false)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"token": token.DisplayName(),
	})
}
