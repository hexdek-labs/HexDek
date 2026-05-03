package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Batch #19 — enchantment toolbox commanders.
//
// Both handlers fire on creature_attacks. Each searches the controller's
// library for the eligible card with the highest CMC and puts it onto
// the battlefield via gameengine.MoveCard, then shuffles.

// ---------------------------------------------------------------------------
// Zur the Enchanter
//
// Oracle text:
//   Whenever Zur the Enchanter attacks, you may search your library for
//   an enchantment card with mana value 3 or less, put it onto the
//   battlefield, then shuffle.
// ---------------------------------------------------------------------------

func registerZurTheEnchanter(r *Registry) {
	r.OnTrigger("Zur the Enchanter", "creature_attacks", zurAttackTrigger)
}

func zurAttackTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "zur_the_enchanter_attack"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	attackerPerm, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if attackerPerm != perm {
		return
	}
	if perm.Card.DisplayName() != "Zur the Enchanter" {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	bestIdx := -1
	bestCMC := -1
	for i, c := range s.Library {
		if c == nil || !cardHasType(c, "enchantment") {
			continue
		}
		if c.CMC > 3 {
			continue
		}
		if c.CMC > bestCMC {
			bestCMC = c.CMC
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		shuffleLibraryPerCard(gs, seat)
		emitFail(gs, slug, perm.Card.DisplayName(), "no_enchantment_cmc_le_3", nil)
		return
	}

	card := s.Library[bestIdx]
	gameengine.MoveCard(gs, card, seat, "library", "battlefield", "zur_the_enchanter")
	enterBattlefieldWithETB(gs, seat, card, false)
	shuffleLibraryPerCard(gs, seat)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  seat,
		"found": card.DisplayName(),
		"cmc":   card.CMC,
	})
}

// ---------------------------------------------------------------------------
// Light-Paws, Emperor's Voice — moved to light_paws_emperors_voice.go
// (ETB-triggered Aura tutor, replaces the old attack-trigger stub).
// ---------------------------------------------------------------------------
