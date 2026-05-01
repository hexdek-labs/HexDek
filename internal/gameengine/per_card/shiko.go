package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerShiko wires Shiko, Paragon of the Way.
//
// Oracle text:
//
//	Flying, vigilance
//	When Shiko enters, exile target nonland card with mana value 3 or
//	less from your graveyard. Copy it, then you may cast the copy
//	without paying its mana cost. (A copy of a permanent spell becomes
//	a token.)
//
// Implementation:
//   - Flying/vigilance are AST keywords; no per-card hook.
//   - ETB picks the highest-CMC nonland card (CMC <= 3) from Shiko's
//     controller's graveyard that we know how to "free-cast." Permanent
//     types short-circuit to a token entering the battlefield via the
//     standard ETB cascade. Instant/sorcery free-cast resolution is not
//     yet implemented engine-wide (see etali.go) — emitPartial when the
//     best target is a non-permanent.
//   - The original card is exiled per oracle text regardless of whether
//     the copy could be cast.
func registerShiko(r *Registry) {
	r.OnETB("Shiko, Paragon of the Way", shikoETB)
}

func shikoETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "shiko_paragon_etb_copy_cast"
	if gs == nil || perm == nil {
		return
	}
	controller := perm.Controller
	if controller < 0 || controller >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[controller]
	if seat == nil {
		return
	}

	// Choose the best nonland card with CMC <= 3 in the graveyard. Prefer
	// a permanent-type target (we can resolve those); fall back to the
	// best instant/sorcery for an emitPartial.
	var bestPerm *gameengine.Card
	bestPermCMC := -1
	var bestSpell *gameengine.Card
	bestSpellCMC := -1
	for _, c := range seat.Graveyard {
		if c == nil || cardHasType(c, "land") {
			continue
		}
		cmc := gameengine.ManaCostOf(c)
		if cmc > 3 {
			continue
		}
		if isPermanentCard(c) {
			if cmc > bestPermCMC {
				bestPermCMC = cmc
				bestPerm = c
			}
			continue
		}
		if cardHasType(c, "instant") || cardHasType(c, "sorcery") {
			if cmc > bestSpellCMC {
				bestSpellCMC = cmc
				bestSpell = c
			}
		}
	}

	target := bestPerm
	cmc := bestPermCMC
	isPermTarget := bestPerm != nil
	if target == nil {
		target = bestSpell
		cmc = bestSpellCMC
	}
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_eligible_card_in_graveyard", map[string]interface{}{
			"seat": controller,
		})
		return
	}

	// Exile the original card.
	gameengine.MoveCard(gs, target, controller, "graveyard", "exile", "shiko_etb_exile")

	if !isPermTarget {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   controller,
			"exiled": target.DisplayName(),
			"cmc":    cmc,
			"copied": false,
		})
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"instant_or_sorcery_free_cast_resolution_shortcut_unimplemented")
		return
	}

	// Permanent: a copy of a permanent spell becomes a token. Build a
	// token clone of the exiled card and run it through the full ETB
	// cascade. The original stays in exile.
	token := target.DeepCopy()
	token.Owner = controller
	token.IsCopy = true
	hasToken := false
	for _, t := range token.Types {
		if t == "token" {
			hasToken = true
			break
		}
	}
	if !hasToken {
		token.Types = append(token.Types, "token")
	}
	enterBattlefieldWithETB(gs, controller, token, false)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         controller,
		"exiled":       target.DisplayName(),
		"cmc":          cmc,
		"copied":       true,
		"token_entered": token.DisplayName(),
	})
}
