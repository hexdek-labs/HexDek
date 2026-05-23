package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDralnuLichLord wires Dralnu, Lich Lord.
//
// Oracle text (Time Spiral Remastered, verified via hexdek.dev oracle
// endpoint 2026-05-22):
//
//	{3}{U}{B}
//	Legendary Creature — Zombie Wizard
//	If damage would be dealt to Dralnu, sacrifice that many permanents
//	instead.
//	{T}: Target instant or sorcery card in your graveyard gains
//	flashback until end of turn. The flashback cost is equal to its
//	mana cost.
//
// Implementation:
//   - OnActivated abilityIdx 0 ({T} flashback grant): pays the tap cost,
//     calls gameengine.ActivatedFlashbackGrant with the
//     ctx["target_card"] hint (auto-picks the highest-CMC i/s in seat's
//     graveyard if no target supplied). Logs activated_flashback_grant.
//     EOT cleanup is handled automatically by ExpireZoneCastGrants via
//     the Duration="until_end_of_turn" set by GrantFlashbackUntilEOT.
//   - "If damage would be dealt to Dralnu, sacrifice that many permanents
//     instead" is a replacement effect on damage assignment — engine-side
//     damage routing doesn't expose a per-card hook to swap damage for a
//     sacrifice-N replacement; emitPartial.
func registerDralnuLichLord(r *Registry) {
	r.OnActivated("Dralnu, Lich Lord", dralnuLichLordActivate)
}

func dralnuLichLordActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "dralnu_lich_lord_flashback_grant"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	src.Tapped = true

	var target *gameengine.Card
	if v, ok := ctx["target_card"].(*gameengine.Card); ok {
		target = v
	}
	granted := gameengine.ActivatedFlashbackGrant(gs, gameengine.ActivatedFlashbackGrantOptions{
		Source: src.Card.DisplayName(),
		Seat:   src.Controller,
		Target: target,
	})
	if len(granted) == 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "no_instant_or_sorcery_in_graveyard", nil)
		return
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":   src.Controller,
		"target": granted[0].DisplayName(),
	})
	emitPartial(gs, slug, src.Card.DisplayName(),
		"damage_replacement_sacrifice_n_permanents_not_modeled")
}
