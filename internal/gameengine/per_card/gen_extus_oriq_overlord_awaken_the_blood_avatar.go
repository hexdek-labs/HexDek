package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerExtusOriqOverlordAwakenTheBloodAvatar wires Extus, Oriq
// Overlord // Awaken the Blood Avatar.
//
// Oracle text (Scryfall, verified, front face Extus):
//
//	Double strike
//	Magecraft — Whenever you cast or copy an instant or sorcery
//	spell, return target nonlegendary creature card from your
//	graveyard to your hand.
//
// Back face (Awaken the Blood Avatar) — alt-cast sorcery that
// sacrifices a 3/6 Avatar token plus an opponent-sac rider — is a
// separate spell card pathway; per_card layer only sees the front
// face permanent. The back-face mass-attack rider stays on the
// stack-resolve surface and is emitPartial here.
//
// Implementation (R44 stub port):
//   - Double strike: AST keyword pipeline.
//   - Magecraft trigger via "instant_or_sorcery_cast" (the
//     keywords_magecraft.go canonical surface — fires on both real
//     casts and copies). Gated on caster_seat == controller. Picks
//     the highest-CMC nonlegendary creature card in the controller's
//     graveyard and returns it to hand via MoveCard. Mirrors the
//     custom_feather pattern for graveyard returns at trigger time.
//   - Back-face Awaken the Blood Avatar token creation + opponent
//     sac rider: emitPartial breadcrumb on ETB (separate spell card
//     surface; not exercised by the front-face permanent).
func registerExtusOriqOverlordAwakenTheBloodAvatar(r *Registry) {
	r.OnETB("Extus, Oriq Overlord // Awaken the Blood Avatar", extusOriqOverlordETB)
	r.OnTrigger("Extus, Oriq Overlord // Awaken the Blood Avatar",
		"instant_or_sorcery_cast", extusMagecraftReturnCreature)
	// Some corpora carry the front-face name without the "//" half.
	r.OnETB("Extus, Oriq Overlord", extusOriqOverlordETB)
	r.OnTrigger("Extus, Oriq Overlord",
		"instant_or_sorcery_cast", extusMagecraftReturnCreature)
}

func extusOriqOverlordETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "extus_oriq_overlord_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"back_face_awaken_avatar_token_and_opponent_sac_rider_separate_card_surface_not_modeled")
}

func extusMagecraftReturnCreature(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "extus_magecraft_return_creature"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	s := gs.Seats[perm.Controller]
	if s == nil || s.Lost {
		return
	}

	bestIdx := -1
	bestCMC := -1
	for i, c := range s.Graveyard {
		if c == nil {
			continue
		}
		if !cardHasType(c, "creature") {
			continue
		}
		if cardHasType(c, "legendary") {
			continue
		}
		cmc := gameengine.ManaCostOf(c)
		if cmc > bestCMC {
			bestCMC = cmc
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_nonlegendary_creature_in_graveyard", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	card := s.Graveyard[bestIdx]
	gameengine.MoveCard(gs, card, perm.Controller, "graveyard", "hand", "extus_magecraft_return")

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"returned": card.DisplayName(),
		"cmc":      bestCMC,
	})
}
