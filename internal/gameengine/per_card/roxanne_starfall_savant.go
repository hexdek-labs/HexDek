package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRoxanneStarfallSavant wires Roxanne, Starfall Savant.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	Whenever Roxanne enters or attacks, create a tapped colorless
//	artifact token named Meteorite with "When this token enters, it
//	deals 2 damage to any target" and "{T}: Add one mana of any
//	color."
//	Whenever you tap an artifact token for mana, add one mana of any
//	type that artifact token produced.
//
// Implementation:
//   - ETB and "creature_attacks" both call roxanneMintMeteorite, which
//     creates a tapped Meteorite artifact token for Roxanne's controller.
//     The Meteorite ETB damage clause shoots the lowest-life opponent
//     for 2 damage.
//   - "artifact_tapped_for_mana" trigger doubles the mana production
//     for Roxanne's controller — we emit a "roxanne_doubled_mana" event
//     for the mana pool layer to honor.
func registerRoxanneStarfallSavant(r *Registry) {
	r.OnETB("Roxanne, Starfall Savant", roxanneETB)
	r.OnTrigger("Roxanne, Starfall Savant", "creature_attacks", roxanneAttacks)
	r.OnTrigger("Roxanne, Starfall Savant", "artifact_tapped_for_mana", roxanneArtifactTapped)
}

func roxanneETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	roxanneMintMeteorite(gs, perm, "etb")
}

func roxanneAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	roxanneMintMeteorite(gs, perm, "attack")
}

func roxanneMintMeteorite(gs *gameengine.GameState, perm *gameengine.Permanent, source string) {
	const slug = "roxanne_mint_meteorite"
	tok := &gameengine.Card{
		Name:  "Meteorite",
		Owner: perm.Controller,
		Types: []string{"token", "artifact"},
	}
	tokenPerm := createPermanent(gs, perm.Controller, tok, true)
	if tokenPerm == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "token_create_failed", nil)
		return
	}
	gameengine.RegisterReplacementsForPermanent(gs, tokenPerm)
	gameengine.FirePermanentETBTriggers(gs, tokenPerm)

	// Inline the Meteorite ETB damage clause.
	tgt := -1
	bestLife := 1 << 30
	for _, oppIdx := range gs.Opponents(perm.Controller) {
		s := gs.Seats[oppIdx]
		if s == nil || s.Lost {
			continue
		}
		if s.Life < bestLife {
			bestLife = s.Life
			tgt = oppIdx
		}
	}
	if tgt >= 0 {
		gameengine.DealDamage(gs, tgt, 2, "Meteorite")
		_ = gs.CheckEnd()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"trigger":        source,
		"meteorite_hit":  tgt,
	})
}

func roxanneArtifactTapped(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "roxanne_artifact_token_double_mana"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// Engine fires artifact_tapped_for_mana with controller_seat + perm +
	// card + pips + artifact_name (mana_artifacts.go:125). Legacy "seat"
	// + "mana_type" reads kept as fallbacks.
	tapperSeat, ok := ctx["controller_seat"].(int)
	if !ok {
		tapperSeat, _ = ctx["seat"].(int)
	}
	if tapperSeat != perm.Controller {
		return
	}
	tappedPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if tappedPerm == nil || tappedPerm.Card == nil {
		return
	}
	if !cardHasType(tappedPerm.Card, "token") || !cardHasType(tappedPerm.Card, "artifact") {
		return
	}
	pips, _ := ctx["pips"].(int)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"token": tappedPerm.Card.DisplayName(),
		"pips":  pips,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"doubled_mana_payload_not_added_to_pool_directly_by_handler")
}
