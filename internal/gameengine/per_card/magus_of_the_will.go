package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMagusOfTheWill wires Magus of the Will.
//
// Oracle text (Double Masters 2020, verified via hexdek.dev oracle
// endpoint 2026-05-22):
//
//	{2}{B}
//	Creature — Human Wizard
//	{2}{B}, {T}, Exile this creature: Until end of turn, you may play
//	lands and cast spells from your graveyard. If a card would be put
//	into your graveyard from anywhere this turn, exile that card
//	instead.
//
// Effectively a Yawgmoth's Will on a stick.
//
// Implementation:
//   - OnActivated abilityIdx 0: pays {2}{B} from the mana pool, taps
//     Magus, exiles him from the battlefield, then calls
//     gameengine.ActivatedFlashbackGrant in AllInZone mode so every
//     instant/sorcery card in the activator's graveyard becomes
//     flashback-castable until end of turn (the closest the engine's
//     current zone-cast machinery comes to "cast spells from your
//     graveyard"). EOT cleanup is automatic via Duration="until_end_of_
//     turn" expiry in ExpireZoneCastGrants.
//   - emitPartial documents the parts not modeled:
//       * permanent-card spells from the graveyard (Yawg Will lets you
//         cast creatures/artifacts/enchantments/planeswalkers too — our
//         flashback grant only covers instants/sorceries)
//       * "play lands from your graveyard"
//       * "if a card would be put into your graveyard from anywhere this
//         turn, exile that card instead" — a replacement effect that
//         needs registration on the graveyard zone-change pipeline.
func registerMagusOfTheWill(r *Registry) {
	r.OnActivated("Magus of the Will", magusOfTheWillActivate)
}

func magusOfTheWillActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "magus_of_the_will_yawg_will_grant"
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
	const cost = 3 // {2}{B}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	if seat.ManaPool < cost {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"required": cost,
			"have":     seat.ManaPool,
		})
		return
	}
	seat.ManaPool -= cost
	gameengine.SyncManaAfterSpend(seat)
	src.Tapped = true

	// Exile-self is a cost — perform it BEFORE the effect resolves so
	// that Magus himself is not eligible for the grant (he won't be in
	// the graveyard anyway, but order matters for log clarity).
	if !gameengine.ExilePermanent(gs, src, src) {
		emitFail(gs, slug, src.Card.DisplayName(), "could_not_exile_self", nil)
		return
	}

	granted := gameengine.ActivatedFlashbackGrant(gs, gameengine.ActivatedFlashbackGrantOptions{
		Source:    src.Card.DisplayName(),
		Seat:      src.Controller,
		AllInZone: true,
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":            src.Controller,
		"granted_count":   len(granted),
		"yawg_will_proxy": "flashback_grant_all_instants_and_sorceries",
	})
	emitPartial(gs, slug, src.Card.DisplayName(),
		"play_lands_from_graveyard_and_cast_permanents_from_graveyard_not_modeled")
	emitPartial(gs, slug, src.Card.DisplayName(),
		"graveyard_bound_cards_exile_instead_replacement_not_registered")
}
